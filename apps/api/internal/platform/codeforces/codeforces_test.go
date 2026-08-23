package codeforces

import (
	"testing"
)

func TestNormalizeProblemTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "removes dot label", input: "A. Bear and Prime 100", want: "Bear and Prime 100"},
		{name: "removes hyphen label", input: "B - Bear and Prime 100", want: "Bear and Prime 100"},
		{name: "keeps unprefixed title", input: "Bear and Prime 100", want: "Bear and Prime 100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeProblemTitle(tt.input); got != tt.want {
				t.Fatalf("normalizeProblemTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFetchProblemTitle(t *testing.T) {
	title, ok := extractProblemTitle(`<html><title>2071B - B. Perfecto - Codeforces</title><div class="problem-statement"><div class="title">B. Perfecto</div></div></html>`)
	if !ok {
		t.Fatal("fetchProblemTitle() did not find the title")
	}
	if title != "Perfecto" {
		t.Fatalf("fetchProblemTitle() = %q, want %q", title, "Perfecto")
	}
}
