package submission

import (
	"testing"

	"github.com/cpbridge/api/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachAdminSourceURL(t *testing.T) {
	externalID := "123456789"
	sub := Submission{
		Platform:             platform.Codeforces,
		ProblemExternalID:    "2048/A",
		ExternalSubmissionID: &externalID,
	}

	attachAdminSourceURL(&sub, false)
	assert.Nil(t, sub.SourceURL, "regular users must not receive the external source URL")

	attachAdminSourceURL(&sub, true)
	require.NotNil(t, sub.SourceURL)
	assert.Equal(t, "https://codeforces.com/contest/2048/submission/123456789", *sub.SourceURL)
}

func TestAttachAdminSourceURLUsesSubmissionPartOfCompoundID(t *testing.T) {
	externalID := "abc300/987654"
	sub := Submission{
		Platform:             platform.AtCoder,
		ProblemExternalID:    "abc300/a",
		ExternalSubmissionID: &externalID,
	}

	attachAdminSourceURL(&sub, true)
	require.NotNil(t, sub.SourceURL)
	assert.Equal(t, "https://atcoder.jp/contests/abc300/submissions/987654", *sub.SourceURL)
}
