package submission

import "testing"

func TestSupportedSubmissionLanguages(t *testing.T) {
	for _, language := range []string{"cpp23", "python3", "java21"} {
		if !isSupportedLanguage(language) {
			t.Fatalf("expected %q to be supported", language)
		}
	}

	for _, language := range []string{"go", "rust", ""} {
		if isSupportedLanguage(language) {
			t.Fatalf("expected %q to be rejected", language)
		}
	}
}
