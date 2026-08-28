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

func TestExtractDivContentByClassPreservesNestedStatementContent(t *testing.T) {
	page := `<html><body><div class="problem-statement">
<div class="header"><div class="title">C. Round Corridor</div></div>
<div><p>First paragraph.</p><center><img src="https://espresso.codeforces.com/diagram.png"></center><p>Text after image.</p></div>
<div class="input-specification"><div class="section-title">Input</div><p>Input details.</p></div>
</div><!-- end problem statement --><footer>Codeforces</footer></body></html>`

	statement, ok := extractDivContentByClass(page, "problem-statement")
	if !ok {
		t.Fatal("extractDivContentByClass() did not find the statement")
	}
	for _, want := range []string{"diagram.png", "Text after image.", "Input details."} {
		if !strings.Contains(statement, want) {
			t.Fatalf("extractDivContentByClass() omitted %q: %q", want, statement)
		}
	}
	if strings.Contains(statement, "<footer>") {
		t.Fatalf("extractDivContentByClass() included content after the statement: %q", statement)
	}
}
