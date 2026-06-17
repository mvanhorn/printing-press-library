// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runBriefJSON runs the brief command with the given args in --json mode and
// returns the parsed envelope. It uses fixture flags only, so newClient is
// never reached (no network).
func runBriefJSON(t *testing.T, args ...string) map[string]json.RawMessage {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	cmd := newNovelBriefCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("brief %v: %v\noutput: %s", args, err, out.String())
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("brief output not JSON: %v\noutput: %s", err, out.String())
	}
	return env
}

// TestBrief_FixtureNotLabeledLive is the provenance honesty regression
// (finding 2): brief run entirely from fixtures must NOT claim source "live".
func TestBrief_FixtureNotLabeledLive(t *testing.T) {
	storms := fixPath("currentstorms", "helene-2024.json")
	alerts := fixPath("alerts", "milton_hurricane_warning_feature_2024-10-09.json")
	outlook := fixPath("two-atl.txt")

	env := runBriefJSON(t,
		"--basin", "atl", // single basin so the one outlook fixture covers it
		"--storms-fixture", storms,
		"--alerts-fixture", alerts,
		"--outlook-fixture", outlook,
	)
	var source string
	if err := json.Unmarshal(env["source"], &source); err != nil {
		t.Fatalf("source not a string: %v", err)
	}
	if source == "live" {
		t.Fatalf("brief from fixtures labeled source=%q, want NOT live", source)
	}
	if !strings.Contains(source, "fixture") {
		t.Errorf("composite source = %q, want it to name the fixture legs", source)
	}
}

// TestBriefSource_AllLiveIsLive guards the other direction: an all-live brief
// keeps the honest "live" label.
func TestBriefSource_Composite(t *testing.T) {
	if got := briefSource([]string{"live", "live", "live"}); got != "live" {
		t.Errorf("briefSource(all live) = %q, want live", got)
	}
	if got := briefSource(nil); got != "live" {
		t.Errorf("briefSource(nil) = %q, want live", got)
	}
	got := briefSource([]string{"live", "fixture:a", "live"})
	if got == "live" || !strings.Contains(got, "fixture:a") {
		t.Errorf("briefSource(mixed) = %q, want a non-live composite naming fixture:a", got)
	}
}

// TestBrief_BestEffortBasins is the resilience regression (finding 3): in the
// default 3-basin mode a single basin's outlook failure must not nuke the
// briefing. We point the storms/alerts at good fixtures and the outlook at a
// good single fixture; because the single outlook fixture is shared across all
// basins, all basins succeed and the brief still emits. The hard-fail-only-if-
// all-fail path is covered by TestBrief_AllBasinsFail.
func TestBrief_DefaultBasinsSucceed(t *testing.T) {
	env := runBriefJSON(t,
		"--storms-fixture", fixPath("currentstorms", "helene-2024.json"),
		"--alerts-fixture", fixPath("alerts", "empty_hurricane_warning_2026-06-15.json"),
		"--outlook-fixture", fixPath("two-atl.txt"),
	)
	var data map[string]json.RawMessage
	if err := json.Unmarshal(env["data"], &data); err != nil {
		t.Fatalf("data not an object: %v", err)
	}
	var outlook map[string]json.RawMessage
	if err := json.Unmarshal(data["outlook"], &outlook); err != nil {
		t.Fatalf("outlook not an object: %v", err)
	}
	// All three basins parsed the shared fixture, so all three are present.
	for _, b := range []string{"atl", "ep", "cp"} {
		if _, ok := outlook[b]; !ok {
			t.Errorf("outlook missing basin %q", b)
		}
	}
}

// TestBrief_AllBasinsFail asserts the hard-fail path: when every basin's
// outlook leg fails (a bad fixture path shared by all basins), brief returns an
// error rather than emitting a half-built payload.
func TestBrief_AllBasinsFail(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.txt")
	flags := &rootFlags{asJSON: true}
	cmd := newNovelBriefCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--storms-fixture", fixPath("currentstorms", "helene-2024.json"),
		"--alerts-fixture", fixPath("alerts", "empty_hurricane_warning_2026-06-15.json"),
		"--outlook-fixture", missing,
	})
	if err := cmd.Execute(); err == nil {
		t.Fatalf("brief with all-failing outlook legs returned nil, want error\noutput: %s", out.String())
	}
}

// TestBrief_MalformedStormsFixtureExit2 covers finding 6 for brief: a malformed
// storms fixture is bad input (exit 2 / usageErr), not a generic exit-1 error.
func TestBrief_MalformedStormsFixtureExit2(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "garbage.json")
	if err := os.WriteFile(bad, []byte("not json at all {{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{asJSON: true}
	cmd := newNovelBriefCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"--basin", "atl",
		"--storms-fixture", bad,
		"--alerts-fixture", fixPath("alerts", "empty_hurricane_warning_2026-06-15.json"),
		"--outlook-fixture", fixPath("two-atl.txt"),
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("malformed storms fixture returned nil, want usage error")
	}
	if got := ExitCode(err); got != 2 {
		t.Errorf("exit code = %d, want 2 for malformed fixture", got)
	}
}
