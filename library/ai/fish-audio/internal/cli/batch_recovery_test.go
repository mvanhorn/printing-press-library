package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/fishaudio"
)

func TestBatchCommandPreservesFirstDuplicateWhenSecondWriteFails(t *testing.T) {
	const audio = "ID3test audio"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != fishTTSPath {
			t.Errorf("unexpected provider request %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte(audio))
	}))
	defer server.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("FISH_AUDIO_CONFIG", filepath.Join(home, "missing.toml"))
	t.Setenv("FISH_AUDIO_API_KEY", "synthetic-test-key")
	t.Setenv("FISH_AUDIO_BASE_URL", server.URL)

	outDir := filepath.Join(home, "out")
	secondPath := filepath.Join(outDir, fishaudio.BatchOutputName(2, "mp3"))
	if err := os.MkdirAll(secondPath, 0o700); err != nil {
		t.Fatalf("creating second output path as a directory: %v", err)
	}
	stdout, _, err := runCLI(t, "--json", "tts", "batch",
		"--line", "hello", "--line", "hello", "--voice", "test-voice",
		"--out-dir", outDir, "--db", filepath.Join(home, "renders.db"),
		"--concurrency", "1")
	if got := ExitCode(err); got != 6 {
		t.Fatalf("exit code = %d (error %v), want partial failure 6", got, err)
	}
	var summary batchSummary
	if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
		t.Fatalf("parsing batch summary: %v; stdout %q", err, stdout)
	}
	firstPath := filepath.Join(outDir, fishaudio.BatchOutputName(1, "mp3"))
	if calls.Load() != 1 || summary.Count != 1 || summary.Deduped != 1 || summary.Files != 1 {
		t.Fatalf("provider calls = %d, batch summary = %+v; want one call and one persisted file", calls.Load(), summary)
	}
	if len(summary.Renders) != 1 || summary.Renders[0].ID <= 0 || summary.Renders[0].File != firstPath {
		t.Fatalf("renders = %+v, want first file and its render-log row ID", summary.Renders)
	}
	if len(summary.Failed) != 1 || summary.Failed[0].LineNo != 2 {
		t.Fatalf("failed lines = %+v, want only second duplicate", summary.Failed)
	}
	if got, err := os.ReadFile(firstPath); err != nil || string(got) != audio {
		t.Fatalf("first output = %q, error = %v; want persisted synthetic audio", got, err)
	}
}

func TestAccumulateBatchResultPreservesPartialRecoveryManifest(t *testing.T) {
	unitOne := batchUnit{path: "one.mp3", req: fishaudio.RenderRequest{Text: "hello"}}
	unitTwo := batchUnit{path: "two.mp3", req: fishaudio.RenderRequest{Text: "hello"}}
	result := batchResult{
		job:                batchJob{units: []batchUnit{unitOne, unitTwo}},
		err:                errors.New("second output failed"),
		synthesisCompleted: true,
		costUSD:            0.25,
		manifests: []renderManifest{{
			ID: 41, File: "one.mp3", BytesOut: 12, CostUSD: 0.25,
		}},
	}
	summary := batchSummary{Renders: []renderManifest{}, Failed: []batchFailure{}}
	completed := accumulateBatchResult(&summary, result)
	if !completed["one.mp3"] || completed["two.mp3"] {
		t.Fatalf("completed paths = %v", completed)
	}
	if summary.Count != 1 || summary.Files != 1 || summary.BytesOut != 12 || len(summary.Renders) != 1 || summary.Renders[0].ID != 41 {
		t.Fatalf("partial recovery summary = %+v", summary)
	}
}

func TestAccumulateBatchResultAccountsForSynthesisBeforePersistenceFailure(t *testing.T) {
	unit := batchUnit{path: "unwritten.mp3", req: fishaudio.RenderRequest{Text: "provider returned this audio"}}
	result := batchResult{
		job:                batchJob{units: []batchUnit{unit}},
		synthesisCompleted: true,
		costUSD:            0.25,
		paidEquivUSD:       0.50,
		err:                errors.New("first output write failed"),
	}
	summary := batchSummary{Renders: []renderManifest{}, Failed: []batchFailure{}}
	completed := accumulateBatchResult(&summary, result)
	if len(completed) != 0 || summary.Files != 0 || len(summary.Renders) != 0 || summary.BytesOut != 0 {
		t.Fatalf("persistence totals = completed %v, summary %+v; want no durable output", completed, summary)
	}
	if summary.Count != 1 || summary.BytesIn != unit.req.BytesIn64() || summary.CostUSD != 0.25 || summary.CostUSDPaidEquiv != 0.50 {
		t.Fatalf("provider accounting = %+v, want one completed billed synthesis", summary)
	}
}
