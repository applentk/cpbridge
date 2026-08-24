package queue_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cpbridge/api/internal/queue"
	"github.com/stretchr/testify/assert"
)

func TestNewPollVerdictTask(t *testing.T) {
	task, err := queue.NewPollVerdictTask("sub_123", "2000/A", "CODEFORCES", "prb_456")
	assert.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, queue.TypePollSubmissionVerdict, task.Type())

	var p queue.PollVerdictPayload
	err = json.Unmarshal(task.Payload(), &p)
	assert.NoError(t, err)
	assert.Equal(t, "sub_123", p.SubmissionID)
	assert.Equal(t, "2000/A", p.ExternalSubmissionID)
	assert.Equal(t, "CODEFORCES", p.Platform)
	assert.Equal(t, "prb_456", p.ProblemID)
}

func TestCustomRetryDelay(t *testing.T) {
	task, _ := queue.NewPollVerdictTask("sub_1", "1/A", "CODEFORCES", "prb_1")

	assert.Equal(t, 3*time.Second, queue.CustomRetryDelay(1, nil, task))
	assert.Equal(t, 3*time.Second, queue.CustomRetryDelay(4, nil, task))
	assert.Equal(t, 4*time.Second, queue.CustomRetryDelay(5, nil, task))
	assert.Equal(t, 4*time.Second, queue.CustomRetryDelay(14, nil, task))
	assert.Equal(t, 6*time.Second, queue.CustomRetryDelay(15, nil, task))
}
