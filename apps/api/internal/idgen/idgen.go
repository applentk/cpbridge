package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Prefix constants for application IDs
const (
	PrefixUser       = "usr_"
	PrefixProblem    = "prb_"
	PrefixProblemSet = "set_"
	PrefixContest    = "con_"
	PrefixSubmission = "sub_"
	PrefixSession    = "ses_"
)

// New generates a time-sortable opaque ID with the given prefix.
func New(prefix string) string {
	// Attempt UUIDv7 first
	id, err := uuid.NewV7()
	if err == nil {
		clean := strings.ReplaceAll(id.String(), "-", "")
		return fmt.Sprintf("%s%s", prefix, clean)
	}

	// Fallback to timestamp + random hex
	bytes := make([]byte, 8)
	_, _ = rand.Read(bytes)
	return fmt.Sprintf("%s%x%s", prefix, time.Now().UTC().UnixNano(), hex.EncodeToString(bytes))
}
