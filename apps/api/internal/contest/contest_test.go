package contest_test

import (
	"testing"
	"time"

	"github.com/cpbridge/api/internal/contest"
	"github.com/stretchr/testify/assert"
)

func TestContestStateCalculation(t *testing.T) {
	startAt := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	endAt := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	// 1. Before start
	beforeTime := time.Date(2026, 8, 23, 9, 59, 59, 0, time.UTC)
	assert.Equal(t, contest.Upcoming, contest.CalculateState(beforeTime, startAt, endAt))

	// 2. Exactly at start
	atStart := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, contest.Active, contest.CalculateState(atStart, startAt, endAt))

	// 3. During contest
	duringTime := time.Date(2026, 8, 23, 11, 0, 0, 0, time.UTC)
	assert.Equal(t, contest.Active, contest.CalculateState(duringTime, startAt, endAt))

	// 4. Exactly at end
	atEnd := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	assert.Equal(t, contest.Finished, contest.CalculateState(atEnd, startAt, endAt))

	// 5. After end
	afterTime := time.Date(2026, 8, 23, 12, 30, 0, 0, time.UTC)
	assert.Equal(t, contest.Finished, contest.CalculateState(afterTime, startAt, endAt))
}

func TestGenerateLabel(t *testing.T) {
	assert.Equal(t, "A", contest.GenerateLabel(0))
	assert.Equal(t, "B", contest.GenerateLabel(1))
	assert.Equal(t, "Z", contest.GenerateLabel(25))
	assert.Equal(t, "P27", contest.GenerateLabel(26))
}
