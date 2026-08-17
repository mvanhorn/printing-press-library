// uk-train-goat hand-authored: tests for RJFAF download, auth, and freshness probe.
// Uses httptest.Server; no live network calls.
package fares

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/cliutil"
)

// buildZip returns the bytes of a zip archive containing one entry named
// "fixture.txt" with the content "hello".
func buildZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	fw, err := zw.Create("fixture.txt")
	if err != nil {
		t.Fatalf("zip.Create: %v", err)
	}
	if _, err := fw.Write([]byte("hello")); err != nil {
		t.Fatalf("zip entry write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip.Close: %v", err)
	}
	return buf.Bytes()
}

// newFakeServer returns an httptest.Server that handles both /authenticate and
// the feed endpoint. It records the HTTP method of the most recent feed request
// into *lastFeedMethod.
func newFakeServer(t *testing.T, zipBytes []byte, lastFeedMethod *string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/authenticate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "want POST", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"token": "x"}); err != nil {
			t.Errorf("encode token: %v", err)
		}
	})
	mux.HandleFunc("/api/staticfeeds/2.0/fares", func(w http.ResponseWriter, r *http.Request) {
		if lastFeedMethod != nil {
			*lastFeedMethod = r.Method
		}
		w.Header().Set("Last-Modified", "Thu, 19 Jun 2026 12:00:00 GMT")
		w.Header().Set("Content-Disposition", `attachment; filename="RJFAF999.ZIP"`)
		if r.Method == http.MethodHead {
			// HEAD: no body, just headers.
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		if _, err := w.Write(zipBytes); err != nil {
			t.Errorf("write zip: %v", err)
		}
	})
	return httptest.NewServer(mux)
}

// TestAuthenticate verifies that Authenticate posts to /authenticate and returns
// the token from the JSON response.
func TestAuthenticate(t *testing.T) {
	zipBytes := buildZip(t)
	srv := newFakeServer(t, zipBytes, nil)
	defer srv.Close()

	orig := portalBase
	portalBase = srv.URL
	defer func() { portalBase = orig }()

	ctx := t.Context()
	token, err := Authenticate(ctx, "u", "p")
	if err != nil {
		t.Fatalf("Authenticate returned error: %v", err)
	}
	if token != "x" {
		t.Errorf("token = %q, want %q", token, "x")
	}
}

// TestFetchFeed verifies that FetchFeed writes a valid zip to a temp file,
// parses meta.Sequence from Content-Disposition, and sets meta.LastModified.
func TestFetchFeed(t *testing.T) {
	zipBytes := buildZip(t)
	srv := newFakeServer(t, zipBytes, nil)
	defer srv.Close()

	orig := portalBase
	portalBase = srv.URL
	defer func() { portalBase = orig }()

	ctx := t.Context()
	zipPath, meta, err := FetchFeed(ctx, "x")
	if err != nil {
		t.Fatalf("FetchFeed returned error: %v", err)
	}
	defer os.Remove(zipPath)

	if zipPath == "" {
		t.Fatal("zipPath is empty")
	}

	// meta.Sequence must be "999" (stripped from RJFAF999.ZIP).
	if meta.Sequence != "999" {
		t.Errorf("meta.Sequence = %q, want %q", meta.Sequence, "999")
	}
	if meta.LastModified == "" {
		t.Error("meta.LastModified is empty, want the Last-Modified header value")
	}

	// Unzip the temp file and assert the fixture entry is present.
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("zip.OpenReader(%q): %v", zipPath, err)
	}
	defer r.Close()

	if len(r.File) != 1 {
		t.Fatalf("zip contains %d file(s), want 1", len(r.File))
	}
	f, err := r.File[0].Open()
	if err != nil {
		t.Fatalf("open zip entry: %v", err)
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read zip entry: %v", err)
	}
	if string(content) != "hello" {
		t.Errorf("zip entry content = %q, want %q", string(content), "hello")
	}
}

// TestProbeFreshness verifies that ProbeFreshness uses HEAD (not GET) and
// returns the Last-Modified header without downloading the body.
func TestProbeFreshness(t *testing.T) {
	zipBytes := buildZip(t)
	var lastFeedMethod string
	srv := newFakeServer(t, zipBytes, &lastFeedMethod)
	defer srv.Close()

	orig := portalBase
	portalBase = srv.URL
	defer func() { portalBase = orig }()

	ctx := t.Context()
	lastModified, err := ProbeFreshness(ctx, "x")
	if err != nil {
		t.Fatalf("ProbeFreshness returned error: %v", err)
	}

	// Prove it used HEAD.
	if lastFeedMethod != http.MethodHead {
		t.Errorf("feed endpoint received method %q, want HEAD", lastFeedMethod)
	}
	if lastModified == "" {
		t.Error("lastModified is empty, want Last-Modified header value")
	}
}

// TestVerifyFloor verifies that all three exported functions short-circuit
// without touching the network when PRINTING_PRESS_VERIFY=1.
func TestVerifyFloor(t *testing.T) {
	var hits atomic.Int32
	// This server calls t.Errorf (not t.Fatal) so the goroutine exit is safe.
	neverHit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		t.Errorf("unexpected network call during verify-floor: %s %s", r.Method, r.URL.Path)
		fmt.Fprintln(w, "should not be called")
	}))
	defer neverHit.Close()

	orig := portalBase
	portalBase = neverHit.URL
	defer func() { portalBase = orig }()

	t.Setenv(cliutil.VerifyEnvVar, "1")

	ctx := t.Context()

	// Authenticate must return a synthetic token with no network call.
	token, err := Authenticate(ctx, "u", "p")
	if err != nil {
		t.Errorf("Authenticate error in verify mode: %v", err)
	}
	if token == "" {
		t.Error("Authenticate in verify mode returned empty token, want synthetic value")
	}

	// FetchFeed must return empty path and zero FeedMeta with no network call.
	zipPath, _, err := FetchFeed(ctx, "tok")
	if err != nil {
		t.Errorf("FetchFeed error in verify mode: %v", err)
	}
	if zipPath != "" {
		t.Errorf("FetchFeed in verify mode returned zipPath=%q, want empty", zipPath)
	}

	// ProbeFreshness must return empty string with no network call.
	lm, err := ProbeFreshness(ctx, "tok")
	if err != nil {
		t.Errorf("ProbeFreshness error in verify mode: %v", err)
	}
	if lm != "" {
		t.Errorf("ProbeFreshness in verify mode returned %q, want empty", lm)
	}

	// Confirm the server was never reached.
	if n := hits.Load(); n != 0 {
		t.Errorf("network hit count = %d, want 0 (verify floor breached)", n)
	}
}
