package submission

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNeedsExternalSubmissionVerification(t *testing.T) {
	assert.True(t, needsExternalSubmissionVerification(sql.NullTime{}))
	assert.False(t, needsExternalSubmissionVerification(sql.NullTime{
		Time:  time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}))
}

func TestExternalSubmissionRefPreservesGymContest(t *testing.T) {
	assert.Equal(t, "2048/123456789", externalSubmissionRef("2048/A", "123456789"))
	assert.Equal(t, "gym/105053/987654321", externalSubmissionRef("gym/105053/A", "987654321"))
}
