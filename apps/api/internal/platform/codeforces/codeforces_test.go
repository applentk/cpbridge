package codeforces

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestMatchContestURL(t *testing.T) {
	adapter := New()
	for _, input := range []string{"https://codeforces.com/contest/1931", "codeforces.com/contest/1931/", "http://codeforces.com/contest/1931"} {
		got, ok := adapter.MatchContestURL(input)
		if !ok || got != "1931" {
			t.Fatalf("MatchContestURL(%q) = %q, %v; want 1931, true", input, got, ok)
		}
	}
	if got, ok := adapter.MatchContestURL("https://codeforces.com/gym/105053"); !ok || got != "gym/105053" {
		t.Fatalf("MatchContestURL(gym) = %q, %v; want gym/105053, true", got, ok)
	}
	for _, input := range []string{"1931", "/gym/105053", "gym/105053", "https://codeforces.com/group/example/contest/1931", "not-a-contest"} {
		if got, ok := adapter.MatchContestURL(input); ok {
			t.Fatalf("MatchContestURL(%q) = %q, true; want no match", input, got)
		}
	}
}

func TestMatchURLKeepsGymSource(t *testing.T) {
	adapter := New()
	got, ok := adapter.MatchURL("https://codeforces.com/gym/105053/problem/A")
	if !ok || got != "gym/105053/A" {
		t.Fatalf("MatchURL(gym problem) = %q, %v; want gym/105053/A, true", got, ok)
	}
}

func TestParseGymSubmissionRef(t *testing.T) {
	contestID, submissionID, isGym := parseSubmissionRef("gym/105053/987654321")
	if contestID != "105053" || submissionID != "987654321" || !isGym {
		t.Fatalf("parseSubmissionRef(gym) = %q, %q, %v", contestID, submissionID, isGym)
	}
	urls := codeforcesSubmissionURLs(contestID, submissionID, isGym)
	if len(urls) != 1 || urls[0] != "https://codeforces.com/gym/105053/submission/987654321" {
		t.Fatalf("codeforcesSubmissionURLs(gym) = %v", urls)
	}

	contestID, submissionID, isGym = parseSubmissionRef("gym/987654321")
	if contestID != "" || submissionID != "987654321" || !isGym {
		t.Fatalf("parseSubmissionRef(legacy gym) = %q, %q, %v", contestID, submissionID, isGym)
	}
}

func TestGetGymSubmissionUsesRecentStatusAPI(t *testing.T) {
	const responseBody = `{"status":"OK","result":[{"id":388880843,"contestId":106068,"creationTimeSeconds":1788074369,"problem":{"contestId":106068,"index":"A"},"author":{"members":[{"handle":"applentk"}]},"programmingLanguage":"Java 21","verdict":"OK","passedTestCount":11,"timeConsumedMillis":218,"memoryConsumedBytes":409600}]}`
	var requestedAPIPath string
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasPrefix(req.URL.Path, "/api/") {
			requestedAPIPath = req.URL.Path
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(responseBody)),
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("challenge")),
			Request:    req,
		}, nil
	})}
	adapter := &Adapter{client: client, baseURL: "https://codeforces.com"}

	status, err := adapter.GetSubmission(context.Background(), "gym/106068/388880843")
	if err != nil {
		t.Fatalf("GetSubmission(gym) error = %v", err)
	}
	if requestedAPIPath != "/api/problemset.recentStatus" {
		t.Fatalf("Gym lookup API path = %q", requestedAPIPath)
	}
	if status.Status != "ACCEPTED" || status.ExternalSubmissionID != "gym/106068/388880843" || status.ProblemExternalID != "gym/106068/A" {
		t.Fatalf("unexpected Gym submission status: %+v", status)
	}
	if status.PlatformUsername != "applentk" || status.Language != "Java 21" || status.SubmittedAt == nil {
		t.Fatalf("incomplete Gym verification metadata: %+v", status)
	}

	recovered, err := adapter.GetSubmission(context.Background(), "gym/388880843")
	if err != nil {
		t.Fatalf("GetSubmission(legacy gym) error = %v", err)
	}
	if recovered.ExternalSubmissionID != "gym/106068/388880843" || recovered.Status != "ACCEPTED" {
		t.Fatalf("legacy Gym lookup was not recovered: %+v", recovered)
	}
}

func TestGetGymContestFromPublicPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gym/105053" {
			t.Fatalf("request path = %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`<html><body>
			<a href="/gym/105053">The 2024 ICPC Latin America Championship</a>
			<table><tr><td><a href="/gym/105053/problem/A">A</a></td><td><a href="/gym/105053/problem/A">Almost Aligned</a></td></tr>
			<tr><td><a href="/gym/105053/problem/B">B</a></td><td><a href="/gym/105053/problem/B">Beating the Record</a></td></tr></table>
		</body></html>`))
	}))
	defer server.Close()

	adapter := &Adapter{client: server.Client(), baseURL: server.URL}
	snapshot, err := adapter.GetContest(context.Background(), "gym/105053")
	if err != nil {
		t.Fatalf("GetContest(gym) error = %v", err)
	}
	if snapshot.Name != "The 2024 ICPC Latin America Championship" || len(snapshot.Problems) != 2 {
		t.Fatalf("unexpected gym snapshot: %+v", snapshot)
	}
	if snapshot.Problems[0].ExternalID != "gym/105053/A" || snapshot.Problems[0].URL != "https://codeforces.com/gym/105053/problem/A" {
		t.Fatalf("unexpected first gym problem: %+v", snapshot.Problems[0])
	}
}

func TestGetGymProblemUsesGymSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gym/105053/problem/A" {
			t.Fatalf("request path = %q, want gym problem path", r.URL.Path)
		}
		_, _ = w.Write([]byte(`<html><title>A. Almost Aligned - Codeforces</title><div class="problem-statement"><div class="title">A. Almost Aligned</div></div></html>`))
	}))
	defer server.Close()

	adapter := &Adapter{client: server.Client(), baseURL: server.URL}
	problem, err := adapter.GetProblem(context.Background(), "gym/105053/A")
	if err != nil {
		t.Fatalf("GetProblem(gym) error = %v", err)
	}
	if problem.URL != "https://codeforces.com/gym/105053/problem/A" || problem.Title != "Almost Aligned" {
		t.Fatalf("unexpected gym problem: %+v", problem)
	}
	if gym, _ := problem.Metadata["gym"].(bool); !gym {
		t.Fatalf("gym problem metadata = %+v, want gym=true", problem.Metadata)
	}
}

func TestGetContestPreservesProblemOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/contest/1931" {
			t.Fatalf("request path = %q, want /contest/1931", r.URL.Path)
		}
		_, _ = w.Write([]byte(`<html><body>
			<a href="/contest/1931">Codeforces Round 1931</a>
			<table>
				<tr><td><a href="/contest/1931/problem/A2">A2</a></td><td><a href="/contest/1931/problem/A2">Second</a></td></tr>
				<tr><td><a href="/contest/1931/problem/A1">A1</a></td><td><a href="/contest/1931/problem/A1">First</a></td></tr>
			</table>
		</body></html>`))
	}))
	defer server.Close()

	adapter := &Adapter{client: server.Client(), baseURL: server.URL}
	snapshot, err := adapter.GetContest(context.Background(), "1931")
	if err != nil {
		t.Fatalf("GetContest() error = %v", err)
	}
	if snapshot.Name != "Codeforces Round 1931" || snapshot.ExternalID != "1931" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if len(snapshot.Problems) != 2 || snapshot.Problems[0].ExternalID != "1931/A2" || snapshot.Problems[1].ExternalID != "1931/A1" {
		t.Fatalf("problem order was not preserved: %+v", snapshot.Problems)
	}
}

func TestGetContestRejectsUnrevealedContest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html><body><p>Registration has not started or contest is hidden</p></body></html>`))
	}))
	defer server.Close()

	adapter := &Adapter{client: server.Client(), baseURL: server.URL}
	if _, err := adapter.GetContest(context.Background(), "1931"); err == nil || !strings.Contains(err.Error(), "no importable problems") {
		t.Fatalf("GetContest() error = %v, want unrevealed-contest error", err)
	}
}

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
