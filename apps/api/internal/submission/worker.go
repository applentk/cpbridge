package submission

import (
	"context"
	"database/sql"
	"encoding/json"
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

// ProcessPollVerdict handles the asynq task submission:poll_verdict.
func (w *Worker) ProcessPollVerdict(ctx context.Context, t *asynq.Task) error {
	var p queue.PollVerdictPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		log.Printf("[Worker:Error] Failed to unmarshal payload: %v", err)
		return fmt.Errorf("failed to unmarshal poll verdict payload: %w", err)
	}

	retried, _ := asynq.GetRetryCount(ctx)
	maxRetry, _ := asynq.GetMaxRetry(ctx)

	log.Printf("[Worker:Poll] Checking submission %s on %s (extID: %s, attempt %d/%d)",
		p.SubmissionID, p.Platform, p.ExternalSubmissionID, retried+1, maxRetry)

	now := w.timeClock()

	// 1. Fetch current submission from DB
	query := `
		SELECT id, user_id, problem_id, platform, language, status, external_submission_id, contest_id, external_submitted_at, submitted_at, metadata
		FROM submissions
		WHERE id = $1
	`
	var subID, userID, problemID, platStr, language, statusStr string
	var extSubID sql.NullString
	var contestID sql.NullString
	var externalSubmittedAt sql.NullTime
	var submittedAt time.Time
	var metaBytes []byte

	err := w.db.QueryRowContext(ctx, query, p.SubmissionID).Scan(
		&subID, &userID, &problemID, &platStr, &language, &statusStr, &extSubID, &contestID, &externalSubmittedAt, &submittedAt, &metaBytes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Printf("[Worker:Notice] Submission %s not found in DB, skipping task", p.SubmissionID)
			return nil // submission deleted or not found
		}
		return fmt.Errorf("database query error: %w", err)
	}

	curStatus := Status(statusStr)
	// If already resolved to a terminal verdict, we are done
	if curStatus != Pending && curStatus != Dispatching && curStatus != Judging {
		log.Printf("[Worker:Done] Submission %s is already finalized with status %s", p.SubmissionID, curStatus)
		return nil
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

	// 2. Reject mock IDs immediately
	if externalID == "" || strings.HasPrefix(externalID, "cf_") || strings.HasPrefix(externalID, "ac_") {
		log.Printf("[Worker:Fail] Submission %s has invalid/mock external ID %q", p.SubmissionID, externalID)
		metadata["error"] = "Submission was not created on the external platform (invalid mock ID)"
		metaJSON, _ := json.Marshal(metadata)
		_, _ = w.db.ExecContext(ctx, `
			UPDATE submissions
			SET status = $1, judged_at = $2, metadata = $3
			WHERE id = $4
		`, Failed, now, metaJSON, p.SubmissionID)
		return nil
	}

	// 3. Obtain platform adapter
	platType := platform.Type(platStr)
	adapter, err := w.platRegistry.Get(platType)
	if err != nil {
		log.Printf("[Worker:Error] Platform adapter not found for %s", platStr)
		return fmt.Errorf("unsupported platform %s: %w", platStr, err)
	}

	// Format external submission ID if needed
	prob, _ := w.probSvc.GetByID(ctx, problemID)
	formattedExtID := externalID
	if prob != nil && !strings.Contains(formattedExtID, "/") {
		parts := strings.Split(prob.ExternalID, "/")
		if len(parts) >= 1 {
			formattedExtID = fmt.Sprintf("%s/%s", parts[0], formattedExtID)
		}
	}

	// 4. Poll external platform
	statusObj, err := adapter.GetSubmission(ctx, formattedExtID)
	if err != nil {
		log.Printf("[Worker:Retry] Network/platform error polling %s (extID: %s): %v", p.Platform, formattedExtID, err)
		return fmt.Errorf("failed to fetch submission status from platform: %w", err)
	}

	if statusObj == nil {
		return fmt.Errorf("empty status response from platform")
	}

	// UpdateDispatched performs the one-time verification before enqueueing this
	// task. Once its timestamp is stored, later polls must tolerate status-only
	// adapter fallbacks instead of re-running metadata verification.
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
			ID:                subID,
			UserID:            userID,
			ProblemID:         problemID,
			ProblemExternalID: problemExternalID,
			Platform:          platform.Type(platStr),
			Language:          language,
			ContestID:         contestIDPtr,
			SubmittedAt:       submittedAt,
		}
		service := &Service{db: w.db}
		if err := service.verifyExternalSubmission(ctx, verifiedSub, formattedExtID, statusObj, now); err != nil {
			metadata["error"] = err.Error()
			metaJSON, _ := json.Marshal(metadata)
			_, _ = w.db.ExecContext(ctx, `
				UPDATE submissions
				SET status = $1, judged_at = $2, metadata = $3
				WHERE id = $4 AND status IN ('PENDING', 'DISPATCHING', 'JUDGING')
			`, Failed, now, metaJSON, p.SubmissionID)
			return nil
		}
		if _, err := w.db.ExecContext(ctx, `
			UPDATE submissions
			SET external_submitted_at = $1
			WHERE id = $2 AND external_submitted_at IS NULL
		`, statusObj.SubmittedAt, p.SubmissionID); err != nil {
			return fmt.Errorf("failed to store verified external submission timestamp: %w", err)
		}
	}

	// 5. Handle terminal statuses
	if statusObj.Status != "JUDGING" && statusObj.Status != "PENDING" && statusObj.Status != "" {
		newStatus := Status(statusObj.Status)
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
		_, updateErr := w.db.ExecContext(ctx, `
			UPDATE submissions
			SET status = $1, judged_at = $2, metadata = $3
			WHERE id = $4
		`, newStatus, now, metaJSON, p.SubmissionID)
		if updateErr != nil {
			return fmt.Errorf("failed to update submission in database: %w", updateErr)
		}
		log.Printf("[Worker:Success] Submission %s finalized: Verdict=%s", p.SubmissionID, newStatus)
		return nil // Finished successfully
	}

	// 6. Still judging - check timeout
	if now.Sub(submittedAt) > 15*time.Minute {
		log.Printf("[Worker:Timeout] Submission %s timed out (>15m in judging), marking as FAILED", p.SubmissionID)
		metadata["error"] = "Judging timed out on external platform (>15m)"
		metaJSON, _ := json.Marshal(metadata)
		_, _ = w.db.ExecContext(ctx, `
			UPDATE submissions
			SET status = $1, judged_at = $2, metadata = $3
			WHERE id = $4
		`, Failed, now, metaJSON, p.SubmissionID)
		return nil
	}

	// Return error to trigger Asynq retry
	log.Printf("[Worker:Judging] Submission %s is still %s on %s, scheduling retry...", p.SubmissionID, statusObj.Status, p.Platform)
	return fmt.Errorf("submission %s is still %s (retry scheduled)", p.SubmissionID, statusObj.Status)
}
