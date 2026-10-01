// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/client"
	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/config"
	"github.com/mvanhorn/printing-press-library/library/cloud/browserbase/internal/platform"
)

// TestNovelFetchBatchHelpWires smoke-tests that the fetch batch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelFetchBatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"fetch", "batch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fetch batch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "batch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("fetch batch --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestFetchBatchCheckpointPathScopesRequestIdentity(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "browserbase.db")
	urls := []string{"https://example.com/a", "https://example.com/b"}

	raw := fetchBatchCheckpointPath(dataPath, "client-a", "raw", urls)
	if got := fetchBatchCheckpointPath(dataPath, "client-a", "raw", urls); got != raw {
		t.Fatalf("same job produced different checkpoint paths: %q != %q", got, raw)
	}
	if reordered := fetchBatchCheckpointPath(dataPath, "client-a", "raw", []string{urls[1], urls[0], urls[0]}); reordered != raw {
		t.Fatal("reordering or repeating the same URLs changed checkpoint identity")
	}
	if otherClient := fetchBatchCheckpointPath(dataPath, "client-b", "raw", urls); otherClient == raw {
		t.Fatal("client context must scope the resume checkpoint")
	}
	if markdown := fetchBatchCheckpointPath(dataPath, "client-a", "markdown", urls); markdown == raw {
		t.Fatal("request format must scope the resume checkpoint")
	}
	if otherInput := fetchBatchCheckpointPath(dataPath, "client-a", "raw", []string{"https://example.com/a"}); otherInput == raw {
		t.Fatal("input URL set must scope the resume checkpoint")
	}
}

func TestFetchBatchClientScopeSeparatesEndpointsProfilesAndCredentials(t *testing.T) {
	clientA := &client.Client{
		BaseURL: "https://tenant-a.example.test/",
		Config:  &config.Config{AuthHeaderVal: "secret-a", AuthSource: "env"},
	}
	clientB := &client.Client{
		BaseURL: "https://tenant-b.example.test",
		Config:  &config.Config{AuthHeaderVal: "secret-a", AuthSource: "env"},
	}
	legacyA := fetchBatchClientScope(clientA, &rootFlags{})
	if legacyB := fetchBatchClientScope(clientB, &rootFlags{}); legacyB == legacyA {
		t.Fatal("API base URL must scope legacy-client checkpoints")
	}
	clientB.BaseURL = clientA.BaseURL
	clientB.Config.AuthHeaderVal = "secret-b"
	if legacyB := fetchBatchClientScope(clientB, &rootFlags{}); legacyB == legacyA {
		t.Fatal("credential fingerprint must scope legacy-client checkpoints")
	}
	if strings.Contains(legacyA, "secret-a") {
		t.Fatal("client scope must not expose raw credentials")
	}

	profileA := fetchBatchClientScope(clientA, &rootFlags{platformSession: &platform.Session{
		ProfileName: "tenant-a", Source: "browserbase", CredentialFingerprint: "fingerprint-a",
	}})
	profileB := fetchBatchClientScope(clientA, &rootFlags{platformSession: &platform.Session{
		ProfileName: "tenant-b", Source: "browserbase", CredentialFingerprint: "fingerprint-a",
	}})
	if profileA == profileB {
		t.Fatal("selected client profile must scope tenant-gated checkpoints")
	}
}

func TestFetchBatchCheckpointPersistsEachCompletionAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fetch-batch-checkpoint.done")
	url := "https://example.com/a"
	if err := saveFetchBatchCheckpoint(path, url); err != nil {
		t.Fatalf("saveFetchBatchCheckpoint: %v", err)
	}

	loaded, err := loadFetchBatchCheckpoint(path, "", []string{url})
	if err != nil {
		t.Fatalf("loadFetchBatchCheckpoint: %v", err)
	}
	if !loaded["https://example.com/a"] || len(loaded) != 1 {
		t.Fatalf("loaded checkpoint = %v", loaded)
	}

	entries, err := filepath.Glob(filepath.Join(path, ".*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("atomic checkpoint left temporary files: %v", entries)
	}
}

func TestFetchBatchCheckpointRejectsMalformedJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "fetch-batch-checkpoint.done")
	legacyPath := filepath.Join(root, "fetch-batch-checkpoint.json")
	for _, body := range []string{`["https://example.com"`, `null`, `{}`} {
		if err := os.WriteFile(legacyPath, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadFetchBatchCheckpoint(path, legacyPath, []string{"https://example.com"}); err == nil || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("load malformed checkpoint %q error = %v", body, err)
		}
	}
}

func TestFetchBatchImportsLegacyCheckpointAndAddsMarkers(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "browserbase.db")
	urls := []string{"https://example.com/a", "https://example.com/b"}
	path := fetchBatchCheckpointPath(dataPath, "client-a", "raw", urls)
	legacyPath := fetchBatchLegacyCheckpointPath(dataPath, "client-a", "raw", urls)
	if err := os.WriteFile(legacyPath, []byte(`["https://example.com/a"]`), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadFetchBatchCheckpoint(path, legacyPath, urls)
	if err != nil || !loaded[urls[0]] || loaded[urls[1]] {
		t.Fatalf("legacy progress was not imported: loaded=%v err=%v", loaded, err)
	}
	if err := saveFetchBatchCheckpoint(path, urls[1]); err != nil {
		t.Fatal(err)
	}
	loaded, err = loadFetchBatchCheckpoint(path, legacyPath, urls)
	if err != nil || !loaded[urls[0]] || !loaded[urls[1]] {
		t.Fatalf("legacy and marker progress did not combine: loaded=%v err=%v", loaded, err)
	}
}

func TestFetchBatchConcurrentCompletionsDoNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fetch-batch-checkpoint.done")
	urls := make([]string, 40)
	var wg sync.WaitGroup
	errs := make(chan error, len(urls))
	for i := range urls {
		urls[i] = fmt.Sprintf("https://example.com/%d", i)
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			errs <- saveFetchBatchCheckpoint(path, url)
		}(urls[i])
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := loadFetchBatchCheckpoint(path, "", urls)
	if err != nil || len(loaded) != len(urls) {
		t.Fatalf("concurrent markers lost progress: loaded=%d want=%d err=%v", len(loaded), len(urls), err)
	}
}

func TestFetchBatchCanceledRunReportsFailureAndNoRequests(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	t.Setenv("BROWSERBASE_BASE_URL", server.URL)
	file := filepath.Join(t.TempDir(), "urls.txt")
	if err := os.WriteFile(file, []byte("https://example.com/a\nhttps://example.com/b\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelFetchBatchCmd(&rootFlags{asJSON: true})
	cmd.SilenceUsage = true // RootCmd sets this for machine-readable errors.
	cmd.SetArgs([]string{"--file", file, "--pace", "1s"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := cmd.ExecuteContext(ctx)
	if err != context.Canceled || calls.Load() != 0 {
		t.Fatalf("canceled batch returned %v with %d remote calls", err, calls.Load())
	}
	var view batchFetchView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil || view.FailedCount != 2 {
		t.Fatalf("canceled batch did not report both unattempted URLs: output=%s err=%v", out.String(), err)
	}
}

func TestIndexBatchURLsDeduplicatesWorkAndFansOutResults(t *testing.T) {
	urls := []string{"https://example.com/a", "https://example.com/b", "https://example.com/a"}
	unique, indexes := indexBatchURLs(urls)
	if len(unique) != 2 || unique[0] != urls[0] || unique[1] != urls[1] {
		t.Fatalf("unique URLs = %v", unique)
	}
	results := make([]batchFetchResult, len(urls))
	applyBatchResult(results, indexes, batchFetchResult{URL: urls[0], Fetched: true, StatusCode: 200})
	if !results[0].Fetched || !results[2].Fetched || results[1].Fetched {
		t.Fatalf("fanned results = %+v", results)
	}
	if len(indexes[urls[0]]) != 2 {
		t.Fatalf("duplicate indexes = %v", indexes[urls[0]])
	}
}

func TestWaitForBatchSlotStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	started := time.Now()
	if err := waitForBatchSlot(ctx, make(chan time.Time)); err != context.Canceled {
		t.Fatalf("waitForBatchSlot error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("canceled scheduler waited %s", elapsed)
	}
}

func TestBatchSchedulingHelpersPreferReadyCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for range 1000 {
		ticks := make(chan time.Time, 1)
		ticks <- time.Now()
		if err := waitForBatchSlot(ctx, ticks); err != context.Canceled {
			t.Fatalf("ready tick won over cancellation: %v", err)
		}

		sem := make(chan struct{}, 1)
		if err := acquireBatchWorker(ctx, sem); err != context.Canceled {
			t.Fatalf("ready worker slot won over cancellation: %v", err)
		}
		if len(sem) != 0 {
			t.Fatal("canceled worker acquisition leaked a semaphore slot")
		}
	}
}
