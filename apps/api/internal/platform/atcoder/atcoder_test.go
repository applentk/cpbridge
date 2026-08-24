package atcoder

import (
	"strings"
	"testing"
)

func TestNormalizeProblemTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "removes task label and contest suffix",
			input: "A - A+B Problem - AtCoder Beginner Contest 086 | AtCoder",
			want:  "A+B Problem",
		},
		{name: "removes task label", input: "B: Some Task", want: "Some Task"},
		{name: "decodes entities", input: "C - A &amp; B - AtCoder Regular Contest 001 | AtCoder", want: "A & B"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeProblemTitle(tt.input); got != tt.want {
				t.Fatalf("normalizeProblemTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestCleanStatementHTML(t *testing.T) {
	input := `<div class="part"><section><h3>Score : 100 points</h3><h3>Problem Statement</h3><p>Solve the problem.</p></section></div>`
	got := cleanStatementHTML(input)

	if strings.Contains(got, "Score : 100 points") {
		t.Fatalf("cleanStatementHTML() kept score heading: %q", got)
	}
	if strings.Contains(got, "Problem Statement") {
		t.Fatalf("cleanStatementHTML() kept problem statement heading: %q", got)
	}
	if !strings.Contains(got, "Solve the problem.") {
		t.Fatalf("cleanStatementHTML() removed statement content: %q", got)
	}
}

func TestCleanStatementHTMLRemovesScoreParagraphWithMath(t *testing.T) {
	input := `<p>Score : <span class="katex"><span class="katex-html">300</span></span> points</p><p>Solve the problem.</p>`
	got := cleanStatementHTML(input)

	if strings.Contains(got, "Score") || strings.Contains(got, "katex-html") {
		t.Fatalf("cleanStatementHTML() kept score paragraph: %q", got)
	}
	if !strings.Contains(got, "Solve the problem.") {
		t.Fatalf("cleanStatementHTML() removed statement content: %q", got)
	}
}

func TestCleanStatementTextHeadings(t *testing.T) {
	input := "Score : 100 points\n### Problem Statement\n\nSolve the problem."
	got := cleanStatementHTML(input)

	if got != "Solve the problem." {
		t.Fatalf("cleanStatementHTML(%q) = %q, want %q", input, got, "Solve the problem.")
	}
}
