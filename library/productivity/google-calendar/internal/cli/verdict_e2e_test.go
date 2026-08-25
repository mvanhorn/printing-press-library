// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// End-to-end behavior tests for the verdict command family: a fake gauth
// config dir plus an httptest upstream exercise the real command → client →
// engine path, pinning the exit-code contract (0 clean, 3 findings, 4
// degraded coverage) and the no-cache freshness rule.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// writeVerdictFixtureAuthDir builds a gauth config dir with two profiles
// (personal, work), fresh non-expiring tokens, a parseable installed-app
// client.json, and a two-calendar manifest.
func writeVerdictFixtureAuthDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"profiles.yaml": `accounts:
  - {name: personal, email: personal@example.com, role: writable, default_write: true}
  - {name: work, email: work@example.com, role: readonly}
`,
		"client.json": `{"installed":{"client_id":"test.apps.googleusercontent.com","client_secret":"test-secret","redirect_uris":["http://localhost"],"auth_uri":"https://accounts.google.com/o/oauth2/auth","token_uri":"https://oauth2.googleapis.com/token"}}`,
		"calendars.yaml": `calendars:
  - {account: personal, id: personal@example.com, role: write, default_write: true}
  - {account: work, id: work@example.com, role: read}
`,
		"tokens/personal.json": `{"access_token":"tok-personal","token_type":"Bearer","refresh_token":"r1","expiry":"2099-01-01T00:00:00Z"}`,
		"tokens/work.json":     `{"access_token":"tok-work","token_type":"Bearer","refresh_token":"r2","expiry":"2099-01-01T00:00:00Z"}`,
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func setVerdictTestEnv(t *testing.T, baseURL string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // isolate config/cache/state
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("GOOGLE_CALENDAR_HOME", "")
	t.Setenv("GOOGLE_CALENDAR_STATE_DIR", "")
	t.Setenv("GCAL_CONFIG_DIR", "")
	t.Setenv("GOOGLE_CALENDAR_BASE_URL", baseURL)
	// Disable client retries (the degraded-coverage case returns 500s and
	// must not sleep through exponential backoff).
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
}

func runVerdictCommand(t *testing.T, args ...string) (stdout string, err error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(append(args, "--no-learn"))
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	err = cmd.Execute()
	return out.String(), err
}

func eventsPage(items ...string) string {
	return fmt.Sprintf(`{"kind":"calendar#events","etag":"\"list-etag\"","updated":"2026-08-16T12:00:00.000Z","items":[%s]}`, strings.Join(items, ","))
}

// TestConflictsE2E_CleanWindowExitsZeroWithCompleteCoverage is the
// absence-of-correctness acceptance case: every source returns zero events →
// empty conflicts array, exit 0, coverage complete.
func TestConflictsE2E_CleanWindowExitsZeroWithCompleteCoverage(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/events") {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt64(&hits, 1)
		if got := r.URL.Query().Get("singleEvents"); got != "true" {
			t.Errorf("singleEvents param = %q, want true (RRULEs must be expanded upstream, never locally)", got)
		}
		fmt.Fprint(w, eventsPage())
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "conflicts", "--auth-dir", authDir,
		"--from", "2026-08-18", "--to", "2026-08-25", "--json")
	if err != nil {
		t.Fatalf("clean window must exit 0, got: %v", err)
	}
	var out struct {
		Conflicts []json.RawMessage `json:"conflicts"`
		Coverage  struct {
			Checked  int  `json:"checked"`
			Of       int  `json:"of"`
			Complete bool `json:"complete"`
		} `json:"coverage"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	if out.Conflicts == nil || len(out.Conflicts) != 0 {
		t.Errorf("conflicts must be an empty array, got %s", stdout)
	}
	if !out.Coverage.Complete || out.Coverage.Checked != 2 || out.Coverage.Of != 2 {
		t.Errorf("coverage must be complete 2/2, got %+v", out.Coverage)
	}
	firstRunHits := atomic.LoadInt64(&hits)
	if firstRunHits < 2 {
		t.Fatalf("expected at least one events request per manifest calendar, got %d", firstRunHits)
	}

	// Freshness rule: a second identical run must hit upstream again (the
	// verdict read path bypasses the response cache).
	if _, err := runVerdictCommand(t, "conflicts", "--auth-dir", authDir,
		"--from", "2026-08-18", "--to", "2026-08-25", "--json"); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if got := atomic.LoadInt64(&hits); got != firstRunHits*2 {
		t.Errorf("verdict reads must bypass the response cache: first run %d hits, cumulative %d (want %d)", firstRunHits, got, firstRunHits*2)
	}
}

// TestConflictsE2E_OverlapExitsThree pins exit 3 with a real double-booking
// across accounts, and that a same-time same-title pair lands in mirrors
// instead.
func TestConflictsE2E_OverlapExitsThree(t *testing.T) {
	personal := eventsPage(
		`{"id":"p1","etag":"\"e1\"","status":"confirmed","summary":"Dentist","updated":"2026-08-16T12:00:00Z","start":{"dateTime":"2026-08-18T10:00:00-06:00"},"end":{"dateTime":"2026-08-18T11:00:00-06:00"}}`,
		`{"id":"p2","etag":"\"e2\"","status":"confirmed","summary":"Family sync","updated":"2026-08-16T12:00:00Z","start":{"dateTime":"2026-08-19T09:00:00-06:00"},"end":{"dateTime":"2026-08-19T10:00:00-06:00"}}`,
	)
	work := eventsPage(
		// Same instant as p1 (16:00Z == 10:00-06:00), different title → conflict.
		`{"id":"w1","etag":"\"e3\"","status":"tentative","summary":"Sprint review","updated":"2026-08-16T12:00:00Z","start":{"dateTime":"2026-08-18T16:00:00Z"},"end":{"dateTime":"2026-08-18T17:00:00Z"}}`,
		// Same instant AND same title as p2 → mirror, not conflict.
		`{"id":"w2","etag":"\"e4\"","status":"confirmed","summary":"family sync","updated":"2026-08-16T12:00:00Z","start":{"dateTime":"2026-08-19T15:00:00Z"},"end":{"dateTime":"2026-08-19T16:00:00Z"}}`,
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "personal@example.com"):
			fmt.Fprint(w, personal)
		case strings.Contains(r.URL.Path, "work@example.com"):
			fmt.Fprint(w, work)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "conflicts", "--auth-dir", authDir,
		"--from", "2026-08-18", "--to", "2026-08-25", "--json")
	if err == nil {
		t.Fatalf("overlapping busy events must exit non-zero; stdout: %s", stdout)
	}
	if code := ExitCode(err); code != 3 {
		t.Fatalf("conflicts-found exit code = %d, want 3 (err: %v)", code, err)
	}
	var out struct {
		Conflicts []struct {
			A struct {
				Account string `json:"account"`
			} `json:"a"`
			B struct {
				Account string `json:"account"`
			} `json:"b"`
			OverlapStart string `json:"overlap_start"`
		} `json:"conflicts"`
		Mirrors  []json.RawMessage `json:"mirrors"`
		Coverage struct {
			Complete bool `json:"complete"`
		} `json:"coverage"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	if len(out.Conflicts) != 1 {
		t.Fatalf("want exactly 1 conflict (the mirror must not double-count), got %d in %s", len(out.Conflicts), stdout)
	}
	if out.Conflicts[0].OverlapStart != "2026-08-18T16:00:00Z" {
		t.Errorf("overlap_start = %q, want 2026-08-18T16:00:00Z (tentative counts busy, offsets normalized)", out.Conflicts[0].OverlapStart)
	}
	if len(out.Mirrors) != 1 {
		t.Errorf("want 1 mirror pair, got %d", len(out.Mirrors))
	}
	if !out.Coverage.Complete {
		t.Errorf("both sources answered; coverage must be complete")
	}
}

// TestConflictsE2E_SourceErrorExitsFour pins the degraded-coverage contract:
// one calendar 500s → exit 4, complete=false, checked==M-1.
func TestConflictsE2E_SourceErrorExitsFour(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "work@example.com") {
			http.Error(w, `{"error":{"code":500,"message":"backend"}}`, http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, eventsPage())
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "conflicts", "--auth-dir", authDir,
		"--from", "2026-08-18", "--to", "2026-08-25", "--json")
	if err == nil {
		t.Fatalf("incomplete coverage must not exit 0; stdout: %s", stdout)
	}
	if code := ExitCode(err); code != 4 {
		t.Fatalf("degraded-coverage exit code = %d, want 4 (err: %v)", code, err)
	}
	var out struct {
		Coverage struct {
			Checked  int  `json:"checked"`
			Of       int  `json:"of"`
			Complete bool `json:"complete"`
			Sources  []struct {
				Calendar string `json:"calendar"`
				Error    string `json:"error"`
			} `json:"sources"`
		} `json:"coverage"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	if out.Coverage.Complete || out.Coverage.Checked != 1 || out.Coverage.Of != 2 {
		t.Errorf("coverage = %+v, want checked 1 of 2 complete=false", out.Coverage)
	}
	foundErr := false
	for _, s := range out.Coverage.Sources {
		if s.Calendar == "work@example.com" && s.Error != "" {
			foundErr = true
		}
	}
	if !foundErr {
		t.Errorf("the failed source must carry its error in coverage.sources: %s", stdout)
	}
}

// TestSlotsE2E_FreeBusyInversion drives slots end-to-end over a fixture
// freeBusy response and pins the ranked inversion plus exit 0 on complete
// coverage.
func TestSlotsE2E_FreeBusyInversion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/freeBusy" || r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Items []struct {
				ID string `json:"id"`
			} `json:"items"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		cals := map[string]any{}
		for _, item := range body.Items {
			if item.ID == "personal@example.com" {
				cals[item.ID] = map[string]any{"busy": []map[string]string{
					{"start": "2026-08-18T10:00:00Z", "end": "2026-08-18T11:00:00Z"},
					{"start": "2026-08-18T13:00:00Z", "end": "2026-08-18T14:00:00Z"},
				}}
			} else {
				cals[item.ID] = map[string]any{"busy": []map[string]string{}}
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"kind": "calendar#freeBusy", "calendars": cals})
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "slots", "--auth-dir", authDir,
		"--duration", "90m",
		"--from", "2026-08-18T00:00:00Z", "--to", "2026-08-19T00:00:00Z",
		"--between", "09:00-17:00", "--tz", "UTC", "--json")
	if err != nil {
		t.Fatalf("complete coverage with found slots must exit 0, got: %v\n%s", err, stdout)
	}
	var out struct {
		Slots []struct {
			Start           string `json:"start"`
			End             string `json:"end"`
			DurationMinutes int    `json:"duration_minutes"`
		} `json:"slots"`
		Coverage struct {
			Complete bool `json:"complete"`
			Checked  int  `json:"checked"`
			Of       int  `json:"of"`
		} `json:"coverage"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	if len(out.Slots) != 2 {
		t.Fatalf("want 2 qualifying slots (14-17 and 11-13), got %d: %s", len(out.Slots), stdout)
	}
	if out.Slots[0].Start != "2026-08-18T14:00:00Z" || out.Slots[0].DurationMinutes != 180 {
		t.Errorf("first slot must be the 180m 14:00Z block (longest first), got %+v", out.Slots[0])
	}
	if out.Slots[1].Start != "2026-08-18T11:00:00Z" || out.Slots[1].DurationMinutes != 120 {
		t.Errorf("second slot must be the 120m 11:00Z block, got %+v", out.Slots[1])
	}
	if !out.Coverage.Complete || out.Coverage.Checked != 2 {
		t.Errorf("coverage must be complete 2/2, got %+v", out.Coverage)
	}
}

// TestChangesE2E_CancelledClassification drives changes end-to-end and pins
// change_kind classification plus updated-ascending ordering.
func TestChangesE2E_CancelledClassification(t *testing.T) {
	personal := eventsPage(
		`{"id":"p1","etag":"\"e1\"","status":"cancelled","summary":"Dropped mtg","updated":"2026-08-17T09:00:00Z","start":{"dateTime":"2026-08-18T10:00:00Z"},"end":{"dateTime":"2026-08-18T11:00:00Z"}}`,
		`{"id":"p2","etag":"\"e2\"","status":"confirmed","summary":"New mtg","updated":"2026-08-17T08:00:00Z","start":{"dateTime":"2026-08-19T10:00:00Z"},"end":{"dateTime":"2026-08-19T11:00:00Z"}}`,
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("showDeleted"); got != "true" {
			t.Errorf("showDeleted = %q, want true (deletions must be included)", got)
		}
		if strings.Contains(r.URL.Path, "personal@example.com") {
			fmt.Fprint(w, personal)
			return
		}
		fmt.Fprint(w, eventsPage())
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "changes", "--auth-dir", authDir,
		"--since", "2026-08-17T00:00:00Z", "--json")
	if err != nil {
		t.Fatalf("changes with complete coverage must exit 0, got: %v\n%s", err, stdout)
	}
	var out struct {
		Changes []struct {
			ID         string `json:"id"`
			ChangeKind string `json:"change_kind"`
			Updated    string `json:"updated"`
		} `json:"changes"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	if len(out.Changes) != 2 {
		t.Fatalf("want 2 change rows, got %d: %s", len(out.Changes), stdout)
	}
	if out.Changes[0].ID != "p2" || out.Changes[1].ID != "p1" {
		t.Errorf("changes must sort by updated ascending (p2 08:00 before p1 09:00), got %+v", out.Changes)
	}
	for _, c := range out.Changes {
		want := "new_or_updated"
		if c.ID == "p1" {
			want = "cancelled"
		}
		if c.ChangeKind != want {
			t.Errorf("change_kind for %s = %q, want %q", c.ID, c.ChangeKind, want)
		}
	}
}

// TestVerdictCommandsMissingProfilesActionableError pins the contract that a
// test environment without profiles.yaml gets gauth's actionable error from
// every verdict command — never a panic, never a generic failure.
func TestVerdictCommandsMissingProfilesActionableError(t *testing.T) {
	emptyDir := t.TempDir()
	setVerdictTestEnv(t, "http://127.0.0.1:0")

	cases := [][]string{
		{"conflicts", "--from", "2026-08-18", "--to", "2026-08-19"},
		{"slots", "--duration", "90m", "--from", "2026-08-18", "--to", "2026-08-19"},
		{"changes", "--since", "2026-08-17T00:00:00Z"},
		{"events", "exceptions", "--from", "2026-08-18", "--to", "2026-08-19"},
		{"events", "update", "cal@example.com", "evt1", "--summary", "x"},
		{"manifest", "check"},
	}
	for _, args := range cases {
		args := args
		t.Run(strings.Join(args[:2], "_"), func(t *testing.T) {
			_, err := runVerdictCommand(t, append(args, "--auth-dir", emptyDir)...)
			if err == nil {
				t.Fatalf("%v must fail without profiles.yaml", args)
			}
			if !strings.Contains(err.Error(), "profiles not found") {
				t.Errorf("%v error must be gauth's actionable message, got: %v", args, err)
			}
		})
	}
}

// TestVerdictCommandsDryRunExitZero pins the definition-of-done requirement
// that --dry-run exits 0 for every novel command, with no auth material at
// all.
func TestVerdictCommandsDryRunExitZero(t *testing.T) {
	setVerdictTestEnv(t, "http://127.0.0.1:0")
	cases := [][]string{
		{"conflicts", "--dry-run"},
		{"slots", "--dry-run"},
		{"changes", "--dry-run"},
		{"events", "exceptions", "--dry-run"},
		{"events", "update", "--dry-run"},
		{"events", "update", "cal@example.com", "evt1", "--summary", "x", "--dry-run"},
		{"manifest", "check", "--dry-run"},
	}
	for _, args := range cases {
		args := args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			if _, err := runVerdictCommand(t, args...); err != nil {
				t.Errorf("%v must exit 0 under --dry-run, got: %v", args, err)
			}
		})
	}
}

// TestManifestCheckE2E_FindingsAndSkeleton drives manifest check end-to-end:
// a manifested-but-vanished calendar and a permission-lost calendar produce
// findings (exit 3); --emit-skeleton prints adoptable YAML.
func TestManifestCheckE2E_FindingsAndSkeleton(t *testing.T) {
	calendarList := func(entries string) string {
		return fmt.Sprintf(`{"kind":"calendar#calendarList","etag":"\"cl-etag\"","items":[%s]}`, entries)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/users/me/calendarList":
			auth := r.Header.Get("Authorization")
			if strings.Contains(auth, "tok-personal") {
				// personal@example.com manifested role write, but live accessRole reader → role_insufficient.
				// extra@example.com is live but unmanifested → informational.
				fmt.Fprint(w, calendarList(`{"id":"personal@example.com","summary":"Personal","accessRole":"reader","primary":true},{"id":"extra@example.com","summary":"Extra","accessRole":"owner"}`))
			} else {
				// work@example.com manifested but absent live → missing_upstream.
				fmt.Fprint(w, calendarList(`{"id":"other@example.com","summary":"Other","accessRole":"reader"}`))
			}
		case strings.HasPrefix(r.URL.Path, "/users/me/settings/"):
			fmt.Fprint(w, `{"kind":"calendar#setting","id":"timezone","value":"America/Denver"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "manifest", "check", "--auth-dir", authDir, "--json")
	if err == nil {
		t.Fatalf("manifest drift must exit non-zero; stdout: %s", stdout)
	}
	if code := ExitCode(err); code != 3 {
		t.Fatalf("findings exit code = %d, want 3 (err: %v)", code, err)
	}
	var out struct {
		Findings []struct {
			Kind     string `json:"kind"`
			Calendar string `json:"calendar"`
		} `json:"findings"`
		Unmanifested []struct {
			Calendar string `json:"calendar"`
		} `json:"unmanifested"`
		Timezones []struct {
			Timezone string `json:"timezone"`
		} `json:"timezones"`
		Coverage struct {
			Complete bool `json:"complete"`
		} `json:"coverage"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	kinds := map[string]string{}
	for _, f := range out.Findings {
		kinds[f.Calendar] = f.Kind
	}
	if kinds["personal@example.com"] != "role_insufficient" {
		t.Errorf("declared write over live reader must be role_insufficient, got %+v", out.Findings)
	}
	if kinds["work@example.com"] != "missing_upstream" {
		t.Errorf("manifested-but-absent calendar must be missing_upstream, got %+v", out.Findings)
	}
	if len(out.Unmanifested) == 0 || out.Unmanifested[0].Calendar != "extra@example.com" {
		t.Errorf("live-but-unmanifested calendar must be reported informationally, got %+v", out.Unmanifested)
	}
	if len(out.Timezones) != 2 || out.Timezones[0].Timezone != "America/Denver" {
		t.Errorf("per-account timezone must be reported, got %+v", out.Timezones)
	}
	if !out.Coverage.Complete {
		t.Errorf("both accounts answered; coverage must be complete")
	}

	skeleton, err := runVerdictCommand(t, "manifest", "check", "--auth-dir", authDir, "--emit-skeleton")
	if err != nil {
		t.Fatalf("--emit-skeleton with complete coverage must exit 0, got: %v", err)
	}
	for _, want := range []string{"calendars:", "extra@example.com", "role: write", "role: read", "account: personal"} {
		if !strings.Contains(skeleton, want) {
			t.Errorf("skeleton missing %q:\n%s", want, skeleton)
		}
	}
}

// TestEventsUpdateE2E_PreImageEtagAndUndo drives the safe-mutation flow: GET
// pre-image, PATCH with If-Match, envelope carrying prior + inverse patch.
func TestEventsUpdateE2E_PreImageEtagAndUndo(t *testing.T) {
	const eventPath = "/calendars/personal@example.com/events/evt123"
	var patchIfMatch, patchSendUpdates string
	var patchBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != eventPath {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"kind":"calendar#event","id":"evt123","etag":"\"etag-prior\"","status":"confirmed","summary":"Old title","start":{"dateTime":"2026-08-19T15:00:00-06:00"},"end":{"dateTime":"2026-08-19T16:00:00-06:00"}}`)
		case http.MethodPatch:
			patchIfMatch = r.Header.Get("If-Match")
			patchSendUpdates = r.URL.Query().Get("sendUpdates")
			patchBody, _ = readAll(r)
			fmt.Fprint(w, `{"kind":"calendar#event","id":"evt123","etag":"\"etag-after\"","status":"confirmed","summary":"New title","start":{"dateTime":"2026-08-19T15:00:00-06:00"},"end":{"dateTime":"2026-08-19T16:00:00-06:00"}}`)
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "events", "update", "personal@example.com", "evt123",
		"--summary", "New title", "--auth-dir", authDir, "--json")
	if err != nil {
		t.Fatalf("update must succeed, got: %v\n%s", err, stdout)
	}
	if patchIfMatch != `"etag-prior"` {
		t.Errorf("PATCH If-Match = %q, want the pre-image etag %q", patchIfMatch, `"etag-prior"`)
	}
	if patchSendUpdates != "none" {
		t.Errorf("safety barrier must force sendUpdates=none, got %q", patchSendUpdates)
	}
	if !bytes.Contains(patchBody, []byte(`"summary":"New title"`)) {
		t.Errorf("PATCH body must carry only the changed field, got %s", patchBody)
	}
	var out struct {
		Result struct {
			Etag string `json:"etag"`
		} `json:"result"`
		Prior struct {
			Summary string `json:"summary"`
		} `json:"prior"`
		Undo struct {
			Op   string         `json:"op"`
			Path string         `json:"path"`
			Body map[string]any `json:"body"`
		} `json:"undo"`
		EtagUsed string `json:"etag_used"`
		Blind    bool   `json:"blind"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	if out.Prior.Summary != "Old title" {
		t.Errorf("prior must carry the pre-image, got %+v", out.Prior)
	}
	if out.Undo.Op != "patch" || out.Undo.Path != eventPath || out.Undo.Body["summary"] != "Old title" {
		t.Errorf("undo must be the inverse patch restoring the prior summary, got %+v", out.Undo)
	}
	if out.EtagUsed != `"etag-prior"` || out.Blind {
		t.Errorf("etag_used/blind = %q/%v, want the prior etag and blind=false", out.EtagUsed, out.Blind)
	}
	if out.Result.Etag != `"etag-after"` {
		t.Errorf("result must be the post-write event, got %+v", out.Result)
	}
}

// TestEventsUpdateE2E_AttendeeBarrierRefusal pins that the client safety
// barrier's refusal for attendee-bearing events surfaces cleanly.
func TestEventsUpdateE2E_AttendeeBarrierRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"kind":"calendar#event","id":"evt123","etag":"\"e\"","summary":"Board mtg","attendees":[{"email":"other@example.com","responseStatus":"accepted"}],"start":{"dateTime":"2026-08-19T15:00:00Z"},"end":{"dateTime":"2026-08-19T16:00:00Z"}}`)
			return
		}
		t.Errorf("no mutation may reach the wire for an attendee-bearing event (got %s)", r.Method)
		http.Error(w, "must not mutate", http.StatusForbidden)
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	_, err := runVerdictCommand(t, "events", "update", "personal@example.com", "evt123",
		"--summary", "Renamed", "--auth-dir", authDir, "--json")
	if err == nil {
		t.Fatalf("attendee-bearing event must be refused")
	}
	if !strings.Contains(err.Error(), "attendee") {
		t.Errorf("refusal must surface the barrier's attendee message, got: %v", err)
	}
}

// TestEventsUpdateE2E_ManifestReadOnlyRefusal pins the manifest-role guard:
// a calendar declared role read is refused before any network write.
func TestEventsUpdateE2E_ManifestReadOnlyRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("read-only manifest role must refuse before any request (got %s %s)", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	_, err := runVerdictCommand(t, "events", "update", "work@example.com", "evt9",
		"--summary", "Nope", "--auth-dir", authDir)
	if err == nil {
		t.Fatalf("manifest role read must refuse the write")
	}
	if !strings.Contains(err.Error(), "role: read") {
		t.Errorf("refusal must cite the manifest role, got: %v", err)
	}
}

// TestExceptionsE2E_MovedAndCancelled drives events exceptions end-to-end.
func TestExceptionsE2E_MovedAndCancelled(t *testing.T) {
	personal := eventsPage(
		// Instance sitting where the rule put it → not an exception.
		`{"id":"r1_a","etag":"\"e1\"","status":"confirmed","summary":"Standup","recurringEventId":"r1","originalStartTime":{"dateTime":"2026-08-18T16:00:00Z"},"start":{"dateTime":"2026-08-18T16:00:00Z"},"end":{"dateTime":"2026-08-18T16:15:00Z"}}`,
		// Moved instance.
		`{"id":"r1_b","etag":"\"e2\"","status":"confirmed","summary":"Standup","recurringEventId":"r1","originalStartTime":{"dateTime":"2026-08-19T16:00:00Z"},"start":{"dateTime":"2026-08-19T18:00:00Z"},"end":{"dateTime":"2026-08-19T18:15:00Z"}}`,
		// Cancelled instance stub.
		`{"id":"r1_c","status":"cancelled","recurringEventId":"r1","originalStartTime":{"dateTime":"2026-08-20T16:00:00Z"}}`,
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "personal@example.com") {
			fmt.Fprint(w, personal)
			return
		}
		fmt.Fprint(w, eventsPage())
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "events", "exceptions", "--auth-dir", authDir,
		"--from", "2026-08-18", "--to", "2026-08-25", "--json")
	if err != nil {
		t.Fatalf("exceptions with complete coverage must exit 0, got: %v\n%s", err, stdout)
	}
	var out struct {
		Exceptions []struct {
			ID            string `json:"id"`
			Kind          string `json:"kind"`
			OriginalStart string `json:"original_start"`
			Start         string `json:"start"`
		} `json:"exceptions"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	if len(out.Exceptions) != 2 {
		t.Fatalf("want 2 exceptions (moved + cancelled; the on-schedule instance excluded), got %d: %s", len(out.Exceptions), stdout)
	}
	byID := map[string]string{}
	for _, e := range out.Exceptions {
		byID[e.ID] = e.Kind
	}
	if byID["r1_b"] != "moved" || byID["r1_c"] != "cancelled_instance" {
		t.Errorf("kinds = %+v, want r1_b moved and r1_c cancelled_instance", byID)
	}
	for _, e := range out.Exceptions {
		if e.ID == "r1_b" && (e.OriginalStart != "2026-08-19T16:00:00Z" || e.Start != "2026-08-19T18:00:00Z") {
			t.Errorf("moved exception must report both times, got %+v", e)
		}
	}
}

// TestConflictsE2E_PaginationFollowsNextPageToken pins that the fetch layer
// walks nextPageToken to the end: the conflicting event lives on page 2.
func TestConflictsE2E_PaginationFollowsNextPageToken(t *testing.T) {
	var personalRequests int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "personal@example.com") {
			atomic.AddInt64(&personalRequests, 1)
			if r.URL.Query().Get("pageToken") == "" {
				fmt.Fprint(w, `{"kind":"calendar#events","etag":"\"pg\"","nextPageToken":"page2","items":[{"id":"p1","etag":"\"e1\"","status":"confirmed","summary":"Morning block","updated":"2026-08-16T12:00:00Z","start":{"dateTime":"2026-08-18T09:00:00Z"},"end":{"dateTime":"2026-08-18T10:00:00Z"}}]}`)
				return
			}
			fmt.Fprint(w, eventsPage(`{"id":"p9","etag":"\"e9\"","status":"confirmed","summary":"Late add","updated":"2026-08-16T12:00:00Z","start":{"dateTime":"2026-08-18T16:00:00Z"},"end":{"dateTime":"2026-08-18T17:00:00Z"}}`))
			return
		}
		fmt.Fprint(w, eventsPage(`{"id":"w1","etag":"\"w1\"","status":"confirmed","summary":"Review","updated":"2026-08-16T12:00:00Z","start":{"dateTime":"2026-08-18T16:30:00Z"},"end":{"dateTime":"2026-08-18T17:30:00Z"}}`))
	}))
	defer srv.Close()
	authDir := writeVerdictFixtureAuthDir(t)
	setVerdictTestEnv(t, srv.URL)

	stdout, err := runVerdictCommand(t, "conflicts", "--auth-dir", authDir,
		"--from", "2026-08-18", "--to", "2026-08-19", "--json")
	if err == nil {
		t.Fatalf("page-2 event overlaps the work event; must exit non-zero. stdout: %s", stdout)
	}
	if code := ExitCode(err); code != 3 {
		t.Fatalf("exit = %d, want 3 (the conflicting event lives on page 2)", code)
	}
	if got := atomic.LoadInt64(&personalRequests); got != 2 {
		t.Errorf("pagination must fetch both pages, got %d requests", got)
	}
	var out struct {
		Conflicts []struct {
			A struct {
				ID string `json:"id"`
			} `json:"a"`
			B struct {
				ID string `json:"id"`
			} `json:"b"`
		} `json:"conflicts"`
	}
	if uerr := json.Unmarshal([]byte(stdout), &out); uerr != nil {
		t.Fatalf("stdout is not the JSON envelope: %v\n%s", uerr, stdout)
	}
	if len(out.Conflicts) != 1 || out.Conflicts[0].A.ID != "p9" {
		t.Errorf("the page-2 event must be the conflicting one, got %s", stdout)
	}
}

// readAll drains a request body without importing io in half the fixtures.
func readAll(r *http.Request) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r.Body)
	return buf.Bytes(), err
}
