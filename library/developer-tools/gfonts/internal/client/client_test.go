package client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetReturnsStatusBodyAndSendsHeaders(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Test"); got != "present" {
			t.Errorf("X-Test = %q, want present", got)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = fmt.Fprint(w, "font-data")
	}))
	defer server.Close()

	status, body, err := New(time.Second).Get(server.URL, map[string]string{"X-Test": "present"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if status != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", status, http.StatusAccepted)
	}
	if got := string(body); got != "font-data" {
		t.Fatalf("body = %q, want font-data", got)
	}
}
