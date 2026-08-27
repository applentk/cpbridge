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
	_ "github.com/lib/pq"
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
	require.NotNil(t, updated.ExternalSubmittedAt)
	assert.Equal(t, createdSubmission.SubmittedAt, *updated.ExternalSubmittedAt)

	endedAt := now.Add(-time.Hour)
	endedContest, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           user.ID,
		ProblemIDs:        []string{createdProblem.ID},
		Name:              "Ended verification contest",
		StartAt:           now.Add(-2 * time.Hour),
		EndAt:             endedAt,
		Visibility:        "PRIVATE",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	require.NoError(t, err)

	// The record is created just before the contest ends, but the external
	// submission is created just after it ends. The dispatch window alone would
	// accept this; the contest window must reject it.
	recordAt := endedAt.Add(-30 * time.Second)
	submissionSvc.SetClock(func() time.Time { return recordAt })
	lateSubmission, err := submissionSvc.Create(ctx, user.ID, false, createdProblem.ID, &endedContest.ID, "cpp23", "int main() { return 1; }")
	require.NoError(t, err)

	lateExternalID := fmt.Sprintf("%d", suffix+2)
	lateExternalAt := endedAt.Add(30 * time.Second)
	submissionSvc.SetClock(func() time.Time { return lateExternalAt })
	adapter.status = &platform.SubmissionStatus{
		ExternalSubmissionID: fmt.Sprintf("%d/%s", suffix, lateExternalID),
		Status:               "JUDGING",
		ProblemExternalID:    problemExternalID,
		Language:             "GNU C++23 (64)",
		PlatformUsername:     "verified_handle",
		SubmittedAt:          &lateExternalAt,
	}

	err = submissionSvc.UpdateDispatched(ctx, lateSubmission.ID, user.ID, false, lateExternalID)
	require.ErrorContains(t, err, "outside the contest window")
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

	// A client-dispatch failure is retryable: it must not reserve the source
	// digest and block a subsequent attempt with the same code.
	retryable, err := subSvc.Create(ctx, user1.ID, false, p1.ID, &cActive.ID, "cpp23", "int main(){return 1;}")
	require.NoError(t, err)
	require.NoError(t, subSvc.UpdateResult(ctx, retryable.ID, user1.ID, true, submission.Failed, map[string]any{"error": "bridge unavailable"}))
	retried, err := subSvc.Create(ctx, user1.ID, false, p1.ID, &cActive.ID, "cpp23", "int main(){return 1;}")
	require.NoError(t, err)
	require.NotNil(t, retried)

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

func TestScoreboardPenaltyAndAttemptRules(t *testing.T) {
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
	ownerEmail := fmt.Sprintf("sb_owner_%d@test.com", suffix)
	userAEmail := fmt.Sprintf("sb_ua_%d@test.com", suffix)
	userBEmail := fmt.Sprintf("sb_ub_%d@test.com", suffix)
	userCEmail := fmt.Sprintf("sb_uc_%d@test.com", suffix)
	userDEmail := fmt.Sprintf("sb_ud_%d@test.com", suffix)
	userEEmail := fmt.Sprintf("sb_ue_%d@test.com", suffix)
	probExtID := fmt.Sprintf("sb_p_%d", suffix)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return
		}
		defer tx.Rollback()

		_, _ = tx.ExecContext(ctx, `DELETE FROM submissions WHERE user_id IN (SELECT id FROM users WHERE email IN ($1, $2, $3, $4, $5, $6))`, ownerEmail, userAEmail, userBEmail, userCEmail, userDEmail, userEEmail)
		_, _ = tx.ExecContext(ctx, `DELETE FROM contests WHERE owner_id IN (SELECT id FROM users WHERE email = $1)`, ownerEmail)
		_, _ = tx.ExecContext(ctx, `DELETE FROM problems WHERE external_id = $1`, probExtID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM users WHERE email IN ($1, $2, $3, $4, $5, $6)`, ownerEmail, userAEmail, userBEmail, userCEmail, userDEmail, userEEmail)
		_ = tx.Commit()
	})

	ctx := context.Background()
	authSvc := auth.NewService(database)
	platReg := platform.NewRegistry()
	probSvc := problem.NewService(database, platReg)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)
	subSvc := submission.NewService(database, contestSvc, probSvc, platReg)

	owner, _, err := authSvc.Register(ctx, ownerEmail, fmt.Sprintf("sb_owner_%d", suffix), "password123")
	require.NoError(t, err)
	userA, _, err := authSvc.Register(ctx, userAEmail, fmt.Sprintf("sb_ua_%d", suffix), "password123")
	require.NoError(t, err)
	userB, _, err := authSvc.Register(ctx, userBEmail, fmt.Sprintf("sb_ub_%d", suffix), "password123")
	require.NoError(t, err)
	userC, _, err := authSvc.Register(ctx, userCEmail, fmt.Sprintf("sb_uc_%d", suffix), "password123")
	require.NoError(t, err)
	userD, _, err := authSvc.Register(ctx, userDEmail, fmt.Sprintf("sb_ud_%d", suffix), "password123")
	require.NoError(t, err)
	userE, _, err := authSvc.Register(ctx, userEEmail, fmt.Sprintf("sb_ue_%d", suffix), "password123")
	require.NoError(t, err)

	prob, err := probSvc.CreateCustom(ctx, problem.CreateCustomReq{
		Title:       "Scoreboard Rule Problem",
		Platform:    platform.Codeforces,
		ExternalID:  probExtID,
		Statement:   "Statement",
		TimeLimit:   "1.0s",
		MemoryLimit: "256MB",
	})
	require.NoError(t, err)

	now := time.Now().UTC()
	startAt := now.Add(-2 * time.Hour)
	endAt := now.Add(2 * time.Hour)

	con, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           owner.ID,
		ProblemIDs:        []string{prob.ID},
		Name:              "Scoreboard Rule Contest",
		StartAt:           startAt,
		EndAt:             endAt,
		Visibility:        "PUBLIC",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	require.NoError(t, err)

	// Join all users to the contest
	require.NoError(t, contestSvc.Join(ctx, con.ID, userA.ID))
	require.NoError(t, contestSvc.Join(ctx, con.ID, userB.ID))
	require.NoError(t, contestSvc.Join(ctx, con.ID, userC.ID))
	require.NoError(t, contestSvc.Join(ctx, con.ID, userD.ID))
	require.NoError(t, contestSvc.Join(ctx, con.ID, userE.ID))

	insertSub := func(subID, userID string, status submission.Status, minutesFromStart int) {
		subTime := startAt.Add(time.Duration(minutesFromStart) * time.Minute)
		_, err := database.ExecContext(ctx, `
			INSERT INTO submissions (id, user_id, problem_id, contest_id, platform, language, source_code, status, submitted_at, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, subID, userID, prob.ID, con.ID, prob.Platform, "cpp23", fmt.Sprintf("code_%s", subID), status, subTime, "{}")
		require.NoError(t, err)
	}

	// User A: COMPILE_ERROR, FAILED, JUDGING -> attempts should be 0, not solved, 0 penalty
	insertSub(fmt.Sprintf("sub_a1_%d", suffix), userA.ID, submission.CompileError, 5)
	insertSub(fmt.Sprintf("sub_a2_%d", suffix), userA.ID, submission.Failed, 10)
	insertSub(fmt.Sprintf("sub_a3_%d", suffix), userA.ID, submission.Judging, 15)

	// User B: COMPILE_ERROR, FAILED, then ACCEPTED at 20m -> attempts should be 1 (0 rejected before AC), solved, 20 penalty
	insertSub(fmt.Sprintf("sub_b1_%d", suffix), userB.ID, submission.CompileError, 5)
	insertSub(fmt.Sprintf("sub_b2_%d", suffix), userB.ID, submission.Failed, 10)
	insertSub(fmt.Sprintf("sub_b3_%d", suffix), userB.ID, submission.Accepted, 20)

	// User C: COMPILE_ERROR, WRONG_ANSWER, FAILED, RUNTIME_ERROR, TIME_LIMIT, MEMORY_LIMIT, then ACCEPTED at 30m
	// Rejects before AC: 4 (WA, RE, TL, ML). Total attempts: 5. Penalty: 30 + 20*4 = 110
	insertSub(fmt.Sprintf("sub_c1_%d", suffix), userC.ID, submission.CompileError, 2)
	insertSub(fmt.Sprintf("sub_c2_%d", suffix), userC.ID, submission.WrongAnswer, 5)
	insertSub(fmt.Sprintf("sub_c3_%d", suffix), userC.ID, submission.Failed, 8)
	insertSub(fmt.Sprintf("sub_c4_%d", suffix), userC.ID, submission.RuntimeError, 12)
	insertSub(fmt.Sprintf("sub_c5_%d", suffix), userC.ID, submission.TimeLimit, 16)
	insertSub(fmt.Sprintf("sub_c6_%d", suffix), userC.ID, submission.MemoryLimit, 20)
	insertSub(fmt.Sprintf("sub_c7_%d", suffix), userC.ID, submission.Accepted, 30)

	// User D: COMPILE_ERROR, FAILED, WRONG_ANSWER, RUNTIME_ERROR -> attempts should be 2 (WA, RE), not solved, 0 penalty
	insertSub(fmt.Sprintf("sub_d1_%d", suffix), userD.ID, submission.CompileError, 5)
	insertSub(fmt.Sprintf("sub_d2_%d", suffix), userD.ID, submission.Failed, 10)
	insertSub(fmt.Sprintf("sub_d3_%d", suffix), userD.ID, submission.WrongAnswer, 15)
	insertSub(fmt.Sprintf("sub_d4_%d", suffix), userD.ID, submission.RuntimeError, 20)

	// Codeforces creation timestamps have second precision. The external IDs
	// provide the chronological order when a WA and its following AC share a
	// timestamp; the scoreboard must retain the 20-minute penalty.
	sameSecond := startAt.Add(40 * time.Minute).Truncate(time.Second)
	for _, sub := range []struct {
		id     string
		status submission.Status
		extID  string
	}{
		{id: fmt.Sprintf("sub_e_wa_%d", suffix), status: submission.WrongAnswer, extID: "100"},
		{id: fmt.Sprintf("sub_e_ac_%d", suffix), status: submission.Accepted, extID: "101"},
	} {
		_, err := database.ExecContext(ctx, `
			INSERT INTO submissions (id, user_id, problem_id, contest_id, platform, language, source_code, status, external_submission_id, external_submitted_at, submitted_at, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11)
		`, sub.id, userE.ID, prob.ID, con.ID, prob.Platform, "cpp23", sub.id, sub.status, sub.extID, sameSecond, "{}")
		require.NoError(t, err)
	}

	standingsRes, err := subSvc.CalculateStandings(ctx, con.ID, owner.ID, true)
	require.NoError(t, err)

	scoresByUser := make(map[string]submission.ParticipantScore)
	for _, p := range standingsRes.Standings {
		scoresByUser[p.UserID] = p
	}

	// User A assertions
	scoreA := scoresByUser[userA.ID]
	assert.Equal(t, 0, scoreA.SolvedCount)
	assert.Equal(t, 0, scoreA.TotalPenalty)
	assert.False(t, scoreA.ProblemScores[prob.ID].Solved)
	assert.Equal(t, 0, scoreA.ProblemScores[prob.ID].Attempts, "CompileError and Failed must not count as attempts")
	assert.Equal(t, 0, scoreA.ProblemScores[prob.ID].PenaltyMinutes)

	// User B assertions
	scoreB := scoresByUser[userB.ID]
	assert.Equal(t, 1, scoreB.SolvedCount)
	assert.Equal(t, 20, scoreB.TotalPenalty)
	assert.True(t, scoreB.ProblemScores[prob.ID].Solved)
	assert.Equal(t, 1, scoreB.ProblemScores[prob.ID].Attempts, "CompileError and Failed before AC must not count as attempts")
	assert.Equal(t, 20, scoreB.ProblemScores[prob.ID].PenaltyMinutes)
	require.NotNil(t, scoreB.ProblemScores[prob.ID].FirstSolvedAtMinutes)
	assert.Equal(t, 20, *scoreB.ProblemScores[prob.ID].FirstSolvedAtMinutes)

	// User C assertions
	scoreC := scoresByUser[userC.ID]
	assert.Equal(t, 1, scoreC.SolvedCount)
	assert.Equal(t, 110, scoreC.TotalPenalty, "Penalty should be 30m + 20 * 4 (WA, RE, TL, ML)")
	assert.True(t, scoreC.ProblemScores[prob.ID].Solved)
	assert.Equal(t, 5, scoreC.ProblemScores[prob.ID].Attempts, "Attempts should be 4 penalty rejects + 1 AC")
	assert.Equal(t, 110, scoreC.ProblemScores[prob.ID].PenaltyMinutes)
	require.NotNil(t, scoreC.ProblemScores[prob.ID].FirstSolvedAtMinutes)
	assert.Equal(t, 30, *scoreC.ProblemScores[prob.ID].FirstSolvedAtMinutes)

	// User D assertions
	scoreD := scoresByUser[userD.ID]
	assert.Equal(t, 0, scoreD.SolvedCount)
	assert.Equal(t, 0, scoreD.TotalPenalty)
	assert.False(t, scoreD.ProblemScores[prob.ID].Solved)
	assert.Equal(t, 2, scoreD.ProblemScores[prob.ID].Attempts, "Only WA and RE should count as attempts; CE and Failed ignored")
	assert.Equal(t, 0, scoreD.ProblemScores[prob.ID].PenaltyMinutes)

	// User E's WA (ID 100) must be processed before the AC (ID 101), even
	// though both verified external timestamps are identical.
	scoreE := scoresByUser[userE.ID]
	assert.Equal(t, 1, scoreE.SolvedCount)
	assert.Equal(t, 60, scoreE.TotalPenalty)
	assert.Equal(t, 2, scoreE.ProblemScores[prob.ID].Attempts)
}

func TestScoreboardDeterministicOrderingAndTiedRanks(t *testing.T) {
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
	ownerEmail := fmt.Sprintf("tie_owner_%d@test.com", suffix)
	userAlphaEmail := fmt.Sprintf("tie_alpha_%d@test.com", suffix)
	userBetaEmail := fmt.Sprintf("tie_beta_%d@test.com", suffix)
	userGammaEmail := fmt.Sprintf("tie_gamma_%d@test.com", suffix)
	probExtID := fmt.Sprintf("tie_p_%d", suffix)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return
		}
		defer tx.Rollback()

		_, _ = tx.ExecContext(ctx, `DELETE FROM submissions WHERE user_id IN (SELECT id FROM users WHERE email IN ($1, $2, $3, $4))`, ownerEmail, userAlphaEmail, userBetaEmail, userGammaEmail)
		_, _ = tx.ExecContext(ctx, `DELETE FROM contests WHERE owner_id IN (SELECT id FROM users WHERE email = $1)`, ownerEmail)
		_, _ = tx.ExecContext(ctx, `DELETE FROM problems WHERE external_id = $1`, probExtID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM users WHERE email IN ($1, $2, $3, $4)`, ownerEmail, userAlphaEmail, userBetaEmail, userGammaEmail)
		_ = tx.Commit()
	})

	ctx := context.Background()
	authSvc := auth.NewService(database)
	platReg := platform.NewRegistry()
	probSvc := problem.NewService(database, platReg)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)
	subSvc := submission.NewService(database, contestSvc, probSvc, platReg)

	owner, _, err := authSvc.Register(ctx, ownerEmail, fmt.Sprintf("owner_%d", suffix), "password123")
	require.NoError(t, err)
	// Create usernames: "alpha_user", "beta_user", "gamma_user"
	userAlpha, _, err := authSvc.Register(ctx, userAlphaEmail, fmt.Sprintf("alpha_%d", suffix), "password123")
	require.NoError(t, err)
	userBeta, _, err := authSvc.Register(ctx, userBetaEmail, fmt.Sprintf("beta_%d", suffix), "password123")
	require.NoError(t, err)
	userGamma, _, err := authSvc.Register(ctx, userGammaEmail, fmt.Sprintf("gamma_%d", suffix), "password123")
	require.NoError(t, err)

	prob, err := probSvc.CreateCustom(ctx, problem.CreateCustomReq{
		Title:       "Tied Score Problem",
		Platform:    platform.Codeforces,
		ExternalID:  probExtID,
		Statement:   "Statement",
		TimeLimit:   "1.0s",
		MemoryLimit: "256MB",
	})
	require.NoError(t, err)

	now := time.Now().UTC()
	startAt := now.Add(-1 * time.Hour)
	endAt := now.Add(1 * time.Hour)

	con, err := contestSvc.Create(ctx, contest.CreateContestParams{
		OwnerID:           owner.ID,
		ProblemIDs:        []string{prob.ID},
		Name:              "Tied Score Contest",
		StartAt:           startAt,
		EndAt:             endAt,
		Visibility:        "PUBLIC",
		ScoringType:       contest.ICPC,
		PublicationStatus: contest.PublicationPublished,
	})
	require.NoError(t, err)

	require.NoError(t, contestSvc.Join(ctx, con.ID, userAlpha.ID))
	require.NoError(t, contestSvc.Join(ctx, con.ID, userBeta.ID))
	require.NoError(t, contestSvc.Join(ctx, con.ID, userGamma.ID))

	// Both Alpha and Beta solve at identical timestamp (10m from start) -> Solved: 1, Penalty: 10
	subTime := startAt.Add(10 * time.Minute)
	_, err = database.ExecContext(ctx, `
		INSERT INTO submissions (id, user_id, problem_id, contest_id, platform, language, source_code, status, submitted_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, fmt.Sprintf("sub_alpha_%d", suffix), userAlpha.ID, prob.ID, con.ID, prob.Platform, "cpp23", "code", submission.Accepted, subTime, "{}")
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `
		INSERT INTO submissions (id, user_id, problem_id, contest_id, platform, language, source_code, status, submitted_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, fmt.Sprintf("sub_beta_%d", suffix), userBeta.ID, prob.ID, con.ID, prob.Platform, "cpp23", "code", submission.Accepted, subTime, "{}")
	require.NoError(t, err)

	// Gamma and Owner have 0 solves, 0 penalty.

	// Perform multiple syncs (standings calculations) to verify absolute deterministic stability across calls
	var firstSyncUserIDs []string
	for i := 0; i < 20; i++ {
		res, err := subSvc.CalculateStandings(ctx, con.ID, owner.ID, true)
		require.NoError(t, err)
		require.Len(t, res.Standings, 4) // owner, alpha, beta, gamma

		// Check tied rank assignments
		// Alpha & Beta tied for 1st place -> both Rank 1
		assert.Equal(t, 1, res.Standings[0].Rank)
		assert.Equal(t, 1, res.Standings[0].SolvedCount)
		assert.Equal(t, 10, res.Standings[0].TotalPenalty)

		assert.Equal(t, 1, res.Standings[1].Rank)
		assert.Equal(t, 1, res.Standings[1].SolvedCount)
		assert.Equal(t, 10, res.Standings[1].TotalPenalty)

		// Alpha comes before Beta because "alpha_..." < "beta_..."
		assert.Equal(t, userAlpha.ID, res.Standings[0].UserID)
		assert.Equal(t, userBeta.ID, res.Standings[1].UserID)

		// Gamma & Owner tied with 0 solves -> both Rank 3
		assert.Equal(t, 3, res.Standings[2].Rank)
		assert.Equal(t, 0, res.Standings[2].SolvedCount)
		assert.Equal(t, 0, res.Standings[2].TotalPenalty)

		assert.Equal(t, 3, res.Standings[3].Rank)
		assert.Equal(t, 0, res.Standings[3].SolvedCount)
		assert.Equal(t, 0, res.Standings[3].TotalPenalty)

		currentUserIDs := []string{
			res.Standings[0].UserID,
			res.Standings[1].UserID,
			res.Standings[2].UserID,
			res.Standings[3].UserID,
		}
		if i == 0 {
			firstSyncUserIDs = currentUserIDs
		} else {
			assert.Equal(t, firstSyncUserIDs, currentUserIDs, "Scoreboard order must be identical across all syncs")
		}
	}
}

func TestSubmissionListPagination(t *testing.T) {
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
	userEmail := fmt.Sprintf("pag_user_%d@test.com", suffix)
	probExtID := fmt.Sprintf("pag_prob_%d", suffix)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		tx, err := database.BeginTx(ctx, nil)
		if err != nil {
			return
		}
		defer tx.Rollback()

		_, _ = tx.ExecContext(ctx, `DELETE FROM submissions WHERE user_id IN (SELECT id FROM users WHERE email = $1)`, userEmail)
		_, _ = tx.ExecContext(ctx, `DELETE FROM problems WHERE external_id = $1`, probExtID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM users WHERE email = $1`, userEmail)
		_ = tx.Commit()
	})

	ctx := context.Background()
	authSvc := auth.NewService(database)
	platReg := platform.NewRegistry()
	probSvc := problem.NewService(database, platReg)
	setSvc := problemset.NewService(database)
	contestSvc := contest.NewService(database, setSvc)
	subSvc := submission.NewService(database, contestSvc, probSvc, platReg)

	user, _, err := authSvc.Register(ctx, userEmail, fmt.Sprintf("paguser_%d", suffix), "password123")
	require.NoError(t, err)

	prb, err := probSvc.CreateCustom(ctx, problem.CreateCustomReq{
		Platform:   platform.Codeforces,
		ExternalID: probExtID,
		Title:      "Pagination Test Problem",
		URL:        fmt.Sprintf("https://codeforces.com/problemset/problem/%d/A", suffix),
		Statement:  "Solve it",
	})
	require.NoError(t, err)

	// Create 5 submissions (as admin to bypass contest requirement)
	var createdIDs []string
	for i := 1; i <= 5; i++ {
		sub, err := subSvc.Create(ctx, user.ID, true, prb.ID, nil, "cpp23", fmt.Sprintf("// solution variant %d", i))
		require.NoError(t, err)
		createdIDs = append([]string{sub.ID}, createdIDs...) // newest first
	}

	// Test page 1 with limit 2
	page1, err := subSvc.ListForViewer(ctx, user.ID, "", "", 2, 0, false)
	require.NoError(t, err)
	require.Len(t, page1, 2)
	assert.Equal(t, createdIDs[0], page1[0].ID)
	assert.Equal(t, createdIDs[1], page1[1].ID)

	// Test page 2 with limit 2 (offset 2)
	page2, err := subSvc.ListForViewer(ctx, user.ID, "", "", 2, 2, false)
	require.NoError(t, err)
	require.Len(t, page2, 2)
	assert.Equal(t, createdIDs[2], page2[0].ID)
	assert.Equal(t, createdIDs[3], page2[1].ID)

	// Test page 3 with limit 2 (offset 4) -> 1 remaining item
	page3, err := subSvc.ListForViewer(ctx, user.ID, "", "", 2, 4, false)
	require.NoError(t, err)
	require.Len(t, page3, 1)
	assert.Equal(t, createdIDs[4], page3[0].ID)
}
