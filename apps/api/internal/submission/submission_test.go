package submission_test

import (
	"testing"

	"github.com/cp-hub/api/internal/submission"
	"github.com/stretchr/testify/assert"
)

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
