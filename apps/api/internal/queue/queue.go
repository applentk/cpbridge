package queue

import (
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TypePollSubmissionVerdict = "submission:poll_verdict"
	QueueSubmissions          = "submissions"
)

type PollVerdictPayload struct {
	SubmissionID         string `json:"submissionId"`
	ExternalSubmissionID string `json:"externalSubmissionId"`
	Platform             string `json:"platform"`
	ProblemID            string `json:"problemId"`
}

// NewPollVerdictTask constructs an asynq.Task to poll the status of an external submission.
func NewPollVerdictTask(submissionID, externalSubmissionID, platform, problemID string) (*asynq.Task, error) {
	payload, err := json.Marshal(PollVerdictPayload{
		SubmissionID:         submissionID,
		ExternalSubmissionID: externalSubmissionID,
		Platform:             platform,
		ProblemID:            problemID,
	})
	if err != nil {
		return nil, err
	}

	return asynq.NewTask(
		TypePollSubmissionVerdict,
		payload,
		asynq.Queue(QueueSubmissions),
		asynq.MaxRetry(40),
		asynq.Timeout(25*time.Second),
		asynq.Retention(1*time.Hour),
		asynq.ProcessIn(3*time.Second),
	), nil
}

// CustomRetryDelay provides a short backoff between polling attempts (3-6 seconds).
func CustomRetryDelay(n int, e error, t *asynq.Task) time.Duration {
	if n < 5 {
		return 3 * time.Second
	} else if n < 15 {
		return 4 * time.Second
	}
	return 6 * time.Second
}
