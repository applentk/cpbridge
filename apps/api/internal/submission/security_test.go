package submission

import (
	"errors"
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
		SourceCode:        "int main() {}\n// cpbridge-dispatch-proof:test",
		SubmittedAt:       now,
	}
	status := &platform.SubmissionStatus{
		ExternalSubmissionID: "2048/123456",
		ProblemExternalID:    "2048/A",
		Language:             "GNU C++23 (64)",
		PlatformUsername:     "contestant",
		SourceCode:           "int main() {}\n// cpbridge-dispatch-proof:test",
		SubmittedAt:          timePtr(now.Add(5 * time.Second)),
	}

	require.NoError(t, validateExternalSubmissionMetadata(sub, "2048/123456", status, now))

	withoutSource := *status
	withoutSource.SourceCode = ""
	require.NoError(t, validateExternalSubmissionMetadata(sub, "2048/123456", &withoutSource, now))

	atCoderSub := *sub
	atCoderSub.Platform = platform.AtCoder
	require.Error(t, validateExternalSubmissionMetadata(&atCoderSub, "2048/123456", &withoutSource, now))

	tests := []struct {
		name   string
		change func(*platform.SubmissionStatus)
	}{
		{name: "different problem", change: func(value *platform.SubmissionStatus) { value.ProblemExternalID = "2048/B" }},
		{name: "different language", change: func(value *platform.SubmissionStatus) { value.Language = "Python 3" }},
		{name: "old timestamp", change: func(value *platform.SubmissionStatus) { value.SubmittedAt = timePtr(now.Add(-3 * time.Minute)) }},
		{name: "late timestamp", change: func(value *platform.SubmissionStatus) { value.SubmittedAt = timePtr(now.Add(3 * time.Minute)) }},
		{name: "missing identity", change: func(value *platform.SubmissionStatus) { value.PlatformUsername = "" }},
		{name: "different source", change: func(value *platform.SubmissionStatus) {
			value.SourceCode = "int main() {}\n// cpbridge-dispatch-proof:other"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			copy := *status
			tt.change(&copy)
			require.Error(t, validateExternalSubmissionMetadata(sub, "2048/123456", &copy, now))
		})
	}
}

func TestValidateExternalSubmissionMetadataClassifiesIncompleteResponses(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	sub := &Submission{
		ProblemExternalID: "2048/A",
		Platform:          platform.Codeforces,
		Language:          "cpp23",
		SourceCode:        "int main() {}\n// cpbridge-dispatch-proof:test",
		SubmittedAt:       now,
	}
	incomplete := &platform.SubmissionStatus{
		ExternalSubmissionID: "2048/123456",
		Status:               "JUDGING",
	}
	err := validateExternalSubmissionMetadata(sub, "2048/123456", incomplete, now)
	var verificationErr *VerificationError
	require.True(t, errors.As(err, &verificationErr))
	require.Equal(t, VerificationRetryable, verificationErr.Kind)

	wrongProblem := &platform.SubmissionStatus{
		ExternalSubmissionID: "2048/123456",
		ProblemExternalID:    "2048/B",
		Language:             "GNU C++23 (64)",
		PlatformUsername:     "contestant",
		SubmittedAt:          timePtr(now),
	}
	err = validateExternalSubmissionMetadata(sub, "2048/123456", wrongProblem, now)
	require.True(t, errors.As(err, &verificationErr))
	require.Equal(t, VerificationDefinitive, verificationErr.Kind)
}

func TestValidateExternalSubmissionMetadataKeepsGymProblemAndSubmissionScope(t *testing.T) {
	now := time.Date(2026, 8, 30, 7, 20, 0, 0, time.UTC)
	sub := &Submission{
		ProblemExternalID: "gym/106068/A",
		Platform:          platform.Codeforces,
		Language:          "java21",
		SourceCode:        "class Main {}\n// cpbridge-dispatch-proof:test",
		SubmittedAt:       now,
	}
	status := &platform.SubmissionStatus{
		ExternalSubmissionID: "gym/106068/388880843",
		ProblemExternalID:    "gym/106068/A",
		Language:             "Java 21",
		PlatformUsername:     "contestant",
		SubmittedAt:          timePtr(now.Add(5 * time.Second)),
	}

	require.NoError(t, validateExternalSubmissionMetadata(sub, "gym/106068/388880843", status, now))

	wrongProblem := *status
	wrongProblem.ProblemExternalID = "gym/106068/B"
	require.Error(t, validateExternalSubmissionMetadata(sub, "gym/106068/388880843", &wrongProblem, now))

	wrongSubmission := *status
	wrongSubmission.ExternalSubmissionID = "gym/106068/388880844"
	require.Error(t, validateExternalSubmissionMetadata(sub, "gym/106068/388880843", &wrongSubmission, now))
}

func TestWithDispatchProofAddsLanguageSafeUniqueMarker(t *testing.T) {
	cppSource, err := withDispatchProof("int main() {}", "cpp23")
	require.NoError(t, err)
	pythonSource, err := withDispatchProof("print('ok')", "python3")
	require.NoError(t, err)

	require.Contains(t, cppSource, "// cpbridge-dispatch-proof:")
	require.Contains(t, pythonSource, "# cpbridge-dispatch-proof:")
	if cppSource == pythonSource {
		t.Fatal("dispatch proofs must be unique")
	}
}

func TestValidateExternalSubmissionContestWindow(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	contestID := "con_test"
	sub := &Submission{ContestID: &contestID}

	require.NoError(t, validateExternalSubmissionContestWindow(sub, now, now.Add(-time.Hour), now.Add(time.Hour)))
	require.Error(t, validateExternalSubmissionContestWindow(sub, now.Add(-time.Nanosecond), now, now.Add(time.Hour)))
	require.Error(t, validateExternalSubmissionContestWindow(sub, now.Add(time.Hour), now.Add(-time.Hour), now.Add(time.Hour)))
}

func timePtr(value time.Time) *time.Time {
	return &value
}
