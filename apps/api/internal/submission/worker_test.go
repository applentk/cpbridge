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
