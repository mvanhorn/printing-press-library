// Copyright 2026 Matthew Vassallo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDeliverWebhookPostsBodyWithActualContentType(t *testing.T) {
	body := []byte("a,b\n1,2\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "text/csv" {
			t.Errorf("Content-Type = %q, want text/csv", got)
		}
		var received bytes.Buffer
		_, _ = received.ReadFrom(r.Body)
		if !bytes.Equal(received.Bytes(), body) {
			t.Errorf("body = %q, want %q", received.Bytes(), body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := deliverWebhook(server.URL, body, "text/csv"); err != nil {
		t.Fatalf("deliverWebhook: %v", err)
	}
}

func TestDeliverWebhookRejectsRedirectWithoutFollowing(t *testing.T) {
	var destinationCalls int
	destination := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		destinationCalls++
	}))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusFound)
	}))
	defer redirect.Close()

	err := deliverWebhook(redirect.URL, []byte(`{"ok":true}`), "application/json")
	if err == nil || !strings.Contains(err.Error(), "302 Found") {
		t.Fatalf("redirect error = %v, want explicit 302 rejection", err)
	}
	if destinationCalls != 0 {
		t.Fatalf("redirect destination calls = %d, want 0", destinationCalls)
	}
}

func TestWebhookDiagnosticsNeverExposeTargetSecrets(t *testing.T) {
	const secret = "account/token/secret-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	target := server.URL + "/" + secret + "?signature=more-secret"
	sink := DeliverSink{Scheme: "webhook", Target: target}

	err := deliverWebhook(target, nil, "application/json")
	if err == nil {
		t.Fatal("deliverWebhook succeeded, want failure")
	}
	combined := sink.Redacted() + " " + err.Error()
	for _, forbidden := range []string{secret, "more-secret", target} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("diagnostic leaked %q: %s", forbidden, combined)
		}
	}
	if sink.Redacted() != "webhook:[redacted]" {
		t.Fatalf("redacted sink = %q", sink.Redacted())
	}
}

func TestDeliveryContentTypeUsesActualOutput(t *testing.T) {
	cases := []struct {
		name  string
		flags rootFlags
		body  string
		want  string
	}{
		{name: "json", body: `{"ok":true}`, want: "application/json"},
		{name: "ndjson", body: "{\"id\":1}\n{\"id\":2}\n", want: "application/x-ndjson"},
		{name: "csv", flags: rootFlags{csv: true}, body: "a,b\n1,2\n", want: "text/csv"},
		{name: "plain", flags: rootFlags{plain: true}, body: "one\ttwo\n", want: "text/plain"},
		{name: "unknown", body: "not json", want: "text/plain"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deliveryContentType(&tc.flags, []byte(tc.body)); got != tc.want {
				t.Fatalf("deliveryContentType = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDeliverFileConcurrentWritersUseIndependentTemps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "result.json")
	const writers = 32
	bodies := make([][]byte, writers)
	for i := range bodies {
		bodies[i] = bytes.Repeat([]byte(fmt.Sprintf("writer-%02d\n", i)), 4096)
	}

	start := make(chan struct{})
	errs := make(chan error, writers)
	var wg sync.WaitGroup
	for _, body := range bodies {
		body := body
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- deliverFile(path, body)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent deliverFile: %v", err)
		}
	}

	final, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	matched := false
	for _, body := range bodies {
		if bytes.Equal(final, body) {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatal("final file combined or truncated concurrent writer bodies")
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".result.json.tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}
