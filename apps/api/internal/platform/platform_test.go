package platform_test

import (
	"testing"

	"github.com/cpbridge/api/internal/platform"
	"github.com/cpbridge/api/internal/platform/atcoder"
	"github.com/cpbridge/api/internal/platform/codeforces"
	"github.com/stretchr/testify/assert"
)

func TestPlatformURLMatching(t *testing.T) {
	reg := platform.NewRegistry()
	reg.Register(codeforces.New())
	reg.Register(atcoder.New())

	tests := []struct {
		url        string
		expectType platform.Type
		expectID   string
		expectErr  bool
	}{
		{
			url:        "https://codeforces.com/problemset/problem/1900/A",
			expectType: platform.Codeforces,
			expectID:   "1900/A",
			expectErr:  false,
		},
		{
			url:        "https://codeforces.com/contest/1800/problem/B",
			expectType: platform.Codeforces,
			expectID:   "1800/B",
			expectErr:  false,
		},
		{
			url:        "https://atcoder.jp/contests/abc350/tasks/abc350_a",
			expectType: platform.AtCoder,
			expectID:   "abc350/abc350_a",
			expectErr:  false,
		},
		{
			url:        "https://unknownplatform.org/problem/123",
			expectType: "",
			expectID:   "",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		pType, extID, _, err := reg.ParseURL(tt.url)
		if tt.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expectType, pType)
			assert.Equal(t, tt.expectID, extID)
		}
	}
}

func TestPlatformContestURLMatching(t *testing.T) {
	reg := platform.NewRegistry()
	reg.Register(codeforces.New())
	reg.Register(atcoder.New())

	tests := []struct {
		url        string
		expectType platform.Type
		expectID   string
		expectErr  bool
	}{
		{
			url:        "https://codeforces.com/contest/1931",
			expectType: platform.Codeforces,
			expectID:   "1931",
			expectErr:  false,
		},
		{
			url:        "https://codeforces.com/gym/105053",
			expectType: platform.Codeforces,
			expectID:   "gym/105053",
			expectErr:  false,
		},
		{
			url:        "https://atcoder.jp/contests/abc350",
			expectType: platform.AtCoder,
			expectID:   "abc350",
			expectErr:  false,
		},
		{
			url:        "https://atcoder.jp/contests/abc350/tasks",
			expectType: platform.AtCoder,
			expectID:   "abc350",
			expectErr:  false,
		},
		{
			url:        "/gym/105053",
			expectType: "",
			expectID:   "",
			expectErr:  true,
		},
		{
			url:        "1931",
			expectType: "",
			expectID:   "",
			expectErr:  true,
		},
		{
			url:        "abc350",
			expectType: "",
			expectID:   "",
			expectErr:  true,
		},
		{
			url:        "https://unknownplatform.org/contest/123",
			expectType: "",
			expectID:   "",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		pType, extID, _, err := reg.ParseContestURL(tt.url)
		if tt.expectErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expectType, pType)
			assert.Equal(t, tt.expectID, extID)
		}
	}
}

func TestMockSubmissionIDsReturnFailed(t *testing.T) {
	cf := codeforces.New()
	cfStatus, err := cf.GetSubmission(t.Context(), "cf_123456789")
	assert.NoError(t, err)
	assert.NotNil(t, cfStatus)
	assert.Equal(t, "FAILED", cfStatus.Status)

	ac := atcoder.New()
	acStatus, err := ac.GetSubmission(t.Context(), "ac_123456789")
	assert.NoError(t, err)
	assert.NotNil(t, acStatus)
	assert.Equal(t, "FAILED", acStatus.Status)
}
