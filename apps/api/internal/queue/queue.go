package queue

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypePollSubmissionVerdict = "submission:poll_verdict"
	QueueSubmissions          = "submissions"
	PollInterval              = 5 * time.Second
	PollRequestLease          = time.Minute
	MaxPollRetries            = 200
)

type PollVerdictPayload struct {
	SubmissionID         string `json:"submissionId"`
	ExternalSubmissionID string `json:"externalSubmissionId"`
	Platform             string `json:"platform"`
	ProblemID            string `json:"problemId"`
	RequestID            string `json:"requestId,omitempty"`
}

// NewPollVerdictTask constructs an asynq.Task to poll the status of an external submission.
func NewPollVerdictTask(submissionID, externalSubmissionID, platform, problemID string) (*asynq.Task, error) {
	return NewPollVerdictTaskAfter(submissionID, externalSubmissionID, platform, problemID, "", 3*time.Second)
}

// NewPollVerdictTaskAfter constructs a poll task with an idempotency token and
// an explicit delay. The token is backed by the submission row, so repeated
// sync requests coalesce before they reach Redis or the external platform.
func NewPollVerdictTaskAfter(submissionID, externalSubmissionID, platform, problemID, requestID string, delay time.Duration) (*asynq.Task, error) {
	payload, err := json.Marshal(PollVerdictPayload{
		SubmissionID:         submissionID,
		ExternalSubmissionID: externalSubmissionID,
		Platform:             platform,
		ProblemID:            problemID,
		RequestID:            requestID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(
		TypePollSubmissionVerdict,
		payload,
		asynq.Queue(QueueSubmissions),
		asynq.MaxRetry(MaxPollRetries),
		asynq.Timeout(25*time.Second),
		asynq.Retention(1*time.Hour),
		asynq.ProcessIn(delay),
	), nil
}

// CustomRetryDelay provides a bounded backoff for actual platform/network
// failures. Normal JUDGING responses are scheduled explicitly by the worker.
func CustomRetryDelay(n int, e error, t *asynq.Task) time.Duration {
	if n < 5 {
		return 3 * time.Second
	} else if n < 20 {
		return 4 * time.Second
	}
	return 5 * time.Second
}
