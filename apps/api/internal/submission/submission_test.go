package submission_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"testing"
	"time"

	"github.com/cpbridge/api/internal/auth"
	"github.com/cpbridge/api/internal/contest"
	"github.com/cpbridge/api/internal/db"
	"github.com/cpbridge/api/internal/platform"
	"github.com/cpbridge/api/internal/problem"
	"github.com/cpbridge/api/internal/problemset"
	"github.com/cpbridge/api/internal/submission"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	log.SetOutput(io.Discard)
}

type verifiedSubmissionPlatform struct {
	status *platform.SubmissionStatus
}

func (p *verifiedSubmissionPlatform) Type() platform.Type {
	return platform.Codeforces
}

func (p *verifiedSubmissionPlatform) MatchURL(string) (string, bool) {
	return "", false
}

func (p *verifiedSubmissionPlatform) GetProblem(context.Context, string) (*platform.NormalizedProblem, error) {
	return nil, nil
}

func (p *verifiedSubmissionPlatform) GetStatement(context.Context, string) (*platform.ProblemStatement, error) {
	return nil, nil
}

func (p *verifiedSubmissionPlatform) GetSubmission(context.Context, string) (*platform.SubmissionStatus, error) {
	return p.status, nil
}

func TestICPCPenaltyCalculation(t *testing.T) {
	// Formula: penalty = minutes_from_start_to_first_AC + 20 * rejected_before_first_AC
	tests := []struct {
		name             string
		elapsedMinutes   int
		rejectedBeforeAC int
		expectedPenalty  int
	}{
		{
			name:             "AC on first attempt at 15m",
			elapsedMinutes:   15,
			rejectedBeforeAC: 0,
			expectedPenalty:  15,
		},
		{
			name:             "AC on 3rd attempt (2 WA before) at 45m",
			elapsedMinutes:   45,
			rejectedBeforeAC: 2,
			expectedPenalty:  85, // 45 + 20*2 = 85
		},
		{
			name:             "AC on 6th attempt (5 WA before) at 110m",
			elapsedMinutes:   110,
			rejectedBeforeAC: 5,
			expectedPenalty:  210, // 110 + 20*5 = 210
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			penalty := tt.elapsedMinutes + (20 * tt.rejectedBeforeAC)
			assert.Equal(t, tt.expectedPenalty, penalty)
		})
	}
}

func TestSubmissionStatuses(t *testing.T) {
	assert.Equal(t, submission.Status("PENDING"), submission.Pending)
	assert.Equal(t, submission.Status("JUDGING"), submission.Judging)
	assert.Equal(t, submission.Status("ACCEPTED"), submission.Accepted)
	assert.Equal(t, submission.Status("WRONG_ANSWER"), submission.WrongAnswer)
	assert.Equal(t, submission.Status("FAILED"), submission.Failed)
}

func TestUpdateDispatchedLinksFirstVerifiedPlatformIdentity(t *testing.T) {
	database, err := db.Connect()
	if err != nil {
		t.Skipf("Skipping database integration tests: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := database.Ping(); err != nil {
		t.Skipf("Skipping database integration tests: %v", err)
	}
	require.NoError(t, db.EnsureSchema(database))

	suffix := time.Now().UnixNano()
	email := fmt.Sprintf("verified_%d@test.com", suffix)
	problemExternalID := fmt.Sprintf("%d/A", suffix)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = database.ExecContext(ctx, `DELETE FROM submissions WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, email)
		_, _ = database.ExecContext(ctx, `DELETE FROM integrations WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, email)
		_, _ = database.ExecContext(ctx, `DELETE FROM contests WHERE owner_id IN (SELECT id FROM users WHERE email = $1)`, email)
		_, _ = database.ExecContext(ctx, `DELETE FROM problems WHERE platform = $1 AND external_id = $2`, platform.Codeforces, problemExternalID)
		_, _ = database.ExecContext(ctx, `DELETE FROM users WHERE email = $1`, email)
	})

	ctx := context.Background()
	authSvc := auth.NewService(database)
	adapter := &verifiedSubmissionPlatform{}
	registry := platform.NewRegistry()
	registry.Register(adapter)
	problemSvc := problem.NewService(database, registry)
	contestSvc := contest.NewService(database, problemset.NewService(database))
	submissionSvc := submission.NewService(database, contestSvc, problemSvc, registry)

	user, _, err := authSvc.Register(ctx, email, fmt.Sprintf("verified_%d", suffix), "password123")
	require.NoError(t, err)
	createdProblem, err := problemSvc.CreateCustom(ctx, problem.CreateCustomReq{
		Title:       "Verified identity problem",
		Platform:    platform.Codeforces,
		ExternalID:  problemExternalID,
		Statement:   "Test statement",
		TimeLimit:   "1.0s",
		MemoryLimit: "256MB",
	})
	require.NoError(t, err)
	now := time.Now().UTC()
	activeContest, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           user.ID,
		ProblemIDs:        []string{createdProblem.ID},
		Name:              "Verified identity contest",
		StartAt:           now.Add(-time.Hour),
		EndAt:             now.Add(time.Hour),
		Visibility:        "PRIVATE",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	require.NoError(t, err)
	createdSubmission, err := submissionSvc.Create(ctx, user.ID, false, createdProblem.ID, &activeContest.ID, "cpp23", "int main() { return 0; }")
	require.NoError(t, err)

	externalSubmissionID := fmt.Sprintf("%d", suffix+1)
	adapter.status = &platform.SubmissionStatus{
		ExternalSubmissionID: fmt.Sprintf("%d/%s", suffix, externalSubmissionID),
		Status:               "JUDGING",
		ProblemExternalID:    problemExternalID,
		Language:             "GNU C++23 (64)",
		PlatformUsername:     "verified_handle",
		SubmittedAt:          &createdSubmission.SubmittedAt,
	}

	require.NoError(t, submissionSvc.UpdateDispatched(ctx, createdSubmission.ID, user.ID, false, externalSubmissionID))

	var username, connectionStatus string
	err = database.QueryRowContext(ctx, `
		SELECT external_username, connection_status
		FROM integrations
		WHERE user_id = $1 AND platform = $2
	`, user.ID, platform.Codeforces).Scan(&username, &connectionStatus)
	require.NoError(t, err)
	assert.Equal(t, "verified_handle", username)
	assert.Equal(t, "CONNECTED", connectionStatus)

	updated, err := submissionSvc.GetByID(ctx, createdSubmission.ID, user.ID, false)
	require.NoError(t, err)
	assert.Equal(t, submission.Judging, updated.Status)
	require.NotNil(t, updated.ExternalSubmissionID)
	assert.Equal(t, externalSubmissionID, *updated.ExternalSubmissionID)
}

func TestContestEndedSubmissionAndScoreboardRules(t *testing.T) {
	database, err := db.Connect()
	if err != nil {
		t.Skipf("Skipping database integration tests: %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := database.Ping(); err != nil {
		t.Skipf("Skipping database integration tests: %v", err)
	}

	err = db.EnsureSchema(database)
	require.NoError(t, err)

	suffix := time.Now().UnixNano()
	ownerEmail := fmt.Sprintf("owner_%d@test.com", suffix)
	user1Email := fmt.Sprintf("u1_%d@test.com", suffix)
	user2Email := fmt.Sprintf("u2_%d@test.com", suffix)
	prob1ExtID := fmt.Sprintf("p1_%d", suffix)
	prob2ExtID := fmt.Sprintf("p2_%d", suffix)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return
		}
		defer tx.Rollback()

		_, _ = tx.ExecContext(ctx, `DELETE FROM submissions WHERE user_id IN (SELECT id FROM users WHERE email IN ($1, $2, $3))`, ownerEmail, user1Email, user2Email)
		_, _ = tx.ExecContext(ctx, `DELETE FROM contests WHERE owner_id IN (SELECT id FROM users WHERE email IN ($1, $2, $3))`, ownerEmail, user1Email, user2Email)
		_, _ = tx.ExecContext(ctx, `DELETE FROM problems WHERE external_id IN ($1, $2)`, prob1ExtID, prob2ExtID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM users WHERE email IN ($1, $2, $3)`, ownerEmail, user1Email, user2Email)
		_ = tx.Commit()
	})

	ctx := context.Background()
	authSvc := auth.NewService(database)
	platReg := platform.NewRegistry()
	probSvc := problem.NewService(database, platReg)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)
	subSvc := submission.NewService(database, contestSvc, probSvc, platReg)

	// Create users
	owner, _, err := authSvc.Register(ctx, ownerEmail, fmt.Sprintf("owner_%d", suffix), "password123")
	require.NoError(t, err)
	user1, _, err := authSvc.Register(ctx, user1Email, fmt.Sprintf("user1_%d", suffix), "password123")
	require.NoError(t, err)
	user2, _, err := authSvc.Register(ctx, user2Email, fmt.Sprintf("user2_%d", suffix), "password123")
	require.NoError(t, err)

	// Create problems
	p1, err := probSvc.CreateCustom(ctx, problem.CreateCustomReq{
		Title:       "Problem A",
		Platform:    platform.Codeforces,
		ExternalID:  prob1ExtID,
		Statement:   "Statement A",
		TimeLimit:   "1.0s",
		MemoryLimit: "256MB",
	})
	require.NoError(t, err)

	p2, err := probSvc.CreateCustom(ctx, problem.CreateCustomReq{
		Title:       "Problem B",
		Platform:    platform.Codeforces,
		ExternalID:  prob2ExtID,
		Statement:   "Statement B",
		TimeLimit:   "1.0s",
		MemoryLimit: "256MB",
	})
	require.NoError(t, err)

	now := time.Now().UTC()

	for _, removedLanguage := range []string{"go", "rust"} {
		removed, err := subSvc.Create(ctx, user1.ID, false, p1.ID, nil, removedLanguage, "removed language")
		assert.Nil(t, removed)
		assert.ErrorContains(t, err, "unsupported submission language")
	}

	// 1. Upcoming contest -> submission rejected
	cUpcoming, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           owner.ID,
		ProblemIDs:        []string{p1.ID, p2.ID},
		Name:              "Upcoming Contest",
		StartAt:           now.Add(1 * time.Hour),
		EndAt:             now.Add(3 * time.Hour),
		Visibility:        "PUBLIC",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	require.NoError(t, err)

	_, err = subSvc.Create(ctx, user1.ID, false, p1.ID, &cUpcoming.ID, "cpp23", "int main() {}")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "contest has not started yet")

	// 2. Active contest -> submission accepted & scores on scoreboard
	cActive, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           owner.ID,
		ProblemIDs:        []string{p1.ID, p2.ID},
		Name:              "Active Contest",
		StartAt:           now.Add(-1 * time.Hour),
		EndAt:             now.Add(1 * time.Hour),
		Visibility:        "PUBLIC",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	require.NoError(t, err)

	sub1, err := subSvc.Create(ctx, user1.ID, false, p1.ID, &cActive.ID, "cpp23", "int main() {}")
	require.NoError(t, err)
	require.NotNil(t, sub1)

	// The duplicate guard runs before a cpbridge record is created, which means
	// the caller cannot reach the extension dispatch step for this source.
	duplicate, err := subSvc.Create(ctx, user1.ID, false, p1.ID, &cActive.ID, "cpp23", "int main() {}")
	assert.Nil(t, duplicate)
	assert.ErrorIs(t, err, submission.ErrDuplicateSource)

	// A different language remains a separate submission even if the text is
	// the same, because judges compile it under different semantics.
	differentLanguage, err := subSvc.Create(ctx, user1.ID, false, p1.ID, &cActive.ID, "python3", "int main() {}")
	require.NoError(t, err)
	require.NotNil(t, differentLanguage)

	// Mark sub1 as ACCEPTED
	err = subSvc.UpdateResult(ctx, sub1.ID, user1.ID, true, submission.Accepted, map[string]any{})
	require.NoError(t, err)

	standingsActive, err := subSvc.CalculateStandings(ctx, cActive.ID, user1.ID, false)
	require.NoError(t, err)
	require.Len(t, standingsActive.Standings, 2) // owner (auto-joined) + user1 (auto-joined)

	var u1ScoreActive *submission.ParticipantScore
	for i := range standingsActive.Standings {
		if standingsActive.Standings[i].UserID == user1.ID {
			u1ScoreActive = &standingsActive.Standings[i]
		}
	}
	require.NotNil(t, u1ScoreActive)
	assert.Equal(t, 1, u1ScoreActive.SolvedCount)
	assert.True(t, u1ScoreActive.ProblemScores[p1.ID].Solved)

	// 3. Ended contest
	cEnded, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           owner.ID,
		ProblemIDs:        []string{p1.ID, p2.ID},
		Name:              "Ended Contest",
		StartAt:           now.Add(-3 * time.Hour),
		EndAt:             now.Add(-1 * time.Hour),
		Visibility:        "PUBLIC",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	require.NoError(t, err)

	// In cEnded, user1 was an existing participant who solved problem A during the contest window
	// Insert user1 as participant and an in-contest submission for problem A
	_, err = database.ExecContext(ctx, `INSERT INTO contest_participants (contest_id, user_id, joined_at) VALUES ($1, $2, $3)`, cEnded.ID, user1.ID, now.Add(-3*time.Hour))
	require.NoError(t, err)
	inContestSubTime := now.Add(-2 * time.Hour) // 1 hour into contest
	subDuringID := "sub_during_contest_test"
	_, err = database.ExecContext(ctx, `
		INSERT INTO submissions (id, user_id, problem_id, contest_id, platform, language, source_code, status, submitted_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, subDuringID, user1.ID, p1.ID, cEnded.ID, p1.Platform, "cpp23", "int main() {}", submission.Accepted, inContestSubTime, "{}")
	require.NoError(t, err)

	// 3a. Existing participant user1 submits problem B after contest end
	sub2, err := subSvc.Create(ctx, user1.ID, false, p2.ID, &cEnded.ID, "cpp23", "int main() { return 0; }")
	require.NoError(t, err, "Submitting to ended contest by existing participant should succeed")
	require.NotNil(t, sub2)

	// Mark sub2 as ACCEPTED
	err = subSvc.UpdateResult(ctx, sub2.ID, user1.ID, true, submission.Accepted, map[string]any{})
	require.NoError(t, err)

	// 3b. Non-participant user2 submits problem A after contest end
	sub3, err := subSvc.Create(ctx, user2.ID, false, p1.ID, &cEnded.ID, "cpp23", "int main() {}")
	require.NoError(t, err, "Submitting to ended contest by non-participant should succeed")
	require.NotNil(t, sub3)

	// Mark sub3 as ACCEPTED
	err = subSvc.UpdateResult(ctx, sub3.ID, user2.ID, true, submission.Accepted, map[string]any{})
	require.NoError(t, err)

	// Verify Standings for ended contest:
	// - user1 should STILL only have 1 solved problem (problem A from during contest) and 60 penalty (no points for problem B)
	// - user2 should NOT appear on the scoreboard at all
	standingsPostContest, err := subSvc.CalculateStandings(ctx, cEnded.ID, user1.ID, false)
	require.NoError(t, err)

	// user2 must NOT be in standings
	for _, p := range standingsPostContest.Standings {
		assert.NotEqual(t, user2.ID, p.UserID, "User2 who only submitted after contest ended should NOT show up on the scoreboard")
	}

	var u1ScorePost *submission.ParticipantScore
	for i := range standingsPostContest.Standings {
		if standingsPostContest.Standings[i].UserID == user1.ID {
			u1ScorePost = &standingsPostContest.Standings[i]
		}
	}
	require.NotNil(t, u1ScorePost)
	assert.Equal(t, 1, u1ScorePost.SolvedCount, "User1 should not get points for problem B submitted after contest end")
	assert.Equal(t, 60, u1ScorePost.TotalPenalty, "User1 penalty should not change from post-contest submissions")
	assert.False(t, u1ScorePost.ProblemScores[p2.ID].Solved, "Problem B on scoreboard should remain unsolved for contest standings")
}
