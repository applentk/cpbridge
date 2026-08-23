package platform_test

import (
	"testing"

	"github.com/cp-hub/api/internal/platform"
	"github.com/cp-hub/api/internal/platform/atcoder"
	"github.com/cp-hub/api/internal/platform/codeforces"
	"github.com/cp-hub/api/internal/platform/leetcode"
	"github.com/stretchr/testify/assert"
)

func TestURLParsing(t *testing.T) {
	registry := platform.NewRegistry()
	registry.Register(codeforces.New())
	registry.Register(atcoder.New())
	registry.Register(leetcode.New())

	tests := []struct {
		url          string
		expectedPlat platform.Type
		expectedExt  string
		shouldError  bool
	}{
		{
			url:          "https://codeforces.com/problemset/problem/1900/A",
			expectedPlat: platform.Codeforces,
			expectedExt:  "1900/A",
		},
		{
			url:          "https://codeforces.com/contest/1800/problem/E2",
			expectedPlat: platform.Codeforces,
			expectedExt:  "1800/E2",
		},
		{
			url:          "https://atcoder.jp/contests/abc350/tasks/abc350_f",
			expectedPlat: platform.AtCoder,
			expectedExt:  "abc350/abc350_f",
		},
		{
			url:          "https://leetcode.com/problems/two-sum/",
			expectedPlat: platform.LeetCode,
			expectedExt:  "two-sum",
		},
		{
			url:          "https://leetcode.com/problems/longest-substring-without-repeating-characters/description/",
			expectedPlat: platform.LeetCode,
			expectedExt:  "longest-substring-without-repeating-characters",
		},
		{
			url:         "https://unknownplatform.com/problem/123",
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			pType, extID, _, err := registry.ParseURL(tt.url)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPlat, pType)
				assert.Equal(t, tt.expectedExt, extID)
			}
		})
	}
}
