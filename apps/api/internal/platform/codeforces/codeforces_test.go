package codeforces

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

func TestExtractNote(t *testing.T) {
	statement := `<div class="statement"><p>Problem body.</p></div>
<div class="note"><div class="section-title">Note</div><p>The answer is explained here.</p><div class="extra"><p>More detail.</p></div></div>
<div class="after-note"><p>After the note.</p></div>`

	note, withoutNote := extractNote(statement)
	if !strings.Contains(note, "The answer is explained here.") || !strings.Contains(note, "More detail.") {
		t.Fatalf("extractNote() note = %q, want note content", note)
	}
	if strings.Contains(note, "section-title") || strings.Contains(withoutNote, "The answer is explained here.") {
		t.Fatalf("extractNote() did not separate the note: note=%q statement=%q", note, withoutNote)
	}
	if !strings.Contains(withoutNote, "Problem body.") || !strings.Contains(withoutNote, "After the note.") {
		t.Fatalf("extractNote() removed non-note statement content: %q", withoutNote)
	}
}
