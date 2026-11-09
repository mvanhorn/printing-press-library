package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// PATCH(gocli1219-live-gate-defects) / PATCH(gocli1219-live-gate-fixtures): the real CLI defects the 11-08 publish live
// gate found (gocli1219-logs/triage.md, class a), each pinned here.

type liveGateServer struct {
	mu    sync.Mutex
	paths []string
}

func (s *liveGateServer) seen() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.paths...)
}

// runLiveGateRoot runs a full command path through the root command against a fake API answering routes[path].
func runLiveGateRoot(t *testing.T, routes map[string]string, args ...string) (stdout string, srv *liveGateServer, err error) {
	t.Helper()
	srv = &liveGateServer{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		srv.mu.Lock()
		srv.paths = append(srv.paths, r.URL.Path)
		srv.mu.Unlock()
		body, ok := routes[strings.TrimPrefix(r.URL.Path, "/api/v1")]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"Not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", ts.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_HOME", t.TempDir())
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append(args, "--no-cache"))
	err = root.Execute()
	return out.String(), srv, err
}

func TestLiveGateFixturesTargetExistingCommands(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, f := range liveGateFixtures {
		cmd := findCommandPath(root, f.path)
		if cmd == nil {
			t.Errorf("live-gate fixture %q names no command (renamed or dropped by a reprint?)", f.path)
			continue
		}
		for k, v := range f.annotations {
			if cmd.Annotations[k] != v {
				t.Errorf("%s: annotation %s = %q, want %q", f.path, k, cmd.Annotations[k], v)
			}
		}
		if f.example != "" && cmd.Example != f.example {
			t.Errorf("%s: example not applied", f.path)
		}
	}
}

func TestSendCheckEmptyScopeIsACleanAnswer(t *testing.T) {
	out, srv, err := runLiveGateRoot(t, map[string]string{"/contacts": `{"data":[]}`}, "send-check", "--tag", "nobody-has-this", "--json")
	if err != nil {
		t.Fatalf("empty scope: err=%v (exit %d), want a clean exit 0 answer", err, ExitCode(err))
	}
	var got map[string]any
	if json.Unmarshal([]byte(out), &got) != nil {
		t.Fatalf("stdout is not JSON: %q", out)
	}
	if s, _ := got["summary"].(map[string]any); s == nil || s["checked"] != float64(0) {
		t.Fatalf("summary = %v, want checked 0", got["summary"])
	}
	if len(srv.seen()) == 0 {
		t.Fatal("send-check sent no request: the scope must be read from the API")
	}
}

func TestSendCheckUnknownContactIsStillNotFound(t *testing.T) {
	_, _, err := runLiveGateRoot(t, map[string]string{}, "send-check", "--contact", "3f8a0c4e-1111-4222-8333-444455556666", "--json")
	if err == nil || ExitCode(err) != 3 {
		t.Fatalf("unknown --contact: err=%v exit=%d, want the API's 404 (exit 3)", err, ExitCode(err))
	}
}

func TestDripsAuditRefusesAJunkCampaignID(t *testing.T) {
	_, _, err := runLiveGateRoot(t, nil, "drips", "audit", "__printing_press_invalid__")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("junk campaignId: err=%v exit=%d, want a usage error (exit 2)", err, ExitCode(err))
	}
	if _, _, err := runLiveGateRoot(t, nil, "drips", "audit", "a", "b"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("two positionals: err=%v, want a usage error", err)
	}
}

func TestNovelDryRunJSONIsADryRunObject(t *testing.T) {
	for _, args := range [][]string{
		{"drips", "audit", "--since", "2026-07-01"},
		{"send-check", "--tag", "x"},
		{"imports", "blame", "--latest"},
		{"imports", "watch", "--latest"},
		{"contacts", "export"},
		{"contacts", "get-import-template"},
		{"activity"},
	} {
		out, srv, err := runLiveGateRoot(t, nil, append(args, "--dry-run", "--json")...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		var got map[string]any
		if json.Unmarshal([]byte(strings.TrimSpace(out)), &got) != nil {
			t.Fatalf("%v: stdout is not one JSON object: %q", args, out)
		}
		if got["dry_run"] != true || strings.TrimSpace(asString(got["action"])) == "" {
			t.Fatalf("%v: %v, want dry_run true and a non-empty action", args, got)
		}
		if n := len(srv.seen()); n != 0 {
			t.Fatalf("%v: --dry-run sent %d requests", args, n)
		}
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func TestImportsLatestResolvesTheNewestJob(t *testing.T) {
	const job = "0f911935-c98a-450a-8e24-93bf21016bfd"
	// novel1219: blame and watch now read the job's counters, its launch monitor and its job-scoped side effects
	// (the public entry's behaviour), so the fake API answers those routes too.
	routes := map[string]string{
		"/imports":                            `{"data":[{"id":"` + job + `","status":"completed"}],"meta":{}}`,
		"/imports/" + job:                     `{"data":{"id":"` + job + `","status":"completed","totalRows":1,"createdRows":1,"updatedRows":0,"skippedRows":0,"duplicateRows":0,"errorRows":0}}`,
		"/imports/" + job + "/rows":           `{"data":{"rows":[],"meta":{"page":1,"per_page":100,"total":0},"tabs":{"all":1,"success":1,"error":0,"warning":0}}}`,
		"/imports/" + job + "/launch-monitor": `{"data":{"stage":"complete"}}`,
		"/imports/" + job + "/side-effects":   `{"data":{"total":1,"completed":1,"failed":0,"retryableFailed":0,"exhaustedFailed":0,"queued":0,"held":0,"actionRequired":false,"isComplete":true}}`,
	}
	for _, cmd := range [][]string{{"imports", "blame"}, {"imports", "watch"}} {
		out, srv, err := runLiveGateRoot(t, routes, append(cmd, "--latest", "--json")...)
		if err != nil {
			t.Fatalf("%v --latest: %v", cmd, err)
		}
		if !strings.Contains(out, job) {
			t.Fatalf("%v --latest: output %q does not name the newest job", cmd, out)
		}
		paths := srv.seen()
		if len(paths) < 2 || paths[0] != "/api/v1/imports" || paths[1] != "/api/v1/imports/"+job {
			t.Fatalf("%v --latest: requests %v, want the list then the newest job", cmd, paths)
		}
	}
	out, _, err := runLiveGateRoot(t, map[string]string{"/imports": `{"data":[],"meta":{}}`}, "imports", "blame", "--latest", "--json")
	if err != nil || !strings.Contains(out, "no import job yet") {
		t.Fatalf("no jobs: err=%v out=%q, want the exit-0 'no import job yet' answer", err, out)
	}
	if _, _, err := runLiveGateRoot(t, routes, "imports", "blame", job, "--latest"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("jobId plus --latest: err=%v, want a usage error", err)
	}
}

func TestVerifyLineTypeEstimateUnderDryRunCarriesTheMarker(t *testing.T) {
	// novel1219: the estimate falls back to a live count when the mirror is missing (the public entry's behaviour),
	// so --dry-run answers the dry_run object without any request, and a plain estimate reads /contacts.
	db := filepath.Join(t.TempDir(), "data.db")
	out, srv, err := runLiveGateRoot(t, nil, "contacts", "verify-line-type", "--estimate", "--db", db, "--dry-run", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if json.Unmarshal([]byte(out), &got) != nil || got["dry_run"] != true || got["action"] != "contacts verify-line-type" {
		t.Fatalf("estimate under --dry-run = %q, want the dry_run object", out)
	}
	if len(srv.seen()) != 0 {
		t.Fatal("--estimate --dry-run must not call the API")
	}
	out, _, err = runLiveGateRoot(t, map[string]string{"/contacts": `{"data":[],"meta":{"total":0}}`}, "contacts", "verify-line-type", "--estimate", "--db", db, "--json")
	if err != nil || strings.Contains(out, "dry_run") || !strings.Contains(compactTestJSON(out), `"unverified":0`) {
		t.Fatalf("plain estimate: err=%v out=%q, want the estimate without a dry_run marker", err, out)
	}
}

func TestTailEmptyPageEmitsOneEvent(t *testing.T) {
	var buf bytes.Buffer
	getter := fakeTailGetter(`{"data":[]}`)
	if err := fetchAndEmit(context.Background(), getter, "/messages", resourceReadConfigFor("messages"), json.NewEncoder(&buf)); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var ev map[string]any
	if len(lines) != 1 || json.Unmarshal([]byte(lines[0]), &ev) != nil || ev["event"] != "empty" {
		t.Fatalf("empty page emitted %q, want one {event: empty} line", buf.String())
	}
}

type fakeTailGetter string

func (f fakeTailGetter) Get(context.Context, string, map[string]string) (json.RawMessage, error) {
	return json.RawMessage(f), nil
}

func TestLocalDataPurgeDryRunNamesItsAction(t *testing.T) {
	out, _, err := runLiveGateRoot(t, nil, "local-data", "purge", "--dry-run", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if json.Unmarshal([]byte(out), &got) != nil || got["dry_run"] != true || got["action"] != "local-data purge" {
		t.Fatalf("purge --dry-run = %q, want dry_run true and action", out)
	}
}
