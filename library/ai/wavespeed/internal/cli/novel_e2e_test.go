// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// fakeServerOpts configures the stub WaveSpeed API used by the e2e tests.
type fakeServerOpts struct {
	price        float64
	balance      float64
	deterministic bool // fixed task id => identical result JSON across runs
}

// newFakeWaveSpeed returns an httptest server mimicking the endpoints the novel
// commands exercise: catalog, balance, pricing, model submit, prediction poll,
// and file download.
func newFakeWaveSpeed(opts fakeServerOpts) *httptest.Server {
	var counter int64
	pngBytes := makePNGBytes(1080, 1350)
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(nil)
	base := func() string { return "http://" + srv.Listener.Addr().String() }

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/models":
			_, _ = w.Write([]byte(`{"data":[{"model_id":"wavespeed-ai/flux-dev","type":"image","description":"text to image"}]}`))
		case r.URL.Path == "/balance":
			_, _ = fmt.Fprintf(w, `{"data":{"balance":%v}}`, opts.balance)
		case r.URL.Path == "/model/pricing":
			_, _ = fmt.Fprintf(w, `{"data":{"price":%v}}`, opts.price)
		case r.URL.Path == "/predictions/fixed/result":
			_, _ = fmt.Fprintf(w, `{"data":{"id":"fixed","status":"completed","outputs":["%s/files/fixed.png"]}}`, base())
		case len(r.URL.Path) > 13 && r.URL.Path[:13] == "/predictions/":
			id := r.URL.Path[len("/predictions/") : len(r.URL.Path)-len("/result")]
			_, _ = fmt.Fprintf(w, `{"data":{"id":"%s","status":"completed","outputs":["%s/files/%s.png"]}}`, id, base(), id)
		case len(r.URL.Path) > 7 && r.URL.Path[:7] == "/files/":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		case r.Method == http.MethodPost:
			// A model run submission.
			id := "fixed"
			if !opts.deterministic {
				id = fmt.Sprintf("task-%d", atomic.AddInt64(&counter, 1))
			}
			_, _ = fmt.Fprintf(w, `{"data":{"id":"%s","status":"created"}}`, id)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv.Config.Handler = mux
	srv.Start()
	return srv
}

func makePNGBytes(w, h int) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)))
	return buf.Bytes()
}

// e2eEnv wires env + cwd for an isolated run against the fake server.
func e2eEnv(t *testing.T, baseURL string) (libDB string) {
	t.Helper()
	work := t.TempDir()
	t.Chdir(work)
	t.Setenv("WAVESPEED_BASE_URL", baseURL)
	t.Setenv("WAVESPEED_API_KEY", "test-token")
	t.Setenv("WAVESPEED_CONFIG", filepath.Join(work, "nonexistent-config.toml"))
	libDB = filepath.Join(work, "library.db")
	t.Setenv("WAVESPEED_LIBRARY_DB", libDB)
	t.Setenv("WAVESPEED_ARCHIVE_DB", filepath.Join(work, "archive.db"))
	return libDB
}

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errb.String(), err
}

func decodeEnvelope(t *testing.T, stdout string) AgentEnvelope {
	t.Helper()
	var env AgentEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("envelope decode: %v\nstdout: %s", err, stdout)
	}
	return env
}

func TestE2EAgentChain(t *testing.T) {
	srv := newFakeWaveSpeed(fakeServerOpts{price: 1.0, balance: 100})
	defer srv.Close()
	e2eEnv(t, srv.URL)

	// brand init
	if _, _, err := runCLI(t, "brand", "init", "helm",
		"--voice", "premium", "--style-anchors", "matte black",
		"--models", "wavespeed-ai/flux-dev", "--platforms", "instagram,tiktok", "--agent"); err != nil {
		t.Fatalf("brand init: %v", err)
	}
	// brand apply
	if _, _, err := runCLI(t, "brand", "apply", "helm", "--agent"); err != nil {
		t.Fatalf("brand apply: %v", err)
	}
	// plan brief-to-shotlist -> shotlist.json
	planOut, _, err := runCLI(t, "plan", "brief-to-shotlist",
		"--prompt", "Helm Black launch", "--platforms", "instagram,tiktok", "--agent")
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if err := os.WriteFile("shotlist.json", []byte(planOut), 0o644); err != nil {
		t.Fatal(err)
	}
	// qa preflight on the shotlist
	qaOut, _, err := runCLI(t, "qa", "preflight", "shotlist.json", "--agent")
	if err != nil {
		t.Fatalf("qa: %v", err)
	}
	qaEnv := decodeEnvelope(t, qaOut)
	if qaEnv.Command != "qa preflight" {
		t.Fatalf("qa command = %q", qaEnv.Command)
	}

	// pack
	packOut, _, err := runCLI(t, "pack", "--concept", "Helm Black hero",
		"--platforms", "instagram,tiktok", "--model", "wavespeed-ai/flux-dev",
		"--concurrency", "2", "--agent")
	if err != nil {
		t.Fatalf("pack: %v\n%s", err, packOut)
	}
	packEnv := decodeEnvelope(t, packOut)
	if packEnv.PartialFailure {
		t.Fatalf("pack should not be partial failure: %s", packOut)
	}
	if len(packEnv.LibraryRecordErrors) != 0 {
		t.Fatalf("unexpected record errors: %v", packEnv.LibraryRecordErrors)
	}

	// Files at stable paths + manifest.
	igFeed := filepath.Join("packs", "helm-black-hero", "instagram", "feed.png")
	if _, err := os.Stat(igFeed); err != nil {
		t.Fatalf("expected %s: %v", igFeed, err)
	}
	manifestPath := filepath.Join("packs", "helm-black-hero", "instagram", "manifest.json")
	manRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	var man platformManifest
	if err := json.Unmarshal(manRaw, &man); err != nil {
		t.Fatal(err)
	}
	if man.Platform != "instagram" || man.Format != "feed" || len(man.Files) == 0 {
		t.Fatalf("manifest = %#v", man)
	}

	// Library recorded the generations.
	libOut, _, err := runCLI(t, "library", "list", "--agent")
	if err != nil {
		t.Fatalf("library list: %v", err)
	}
	libEnv := decodeEnvelope(t, libOut)
	if len(libEnv.Results) != 2 {
		t.Fatalf("library recorded %d generations, want 2", len(libEnv.Results))
	}
}

func TestE2EPackMultiAspectNoCollision(t *testing.T) {
	srv := newFakeWaveSpeed(fakeServerOpts{price: 1.0, balance: 100})
	defer srv.Close()
	e2eEnv(t, srv.URL)

	// Multiple aspects for one platform must write distinct files (no collision)
	// and a single manifest listing all of them.
	out, _, err := runCLI(t, "pack", "--concept", "multi aspect",
		"--platforms", "instagram", "--aspects", "16:9,9:16,1:1",
		"--model", "wavespeed-ai/flux-dev", "--concurrency", "3", "--agent")
	if err != nil {
		t.Fatalf("pack: %v\n%s", err, out)
	}
	dir := filepath.Join("packs", "multi-aspect", "instagram")
	for _, name := range []string{"feed-16x9.png", "feed-9x16.png", "feed-1x1.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected distinct file %s: %v", name, err)
		}
	}
	manRaw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	var man platformManifest
	if err := json.Unmarshal(manRaw, &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Files) != 3 || len(man.Assets) != 3 {
		t.Fatalf("manifest should aggregate 3 assets: files=%d assets=%d", len(man.Files), len(man.Assets))
	}
}

func TestE2EPackMaxCostAbort(t *testing.T) {
	srv := newFakeWaveSpeed(fakeServerOpts{price: 1.0, balance: 100})
	defer srv.Close()
	e2eEnv(t, srv.URL)

	// concurrency 1 so the per-launch ceiling check skips remaining shots.
	out, _, err := runCLI(t, "pack", "--concept", "ceiling test",
		"--platforms", "instagram,tiktok,facebook", "--model", "wavespeed-ai/flux-dev",
		"--concurrency", "1", "--max-cost", "1.5", "--agent")
	if err == nil {
		t.Fatalf("expected a partial-failure error at the cost ceiling")
	}
	env := decodeEnvelope(t, out)
	if !env.PartialFailure {
		t.Fatalf("expected partial_failure in envelope")
	}
	// Completed shots are recorded before the abort.
	libOut, _, _ := runCLI(t, "library", "list", "--agent")
	libEnv := decodeEnvelope(t, libOut)
	if len(libEnv.Results) == 0 || len(libEnv.Results) >= 3 {
		t.Fatalf("expected partial records (1-2), got %d", len(libEnv.Results))
	}
}

func TestE2EPackIdempotentPathsAndSeedHash(t *testing.T) {
	srv := newFakeWaveSpeed(fakeServerOpts{price: 1.0, balance: 100, deterministic: true})
	defer srv.Close()
	e2eEnv(t, srv.URL)

	run := func() string {
		out, _, err := runCLI(t, "pack", "--concept", "Helm Black hero",
			"--platforms", "instagram", "--model", "wavespeed-ai/flux-dev",
			"--seed", "42", "--agent")
		if err != nil {
			t.Fatalf("pack: %v\n%s", err, out)
		}
		return out
	}

	out1 := run()
	feed := filepath.Join("packs", "helm-black-hero", "instagram", "feed.png")
	if _, err := os.Stat(feed); err != nil {
		t.Fatalf("first run missing %s", feed)
	}
	out2 := run()
	if _, err := os.Stat(feed); err != nil {
		t.Fatalf("second run missing %s (paths not stable)", feed)
	}

	// Deterministic server => identical content hash across runs.
	h1 := firstContentHash(t, out1)
	h2 := firstContentHash(t, out2)
	if h1 == "" || h1 != h2 {
		t.Fatalf("seed-locked content hash not stable: %q vs %q", h1, h2)
	}
}

func TestE2EPackLibraryRecordErrorSurfaces(t *testing.T) {
	srv := newFakeWaveSpeed(fakeServerOpts{price: 1.0, balance: 100})
	defer srv.Close()
	work := t.TempDir()
	t.Chdir(work)
	t.Setenv("WAVESPEED_BASE_URL", srv.URL)
	t.Setenv("WAVESPEED_API_KEY", "test-token")
	t.Setenv("WAVESPEED_CONFIG", filepath.Join(work, "no-config.toml"))
	t.Setenv("WAVESPEED_ARCHIVE_DB", filepath.Join(work, "archive.db"))
	// Make the library DB unwritable: its parent is a regular file.
	blocker := filepath.Join(work, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAVESPEED_LIBRARY_DB", filepath.Join(blocker, "library.db"))

	out, _, err := runCLI(t, "pack", "--concept", "record fail",
		"--platforms", "instagram", "--model", "wavespeed-ai/flux-dev", "--agent")
	if err != nil {
		t.Fatalf("pack should succeed despite library record failure: %v\n%s", err, out)
	}
	env := decodeEnvelope(t, out)
	if len(env.LibraryRecordErrors) == 0 {
		t.Fatalf("expected library_record_errors to surface the write failure")
	}
	// The asset was still written.
	if _, err := os.Stat(filepath.Join("packs", "record-fail", "instagram", "feed.png")); err != nil {
		t.Fatalf("generation should have produced a file: %v", err)
	}
}

func firstContentHash(t *testing.T, stdout string) string {
	t.Helper()
	env := decodeEnvelope(t, stdout)
	for _, r := range env.Results {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if h, ok := m["content_hash"].(string); ok && h != "" {
			return h
		}
	}
	return ""
}
