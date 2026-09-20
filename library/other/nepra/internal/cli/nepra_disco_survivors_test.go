// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// Mutation-closure tests for internal/cli/nepra_disco.go. Every assertion
// below was verified by APPLYING the mutation it names, watching this file
// fail, and reverting it. Every number is one this build measured against the
// committed fixtures; none is copied out of a comment.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// nepra_disco.go:104 — `len(args) == 0 && cmd.Flags().NFlag() == 0`
// ---------------------------------------------------------------------------

// TestDiscoBarePositionalArgIsNotTheCatalogue pins the ONE input that separates
// "no selector at all" from "a positional argument and no flags": `disco foo`.
// The existing positional-argument case always passes a flag alongside it, so
// flipping the len(args) test to != leaves that case green while turning this
// one from a usage refusal into a silent exit-0 catalogue — a caller who
// mistyped `disco saidi` would be told nothing was wrong.
//
// MUTATION-VERIFIED: `len(args) != 0` on line 104 makes this exit 0 and print
// the catalogue to stdout.
func TestDiscoBarePositionalArgIsNotTheCatalogue(t *testing.T) {
	calls := discoForbidRequests(t)
	stdout, stderr, code := runDisco(t, "disco", "saidi")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (a positional argument is a usage error)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if *calls != 0 {
		t.Fatalf("%d HTTP request(s) made; want 0", *calls)
	}
	if !strings.Contains(stderr, "takes no positional arguments") {
		t.Errorf("stderr does not refuse the positional argument:\n%s", stderr)
	}
	if !strings.Contains(stderr, `"saidi"`) {
		t.Errorf("the refusal does not quote back the argument it rejected:\n%s", stderr)
	}
	// The catalogue is the other branch's answer; it must not be the answer here.
	if strings.Contains(stdout, "report_years") {
		t.Errorf("stdout carried the catalogue for a rejected invocation:\n%s", stdout)
	}
}

// ---------------------------------------------------------------------------
// nepra_disco.go:413 — `Selectable: tokened[m] != ""`
// ---------------------------------------------------------------------------

// TestDiscoCatalogueSelectableTracksTheToken pins that `selectable` means
// exactly "this metric has a --metric token". nepraper publishes 13 metric keys
// and this build exposes 6 of them; a caller reads this field to know which
// --metric values exist, so inverting it advertises the seven unselectable keys
// and hides every working one.
//
// MUTATION-VERIFIED: `tokened[m] == ""` on line 413 flips all 13 entries.
func TestDiscoCatalogueSelectableTracksTheToken(t *testing.T) {
	discoForbidRequests(t)
	// --json is a flag, so this takes the validated no-metric route to the
	// catalogue rather than the bare-invocation shortcut.
	stdout, stderr, code := runDisco(t, "disco", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	metrics, ok := discoCatalogueSections(t, stdout)["metrics"].([]any)
	if !ok {
		t.Fatalf("no metrics array in the catalogue:\n%s", stdout)
	}
	if len(metrics) != 13 {
		t.Fatalf("metrics = %d, want the 13 keys nepraper publishes", len(metrics))
	}
	selectable := map[string]bool{}
	nSelectable := 0
	for _, m := range metrics {
		e := m.(map[string]any)
		key, _ := e["key"].(string)
		token, _ := e["token"].(string)
		sel, isBool := e["selectable"].(bool)
		if !isBool {
			t.Fatalf("%s selectable = %v, want a bool", key, e["selectable"])
		}
		if sel != (token != "") {
			t.Errorf("%s: selectable = %v with token %q; selectable must mean 'has a --metric token'", key, sel, token)
		}
		if sel {
			nSelectable++
		}
		selectable[key] = sel
	}
	if nSelectable != 6 {
		t.Errorf("selectable metrics = %d, want 6 (tnd, recovery, saifi, saidi, complaints, safety)", nSelectable)
	}
	// Named both ways round so a wholesale flip cannot pass.
	for _, key := range []string{"saidi", "saifi", "td_losses", "recovery", "consumer_complaints", "safety_fatalities"} {
		if !selectable[key] {
			t.Errorf("%s is not selectable, but --metric reaches it", key)
		}
	}
	for _, key := range []string{"fault_rate", "load_shedding", "nominal_voltage", "new_connections_time_frame"} {
		if selectable[key] {
			t.Errorf("%s is advertised as selectable, but no --metric token resolves to it", key)
		}
	}
}

// ---------------------------------------------------------------------------
// nepra_disco.go:266 — `if gerr != nil` around the live fetch
// ---------------------------------------------------------------------------

// discoLiveServer serves one body for every path, recording each request, and
// points the CLI's base URL at itself.
type discoLiveServer struct {
	mu     sync.Mutex
	paths  []string
	status int
	body   []byte
	ctype  string
}

func (s *discoLiveServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.paths = append(s.paths, r.URL.Path)
		s.mu.Unlock()
		if s.ctype != "" {
			w.Header().Set("Content-Type", s.ctype)
		}
		w.WriteHeader(s.status)
		_, _ = w.Write(s.body)
	})
}

func (s *discoLiveServer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.paths)
}

// runDiscoLive drives the real command against a local server, so the fetch leg
// (which --pdf skips entirely) is exercised end to end with no network.
func runDiscoLive(t *testing.T, srv *discoLiveServer, args ...string) (string, string, int) {
	t.Helper()
	ts := httptest.NewServer(srv.handler())
	t.Cleanup(ts.Close)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("base_url = \""+ts.URL+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEPRA_CONFIG", cfgPath)
	t.Setenv("NEPRA_BASE_URL", ts.URL)
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append([]string{"disco"}, append(args, "--no-cache", "--rate-limit", "0")...))
	err := cmd.Execute()
	code := 0
	if err != nil {
		code = ExitCode(err)
		errOut.WriteString("\n" + err.Error())
	}
	return out.String(), errOut.String(), code
}

// discoSyntheticPDFBytes is the committed fixture as raw PDF bytes.
func discoSyntheticPDFBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(discoSyntheticPDF(t))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return raw
}

// TestDiscoLiveFetchErrorIsClassifiedNotParsed covers both sides of the
// fetch-error guard, which no --pdf test can reach.
//
// On success the panel must be built from the fetched bytes; on failure the
// transport error must be classified into its own exit code and never carried
// forward as if a document had arrived.
//
// MUTATION-VERIFIED: `if gerr == nil` on line 266 returns classifyAPIError(nil)
// — i.e. nil — the instant the fetch SUCCEEDS, so the success leg below exits 0
// with empty stdout; and on the 500 it falls through to the binary-envelope
// check, turning exit 5 into exit 3.
func TestDiscoLiveFetchErrorIsClassifiedNotParsed(t *testing.T) {
	t.Run("success builds the panel from the fetched bytes", func(t *testing.T) {
		srv := &discoLiveServer{status: 200, body: discoSyntheticPDFBytes(t), ctype: "application/pdf"}
		stdout, stderr, code := runDiscoLive(t, srv, "--metric", "saidi", "--fy", "2018-19", "--variant", "headline", "--json")
		if code != 0 {
			t.Fatalf("exit = %d, want 0\nstderr:\n%s", code, stderr)
		}
		if srv.count() != 1 {
			t.Fatalf("requests = %d, want 1", srv.count())
		}
		p := discoDecode(t, stdout)
		meta := p["meta"].(map[string]any)
		if meta["source"] != "live" {
			t.Errorf("meta.source = %v, want live", meta["source"])
		}
		if meta["request_made"] != true {
			t.Errorf("meta.request_made = %v, want true", meta["request_made"])
		}
		rows, ok := p["results"].([]any)
		if !ok || len(rows) == 0 {
			t.Fatalf("results = %v, want the fetched document's rows", p["results"])
		}
		// The bytes actually parsed are the fixture's, measured here rather
		// than quoted: the document block must describe what arrived.
		doc := meta["document"].(map[string]any)
		if got := int(doc["bytes"].(float64)); got != len(srv.body) {
			t.Errorf("meta.document.bytes = %d, want the %d bytes served", got, len(srv.body))
		}
	})

	t.Run("HTTP 500 is classified, not parsed as a document", func(t *testing.T) {
		// The client retries a 5xx three times with backoff unless the harness
		// env var is set. This leg is about classification, not the retry
		// schedule, so it opts out of the 7s wait.
		t.Setenv("PRINTING_PRESS_VERIFY", "1")
		srv := &discoLiveServer{status: 500, body: []byte("upstream exploded"), ctype: "text/html"}
		_, stderr, code := runDiscoLive(t, srv, "--metric", "saidi", "--fy", "2018-19", "--json")
		if code != 5 {
			t.Fatalf("exit = %d, want 5 (the classified API failure)\nstderr:\n%s", code, stderr)
		}
		if !strings.Contains(stderr, "500") {
			t.Errorf("stderr does not name the HTTP status:\n%s", stderr)
		}
		// Falling through to the PDF path reports the HTML-decoy shape, which
		// would tell the caller the report is missing rather than that the
		// request failed.
		if strings.Contains(stderr, "binary document envelope") {
			t.Errorf("a failed request was reported as a missing document:\n%s", stderr)
		}
	})
}

// ---------------------------------------------------------------------------
// nepra_disco.go:825 — `if p.Meta.Artifact != nil` in the human header
// ---------------------------------------------------------------------------

// TestDiscoHumanHeaderCitesTheArtifact pins the human-mode provenance line and
// the variant cell. discoRun always sets Meta.Artifact before rendering, so
// inverting the nil test drops the only line that says WHERE the panel's bytes
// came from; and discoDash's empty test decides whether a real variant renders
// as itself or as a dash.
//
// MUTATION-VERIFIED: `p.Meta.Artifact == nil` on line 825 removes the "source"
// line; `if s != ""` in discoDash (line 943) renders the variant, table and
// every other dashed cell as "-".
func TestDiscoHumanHeaderCitesTheArtifact(t *testing.T) {
	discoForbidRequests(t)
	path := discoSyntheticPDF(t)
	stdout, stderr, code := runDisco(t, "disco", "--metric", "saidi", "--pdf", path,
		"--fy", "2018-19", "--variant", "headline", "--human-friendly")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	// The header goes to stderr so stdout stays the table.
	if !strings.Contains(stderr, "source file  "+path) {
		t.Errorf("human header does not cite the artifact it parsed (want \"source file  %s\"):\n%s", path, stderr)
	}
	if !strings.Contains(stdout, "VARIANT") {
		t.Fatalf("no table on stdout:\n%s", stdout)
	}
	// The variant and table cells are rendered through discoDash. A present
	// value must render as itself.
	if !strings.Contains(stdout, "headline") {
		t.Errorf("the variant cell does not carry the variant name:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Table 6") {
		t.Errorf("the table cell does not carry the table label:\n%s", stdout)
	}
}

// TestDiscoDashOnlyDashesTheEmptyCell is the unit-level pin under the same
// discoDash flip: a dash means "this cell has no label", and a labelled cell
// must never be blanked into one.
//
// MUTATION-VERIFIED: `if s != ""` on line 943 fails both cases below.
func TestDiscoDashOnlyDashesTheEmptyCell(t *testing.T) {
	if got := discoDash(""); got != "-" {
		t.Errorf("discoDash(%q) = %q, want %q", "", got, "-")
	}
	for _, s := range []string{"headline", "five_year_comparison", "Table 6", " "} {
		if got := discoDash(s); got != s {
			t.Errorf("discoDash(%q) = %q, want it unchanged", s, got)
		}
	}
}
