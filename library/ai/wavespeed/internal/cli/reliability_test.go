// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
)

// fastUploadBackoff is kept as a marker for upload tests. Upload retries now
// run in the generated client (x-pp-replay-safe), whose 1s/2s backoff is
// short enough for these cases.
func fastUploadBackoff(t *testing.T) {
	t.Helper()
}

// The billings endpoint rejects string pagination ("Field \"page\" must be a
// number."). Every numeric body field must reach the wire as a JSON number.
func TestBillingsSendsNumericPagination(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/billings/search" {
			http.NotFound(w, r)
			return
		}
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"items":[]}}`))
	}))
	defer server.Close()

	for _, tc := range []struct {
		args         []string
		page, size   string
		createdAfter string
	}{
		{args: nil, page: "1", size: "20"},
		{args: []string{"--page", "3", "--page-size", "10", "--created-after", "2026-09-01T00:00:00Z"}, page: "3", size: "10", createdAfter: "2026-09-01T00:00:00Z"},
	} {
		got = nil
		_, stderr, err := executeRootForTest(t, append([]string{"--json", "--no-cache", "billings"}, tc.args...), server.URL)
		if err != nil {
			t.Fatalf("billings %v: %v (stderr=%s)", tc.args, err, stderr)
		}
		page, ok := got["page"].(json.Number)
		if !ok || page.String() != tc.page {
			t.Fatalf("page = %#v (%T), want JSON number %s", got["page"], got["page"], tc.page)
		}
		size, ok := got["page_size"].(json.Number)
		if !ok || size.String() != tc.size {
			t.Fatalf("page_size = %#v (%T), want JSON number %s", got["page_size"], got["page_size"], tc.size)
		}
		if tc.createdAfter != "" && got["created_after"] != tc.createdAfter {
			t.Fatalf("created_after = %#v", got["created_after"])
		}
	}
}

// A submission POST that fails ambiguously (connection dropped or 5xx after
// the server may have accepted it) must not be replayed: every replay would
// start and bill another prediction.
func TestPredictionSubmitIsNotReplayedAfterAmbiguousFailure(t *testing.T) {
	for _, mode := range []string{"5xx", "drop"} {
		t.Run(mode, func(t *testing.T) {
			var posts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				posts.Add(1)
				if mode == "5xx" {
					http.Error(w, "gateway timeout", http.StatusGatewayTimeout)
					return
				}
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("hijack unsupported")
				}
				conn, _, _ := hj.Hijack()
				_ = conn.Close()
			}))
			defer server.Close()
			c := newTestClient(server.URL)
			_, err := submitAndAwait(context.Background(), c, submitRequest{modelID: "google/nano-banana-2/edit", inputs: map[string]any{"prompt": "x"}})
			if err == nil {
				t.Fatal("expected an error")
			}
			if n := posts.Load(); n != 1 {
				t.Fatalf("prediction POST sent %d times, want exactly 1", n)
			}
		})
	}
}

// Reads stay retryable on 5xx.
func TestReadsStillRetryOnServerError(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "boom", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"balance":1.5}}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	if _, err := c.GetNoCache(context.Background(), "/balance", nil); err != nil {
		t.Fatalf("GET should retry past one 502: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", calls.Load())
	}
}

// A dropped connection mid-upload (the reported "read: operation timed out")
// is retried because uploads are free and idempotent from a billing view.
func TestUploadRetriesTransientTransportFailure(t *testing.T) {
	fastUploadBackoff(t)
	name, payload := mediaUploadFixture(t)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		body, _ := io.ReadAll(r.Body)
		if !bytes.Contains(body, payload) {
			t.Errorf("attempt %d missing file bytes (len %d)", n, len(body))
		}
		if n < 3 {
			hj := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"download_url":"https://cdn.example/x.png"}}`))
	}))
	defer server.Close()
	stdout, stderr, err := executeMediaUpload(t, server.URL, false, name)
	if err != nil {
		t.Fatalf("upload should succeed after retries: %v (stderr=%s)", err, stderr)
	}
	if attempts.Load() != 3 || !strings.Contains(stdout, "cdn.example") {
		t.Fatalf("attempts=%d stdout=%s", attempts.Load(), stdout)
	}
	// The generated client reports each retry on the process stderr.
	_ = stderr
}

func TestUploadDoesNotRetryClientErrors(t *testing.T) {
	fastUploadBackoff(t)
	name, _ := mediaUploadFixture(t)
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer server.Close()
	_, _, err := executeMediaUpload(t, server.URL, false, name)
	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400 APIError, got %v", err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("400 retried %d times", attempts.Load())
	}
}

// A poll failure after submission must keep the prediction ID recoverable.
func TestSubmitAndAwaitPollFailureKeepsPredictionID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"data":{"id":"pred-789","status":"created"}}`))
			return
		}
		http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	res, err := submitAndAwait(context.Background(), c, submitRequest{
		modelID: "m/x", inputs: map[string]any{"prompt": "x"}, wait: true,
		waitTimeout: time.Second, pollInitial: time.Millisecond,
	})
	var submitted *submittedPredictionError
	if !errors.As(err, &submitted) || submitted.ID != "pred-789" {
		t.Fatalf("want submittedPredictionError for pred-789, got %v", err)
	}
	if !strings.Contains(err.Error(), "prediction-results pred-789") {
		t.Fatalf("error should give the recovery command: %v", err)
	}
	if res.PredictionID != "pred-789" || len(res.Result) == 0 {
		t.Fatalf("result lost: %+v", res)
	}
}

// Transient poll failures do not abandon a running (billed) prediction.
func TestWaitForPredictionToleratesTransientPollErrors(t *testing.T) {
	var polls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"data":{"id":"p1","status":"created"}}`))
			return
		}
		if polls.Add(1) <= 2 {
			// Close without a response: a network-level failure on a GET,
			// which the client already retries; make it exhaust with 4xx-free 5xx.
			hj := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"p1","status":"completed","outputs":["https://cdn.example/a.png"]}}`))
	}))
	defer server.Close()
	c := newTestClient(server.URL)
	res, err := submitAndAwait(context.Background(), c, submitRequest{
		modelID: "m/x", inputs: map[string]any{}, wait: true,
		waitTimeout: 30 * time.Second, pollInitial: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("transient poll errors should be tolerated: %v", err)
	}
	if res.Status != "completed" {
		t.Fatalf("status = %q", res.Status)
	}
}

// Novel producers get the download failure as data, not as a lost result.
func TestSubmitAndAwaitDownloadFailureIsNotFatal(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer cdn.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"data":{"id":"p2","status":"created"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"p2","status":"completed","outputs":["` + cdn.URL + `/out.png"]}}`))
	}))
	defer api.Close()
	c := newTestClient(api.URL)
	res, err := submitAndAwait(context.Background(), c, submitRequest{
		modelID: "m/x", inputs: map[string]any{}, wait: true, waitTimeout: 5 * time.Second,
		pollInitial: time.Millisecond, download: true, downloadSpec: filepath.Join(t.TempDir(), "{index}.{ext}"),
	})
	if err != nil {
		t.Fatalf("download failure must not be returned as the call error: %v", err)
	}
	if res.DownloadErr == nil || res.Status != "completed" {
		t.Fatalf("want DownloadErr and completed status, got %+v", res)
	}
	msg := downloadFailureMessage(res)
	if !strings.Contains(msg, "p2") || !strings.Contains(msg, cdn.URL+"/out.png") {
		t.Fatalf("message must name prediction and URL: %q", msg)
	}
}

// A completed but undownloadable step must not stop compose: its output URL
// still feeds the next step, and the run reports partial failure at the end.
func TestComposeContinuesPastDownloadFailure(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer cdn.Close()
	var submits atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && !strings.Contains(r.URL.Path, "pric") {
			submits.Add(1)
			_, _ = w.Write([]byte(`{"data":{"id":"c1","status":"created"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"c1","status":"completed","outputs":["` + cdn.URL + `/out.png"]}}`))
	}))
	defer api.Close()
	stdout, stderr, err := executeRootForTest(t, []string{"compose", "--prompt", "p", "--steps", "text->image,image->video", "--models", "m/a,m/b", "--no-record", "--out-dir", t.TempDir(), "--json"}, api.URL)
	if submits.Load() != 2 {
		t.Fatalf("compose submitted %d steps, want 2 (stdout=%s stderr=%s)", submits.Load(), stdout, stderr)
	}
	if err == nil || !strings.Contains(stdout, `"partial_failure": true`) || !strings.Contains(stdout, cdn.URL+"/out.png") {
		t.Fatalf("want partial failure with output URL, err=%v stdout=%s", err, stdout)
	}
}

// A restyle whose output cannot be downloaded is reported as a partial
// failure that keeps the prediction ID and output URL.
func TestRestyleDownloadFailureIsPartialFailure(t *testing.T) {
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer cdn.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost && !strings.Contains(r.URL.Path, "pric") {
			_, _ = w.Write([]byte(`{"data":{"id":"r1","status":"created"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"r1","status":"completed","outputs":["` + cdn.URL + `/out.png"]}}`))
	}))
	defer api.Close()
	stdout, _, err := executeRootForTest(t, []string{"restyle", "https://example.com/in.png", "--model", "m/x", "--style", "noir", "--no-record", "--out-dir", t.TempDir(), "--json"}, api.URL)
	if err == nil {
		t.Fatalf("restyle without a local file must not report success: %s", stdout)
	}
	if !strings.Contains(stdout, `"partial_failure": true`) || !strings.Contains(stdout, "r1") || !strings.Contains(stdout, cdn.URL+"/out.png") {
		t.Fatalf("want partial failure naming prediction and URL: %s", stdout)
	}
}

// A stalled poll must not hold the command past the wait deadline.
func TestWaitForPredictionRespectsDeadlineDuringStalledPoll(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)
	c := newTestClient(server.URL)
	start := time.Now()
	_, err := waitForPrediction(context.Background(), c, "stall-1", 300*time.Millisecond, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "stall-1") {
		t.Fatalf("want timeout naming the prediction, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("poll outlasted the wait deadline: %s", elapsed)
	}
}

func TestNoteDownloadFailureIsWarningNotError(t *testing.T) {
	oc := shotOutcome{Warning: "dims mismatch"}
	noteDownloadFailure(&oc, submitResult{PredictionID: "p9", DownloadErr: errors.New("401"), Result: json.RawMessage(`{"data":{"outputs":["https://cdn.example/o.png"]}}`)})
	if oc.Err != "" || !oc.DownloadFailed || !strings.Contains(oc.Warning, "dims mismatch; ") || !strings.Contains(oc.Warning, "p9") {
		t.Fatalf("unexpected outcome: %+v", oc)
	}
	data := string(oc.recoveryData())
	if !strings.Contains(data, `"prediction_id":"p9"`) || !strings.Contains(data, "https://cdn.example/o.png") {
		t.Fatalf("library record must keep recovery details: %s", data)
	}
}

// A shot whose outputs did not all download is excluded from the post-ready
// platform manifest.
func TestPackManifestSkipsShotsWithMissingDownloads(t *testing.T) {
	dir := t.TempDir()
	shots := []Shot{{Platform: "instagram", Format: "square"}, {Platform: "instagram", Format: "square"}}
	outcomes := []shotOutcome{
		{Files: []string{filepath.Join(dir, "ok.png")}},
		{Files: []string{filepath.Join(dir, "partial.png")}, DownloadFailed: true, Warning: "prediction p completed but download failed"},
	}
	written := writePlatformManifests(packFlags{outDir: dir}, "slug", shots, outcomes)
	if len(written) != 1 {
		t.Fatalf("manifests written = %v", written)
	}
	raw, err := os.ReadFile(written[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "partial.png") || !strings.Contains(string(raw), "ok.png") {
		t.Fatalf("manifest must list only fully downloaded shots: %s", raw)
	}

	// A later run where every shot for the platform misses an output must
	// remove the earlier manifest instead of leaving it post-ready.
	outcomes[0].DownloadFailed = true
	if again := writePlatformManifests(packFlags{outDir: dir}, "slug", shots, outcomes); len(again) != 0 {
		t.Fatalf("no manifest expected, got %v", again)
	}
	if _, err := os.Stat(written[0]); !os.IsNotExist(err) {
		t.Fatalf("stale manifest left under the post-ready name: %v", err)
	}
	kept, _ := filepath.Glob(filepath.Join(filepath.Dir(written[0]), "manifest.superseded-*.json"))
	if len(kept) != 1 {
		t.Fatalf("earlier manifest must be kept recoverable, found %v", kept)
	}
}

func TestDoctorVerifiesCredentialsWithBalance(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"ok", 200, `{"code":200,"data":{"balance":12.3456}}`, "verified (balance $12.3456)"},
		{"bad key", 401, `{"message":"unauthorized"}`, "invalid (HTTP 401"},
		{"unrecognized 200", 200, `<html>sign in</html>`, "present, not verified"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/balance" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			stdout, stderr, _ := executeRootForTest(t, []string{"doctor", "--json"}, server.URL)
			var report map[string]any
			if err := json.Unmarshal([]byte(stdout), &report); err != nil {
				t.Fatalf("doctor json: %v\n%s\n%s", err, stdout, stderr)
			}
			cred, _ := report["credentials"].(string)
			if !strings.HasPrefix(cred, tc.want) {
				t.Fatalf("credentials = %q, want prefix %q", cred, tc.want)
			}
		})
	}
}
