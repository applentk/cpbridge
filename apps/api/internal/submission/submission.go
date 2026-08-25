package submission

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cpbridge/api/internal/auth"
	"github.com/cpbridge/api/internal/contest"
	"github.com/cpbridge/api/internal/idgen"
	"github.com/cpbridge/api/internal/platform"
	"github.com/cpbridge/api/internal/problem"
	"github.com/cpbridge/api/internal/queue"
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
)

type Status string

// ErrDuplicateSource is returned before a submission is created when this
// user already submitted the exact same source in the same language for the
// problem. The web client therefore never dispatches it to an external judge.
var ErrDuplicateSource = errors.New("an identical solution was already submitted for this problem")

var supportedLanguages = map[string]struct{}{
	"cpp23":   {},
	"python3": {},
	"java21":  {},
}

func isSupportedLanguage(language string) bool {
	_, supported := supportedLanguages[language]
	return supported
}

const (
	Pending      Status = "PENDING"
	Dispatching  Status = "DISPATCHING"
	Judging      Status = "JUDGING"
	Accepted     Status = "ACCEPTED"
	WrongAnswer  Status = "WRONG_ANSWER"
	TimeLimit    Status = "TIME_LIMIT"
	MemoryLimit  Status = "MEMORY_LIMIT"
	RuntimeError Status = "RUNTIME_ERROR"
	CompileError Status = "COMPILE_ERROR"
	Failed       Status = "FAILED"
)

type Submission struct {
	ID                   string         `json:"id"`
	UserID               string         `json:"userId"`
	Username             string         `json:"username,omitempty"`
	ProblemID            string         `json:"problemId"`
	ProblemExternalID    string         `json:"-"`
	ProblemTitle         string         `json:"problemTitle,omitempty"`
	ContestID            *string        `json:"contestId,omitempty"`
	Platform             platform.Type  `json:"platform"`
	Language             string         `json:"language"`
	SourceCode           string         `json:"sourceCode"`
	Status               Status         `json:"status"`
	ExternalSubmissionID *string        `json:"externalSubmissionId,omitempty"`
	SourceURL            *string        `json:"sourceUrl,omitempty"`
	SubmittedAt          time.Time      `json:"submittedAt"`
	JudgedAt             *time.Time     `json:"judgedAt,omitempty"`
	Metadata             map[string]any `json:"metadata"`
}

func attachAdminSourceURL(sub *Submission, isAdmin bool) {
	if !isAdmin || sub == nil || sub.ExternalSubmissionID == nil {
		return
	}

	externalSubmissionID := strings.TrimSpace(*sub.ExternalSubmissionID)
	problemParts := strings.Split(strings.TrimSpace(sub.ProblemExternalID), "/")
	if externalSubmissionID == "" || len(problemParts) < 1 || problemParts[0] == "" {
		return
	}
	if submissionParts := strings.Split(externalSubmissionID, "/"); len(submissionParts) > 1 {
		externalSubmissionID = submissionParts[len(submissionParts)-1]
	}

	contestID := url.PathEscape(problemParts[0])
	submissionID := url.PathEscape(externalSubmissionID)
	var sourceURL string
	switch sub.Platform {
	case platform.Codeforces:
		sourceURL = fmt.Sprintf("https://codeforces.com/contest/%s/submission/%s", contestID, submissionID)
	case platform.AtCoder:
		sourceURL = fmt.Sprintf("https://atcoder.jp/contests/%s/submissions/%s", contestID, submissionID)
	default:
		return
	}
	sub.SourceURL = &sourceURL
}

type ProblemScore struct {
	ProblemID            string `json:"problemId"`
	Label                string `json:"label"`
	Solved               bool   `json:"solved"`
	Attempts             int    `json:"attempts"`
	PenaltyMinutes       int    `json:"penaltyMinutes"`
	FirstSolvedAtMinutes *int   `json:"firstSolvedAtMinutes,omitempty"`
}

type ParticipantScore struct {
	UserID        string                  `json:"userId"`
	Username      string                  `json:"username"`
	Rank          int                     `json:"rank"`
	SolvedCount   int                     `json:"solvedCount"`
	TotalPenalty  int                     `json:"totalPenalty"`
	ProblemScores map[string]ProblemScore `json:"problemScores"`
}

type StandingsResponse struct {
	ContestID   string             `json:"contestId"`
	ScoringType string             `json:"scoringType"`
	Standings   []ParticipantScore `json:"standings"`
	Problems    []ProblemHeader    `json:"problems"`
	GeneratedAt time.Time          `json:"generatedAt"`
}

type ProblemHeader struct {
	ProblemID string `json:"problemId"`
	Label     string `json:"label"`
	Title     string `json:"title"`
	Platform  string `json:"platform"`
}

type Service struct {
	db           *sql.DB
	contestSvc   *contest.Service
	probSvc      *problem.Service
	platRegistry *platform.Registry
	asynqClient  *asynq.Client
	timeClock    func() time.Time
}

func NewService(db *sql.DB, contestSvc *contest.Service, probSvc *problem.Service, platRegistry *platform.Registry) *Service {
	return &Service{
		db:           db,
		contestSvc:   contestSvc,
		probSvc:      probSvc,
		platRegistry: platRegistry,
		timeClock:    func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetClock(clock func() time.Time) {
	s.timeClock = clock
}

func (s *Service) SetPlatformRegistry(reg *platform.Registry) {
	s.platRegistry = reg
}

func (s *Service) SetAsynqClient(client *asynq.Client) {
	s.asynqClient = client
}

func canonicalExternalID(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) < 2 {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(parts[0]) + "/" + strings.ToUpper(strings.TrimSpace(parts[1]))
}

func platformLanguageMatches(platformType platform.Type, expected, actual string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	actual = strings.ToLower(strings.TrimSpace(actual))
	if expected == "" || actual == "" {
		return false
	}
	switch expected {
	case "cpp23":
		return strings.Contains(actual, "c++") || strings.Contains(actual, "gnu c")
	case "python3":
		return strings.Contains(actual, "python") || strings.Contains(actual, "pypy")
	case "java21":
		return strings.Contains(actual, "java")
	default:
		return false
	}
}

func validateExternalSubmissionMetadata(sub *Submission, externalID string, statusObj *platform.SubmissionStatus, now time.Time) error {
	if sub == nil || statusObj == nil {
		return errors.New("external submission could not be verified")
	}
	if statusObj.ExternalSubmissionID != "" && canonicalExternalID(statusObj.ExternalSubmissionID) != canonicalExternalID(externalID) {
		return errors.New("external submission ID could not be verified")
	}
	if canonicalExternalID(statusObj.ProblemExternalID) != canonicalExternalID(sub.ProblemExternalID) {
		return errors.New("external submission targets a different problem")
	}
	if !platformLanguageMatches(sub.Platform, sub.Language, statusObj.Language) {
		return errors.New("external submission uses a different language")
	}
	if statusObj.SubmittedAt == nil || statusObj.SubmittedAt.Before(sub.SubmittedAt.Add(-2*time.Minute)) || statusObj.SubmittedAt.After(now.Add(2*time.Minute)) {
		return errors.New("external submission timestamp is outside the dispatch window")
	}
	if strings.TrimSpace(statusObj.PlatformUsername) == "" {
		return errors.New("external submission platform identity is missing")
	}
	return nil
}

// verifyExternalSubmission checks data obtained from the official platform,
// rather than data supplied by the participant. An existing linked identity
// must match; otherwise the first fully verified submission establishes it.
func (s *Service) verifyExternalSubmission(ctx context.Context, sub *Submission, externalID string, statusObj *platform.SubmissionStatus, now time.Time) error {
	if err := validateExternalSubmissionMetadata(sub, externalID, statusObj, now); err != nil {
		return err
	}

	var expectedUsername, connectionStatus string
	err := s.db.QueryRowContext(ctx, `
		SELECT external_username, connection_status
		FROM integrations
		WHERE user_id = $1 AND platform = $2
	`, sub.UserID, sub.Platform).Scan(&expectedUsername, &connectionStatus)
	if errors.Is(err, sql.ErrNoRows) {
		// Trust on first verified submission: the identity comes from the
		// official platform response after the submission ID, problem, language,
		// and timestamp have all been validated. This keeps the browser bridge
		// zero-cookie while avoiding a separate manual identity-linking step.
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO integrations (user_id, platform, external_username, connection_status, updated_at)
			VALUES ($1, $2, $3, 'CONNECTED', $4)
			ON CONFLICT (user_id, platform) DO NOTHING
		`, sub.UserID, sub.Platform, statusObj.PlatformUsername, now)
		if err != nil {
			return fmt.Errorf("failed to link verified platform identity: %w", err)
		}
		err = s.db.QueryRowContext(ctx, `
			SELECT external_username, connection_status
			FROM integrations
			WHERE user_id = $1 AND platform = $2
		`, sub.UserID, sub.Platform).Scan(&expectedUsername, &connectionStatus)
	}
	if err != nil {
		return fmt.Errorf("failed to verify platform identity: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(expectedUsername), strings.TrimSpace(statusObj.PlatformUsername)) {
		return errors.New("external submission belongs to a different platform identity")
	}
	if connectionStatus != "CONNECTED" {
		_, err = s.db.ExecContext(ctx, `
			UPDATE integrations
			SET connection_status = 'CONNECTED', updated_at = $1
			WHERE user_id = $2 AND platform = $3
		`, now, sub.UserID, sub.Platform)
		if err != nil {
			return fmt.Errorf("failed to reconnect verified platform identity: %w", err)
		}
	}
	return nil
}

func (s *Service) Create(ctx context.Context, userID string, isAdmin bool, problemID string, contestID *string, language, sourceCode string) (*Submission, error) {
	language = strings.TrimSpace(language)
	if !isSupportedLanguage(language) {
		return nil, fmt.Errorf("unsupported submission language: %s", language)
	}

	prob, err := s.probSvc.GetByID(ctx, problemID)
	if err != nil {
		return nil, fmt.Errorf("problem not found: %w", err)
	}

	now := s.timeClock()

	// Validate contest window if contestId is present
	if contestID != nil && *contestID != "" {
		c, err := s.contestSvc.GetByID(ctx, *contestID, userID, isAdmin)
		if err != nil {
			return nil, fmt.Errorf("contest not accessible: %w", err)
		}

		if now.Before(c.StartAt) {
			return nil, errors.New("contest has not started yet")
		}

		// Verify problem belongs to this contest
		var inContest int
		err = s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM contest_problems WHERE contest_id = $1 AND problem_id = $2", *contestID, problemID).Scan(&inContest)
		if err != nil || inContest == 0 {
			return nil, errors.New("problem does not belong to this contest")
		}

		// Ensure user joined contest if contest is currently active
		if now.Before(c.EndAt) {
			if err := s.contestSvc.Join(ctx, *contestID, userID); err != nil {
				if !c.IsParticipant && c.OwnerID != userID && !isAdmin {
					return nil, fmt.Errorf("failed to join contest: %w", err)
				}
			}
		}
	} else if !isAdmin {
		var startedContestCount int
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM contest_problems cp
			JOIN contests c ON cp.contest_id = c.id
			WHERE cp.problem_id = $1
			  AND c.publication_status = 'PUBLISHED'
			  AND c.start_at <= $2
			  AND (c.visibility = 'PUBLIC' OR c.owner_id = $3 OR EXISTS(SELECT 1 FROM contest_participants part WHERE part.contest_id = c.id AND part.user_id = $3))
		`, problemID, now, userID).Scan(&startedContestCount)
		if err != nil || startedContestCount == 0 {
			return nil, errors.New("submissions are only allowed for problems in started contests")
		}
	}

	// Check legacy records too: source_hash was introduced after submissions
	// already existed, so its database uniqueness guarantee only covers newer
	// rows. Exact source and language are intentional—formatting or changing
	// language remains a distinct submission.
	var existingID string
	err = s.db.QueryRowContext(ctx, `
		SELECT id
		FROM submissions
		WHERE user_id = $1 AND problem_id = $2 AND language = $3 AND source_code = $4
		LIMIT 1
	`, userID, problemID, language, sourceCode).Scan(&existingID)
	if err == nil {
		return nil, ErrDuplicateSource
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check for duplicate submission: %w", err)
	}

	sourceDigest := sha256.Sum256([]byte(sourceCode))
	sourceHash := hex.EncodeToString(sourceDigest[:])

	sub := &Submission{
		ID:          idgen.New(idgen.PrefixSubmission),
		UserID:      userID,
		ProblemID:   problemID,
		ContestID:   contestID,
		Platform:    prob.Platform,
		Language:    language,
		SourceCode:  sourceCode,
		Status:      Pending,
		SubmittedAt: now,
		Metadata:    map[string]any{},
	}

	query := `
		INSERT INTO submissions (id, user_id, problem_id, contest_id, platform, language, source_code, source_hash, status, submitted_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (user_id, problem_id, language, source_hash) WHERE source_hash IS NOT NULL DO NOTHING
		RETURNING id
	`
	err = s.db.QueryRowContext(ctx, query, sub.ID, sub.UserID, sub.ProblemID, sub.ContestID, sub.Platform, sub.Language, sub.SourceCode, sourceHash, sub.Status, sub.SubmittedAt, "{}").Scan(&sub.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDuplicateSource
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create submission: %w", err)
	}

	log.Printf("[Submission:Created] %s created by user %s for problem %s (%s, lang: %s)",
		sub.ID, sub.UserID, sub.ProblemID, sub.Platform, sub.Language)

	return sub, nil
}

func (s *Service) syncStatusDirect(ctx context.Context, sub *Submission) (*Submission, error) {
	if sub == nil {
		return sub, nil
	}

	now := s.timeClock()

	// 1. If submission has a mock external ID (from legacy bugs), mark as FAILED immediately
	if sub.ExternalSubmissionID != nil && (strings.HasPrefix(*sub.ExternalSubmissionID, "cf_") || strings.HasPrefix(*sub.ExternalSubmissionID, "ac_")) {
		metadata := sub.Metadata
		if metadata == nil {
			metadata = make(map[string]any)
		}
		metadata["error"] = "Submission was not created on the external platform (invalid mock ID)"
		metaJSON, _ := json.Marshal(metadata)
		_, updateErr := s.db.ExecContext(ctx, `
			UPDATE submissions
			SET status = $1, judged_at = $2, metadata = $3
			WHERE id = $4
		`, Failed, now, metaJSON, sub.ID)
		if updateErr == nil {
			sub.Status = Failed
			sub.JudgedAt = &now
			sub.Metadata = metadata
		}
		return sub, nil
	}

	// 2. If the browser bridge has not confirmed the handoff yet, keep the
	// submission recoverable. The extension persists its result and the web
	// client can safely retry the handoff by submission ID; turning this into a
	// terminal failure would make a successful external submission look lost.
	if (sub.Status == Pending || sub.Status == Dispatching) && (sub.ExternalSubmissionID == nil || *sub.ExternalSubmissionID == "") {
		if now.Sub(sub.SubmittedAt) > 2*time.Minute {
			metadata := sub.Metadata
			if metadata == nil {
				metadata = make(map[string]any)
			}
			metadata["dispatchRetryable"] = true
			metadata["dispatchMessage"] = "Dispatch confirmation is delayed; retrying the browser handoff is safe"
			metaJSON, _ := json.Marshal(metadata)
			_, updateErr := s.db.ExecContext(ctx, `
				UPDATE submissions
				SET metadata = $1
				WHERE id = $2
			`, metaJSON, sub.ID)
			if updateErr == nil {
				sub.Metadata = metadata
			}
		}
		return sub, nil
	}

	if sub.ExternalSubmissionID == nil || *sub.ExternalSubmissionID == "" || s.platRegistry == nil {
		return sub, nil
	}

	adapter, err := s.platRegistry.Get(sub.Platform)
	if err != nil {
		return sub, nil
	}

	prob, err := s.probSvc.GetByID(ctx, sub.ProblemID)
	extSubID := *sub.ExternalSubmissionID
	if err == nil && prob != nil && !strings.Contains(extSubID, "/") {
		parts := strings.Split(prob.ExternalID, "/")
		if len(parts) >= 1 {
			extSubID = fmt.Sprintf("%s/%s", parts[0], extSubID)
		}
	}

	statusObj, err := adapter.GetSubmission(ctx, extSubID)
	if err != nil || statusObj == nil {
		return sub, nil
	}

	if statusObj.Status != "JUDGING" && statusObj.Status != "PENDING" && statusObj.Status != "" {
		newStatus := Status(statusObj.Status)
		metadata := sub.Metadata
		if metadata == nil {
			metadata = make(map[string]any)
		}
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
		_, updateErr := s.db.ExecContext(ctx, `
			UPDATE submissions
			SET status = $1, judged_at = $2, metadata = $3
			WHERE id = $4
		`, newStatus, now, metaJSON, sub.ID)
		if updateErr == nil {
			sub.Status = newStatus
			sub.JudgedAt = &now
			sub.Metadata = metadata
		}
	} else if (sub.Status == Judging || statusObj.Status == "JUDGING") && now.Sub(sub.SubmittedAt) > 15*time.Minute {
		// Stale judging submission timeout (>15 min)
		metadata := sub.Metadata
		if metadata == nil {
			metadata = make(map[string]any)
		}
		metadata["error"] = "Judging timed out on external platform"
		metaJSON, _ := json.Marshal(metadata)
		_, updateErr := s.db.ExecContext(ctx, `
			UPDATE submissions
			SET status = $1, judged_at = $2, metadata = $3
			WHERE id = $4
		`, Failed, now, metaJSON, sub.ID)
		if updateErr == nil {
			sub.Status = Failed
			sub.JudgedAt = &now
			sub.Metadata = metadata
		}
	}

	return sub, nil
}

func (s *Service) SyncStatus(ctx context.Context, id, requestingUserID string, isAdmin bool) (*Submission, error) {
	query := `
		SELECT s.id, s.user_id, u.username, s.problem_id, p.external_id, p.title, s.contest_id, s.platform, s.language,
		       s.source_code, s.status, s.external_submission_id, s.submitted_at, s.judged_at, s.metadata
		FROM submissions s
		JOIN users u ON s.user_id = u.id
		JOIN problems p ON s.problem_id = p.id
		WHERE s.id = $1
	`
	var sub Submission
	var metaJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&sub.ID, &sub.UserID, &sub.Username, &sub.ProblemID, &sub.ProblemExternalID, &sub.ProblemTitle, &sub.ContestID,
		&sub.Platform, &sub.Language, &sub.SourceCode, &sub.Status, &sub.ExternalSubmissionID,
		&sub.SubmittedAt, &sub.JudgedAt, &metaJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("submission not found")
		}
		return nil, err
	}

	if !isAdmin && sub.UserID != requestingUserID {
		return nil, errors.New("unauthorized to sync submission")
	}

	_ = json.Unmarshal(metaJSON, &sub.Metadata)

	if sub.Status == Judging || sub.Status == Pending || sub.Status == Dispatching {
		if synced, syncErr := s.syncStatusDirect(ctx, &sub); syncErr == nil && synced != nil {
			sub = *synced
		} else if syncErr != nil {
			return nil, syncErr
		}
	}

	attachAdminSourceURL(&sub, isAdmin)
	return &sub, nil
}

func (s *Service) GetByID(ctx context.Context, id, requestingUserID string, isAdmin bool) (*Submission, error) {
	query := `
		SELECT s.id, s.user_id, u.username, s.problem_id, p.external_id, p.title, s.contest_id, s.platform, s.language,
		       s.source_code, s.status, s.external_submission_id, s.submitted_at, s.judged_at, s.metadata
		FROM submissions s
		JOIN users u ON s.user_id = u.id
		JOIN problems p ON s.problem_id = p.id
		WHERE s.id = $1
	`
	var sub Submission
	var metaJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&sub.ID, &sub.UserID, &sub.Username, &sub.ProblemID, &sub.ProblemExternalID, &sub.ProblemTitle, &sub.ContestID,
		&sub.Platform, &sub.Language, &sub.SourceCode, &sub.Status, &sub.ExternalSubmissionID,
		&sub.SubmittedAt, &sub.JudgedAt, &metaJSON,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("submission not found")
		}
		return nil, err
	}

	if !isAdmin && sub.UserID != requestingUserID {
		return nil, errors.New("unauthorized to view this submission")
	}

	_ = json.Unmarshal(metaJSON, &sub.Metadata)

	if sub.Status == Judging || sub.Status == Pending || sub.Status == Dispatching {
		if synced, err := s.syncStatusDirect(ctx, &sub); err == nil && synced != nil {
			sub = *synced
		}
	}

	attachAdminSourceURL(&sub, isAdmin)
	return &sub, nil
}

func (s *Service) List(ctx context.Context, userID, contestID, problemID string, limit, offset int) ([]Submission, error) {
	return s.list(ctx, userID, contestID, problemID, limit, offset, false)
}

func (s *Service) ListForViewer(ctx context.Context, userID, contestID, problemID string, limit, offset int, isAdmin bool) ([]Submission, error) {
	return s.list(ctx, userID, contestID, problemID, limit, offset, isAdmin)
}

func (s *Service) list(ctx context.Context, userID, contestID, problemID string, limit, offset int, isAdmin bool) ([]Submission, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []any
	idx := 1

	if userID != "" {
		conditions = append(conditions, fmt.Sprintf("s.user_id = $%d", idx))
		args = append(args, userID)
		idx++
	}
	if contestID != "" {
		conditions = append(conditions, fmt.Sprintf("s.contest_id = $%d", idx))
		args = append(args, contestID)
		idx++
	}
	if problemID != "" {
		conditions = append(conditions, fmt.Sprintf("s.problem_id = $%d", idx))
		args = append(args, problemID)
		idx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT s.id, s.user_id, u.username, s.problem_id, p.external_id, p.title, s.contest_id, s.platform, s.language,
		       s.source_code, s.status, s.external_submission_id, s.submitted_at, s.judged_at, s.metadata
		FROM submissions s
		JOIN users u ON s.user_id = u.id
		JOIN problems p ON s.problem_id = p.id
		%s
		ORDER BY s.submitted_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, idx, idx+1)

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []Submission
	for rows.Next() {
		var sub Submission
		var metaJSON []byte
		err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.Username, &sub.ProblemID, &sub.ProblemExternalID, &sub.ProblemTitle, &sub.ContestID,
			&sub.Platform, &sub.Language, &sub.SourceCode, &sub.Status, &sub.ExternalSubmissionID,
			&sub.SubmittedAt, &sub.JudgedAt, &metaJSON,
		)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaJSON, &sub.Metadata)
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range subs {
		if subs[i].Status == Judging || subs[i].Status == Pending || subs[i].Status == Dispatching {
			if synced, err := s.syncStatusDirect(ctx, &subs[i]); err == nil && synced != nil {
				subs[i] = *synced
			}
		}
		attachAdminSourceURL(&subs[i], isAdmin)
	}

	if subs == nil {
		subs = []Submission{}
	}

	return subs, nil
}

func (s *Service) UpdateDispatched(ctx context.Context, id, userID string, isAdmin bool, externalSubmissionID string) error {
	sub, err := s.GetByID(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if sub.UserID != userID && !isAdmin {
		return errors.New("unauthorized to update submission")
	}

	if strings.HasPrefix(externalSubmissionID, "cf_") || strings.HasPrefix(externalSubmissionID, "ac_") {
		return errors.New("invalid external submission ID")
	}
	if s.platRegistry == nil {
		return errors.New("platform verification is unavailable")
	}
	adapter, err := s.platRegistry.Get(sub.Platform)
	if err != nil {
		return err
	}
	lookupID := externalSubmissionID
	if !strings.Contains(lookupID, "/") && strings.Contains(sub.ProblemExternalID, "/") {
		lookupID = strings.Split(sub.ProblemExternalID, "/")[0] + "/" + lookupID
	}
	verified, err := adapter.GetSubmission(ctx, lookupID)
	if err != nil {
		return fmt.Errorf("failed to verify external submission: %w", err)
	}
	if err := s.verifyExternalSubmission(ctx, sub, lookupID, verified, s.timeClock()); err != nil {
		return err
	}

	query := `
		UPDATE submissions
		SET status = $1, external_submission_id = $2
		WHERE id = $3 AND status IN ($4, $5)
	`
	result, err := s.db.ExecContext(ctx, query, Judging, externalSubmissionID, id, Pending, Dispatching)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		// A recovery retry can arrive after the worker has already finalized the
		// submission. Keep the terminal verdict and avoid enqueueing a duplicate.
		return nil
	}

	log.Printf("[Submission:Dispatched] %s marked as JUDGING with external ID %s", sub.ID, externalSubmissionID)

	// Enqueue Asynq worker task for asynchronous status polling
	if s.asynqClient != nil {
		task, taskErr := queue.NewPollVerdictTask(sub.ID, externalSubmissionID, string(sub.Platform), sub.ProblemID)
		if taskErr == nil {
			info, enqErr := s.asynqClient.EnqueueContext(ctx, task)
			if enqErr == nil && info != nil {
				log.Printf("[Queue:Enqueue] Successfully queued poll task (TaskID: %s, Queue: %s) for submission %s",
					info.ID, info.Queue, sub.ID)
			} else if enqErr != nil {
				log.Printf("[Queue:Error] Failed to enqueue task for submission %s: %v", sub.ID, enqErr)
			}
		}
	}

	return nil
}

func (s *Service) UpdateResult(ctx context.Context, id, userID string, isAdmin bool, status Status, metadata map[string]any) error {
	sub, err := s.GetByID(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if sub.UserID != userID && !isAdmin {
		return errors.New("unauthorized to update submission")
	}

	// Regular users are only allowed to mark client-dispatch failures (FAILED)
	if !isAdmin && status != Failed {
		return errors.New("only admins or platform workers can finalize verdicts")
	}
	if status == Failed {
		if metadata == nil {
			metadata = make(map[string]any)
		}
		if message, ok := metadata["error"].(string); !ok || strings.TrimSpace(message) == "" || strings.EqualFold(strings.TrimSpace(message), "Codeforces:") {
			metadata["error"] = "Codeforces submission failed before a submission ID was returned. Check the Codeforces submissions page."
		}
	}

	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	now := s.timeClock()
	query := `
		UPDATE submissions
		SET status = $1, judged_at = $2, metadata = $3
		WHERE id = $4
	`
	_, err = s.db.ExecContext(ctx, query, status, now, metaJSON, id)
	if err == nil {
		log.Printf("[Submission:ResultUpdated] %s updated to status %s by user %s (isAdmin: %v)", id, status, userID, isAdmin)
	}
	return err
}

func (s *Service) CalculateStandings(ctx context.Context, contestID string, requestingUserID string, isAdmin bool) (*StandingsResponse, error) {
	c, err := s.contestSvc.GetByID(ctx, contestID, requestingUserID, isAdmin)
	if err != nil {
		return nil, err
	}

	// Fetch contest problems
	problems, err := s.contestSvc.GetProblems(ctx, contestID, requestingUserID, isAdmin)
	if err != nil {
		return nil, err
	}

	headers := make([]ProblemHeader, len(problems))
	problemLabels := make(map[string]string)
	for i, p := range problems {
		headers[i] = ProblemHeader{
			ProblemID: p.ProblemID,
			Label:     p.Label,
			Title:     p.Problem.Title,
			Platform:  string(p.Problem.Platform),
		}
		problemLabels[p.ProblemID] = p.Label
	}

	// Fetch all participants
	partRows, err := s.db.QueryContext(ctx, `
		SELECT cp.user_id, u.username
		FROM contest_participants cp
		JOIN users u ON cp.user_id = u.id
		WHERE cp.contest_id = $1
	`, contestID)
	if err != nil {
		return nil, err
	}
	defer partRows.Close()

	scoresByParticipant := make(map[string]*ParticipantScore)
	for partRows.Next() {
		var uid, uname string
		if err := partRows.Scan(&uid, &uname); err != nil {
			return nil, err
		}
		scoresByParticipant[uid] = &ParticipantScore{
			UserID:        uid,
			Username:      uname,
			SolvedCount:   0,
			TotalPenalty:  0,
			ProblemScores: make(map[string]ProblemScore),
		}
		for _, p := range problems {
			scoresByParticipant[uid].ProblemScores[p.ProblemID] = ProblemScore{
				ProblemID:      p.ProblemID,
				Label:          p.Label,
				Solved:         false,
				Attempts:       0,
				PenaltyMinutes: 0,
			}
		}
	}
	if err := partRows.Err(); err != nil {
		return nil, err
	}

	// Fetch submissions during contest window only: contest.start_at <= submitted_at < contest.end_at
	subQuery := `
		SELECT s.id, s.user_id, s.problem_id, s.status, s.submitted_at
		FROM submissions s
		WHERE s.contest_id = $1 AND s.submitted_at >= $2 AND s.submitted_at < $3
		ORDER BY s.submitted_at ASC
	`
	rows, err := s.db.QueryContext(ctx, subQuery, contestID, c.StartAt, c.EndAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sid, uid, pid string
		var status Status
		var subTime time.Time
		if err := rows.Scan(&sid, &uid, &pid, &status, &subTime); err != nil {
			return nil, err
		}

		pScore, exists := scoresByParticipant[uid]
		if !exists {
			continue
		}
		pProbScore, pProbExists := pScore.ProblemScores[pid]
		if !pProbExists {
			continue
		}

		// If already solved, ignore further submissions for ICPC penalty
		if pProbScore.Solved {
			continue
		}

		pProbScore.Attempts++

		if status == Accepted {
			pProbScore.Solved = true
			elapsedMinutes := max(0, int(math.Floor(subTime.Sub(c.StartAt).Minutes())))
			pProbScore.FirstSolvedAtMinutes = &elapsedMinutes

			if c.ScoringType == contest.ICPC {
				// penalty = minutes from contest start + 20 * rejected before AC
				rejectedBeforeAC := pProbScore.Attempts - 1
				pProbScore.PenaltyMinutes = elapsedMinutes + (20 * rejectedBeforeAC)
			} else {
				pProbScore.PenaltyMinutes = elapsedMinutes
			}

			pScore.SolvedCount++
			pScore.TotalPenalty += pProbScore.PenaltyMinutes
		}

		pScore.ProblemScores[pid] = pProbScore
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort standings
	var standings []ParticipantScore
	for _, pScore := range scoresByParticipant {
		standings = append(standings, *pScore)
	}

	sort.SliceStable(standings, func(i, j int) bool {
		if standings[i].SolvedCount != standings[j].SolvedCount {
			return standings[i].SolvedCount > standings[j].SolvedCount
		}
		return standings[i].TotalPenalty < standings[j].TotalPenalty
	})

	for i := range standings {
		standings[i].Rank = i + 1
	}

	if standings == nil {
		standings = []ParticipantScore{}
	}

	return &StandingsResponse{
		ContestID:   contestID,
		ScoringType: string(c.ScoringType),
		Standings:   standings,
		Problems:    headers,
		GeneratedAt: s.timeClock(),
	}, nil
}

type Handler struct {
	service *Service
	authSvc *auth.Service
}

func NewHandler(service *Service, authSvc *auth.Service) *Handler {
	return &Handler{service: service, authSvc: authSvc}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Use(h.authSvc.AuthMiddleware(true))
	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Get("/{id}", h.Get)
	r.Get("/{id}/sync", h.Sync)
	r.Post("/{id}/sync", h.Sync)
	r.Post("/{id}/dispatched", h.UpdateDispatched)
	r.Post("/{id}/result", h.UpdateResult)

	return r
}

type createSubReq struct {
	ProblemID  string  `json:"problemId"`
	ContestID  *string `json:"contestId"`
	Language   string  `json:"language"`
	SourceCode string  `json:"sourceCode"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var req createSubReq
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	isAdmin := claims.Role == auth.RoleAdmin
	sub, err := h.service.Create(r.Context(), claims.UserID, isAdmin, req.ProblemID, req.ContestID, req.Language, req.SourceCode)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, ErrDuplicateSource) {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sub)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	isAdmin := claims.Role == auth.RoleAdmin

	sub, err := h.service.GetByID(r.Context(), id, claims.UserID, isAdmin)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sub)
}

func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	isAdmin := claims.Role == auth.RoleAdmin

	sub, err := h.service.SyncStatus(r.Context(), id, claims.UserID, isAdmin)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sub)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	isAdmin := claims.Role == auth.RoleAdmin

	q := r.URL.Query()
	uid := q.Get("userId")
	if !isAdmin {
		// Non-admin users are restricted to viewing only their own submissions
		uid = claims.UserID
	}
	cid := q.Get("contestId")
	pid := q.Get("problemId")

	subs, err := h.service.ListForViewer(r.Context(), uid, cid, pid, 50, 0, isAdmin)
	if err != nil {
		http.Error(w, `{"error":"failed to list submissions"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(subs)
}

type dispatchedReq struct {
	ExternalSubmissionID string `json:"externalSubmissionId"`
}

func (h *Handler) UpdateDispatched(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	isAdmin := claims.Role == auth.RoleAdmin

	var req dispatchedReq
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	err := h.service.UpdateDispatched(r.Context(), id, claims.UserID, isAdmin, req.ExternalSubmissionID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

type resultReq struct {
	Status   Status         `json:"status"`
	Metadata map[string]any `json:"metadata"`
}

func (h *Handler) UpdateResult(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")
	isAdmin := claims.Role == auth.RoleAdmin

	var req resultReq
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	err := h.service.UpdateResult(r.Context(), id, claims.UserID, isAdmin, req.Status, req.Metadata)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
