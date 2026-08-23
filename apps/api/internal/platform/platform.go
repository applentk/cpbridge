package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Type string

const (
	Codeforces Type = "CODEFORCES"
	AtCoder    Type = "ATCODER"
	LeetCode   Type = "LEETCODE"
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
}

type NormalizedProblem struct {
	Platform   Type                   `json:"platform"`
	ExternalID string                 `json:"externalId"`
	Title      string                 `json:"title"`
	URL        string                 `json:"url"`
	Difficulty *int                   `json:"difficulty"`
	Tags       []string               `json:"tags"`
	Metadata   map[string]interface{} `json:"metadata"`
}

type SubmissionStatus struct {
	ExternalSubmissionID string `json:"externalSubmissionId"`
	Status               string `json:"status"` // ACCEPTED, WRONG_ANSWER, JUDGING, etc.
	MemoryKB             *int   `json:"memoryKb,omitempty"`
	TimeMS               *int   `json:"timeMs,omitempty"`
}

type Platform interface {
	Type() Type
	MatchURL(rawURL string) (externalID string, matched bool)
	GetProblem(ctx context.Context, externalID string) (*NormalizedProblem, error)
	GetStatement(ctx context.Context, externalID string) (*ProblemStatement, error)
	GetSubmission(ctx context.Context, externalSubmissionID string) (*SubmissionStatus, error)
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
	p, ok := r.adapters[t]
	if !ok {
		return nil, fmt.Errorf("unsupported platform: %s", t)
	}
	return p, nil
}

func (r *Registry) ParseURL(rawURL string) (Type, string, Platform, error) {
	rawURL = strings.TrimSpace(rawURL)
	for pType, adapter := range r.adapters {
		if extID, ok := adapter.MatchURL(rawURL); ok {
			return pType, extID, adapter, nil
		}
	}
	return "", "", nil, errors.New("unrecognized problem URL or unsupported platform")
}
