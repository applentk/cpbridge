package platform_test

import (
	"testing"

	"github.com/cp-hub/api/internal/platform"
	"github.com/cp-hub/api/internal/platform/atcoder"
	"github.com/cp-hub/api/internal/platform/codeforces"
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
