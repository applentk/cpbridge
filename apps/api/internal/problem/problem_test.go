package problem

import (
	"testing"

	"github.com/cpbridge/api/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestSnapshotStatementRoundTrip(t *testing.T) {
	normalized := &platform.NormalizedProblem{
		Metadata: map[string]any{"contestId": "100"},
	}
	statement := &platform.ProblemStatement{
		HTML:        "<div>Statement</div>",
		TimeLimit:   "2 seconds",
		MemoryLimit: "256 megabytes",
		SampleCases: []platform.SampleCase{{Input: "1", Output: "2", Explanation: "add one"}},
		Note:        "<p>Note</p>",
	}

	require.NoError(t, SnapshotStatement(normalized, statement))
	got, err := problemStatementFromMetadata(normalized.Metadata)
	require.NoError(t, err)
	require.Equal(t, statement.HTML, got.HTML)
	require.Equal(t, statement.TimeLimit, got.TimeLimit)
	require.Equal(t, statement.MemoryLimit, got.MemoryLimit)
	require.Equal(t, statement.SampleCases, got.SampleCases)
	require.Equal(t, statement.Note, got.Note)
	require.Equal(t, true, normalized.Metadata["statementSnapshot"])
}

func TestProblemStatementFromMetadataRequiresSnapshot(t *testing.T) {
	got, err := problemStatementFromMetadata(map[string]any{"contestId": "100"})
	require.NoError(t, err)
	require.Nil(t, got)
}
