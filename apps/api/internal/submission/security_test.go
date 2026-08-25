package submission

import (
	"testing"
	"time"

	"github.com/cpbridge/api/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestValidateExternalSubmissionMetadata(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	sub := &Submission{
		ProblemExternalID: "2048/A",
		Platform:          platform.Codeforces,
		Language:          "cpp23",
		SubmittedAt:       now,
	}
	status := &platform.SubmissionStatus{
		ExternalSubmissionID: "2048/123456",
		ProblemExternalID:    "2048/A",
		Language:             "GNU C++23 (64)",
		PlatformUsername:     "contestant",
		SubmittedAt:          timePtr(now.Add(5 * time.Second)),
	}

	require.NoError(t, validateExternalSubmissionMetadata(sub, "2048/123456", status, now))

	tests := []struct {
		name   string
		change func(*platform.SubmissionStatus)
	}{
		{name: "different problem", change: func(value *platform.SubmissionStatus) { value.ProblemExternalID = "2048/B" }},
		{name: "different language", change: func(value *platform.SubmissionStatus) { value.Language = "Python 3" }},
		{name: "old timestamp", change: func(value *platform.SubmissionStatus) { value.SubmittedAt = timePtr(now.Add(-3 * time.Minute)) }},
		{name: "missing identity", change: func(value *platform.SubmissionStatus) { value.PlatformUsername = "" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := *status
			tt.change(&copy)
			require.Error(t, validateExternalSubmissionMetadata(sub, "2048/123456", &copy, now))
		})
	}
}

func timePtr(value time.Time) *time.Time {
	return &value
}
