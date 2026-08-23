package atcoder

import "testing"

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
