package submission

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/platform"
	"github.com/cpbridge/api/internal/problem"
	"github.com/cpbridge/api/internal/queue"
	"github.com/hibiken/asynq"
)

type Worker struct {
	db           *sql.DB
	probSvc      *problem.Service
	platRegistry *platform.Registry
	asynqClient  *asynq.Client
	timeClock    func() time.Time
}

func needsExternalSubmissionVerification(externalSubmittedAt sql.NullTime) bool {
	return !externalSubmittedAt.Valid
}

func NewWorker(db *sql.DB, probSvc *problem.Service, platRegistry *platform.Registry) *Worker {
	return &Worker{
		db:           db,
		probSvc:      probSvc,
		platRegistry: platRegistry,
		timeClock:    func() time.Time { return time.Now().UTC() },
	}
}

func (w *Worker) SetClock(clock func() time.Time) {
	w.timeClock = clock
}

func (w *Worker) SetAsynqClient(client *asynq.Client) {
	w.asynqClient = client
}

// ProcessPollVerdict handles one asynchronous poll. JUDGING/PENDING are
// expected responses: the task explicitly schedules the next poll and exits
// successfully. Asynq retries are reserved for actual failures.
func (w *Worker) ProcessPollVerdict(ctx context.Context, t *asynq.Task) error {
	var p queue.PollVerdictPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("failed to unmarshal poll verdict payload: %w", err)
	}

	now := w.timeClock()
	var subID, userID, problemID, platStr, language, statusStr, sourceCode string
	var extSubID, requestID, contestID sql.NullString
	var externalSubmittedAt, pollStartedAt sql.NullTime
	var submittedAt time.Time
	var metaBytes []byte
	err := w.db.QueryRowContext(ctx, `
		SELECT id, user_id, problem_id, platform, language, source_code, status,
		       external_submission_id, contest_id, external_submitted_at,
		       poll_started_at, submitted_at, metadata, poll_request_id
		FROM submissions WHERE id = $1
	`, p.SubmissionID).Scan(
		&subID, &userID, &problemID, &platStr, &language, &sourceCode, &statusStr,
		&extSubID, &contestID, &externalSubmittedAt, &pollStartedAt, &submittedAt, &metaBytes, &requestID,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("database query error: %w", err)
	}

	curStatus, err := ParseStatus(statusStr)
	if err != nil {
		return fmt.Errorf("submission %s has invalid database status: %w", p.SubmissionID, err)
	}
	if isTerminalStatus(curStatus) {
		return nil
	}
	if p.RequestID != "" && requestID.Valid && requestID.String != p.RequestID {
		// A newer explicit sync or worker-scheduled poll superseded this task.
		return nil
	}
	if p.RequestID != "" {
		_, _ = w.db.ExecContext(ctx, `
			UPDATE submissions SET poll_requested_at = NOW()
			WHERE id = $1 AND poll_request_id = $2
		`, p.SubmissionID, p.RequestID)
	}

	var metadata map[string]any
	_ = json.Unmarshal(metaBytes, &metadata)
	if metadata == nil {
		metadata = make(map[string]any)
	}

	externalID := p.ExternalSubmissionID
	if extSubID.Valid && extSubID.String != "" {
		externalID = extSubID.String
	}
	if externalID == "" || strings.HasPrefix(externalID, "cf_") || strings.HasPrefix(externalID, "ac_") {
		return w.failOrRetry(ctx, p, judgingDeadlineStart(externalSubmittedAt, pollStartedAt, submittedAt), metadata, "Submission was not created on the external platform (invalid mock ID)", nil)
	}
	deadlineStart := judgingDeadlineStart(externalSubmittedAt, pollStartedAt, submittedAt)
	if w.platRegistry == nil {
		return w.failOrRetry(ctx, p, deadlineStart, metadata, "platform polling is unavailable", fmt.Errorf("platform registry is unavailable"))
	}
	adapter, err := w.platRegistry.Get(platform.Type(platStr))
	if err != nil {
		return w.failOrRetry(ctx, p, deadlineStart, metadata, err.Error(), err)
	}

	prob, _ := w.probSvc.GetByID(ctx, problemID)
	formattedExtID := externalID
	if prob != nil && !strings.Contains(formattedExtID, "/") {
		formattedExtID = externalSubmissionRef(prob.ExternalID, formattedExtID)
	}
	pollRequestedAt := time.Now().UTC()
	log.Printf("[Worker:Poll] Submission %s request started at %s (platform=%s externalID=%s)",
		p.SubmissionID, pollRequestedAt.Format(time.RFC3339Nano), platStr, formattedExtID)
	statusObj, err := adapter.GetSubmission(ctx, formattedExtID)
	pollRespondedAt := time.Now().UTC()
	pollDuration := pollRespondedAt.Sub(pollRequestedAt)
	if err != nil {
		log.Printf("[Worker:Poll] Submission %s response at %s after %s (platform=%s error=%v)",
			p.SubmissionID, pollRespondedAt.Format(time.RFC3339Nano), pollDuration, platStr, err)
		return w.failOrRetry(ctx, p, deadlineStart, metadata, "", fmt.Errorf("failed to fetch submission status from platform: %w", err))
	}
	if statusObj == nil {
		log.Printf("[Worker:Poll] Submission %s response at %s after %s (platform=%s empty response)",
			p.SubmissionID, pollRespondedAt.Format(time.RFC3339Nano), pollDuration, platStr)
		return w.failOrRetry(ctx, p, deadlineStart, metadata, "", errors.New("empty status response from platform"))
	}
	log.Printf("[Worker:Poll] Submission %s response at %s after %s (platform=%s status=%s)",
		p.SubmissionID, pollRespondedAt.Format(time.RFC3339Nano), pollDuration, platStr, statusObj.Status)
	newStatus, err := ParseStatus(statusObj.Status)
	if err != nil {
		return w.failOrRetry(ctx, p, deadlineStart, metadata, "", err)
	}

	if needsExternalSubmissionVerification(externalSubmittedAt) {
		problemExternalID := ""
		if prob != nil {
			problemExternalID = prob.ExternalID
		}
		var contestIDPtr *string
		if contestID.Valid && strings.TrimSpace(contestID.String) != "" {
			contestIDPtr = &contestID.String
		}
		verifiedSub := &Submission{
			ID: subID, UserID: userID, ProblemID: problemID, ProblemExternalID: problemExternalID,
			Platform: platform.Type(platStr), Language: language, SourceCode: sourceCode,
			ContestID: contestIDPtr, SubmittedAt: submittedAt,
		}
		service := &Service{db: w.db}
		if err := service.verifyExternalSubmission(ctx, verifiedSub, formattedExtID, statusObj, now); err != nil {
			var verificationErr *VerificationError
			if errors.As(err, &verificationErr) && verificationErr.Kind == VerificationDefinitive {
				return w.failOrRetry(ctx, p, deadlineStart, metadata, err.Error(), nil)
			}
			return w.failOrRetry(ctx, p, deadlineStart, metadata, "external submission metadata is incomplete or temporarily unavailable", err)
		}
		if _, err := w.db.ExecContext(ctx, `
			UPDATE submissions SET external_submitted_at = $1
			WHERE id = $2 AND status IN ('PENDING', 'DISPATCHING', 'JUDGING')
			  AND external_submitted_at IS NULL
		`, statusObj.SubmittedAt, p.SubmissionID); err != nil {
			return fmt.Errorf("failed to store verified external submission timestamp: %w", err)
		}
		deadlineStart = statusObj.SubmittedAt.UTC()
	}

	if isTerminalStatus(newStatus) {
		if statusObj.ExecutionTimeMs != nil {
			metadata["executionTimeMs"] = *statusObj.ExecutionTimeMs
		}
		if statusObj.MemoryBytes != nil {
			metadata["memoryBytes"] = *statusObj.MemoryBytes
		}
		if statusObj.FailedTestcase != nil {
			metadata["failedTestcase"] = *statusObj.FailedTestcase
		}
		maps.Copy(metadata, statusObj.RawPayload)
		metaJSON, _ := json.Marshal(metadata)
		_, err = w.db.ExecContext(ctx, `
			UPDATE submissions
			SET status = $1, judged_at = $2, metadata = $3,
			    poll_request_id = NULL, poll_requested_at = NULL
			WHERE id = $4 AND status IN ('PENDING', 'DISPATCHING', 'JUDGING')
		`, newStatus, now, metaJSON, p.SubmissionID)
		if err != nil {
			return fmt.Errorf("failed to update submission in database: %w", err)
		}
		log.Printf("[Worker:Success] Submission %s finalized: Verdict=%s", p.SubmissionID, newStatus)
		return nil
	}

	if !now.Before(deadlineStart.Add(judgingTimeout)) {
		return w.failOrRetry(ctx, p, deadlineStart, metadata, "Judging timed out on external platform (>15m)", nil)
	}
	service := &Service{db: w.db, asynqClient: w.asynqClient}
	if err := service.requestPoll(ctx, p.SubmissionID, true, queue.PollInterval); err != nil {
		return err
	}
	log.Printf("[Worker:Judging] Submission %s is still %s; next poll scheduled", p.SubmissionID, newStatus)
	return nil
}

func judgingDeadlineStart(externalSubmittedAt, pollStartedAt sql.NullTime, submittedAt time.Time) time.Time {
	if externalSubmittedAt.Valid {
		return externalSubmittedAt.Time.UTC()
	}
	if pollStartedAt.Valid {
		return pollStartedAt.Time.UTC()
	}
	return submittedAt.UTC()
}

func (w *Worker) failOrRetry(ctx context.Context, p queue.PollVerdictPayload, deadlineStart time.Time, metadata map[string]any, terminalMessage string, retryErr error) error {
	now := w.timeClock()
	if !now.Before(deadlineStart.Add(judgingTimeout)) || retryErr == nil {
		message := terminalMessage
		if message == "" && retryErr != nil {
			message = fmt.Sprintf("Judging polling timed out after repeated platform errors: %v", retryErr)
		}
		if message == "" {
			message = "Judging polling failed without a platform verdict"
		}
		metadata["error"] = message
		metaJSON, _ := json.Marshal(metadata)
		_, err := w.db.ExecContext(ctx, `
			UPDATE submissions
			SET status = $1, judged_at = $2, metadata = $3,
			    poll_request_id = NULL, poll_requested_at = NULL
			WHERE id = $4 AND status IN ('PENDING', 'DISPATCHING', 'JUDGING')
		`, Failed, now, metaJSON, p.SubmissionID)
		if err != nil {
			return fmt.Errorf("failed to mark submission failed: %w", err)
		}
		return nil
	}
	if p.RequestID != "" {
		_, _ = w.db.ExecContext(ctx, `
			UPDATE submissions SET poll_requested_at = NOW()
			WHERE id = $1 AND poll_request_id = $2
		`, p.SubmissionID, p.RequestID)
	}
	return retryErr
}

func externalSubmissionRef(problemExternalID, submissionID string) string {
	parts := strings.Split(strings.TrimSpace(problemExternalID), "/")
	if len(parts) >= 3 && strings.EqualFold(parts[0], "gym") {
		return fmt.Sprintf("gym/%s/%s", parts[1], submissionID)
	}
	if len(parts) >= 1 && parts[0] != "" {
		return fmt.Sprintf("%s/%s", parts[0], submissionID)
	}
	return submissionID
}
