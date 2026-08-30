package submission

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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

const judgingTimeout = 15 * time.Minute

type VerificationErrorKind string

const (
	VerificationRetryable  VerificationErrorKind = "retryable"
	VerificationDefinitive VerificationErrorKind = "definitive"
)

// VerificationError preserves whether an external response is incomplete or
// proves that the dispatched submission is not the one being observed.
type VerificationError struct {
	Kind VerificationErrorKind
	Err  error
}

func (e *VerificationError) Error() string {
	if e == nil || e.Err == nil {
		return "external submission could not be verified"
	}
	return e.Err.Error()
}

func (e *VerificationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func retryableVerificationError(message string) error {
	return &VerificationError{Kind: VerificationRetryable, Err: errors.New(message)}
}

func definitiveVerificationError(message string) error {
	return &VerificationError{Kind: VerificationDefinitive, Err: errors.New(message)}
}

// ParseStatus validates a normalized platform status before it can cross the
// persistence boundary. Platform adapters must return one of these values.
func ParseStatus(raw string) (Status, error) {
	status := Status(strings.TrimSpace(raw))
	switch status {
	case Pending, Dispatching, Judging, Accepted, WrongAnswer, TimeLimit, MemoryLimit, RuntimeError, CompileError, Failed:
		return status, nil
	default:
		return "", fmt.Errorf("unsupported submission status %q", raw)
	}
}

func isTerminalStatus(status Status) bool {
	return status != Pending && status != Dispatching && status != Judging
}

func isPenaltyStatus(status Status) bool {
	switch status {
	case WrongAnswer, RuntimeError, TimeLimit, MemoryLimit:
		return true
	default:
		return false
	}
}

func hasAttempts(problemScores map[string]ProblemScore) bool {
	for _, problemScore := range problemScores {
		if problemScore.Attempts > 0 {
			return true
		}
	}
	return false
}

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
	ExternalSubmittedAt  *time.Time     `json:"externalSubmittedAt,omitempty"`
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
		if len(problemParts) >= 3 && strings.EqualFold(problemParts[0], "gym") {
			contestID = url.PathEscape(problemParts[1])
			sourceURL = fmt.Sprintf("https://codeforces.com/gym/%s/submission/%s", contestID, submissionID)
		} else {
			sourceURL = fmt.Sprintf("https://codeforces.com/contest/%s/submission/%s", contestID, submissionID)
		}
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
	ContestID        string             `json:"contestId"`
	ScoringType      string             `json:"scoringType"`
	Standings        []ParticipantScore `json:"standings"`
	UpsolveStandings []ParticipantScore `json:"upsolveStandings"`
	Problems         []ProblemHeader    `json:"problems"`
	GeneratedAt      time.Time          `json:"generatedAt"`
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
	pollEnqueue  func(context.Context, *asynq.Task) error
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
	s.pollEnqueue = nil
}

func (s *Service) SetPollEnqueuer(enqueue func(context.Context, *asynq.Task) error) {
	s.pollEnqueue = enqueue
}

func canonicalExternalID(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) < 2 {
		return strings.TrimSpace(value)
	}
	if len(parts) >= 3 && strings.EqualFold(strings.TrimSpace(parts[0]), "gym") {
		return "gym/" + strings.TrimSpace(parts[1]) + "/" + strings.ToUpper(strings.TrimSpace(parts[2]))
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

const externalDispatchWindow = 2 * time.Minute

const dispatchProofPrefix = "cpbridge-dispatch-proof:"

func dispatchProofComment(language, proof string) string {
	commentPrefix := "//"
	if language == "python3" {
		commentPrefix = "#"
	}
	return fmt.Sprintf("%s %s%s", commentPrefix, dispatchProofPrefix, proof)
}

func withDispatchProof(source, language string) (string, error) {
	proofBytes := make([]byte, 24)
	if _, err := rand.Read(proofBytes); err != nil {
		return "", fmt.Errorf("failed to create submission proof: %w", err)
	}
	proof := hex.EncodeToString(proofBytes)
	return strings.TrimRight(source, "\r\n") + "\n" + dispatchProofComment(language, proof) + "\n", nil
}

func normalizeSourceCode(source string) string {
	return strings.TrimRight(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
}

func validateExternalSubmissionMetadata(sub *Submission, externalID string, statusObj *platform.SubmissionStatus, _ time.Time) error {
	if sub == nil || statusObj == nil {
		return retryableVerificationError("external submission metadata is temporarily unavailable")
	}
	if statusObj.ExternalSubmissionID != "" && canonicalExternalID(statusObj.ExternalSubmissionID) != canonicalExternalID(externalID) {
		return definitiveVerificationError("external submission ID could not be verified")
	}
	if strings.TrimSpace(statusObj.ProblemExternalID) == "" {
		return retryableVerificationError("external submission problem metadata is not available yet")
	}
	if canonicalExternalID(statusObj.ProblemExternalID) != canonicalExternalID(sub.ProblemExternalID) {
		return definitiveVerificationError("external submission targets a different problem")
	}
	if strings.TrimSpace(statusObj.Language) == "" {
		return retryableVerificationError("external submission language metadata is not available yet")
	}
	if !platformLanguageMatches(sub.Platform, sub.Language, statusObj.Language) {
		return definitiveVerificationError("external submission uses a different language")
	}
	if statusObj.SubmittedAt == nil {
		return retryableVerificationError("external submission timestamp is not available yet")
	}
	if statusObj.SubmittedAt.Before(sub.SubmittedAt.Add(-externalDispatchWindow)) || statusObj.SubmittedAt.After(sub.SubmittedAt.Add(externalDispatchWindow)) {
		return definitiveVerificationError("external submission timestamp is outside the dispatch window")
	}
	if strings.TrimSpace(statusObj.PlatformUsername) == "" {
		return retryableVerificationError("external submission platform identity is not available yet")
	}
	if !strings.Contains(sub.SourceCode, dispatchProofPrefix) {
		return definitiveVerificationError("external submission source could not be verified")
	}
	if strings.TrimSpace(statusObj.SourceCode) == "" {
		// Codeforces' public API does not expose source code, and its submission
		// pages can return a Cloudflare challenge to server-side requests. Keep
		// the proof as a strict check whenever source is available, but fall back
		// to the verified ID, problem, language, timestamp, and platform identity
		// when Codeforces prevents the adapter from reading it.
		if sub.Platform == platform.Codeforces {
			return nil
		}
		return retryableVerificationError("external submission source is not available yet")
	}
	if normalizeSourceCode(statusObj.SourceCode) != normalizeSourceCode(sub.SourceCode) {
		return definitiveVerificationError("external submission source could not be verified")
	}
	return nil
}

func validateExternalSubmissionContestWindow(sub *Submission, externalSubmittedAt time.Time, startAt, endAt time.Time) error {
	if sub == nil || sub.ContestID == nil || strings.TrimSpace(*sub.ContestID) == "" {
		return nil
	}
	// Finished contests remain available for practice. Those submissions retain
	// their contest context in the history, while standings independently ignore
	// timestamps at or after endAt. Only a handoff created during the official
	// window must also reach the external platform before the contest ends.
	if !sub.SubmittedAt.Before(endAt) {
		return nil
	}
	if externalSubmittedAt.Before(startAt) || !externalSubmittedAt.Before(endAt) {
		return definitiveVerificationError("external submission timestamp is outside the contest window")
	}
	return nil
}

func (s *Service) getContestWindow(ctx context.Context, contestID string) (time.Time, time.Time, error) {
	var startAt, endAt time.Time
	err := s.db.QueryRowContext(ctx, `
		SELECT start_at, end_at
		FROM contests
		WHERE id = $1
	`, contestID).Scan(&startAt, &endAt)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, time.Time{}, errors.New("contest not found")
	}
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to load contest window: %w", err)
	}
	return startAt.UTC(), endAt.UTC(), nil
}

// verifyExternalSubmission checks data obtained from the official platform,
// including the server-generated source proof. A platform identity is linked
// only after that proof has been verified; a caller cannot establish identity
// by supplying an unrelated public submission.
func (s *Service) verifyExternalSubmission(ctx context.Context, sub *Submission, externalID string, statusObj *platform.SubmissionStatus, now time.Time) error {
	if err := validateExternalSubmissionMetadata(sub, externalID, statusObj, now); err != nil {
		return err
	}
	if sub.ContestID != nil && strings.TrimSpace(*sub.ContestID) != "" {
		startAt, endAt, err := s.getContestWindow(ctx, *sub.ContestID)
		if err != nil {
			return &VerificationError{Kind: VerificationRetryable, Err: err}
		}
		if err := validateExternalSubmissionContestWindow(sub, *statusObj.SubmittedAt, startAt, endAt); err != nil {
			return err
		}
	}

	var expectedUsername, connectionStatus string
	err := s.db.QueryRowContext(ctx, `
		SELECT external_username, connection_status
		FROM integrations
		WHERE user_id = $1 AND platform = $2
	`, sub.UserID, sub.Platform).Scan(&expectedUsername, &connectionStatus)
	if errors.Is(err, sql.ErrNoRows) {
		// The source proof above establishes that this external submission was
		// dispatched for this cpbridge submission, so its official username is a
		// server-verifiable account-linking proof.
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO integrations (user_id, platform, external_username, connection_status, updated_at)
			VALUES ($1, $2, $3, 'CONNECTED', $4)
			ON CONFLICT (user_id, platform) DO NOTHING
		`, sub.UserID, sub.Platform, statusObj.PlatformUsername, now)
		if err != nil {
			return &VerificationError{Kind: VerificationRetryable, Err: fmt.Errorf("failed to link verified platform identity: %w", err)}
		}
		err = s.db.QueryRowContext(ctx, `
			SELECT external_username, connection_status
			FROM integrations
			WHERE user_id = $1 AND platform = $2
		`, sub.UserID, sub.Platform).Scan(&expectedUsername, &connectionStatus)
	}
	if err != nil {
		return &VerificationError{Kind: VerificationRetryable, Err: fmt.Errorf("failed to verify platform identity: %w", err)}
	}
	if !strings.EqualFold(strings.TrimSpace(expectedUsername), strings.TrimSpace(statusObj.PlatformUsername)) {
		return definitiveVerificationError("external submission platform identity does not match the connected account")
	}
	if connectionStatus != "CONNECTED" {
		_, err = s.db.ExecContext(ctx, `
			UPDATE integrations
			SET connection_status = 'CONNECTED', updated_at = $1
			WHERE user_id = $2 AND platform = $3
		`, now, sub.UserID, sub.Platform)
		if err != nil {
			return &VerificationError{Kind: VerificationRetryable, Err: fmt.Errorf("failed to reconnect verified platform identity: %w", err)}
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
		  AND status <> $5
		LIMIT 1
	`, userID, problemID, language, sourceCode, Failed).Scan(&existingID)
	if err == nil {
		return nil, ErrDuplicateSource
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check for duplicate submission: %w", err)
	}

	sourceDigest := sha256.Sum256([]byte(sourceCode))
	sourceHash := hex.EncodeToString(sourceDigest[:])
	dispatchSource, err := withDispatchProof(sourceCode, language)
	if err != nil {
		return nil, err
	}

	sub := &Submission{
		ID:          idgen.New(idgen.PrefixSubmission),
		UserID:      userID,
		ProblemID:   problemID,
		ContestID:   contestID,
		Platform:    prob.Platform,
		Language:    language,
		SourceCode:  dispatchSource,
		Status:      Pending,
		SubmittedAt: now,
		Metadata:    map[string]any{},
	}

	query := `
		INSERT INTO submissions (id, user_id, problem_id, contest_id, platform, language, source_code, source_hash, status, submitted_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (user_id, problem_id, language, source_hash) WHERE source_hash IS NOT NULL AND status <> 'FAILED' DO NOTHING
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

func newPollRequestID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to create poll request ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// requestPoll atomically reserves one queue request for a submission. The
// lease suppresses repeated /sync calls while allowing a later request to
// recover a task that was lost before it started. Worker-scheduled polls use
// force=true because they replace the completed poll's lease.
func (s *Service) requestPoll(ctx context.Context, submissionID string, force bool, delay time.Duration) error {
	if s.asynqClient == nil && s.pollEnqueue == nil {
		return nil
	}

	requestID, err := newPollRequestID()
	if err != nil {
		return err
	}

	condition := fmt.Sprintf("(poll_requested_at IS NULL OR poll_requested_at < NOW() - INTERVAL '%d seconds')", int(queue.PollRequestLease/time.Second))
	if force {
		condition = "TRUE"
	}
	var externalID, platformName, problemID string
	err = s.db.QueryRowContext(ctx, fmt.Sprintf(`
		UPDATE submissions
		SET poll_request_id = $1, poll_requested_at = NOW()
		WHERE id = $2
		  AND status IN ('PENDING', 'DISPATCHING', 'JUDGING')
		  AND external_submission_id IS NOT NULL
		  AND %s
		RETURNING external_submission_id, platform, problem_id
	`, condition), requestID, submissionID).Scan(&externalID, &platformName, &problemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to reserve submission poll: %w", err)
	}

	task, err := queue.NewPollVerdictTaskAfter(submissionID, externalID, platformName, problemID, requestID, delay)
	if err == nil {
		if s.pollEnqueue != nil {
			err = s.pollEnqueue(ctx, task)
		} else {
			_, err = s.asynqClient.EnqueueContext(ctx, task)
		}
	}
	if err != nil {
		_, clearErr := s.db.ExecContext(ctx, `
			UPDATE submissions
			SET poll_request_id = NULL, poll_requested_at = NULL
			WHERE id = $1 AND poll_request_id = $2
		`, submissionID, requestID)
		if clearErr != nil {
			log.Printf("[Queue:Error] Failed to release poll request %s for submission %s: %v", requestID, submissionID, clearErr)
		}
		return fmt.Errorf("failed to enqueue submission poll: %w", err)
	}
	return nil
}

func (s *Service) SyncStatus(ctx context.Context, id, requestingUserID string, isAdmin bool) (*Submission, error) {
	query := `
		SELECT s.id, s.user_id, u.username, s.problem_id, p.external_id, p.title, s.contest_id, s.platform, s.language,
		       s.source_code, s.status, s.external_submission_id, s.external_submitted_at, s.submitted_at, s.judged_at, s.metadata
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
		&sub.ExternalSubmittedAt, &sub.SubmittedAt, &sub.JudgedAt, &metaJSON,
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

	if (sub.Status == Pending || sub.Status == Dispatching || sub.Status == Judging) && sub.ExternalSubmissionID != nil && strings.TrimSpace(*sub.ExternalSubmissionID) != "" {
		// Sync is an explicit queue nudge, not a synchronous platform read.
		if err := s.requestPoll(ctx, sub.ID, false, 0); err != nil {
			return nil, err
		}
	}

	attachAdminSourceURL(&sub, isAdmin)
	return &sub, nil
}

func (s *Service) GetByID(ctx context.Context, id, requestingUserID string, isAdmin bool) (*Submission, error) {
	query := `
		SELECT s.id, s.user_id, u.username, s.problem_id, p.external_id, p.title, s.contest_id, s.platform, s.language,
		       s.source_code, s.status, s.external_submission_id, s.external_submitted_at, s.submitted_at, s.judged_at, s.metadata
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
		&sub.ExternalSubmittedAt, &sub.SubmittedAt, &sub.JudgedAt, &metaJSON,
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
		       s.source_code, s.status, s.external_submission_id, s.external_submitted_at, s.submitted_at, s.judged_at, s.metadata
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
			&sub.ExternalSubmittedAt, &sub.SubmittedAt, &sub.JudgedAt, &metaJSON,
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

	externalSubmissionID = strings.TrimSpace(externalSubmissionID)
	if externalSubmissionID == "" || strings.HasPrefix(externalSubmissionID, "cf_") || strings.HasPrefix(externalSubmissionID, "ac_") {
		return errors.New("invalid external submission ID")
	}
	storedExternalID := externalSubmissionID
	if !strings.Contains(storedExternalID, "/") && strings.Contains(sub.ProblemExternalID, "/") {
		storedExternalID = externalSubmissionRef(sub.ProblemExternalID, storedExternalID)
	}
	if sub.ExternalSubmissionID != nil && strings.TrimSpace(*sub.ExternalSubmissionID) != "" {
		if canonicalExternalID(*sub.ExternalSubmissionID) != canonicalExternalID(storedExternalID) {
			return errors.New("a different external submission ID is already attached")
		}
		if isTerminalStatus(sub.Status) {
			return nil
		}
		// Recovery is idempotent: the same handoff can request a fresh queue
		// lease after an enqueue failure without changing the attached ID.
		return s.requestPoll(ctx, sub.ID, true, 0)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE submissions
		SET status = $1, external_submission_id = $2, judged_at = NULL,
		    poll_started_at = COALESCE(poll_started_at, NOW()),
		    poll_request_id = NULL, poll_requested_at = NULL
		WHERE id = $3
		  AND status IN ($4, $5)
		  AND (external_submission_id IS NULL OR external_submission_id = $2)
	`, Judging, storedExternalID, id, Pending, Dispatching)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		// Re-read after the compare-and-set. Another handoff may have won the
		// race while this request still had a nil external ID in its snapshot.
		var currentExternalID sql.NullString
		var currentStatus Status
		err = s.db.QueryRowContext(ctx, `
			SELECT external_submission_id, status
			FROM submissions
			WHERE id = $1
		`, id).Scan(&currentExternalID, &currentStatus)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if currentExternalID.Valid && strings.TrimSpace(currentExternalID.String) != "" {
			if canonicalExternalID(currentExternalID.String) != canonicalExternalID(storedExternalID) {
				return errors.New("a different external submission ID is already attached")
			}
			if !isTerminalStatus(currentStatus) {
				return s.requestPoll(ctx, id, true, 0)
			}
		}
		// The row became terminal (or is a client-failure record without an
		// external ID). Keep that result unchanged.
		return nil
	}

	log.Printf("[Submission:Dispatched] %s linked with external ID %s (status: %s)", sub.ID, storedExternalID, Judging)
	if err := s.requestPoll(ctx, sub.ID, true, 3*time.Second); err != nil {
		// The row remains JUDGING with its verified external ID, so the client
		// can safely retain its recovery record and retry this handoff.
		return err
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

	parsedStatus, err := ParseStatus(string(status))
	if err != nil {
		return err
	}
	// This endpoint is exclusively for a client-side handoff failure. External
	// verdicts are written by the asynchronous worker after platform polling.
	if parsedStatus != Failed {
		return errors.New("only client-dispatch failures may be reported here")
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
	_, err = s.db.ExecContext(ctx, `
		UPDATE submissions
		SET status = $1, judged_at = $2, metadata = $3
		WHERE id = $4
		  AND status IN ($5, $6)
		  AND external_submission_id IS NULL
	`, Failed, now, metaJSON, id, Pending, Dispatching)
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

	// Fetch submissions during contest window only, using the verified external
	// timestamp for dispatched submissions. The fallback keeps legacy rows
	// scoreable until they are re-linked.
	subQuery := `
		SELECT s.id, s.user_id, s.problem_id, s.status, COALESCE(s.external_submitted_at, s.submitted_at)
		FROM submissions s
		WHERE s.contest_id = $1
		  AND COALESCE(s.external_submitted_at, s.submitted_at) >= $2
		  AND COALESCE(s.external_submitted_at, s.submitted_at) < $3
		ORDER BY COALESCE(s.external_submitted_at, s.submitted_at) ASC,
			CASE
				WHEN regexp_replace(COALESCE(s.external_submission_id, ''), '^.*/', '') ~ '^[0-9]+$'
				THEN regexp_replace(COALESCE(s.external_submission_id, ''), '^.*/', '')::numeric
			END ASC NULLS LAST,
			s.external_submission_id ASC NULLS LAST,
			s.id ASC
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

		if status == Accepted {
			pProbScore.Attempts++
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
		} else if isPenaltyStatus(status) {
			pProbScore.Attempts++
		}

		pScore.ProblemScores[pid] = pProbScore
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// After-contest standings contain post-contest activity without changing the
	// contest-period standings. This includes contest participants who had no
	// contest-period submissions and users who first submit after the contest.
	upsolveScores := make(map[string]*ParticipantScore, len(scoresByParticipant))
	for uid, participant := range scoresByParticipant {
		upsolveScores[uid] = &ParticipantScore{
			UserID:        participant.UserID,
			Username:      participant.Username,
			ProblemScores: make(map[string]ProblemScore, len(problems)),
		}
		for _, p := range problems {
			upsolveScores[uid].ProblemScores[p.ProblemID] = ProblemScore{
				ProblemID: p.ProblemID,
				Label:     p.Label,
			}
		}
	}

	upsolveRows, err := s.db.QueryContext(ctx, `
		SELECT s.user_id, u.username, s.problem_id, s.status, COALESCE(s.external_submitted_at, s.submitted_at)
		FROM submissions s
		JOIN users u ON u.id = s.user_id
		JOIN contest_problems cpr ON cpr.contest_id = s.contest_id AND cpr.problem_id = s.problem_id
		WHERE s.contest_id = $1
		  AND COALESCE(s.external_submitted_at, s.submitted_at) >= $2
		ORDER BY COALESCE(s.external_submitted_at, s.submitted_at) ASC, s.id ASC
	`, contestID, c.EndAt)
	if err != nil {
		return nil, err
	}
	defer upsolveRows.Close()

	for upsolveRows.Next() {
		var uid, uname, pid string
		var status Status
		var subTime time.Time
		if err := upsolveRows.Scan(&uid, &uname, &pid, &status, &subTime); err != nil {
			return nil, err
		}

		upsolveScore, exists := upsolveScores[uid]
		if !exists {
			upsolveScore = &ParticipantScore{
				UserID:        uid,
				Username:      uname,
				ProblemScores: make(map[string]ProblemScore, len(problems)),
			}
			for _, p := range problems {
				upsolveScore.ProblemScores[p.ProblemID] = ProblemScore{
					ProblemID: p.ProblemID,
					Label:     p.Label,
				}
			}
			upsolveScores[uid] = upsolveScore
		}
		problemScore, exists := upsolveScore.ProblemScores[pid]
		if !exists || problemScore.Solved {
			// A repeat solve is not an upsolve; the problem was already solved
			// during the contest window for participants who were in the contest.
			continue
		}
		if contestScore, wasParticipant := scoresByParticipant[uid]; wasParticipant && contestScore.ProblemScores[pid].Solved {
			continue
		}

		if status == Accepted {
			problemScore.Attempts++
			problemScore.Solved = true
			elapsedMinutes := max(0, int(math.Floor(subTime.Sub(c.EndAt).Minutes())))
			problemScore.FirstSolvedAtMinutes = &elapsedMinutes
			problemScore.PenaltyMinutes = elapsedMinutes + (20 * (problemScore.Attempts - 1))
			upsolveScore.SolvedCount++
			upsolveScore.TotalPenalty += problemScore.PenaltyMinutes
		} else if isPenaltyStatus(status) {
			problemScore.Attempts++
		}
		upsolveScore.ProblemScores[pid] = problemScore
	}
	if err := upsolveRows.Err(); err != nil {
		return nil, err
	}

	var upsolveStandings []ParticipantScore
	for _, participant := range upsolveScores {
		if participant.SolvedCount > 0 || hasAttempts(participant.ProblemScores) {
			upsolveStandings = append(upsolveStandings, *participant)
		}
	}
	sort.SliceStable(upsolveStandings, func(i, j int) bool {
		if upsolveStandings[i].SolvedCount != upsolveStandings[j].SolvedCount {
			return upsolveStandings[i].SolvedCount > upsolveStandings[j].SolvedCount
		}
		if upsolveStandings[i].TotalPenalty != upsolveStandings[j].TotalPenalty {
			return upsolveStandings[i].TotalPenalty < upsolveStandings[j].TotalPenalty
		}
		if upsolveStandings[i].Username != upsolveStandings[j].Username {
			return upsolveStandings[i].Username < upsolveStandings[j].Username
		}
		return upsolveStandings[i].UserID < upsolveStandings[j].UserID
	})
	for i := range upsolveStandings {
		if i > 0 && upsolveStandings[i].SolvedCount == upsolveStandings[i-1].SolvedCount && upsolveStandings[i].TotalPenalty == upsolveStandings[i-1].TotalPenalty {
			upsolveStandings[i].Rank = upsolveStandings[i-1].Rank
		} else {
			upsolveStandings[i].Rank = i + 1
		}
	}
	if upsolveStandings == nil {
		upsolveStandings = []ParticipantScore{}
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
		if standings[i].TotalPenalty != standings[j].TotalPenalty {
			return standings[i].TotalPenalty < standings[j].TotalPenalty
		}
		if standings[i].Username != standings[j].Username {
			return standings[i].Username < standings[j].Username
		}
		return standings[i].UserID < standings[j].UserID
	})

	for i := range standings {
		if i > 0 && standings[i].SolvedCount == standings[i-1].SolvedCount && standings[i].TotalPenalty == standings[i-1].TotalPenalty {
			standings[i].Rank = standings[i-1].Rank
		} else {
			standings[i].Rank = i + 1
		}
	}

	if standings == nil {
		standings = []ParticipantScore{}
	}

	return &StandingsResponse{
		ContestID:        contestID,
		ScoringType:      string(c.ScoringType),
		Standings:        standings,
		UpsolveStandings: upsolveStandings,
		Problems:         headers,
		GeneratedAt:      s.timeClock(),
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

	limit := 50
	if l := q.Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}
	offset := 0
	if o := q.Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	} else if p := q.Get("page"); p != "" {
		if pageNum, err := strconv.Atoi(p); err == nil && pageNum > 0 {
			offset = (pageNum - 1) * limit
		}
	}

	subs, err := h.service.ListForViewer(r.Context(), uid, cid, pid, limit, offset, isAdmin)
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
