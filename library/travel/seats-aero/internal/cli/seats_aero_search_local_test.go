package cli

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSearchNeverCallsLiveAPI(t *testing.T) {
	isolateNovelTest(t)
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer srv.Close()
	t.Setenv("SEATS_AERO_BASE_URL", srv.URL)
	out, stderr, err := executeRoot("search", "anything", "--data-source", "live", "--json")
	if err != nil {
		t.Fatalf("local search failed: %v, stdout=%q stderr=%q", err, out.String(), stderr.String())
	}
	if requests.Load() != 0 {
		t.Fatalf("live requests=%d, want 0", requests.Load())
	}
}
