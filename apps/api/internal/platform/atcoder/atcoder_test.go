package atcoder

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		{name: "current AtCoder page title", input: "A - Product", want: "Product"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeProblemTitle(tt.input); got != tt.want {
				t.Fatalf("normalizeProblemTitle(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMatchContestURL(t *testing.T) {
	adapter := New()
	for _, input := range []string{"https://atcoder.jp/contests/abc350", "https://atcoder.jp/contests/abc350/tasks", "atcoder.jp/contests/abc350"} {
		if got, ok := adapter.MatchContestURL(input); !ok || got != "abc350" {
			t.Fatalf("MatchContestURL(%q) = %q, %v; want abc350, true", input, got, ok)
		}
	}
	for _, input := range []string{"abc350", "/contests/abc350", "not-a-url"} {
		if got, ok := adapter.MatchContestURL(input); ok {
			t.Fatalf("MatchContestURL(%q) = %q, true; want no match", input, got)
		}
	}
}

func TestGetContestParsesOrderedTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contests/abc350/tasks" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`<html><body><span class="h2">AtCoder Beginner Contest 350</span>
		<table><tbody>
		<tr><td><a href="/contests/abc350/tasks/abc350_a">A</a></td><td><a href="/contests/abc350/tasks/abc350_a">Past ABCs</a></td></tr>
		<tr><td><a href="/contests/abc350/tasks/abc350_b">B</a></td><td><a href="/contests/abc350/tasks/abc350_b">Dentist Aoki</a></td></tr>
		</tbody></table></body></html>`))
	}))
	defer server.Close()

	adapter := &Adapter{client: server.Client(), baseURL: server.URL}
	snapshot, err := adapter.GetContest(context.Background(), "abc350")
	if err != nil {
		t.Fatalf("GetContest() error = %v", err)
	}
	if snapshot.Name != "AtCoder Beginner Contest 350" || len(snapshot.Problems) != 2 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.Problems[0].ExternalID != "abc350/abc350_a" || snapshot.Problems[1].Title != "Dentist Aoki" {
		t.Fatalf("task order or metadata is wrong: %+v", snapshot.Problems)
	}
}

func TestExtractTaskStatementUsesEnglishAndStopsAtTaskBoundary(t *testing.T) {
	page := `<main><div id="task-statement">
<span class="lang"><span class="lang-ja"><p>日本語の問題文</p></span>
<span class="lang-en"><p>Score : <var>100</var> points</p><div class="part"><section><h3>Problem Statement</h3><p>Solve this in English.</p></section></div>
<hr /><div class="part"><section><h3>Sample Input 1</h3><pre>1 2</pre></section></div>
<div class="part"><section><h3>Sample Output 1</h3><pre>3</pre></section></div></span></span>
</div></main><footer>Unrelated page footer</footer>`

	statement := extractTaskStatement(page)
	if strings.Contains(statement, "日本語") || strings.Contains(statement, "Unrelated page footer") {
		t.Fatalf("extractTaskStatement() returned content outside the English task statement: %q", statement)
	}

	statement = removeSampleSections(cleanStatementHTML(statement))
	if strings.Contains(statement, "Score") || strings.Contains(statement, "Problem Statement") || strings.Contains(statement, "Sample Input") {
		t.Fatalf("statement cleanup kept metadata or samples: %q", statement)
	}
	if !strings.Contains(statement, "Solve this in English.") {
		t.Fatalf("statement cleanup removed the English problem body: %q", statement)
	}
}

func TestFetchTaskPageRejectsNonSuccessfulResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := New().fetchTaskPage(context.Background(), server.URL)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("fetchTaskPage() error = %v, want an HTTP-status error", err)
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
