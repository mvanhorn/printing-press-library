// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Regression tests for per-command --data-source ownership. The bug fixed
// on 2026-10-05: one shared field held every command's --data-source, and
// pflag wrote each default into it at registration, so every command ran
// with the default of whichever registered last (sync's "live").

package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/uber-jobs/internal/uberjobs"
)

// TestUJCDataSourceDefaultIsPerCommand pins each command's own default; a
// shared binding would make them all equal.
func TestUJCDataSourceDefaultIsPerCommand(t *testing.T) {
	ujcIsolate(t, "", "")
	root := RootCmd()
	want := map[string]string{
		"postings": "auto", "get": "auto", "check": "auto", "new": "auto",
		"screen": "auto", "stats": "auto", "facets": "auto", "sync": "live",
	}
	for name, def := range want {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("command %q not found: %v", name, err)
		}
		fl := cmd.Flags().Lookup("data-source")
		if fl == nil {
			t.Fatalf("%s has no --data-source flag", name)
		}
		if fl.DefValue != def {
			t.Errorf("%s --data-source default = %q, want %q", name, fl.DefValue, def)
		}
		if fl.Value.String() != def {
			t.Errorf("%s --data-source value before parsing = %q, want %q (a shared binding leaks another command's default)", name, fl.Value.String(), def)
		}
	}
	// save and searches are local-only: they must not grow the flag.
	for _, name := range []string{"save", "searches"} {
		cmd, _, _ := root.Find([]string{name})
		if cmd.Flags().Lookup("data-source") != nil {
			t.Errorf("%s registers --data-source but only ever uses the local store", name)
		}
	}
	if root.PersistentFlags().Lookup("data-source") != nil {
		t.Errorf("root registers a persistent --data-source; each command must own its flag")
	}
}

// ujcDryRun decodes the --dry-run --json report.
func ujcDryRun(t *testing.T, r ujcRunResult) dryRunResult {
	t.Helper()
	ujcWantCode(t, r, 0)
	var got dryRunResult
	if err := json.Unmarshal([]byte(r.Stdout), &got); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, r.Stdout)
	}
	if !got.DryRun {
		t.Fatalf("dry_run = false in %s", r.Stdout)
	}
	return got
}

// TestUJCCheckDryRunWithoutFlagUsesLocalStore is the exact regression: with
// a fresh complete sync and no --data-source, check resolves auto to the
// local store. Under the bug it inherited sync's "live" and planned a read.
func TestUJCCheckDryRunWithoutFlagUsesLocalStore(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now().Add(-time.Minute))

	got := ujcDryRun(t, ujcRun(t, "", "check", ujcIDNewestNLD, "--dry-run", "--json"))
	if !strings.Contains(got.Action, "from the local store at "+ujcDefaultDB()) {
		t.Fatalf("check dry-run without --data-source = %q, want it answered from the local store at %s", got.Action, ujcDefaultDB())
	}
	if !strings.Contains(got.Action, "last complete sync") || strings.Contains(got.Action, "GET ") {
		t.Fatalf("check dry-run action = %q, want the local sync named and no request planned", got.Action)
	}

	// Control: an explicit live source plans the real two-request read.
	live := ujcDryRun(t, ujcRun(t, "", "check", ujcIDNewestNLD, "--dry-run", "--json", "--data-source", "live"))
	if !strings.Contains(live.Action, "GET "+site.URL+"/api/jobs/search/?page=1&pagesize=1") || strings.Contains(live.Action, "local store") {
		t.Fatalf("check --data-source live dry-run = %q, want the size probe against the fake", live.Action)
	}
	if n := site.Count(""); n != 0 {
		t.Fatalf("dry-runs sent %d requests, want 0: %v", n, site.Requests())
	}
}

// TestUJCDryRunDefaultsAcrossCommands checks the same ownership for the
// other local-first commands and for sync, whose default stays live.
func TestUJCDryRunDefaultsAcrossCommands(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now().Add(-time.Minute))

	stats := ujcDryRun(t, ujcRun(t, "", "stats", "--by", "country", "--dry-run", "--json"))
	if !strings.HasPrefix(stats.Action, "read postings from the local store at "+ujcDefaultDB()) {
		t.Errorf("stats dry-run without --data-source = %q, want the local store", stats.Action)
	}
	screen := ujcDryRun(t, ujcRun(t, "", "screen", "--country", "NLD", "--exclude", "fluent Dutch", "--dry-run", "--json"))
	if !strings.HasPrefix(screen.Action, "read postings from the local store at "+ujcDefaultDB()) {
		t.Errorf("screen dry-run without --data-source = %q, want the local store", screen.Action)
	}
	sync := ujcDryRun(t, ujcRun(t, "", "sync", "--dry-run", "--json"))
	if !strings.HasPrefix(sync.Action, "GET "+site.URL+"/api/jobs/search/?page=1&pagesize=1") {
		t.Errorf("sync dry-run = %q, want a live read of the fake", sync.Action)
	}
	if n := site.Count(""); n != 0 {
		t.Fatalf("dry-runs sent %d requests, want 0: %v", n, site.Requests())
	}
}

// TestUJCSyncClosesOnlyOnCompleteRead guards B8 through the sync command:
// an incomplete read and an Oracle fallback read close nothing, only a
// complete whole-corpus read marks absent postings closed, and a fallback
// read keeps the descriptions already stored. About 9 paced requests.
func TestUJCSyncClosesOnlyOnCompleteRead(t *testing.T) {
	site, oracle := ujcNewFake(t), ujcNewFake(t)
	ujcIsolate(t, site.URL, oracle.URL)
	ujcWantCode(t, ujcRun(t, "", "sync", "a", "b", "--json"), 2)
	ujcWantCode(t, ujcRun(t, "", "sync", "--max-pages", "-1", "--json"), 2)
	ujcWantCode(t, ujcRun(t, "", "sync", "--data-source", "local", "--json"), 2)

	sync := func() (map[string]any, map[string]any) {
		t.Helper()
		r := ujcRun(t, "", "sync", "--facets=false", "--json", "--no-cache")
		ujcWantCode(t, r, 0)
		env := ujcEnvelope(t, r.Stdout)
		return ujcRows(env)[0], ujcMeta(env)
	}
	local := func(id string) ujcRunResult {
		t.Helper()
		return ujcRun(t, "", "get", id, "--data-source", "local", "--json")
	}

	row, _ := sync()
	if row["inserted"] != float64(56) || row["closed_marked"] != float64(0) || row["scope"] != "all" || row["complete"] != true {
		t.Fatalf("first sync = %v, want 56 inserted into scope all", row)
	}

	site.Drop(ujcIDNewestNLD)
	site.SetTotalBump(2)
	row, meta := sync()
	if row["complete"] != false || row["closed_marked"] != float64(0) || meta["complete"] != false {
		t.Fatalf("incomplete sync = %v (meta %v), want no closures", row, meta)
	}
	ujcWantCode(t, local(ujcIDNewestNLD), 0)

	site.SetTotalBump(0)
	row, _ = sync()
	if row["complete"] != true || row["closed_marked"] != float64(1) {
		t.Fatalf("complete sync = %v, want exactly 1 closure", row)
	}
	ujcWantCode(t, local(ujcIDNewestNLD), 3)

	// The site refuses: Oracle answers, lacks one posting, closes nothing,
	// and the stored descriptions survive its description-less rows.
	site.SetMode("challenge")
	oracle.Drop(ujcIDMidNLD)
	row, meta = sync()
	if meta["source"] != uberjobs.SourceOracle || meta["fallback"] != true || row["scope"] != "all-oracle" || row["closed_marked"] != float64(0) {
		t.Fatalf("fallback sync = %v (meta %v), want an all-oracle read with no closures", row, meta)
	}
	ujcWantCode(t, local(ujcIDMidNLD), 0)
	r := local(ujcIDFirstRow)
	ujcWantCode(t, r, 0)
	got := ujcRows(ujcEnvelope(t, r.Stdout))[0]
	if d, _ := got["description"].(string); len(d) < 200 || got["source"] != uberjobs.SourceSite {
		t.Fatalf("after a fallback sync %s has description %q source %v, want the stored site row kept", ujcIDFirstRow, d, got["source"])
	}
	if site.Count("") != 7 || oracle.Count("") != 2 {
		t.Fatalf("site saw %d and Oracle %d requests, want 7 and 2", site.Count(""), oracle.Count(""))
	}
}

// TestUJCPostingsDefaultFallsBackToLocal runs postings with no flag while
// the site refuses: auto falls back to the populated store. Under the bug
// postings ran as live and exited 7 with no fallback.
func TestUJCPostingsDefaultFallsBackToLocal(t *testing.T) {
	site := ujcNewFake(t)
	ujcIsolate(t, site.URL, "")
	ujcSeedFullSync(t, ujcDefaultDB(), site, time.Now().Add(-time.Hour))
	site.SetMode("challenge")

	r := ujcRun(t, "", "postings", "--country", "GBR", "--json")
	ujcWantCode(t, r, 0)
	env := ujcEnvelope(t, r.Stdout)
	meta := ujcMeta(env)
	if meta["source"] != uberjobs.SourceLocal || meta["fallback"] != true {
		t.Fatalf("meta = %v, want source local with fallback true", meta)
	}
	if from, _ := meta["fallback_from"].(string); !strings.Contains(from, "refused") {
		t.Fatalf("meta.fallback_from = %q, want the refusal that caused the fallback", from)
	}
	rows := ujcRows(env)
	if len(rows) != 7 || ujcInt(t, env["hits"]) != 7 {
		t.Fatalf("local fallback returned %d rows (hits %v), want the 7 GBR postings", len(rows), env["hits"])
	}
	for _, row := range rows {
		if row["country_code"] != "GBR" {
			t.Fatalf("row %v has country_code %v, want GBR", row["id"], row["country_code"])
		}
	}
	if n := site.Count("/api/jobs/search/"); n != 1 {
		t.Fatalf("site saw %d search requests, want exactly 1 (a refusal is never retried)", n)
	}

	// Control: an explicit live source refuses instead of reading the store.
	live := ujcRun(t, "", "postings", "--country", "GBR", "--json", "--data-source", "live")
	ujcWantCode(t, live, 7)
	if live.Stdout != "" {
		t.Fatalf("a refused live read printed output: %s", live.Stdout)
	}
	if n := site.Count("/api/jobs/search/"); n != 1 {
		t.Fatalf("the latched site saw %d search requests, want still 1", n)
	}
}
