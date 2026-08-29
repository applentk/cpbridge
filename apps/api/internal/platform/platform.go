package platform

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Type string

const (
	Codeforces Type = "CODEFORCES"
	AtCoder    Type = "ATCODER"
)

type SampleCase struct {
	Input       string `json:"input"`
	Output      string `json:"output"`
	Explanation string `json:"explanation,omitempty"`
}

type ProblemStatement struct {
	HTML        string       `json:"html"`
	TimeLimit   string       `json:"timeLimit,omitempty"`
	MemoryLimit string       `json:"memoryLimit,omitempty"`
	SampleCases []SampleCase `json:"sampleCases"`
	Note        string       `json:"note,omitempty"`
}

type NormalizedProblem struct {
	Platform   Type           `json:"platform"`
	ExternalID string         `json:"externalId"`
	Title      string         `json:"title"`
	URL        string         `json:"url"`
	Difficulty *int           `json:"difficulty"`
	Tags       []string       `json:"tags"`
	Metadata   map[string]any `json:"metadata"`
}

// ContestSnapshot is a public external contest normalized for importing into a
// cpbridge problem set. Problems remain ordered exactly as the source exposes
// them so labels such as A1/A2 are not accidentally reordered.
type ContestSnapshot struct {
	Platform   Type                `json:"platform"`
	ExternalID string              `json:"externalId"`
	Name       string              `json:"name"`
	URL        string              `json:"url"`
	Phase      string              `json:"phase"`
	Problems   []NormalizedProblem `json:"problems"`
}

type SubmissionStatus struct {
	ExternalSubmissionID string         `json:"externalSubmissionId"`
	Status               string         `json:"status"` // PENDING, JUDGING, ACCEPTED, WRONG_ANSWER, TIME_LIMIT, COMPILATION_ERROR, RUNTIME_ERROR, MEMORY_LIMIT
	ProblemExternalID    string         `json:"problemExternalId,omitempty"`
	Language             string         `json:"language,omitempty"`
	PlatformUsername     string         `json:"platformUsername,omitempty"`
	SourceCode           string         `json:"sourceCode,omitempty"`
	SubmittedAt          *time.Time     `json:"submittedAt,omitempty"`
	ExecutionTimeMs      *int           `json:"executionTimeMs,omitempty"`
	MemoryBytes          *int64         `json:"memoryBytes,omitempty"`
	FailedTestcase       *int           `json:"failedTestcase,omitempty"`
	CompilerOutput       string         `json:"compilerOutput,omitempty"`
	RawPayload           map[string]any `json:"rawPayload,omitempty"`
}

type Platform interface {
	Type() Type
	MatchURL(rawURL string) (externalID string, matched bool)
	GetProblem(ctx context.Context, externalID string) (*NormalizedProblem, error)
	GetStatement(ctx context.Context, externalID string) (*ProblemStatement, error)
	GetSubmission(ctx context.Context, externalSubmissionID string) (*SubmissionStatus, error)
}

// ContestProvider is an optional platform capability. Keeping it separate
// avoids requiring platforms without contest import support to implement it.
type ContestProvider interface {
	MatchContestURL(rawURL string) (externalID string, matched bool)
	GetContest(ctx context.Context, externalID string) (*ContestSnapshot, error)
}

type Registry struct {
	adapters map[Type]Platform
}

func NewRegistry() *Registry {
	return &Registry{
		adapters: make(map[Type]Platform),
	}
}

func (r *Registry) Register(p Platform) {
	r.adapters[p.Type()] = p
}

func (r *Registry) Get(t Type) (Platform, error) {
	adapter, exists := r.adapters[t]
	if !exists {
		return nil, fmt.Errorf("unsupported platform: %s", t)
	}
	return adapter, nil
}

func (r *Registry) ParseURL(rawURL string) (Type, string, Platform, error) {
	for pType, adapter := range r.adapters {
		if extID, matched := adapter.MatchURL(rawURL); matched {
			return pType, extID, adapter, nil
		}
	}
	return "", "", nil, errors.New("unrecognized problem url: must be a supported Codeforces or AtCoder problem link")
}

func (r *Registry) ParseContestURL(rawURL string) (Type, string, ContestProvider, error) {
	for pType, adapter := range r.adapters {
		provider, ok := adapter.(ContestProvider)
		if !ok {
			continue
		}
		if extID, matched := provider.MatchContestURL(rawURL); matched {
			return pType, extID, provider, nil
		}
	}
	return "", "", nil, errors.New("unrecognized contest url: must be a supported public contest link")
}
