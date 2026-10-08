package dropbox

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/client"
	"testing"
)

func TestHasSummaryPrefix(t *testing.T) {
	for _, summary := range []string{"path/not_found/..", "path/not_found/..."} {
		err := fmt.Errorf("wrapped: %w", &client.APIError{StatusCode: 409, Body: fmt.Sprintf(`{"error_summary":%q}`, summary)})
		if !HasSummaryPrefix(err, "path/not_found") {
			t.Errorf("summary %q did not match", summary)
		}
	}
	if HasSummaryPrefix(&client.APIError{StatusCode: 400, Body: `{"error_summary":"path/not_found/..."}`}, "path/not_found") {
		t.Fatal("400 matched")
	}
	if _, ok := ParseError(`{}`); ok {
		t.Fatal("missing summary matched")
	}
}
