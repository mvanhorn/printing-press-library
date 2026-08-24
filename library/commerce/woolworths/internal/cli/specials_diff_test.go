// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/woolworths/internal/store"
)

const ppwHalfPriceGroup = "specialsgroup.3676"

// TestNovelSpecialsDiffHelpWires smoke-tests that the specials-diff command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSpecialsDiffHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"specials-diff", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("specials-diff --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "specials-diff"} {
		if !strings.Contains(help, want) {
			t.Fatalf("specials-diff --help missing %q in output:\n%s", want, help)
		}
	}
}

func ppwMember(group, stockcode string, dayOffset int, price, wasPrice float64) store.SpecialsMember {
	return store.SpecialsMember{
		GroupNode:  group,
		Stockcode:  stockcode,
		ObservedAt: ppwBaseTime + int64(dayOffset)*ppwTestDay,
		Name:       "Product " + stockcode,
		Price:      price,
		WasPrice:   wasPrice,
	}
}

// TestSpecialsDiffAbsenceOfSecondSnapshot is the honesty case: one snapshot is
// not a diff, and every member must NOT be reported as an entrant.
func TestSpecialsDiffAbsenceOfSecondSnapshot(t *testing.T) {
	members := []store.SpecialsMember{
		ppwMember(ppwHalfPriceGroup, "100", 0, 3.00, 6.00),
		ppwMember(ppwHalfPriceGroup, "200", 0, 4.00, 8.00),
		ppwMember(ppwHalfPriceGroup, "300", 0, 5.00, 10.00),
	}
	db := ppwSeedDB(t, nil, members)

	stdout, stderr, err := ppwRunCLI(t, "specials-diff", "--group", ppwHalfPriceGroup, "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("specials-diff returned error: %v", err)
	}
	rows := ppwDecodeRows[specialsDiffRow](t, stdout)
	if len(rows) != 0 {
		t.Fatalf("one snapshot produced %d diff row(s); expected [] because there is nothing to compare against: %+v", len(rows), rows)
	}
	if !strings.Contains(stderr, "two are needed to diff") {
		t.Fatalf("expected an honest 'need two snapshots' note, got stderr:\n%s", stderr)
	}
	if strings.Contains(stdout, "\"entered\": 3") {
		t.Fatalf("the single snapshot was reported as three entrants:\n%s", stdout)
	}
}

// TestSpecialsDiffPositive seeds two snapshots differing by exactly 2 entrants
// and 1 departure, one entrant having been in the group long ago.
func TestSpecialsDiffPositive(t *testing.T) {
	members := []store.SpecialsMember{
		// An old snapshot 30 days back that contained stockcode 400, so its
		// reappearance can be annotated with a real absence length.
		ppwMember(ppwHalfPriceGroup, "100", 0, 3.00, 6.00),
		ppwMember(ppwHalfPriceGroup, "400", 0, 7.00, 14.00),

		// Previous snapshot: 100, 200, 300.
		ppwMember(ppwHalfPriceGroup, "100", 30, 3.00, 6.00),
		ppwMember(ppwHalfPriceGroup, "200", 30, 4.00, 8.00),
		ppwMember(ppwHalfPriceGroup, "300", 30, 5.00, 10.00),

		// Current snapshot: 100, 200, 400, 500 — 300 left, 400 and 500 entered.
		ppwMember(ppwHalfPriceGroup, "100", 37, 3.00, 6.00),
		ppwMember(ppwHalfPriceGroup, "200", 37, 4.00, 8.00),
		ppwMember(ppwHalfPriceGroup, "400", 37, 7.00, 14.00),
		ppwMember(ppwHalfPriceGroup, "500", 37, 9.00, 18.00),
	}
	db := ppwSeedDB(t, nil, members)

	stdout, stderr, err := ppwRunCLI(t, "specials-diff", "--group", ppwHalfPriceGroup, "--db", db, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("specials-diff returned error: %v", err)
	}
	rows := ppwDecodeRows[specialsDiffRow](t, stdout)
	if len(rows) != 1 {
		t.Fatalf("expected 1 diff row, got %d", len(rows))
	}
	r := rows[0]
	if r.Entered != 2 {
		t.Fatalf("entered = %d, want 2", r.Entered)
	}
	if r.Left != 1 {
		t.Fatalf("left = %d, want 1", r.Left)
	}
	if r.Stayed != 2 {
		t.Fatalf("stayed = %d, want 2", r.Stayed)
	}
	if len(r.EnteredItems) != 2 || len(r.LeftItems) != 1 {
		t.Fatalf("item lists = %d entered / %d left, want 2/1", len(r.EnteredItems), len(r.LeftItems))
	}
	if r.LeftItems[0].Stockcode != "300" {
		t.Fatalf("departure = %q, want 300", r.LeftItems[0].Stockcode)
	}
	byCode := map[string]specialsDiffItem{}
	for _, it := range r.EnteredItems {
		byCode[it.Stockcode] = it
	}
	returning, ok := byCode["400"]
	if !ok {
		t.Fatalf("entrant 400 missing from %+v", r.EnteredItems)
	}
	if returning.DaysSinceLastSeen == nil {
		t.Fatalf("returning entrant 400 has no absence length; it was in this group 37 days ago")
	}
	if math.Abs(*returning.DaysSinceLastSeen-37) > 0.05 {
		t.Fatalf("entrant 400 absence = %v days, want ~37", *returning.DaysSinceLastSeen)
	}
	brandNew, ok := byCode["500"]
	if !ok {
		t.Fatalf("entrant 500 missing from %+v", r.EnteredItems)
	}
	if brandNew.DaysSinceLastSeen != nil {
		t.Fatalf("never-before-seen entrant 500 claims an absence of %v days", *brandNew.DaysSinceLastSeen)
	}
	if math.Abs(r.SnapshotGapDays-7) > 0.05 {
		t.Fatalf("snapshot gap = %v days, want 7", r.SnapshotGapDays)
	}
}

// TestSpecialsDiffLimitTruncatesItemLists checks --limit only trims the lists,
// never the counts.
func TestSpecialsDiffLimitTruncatesItemLists(t *testing.T) {
	members := []store.SpecialsMember{ppwMember(ppwHalfPriceGroup, "001", 0, 1, 2)}
	for i := 2; i <= 6; i++ {
		members = append(members, ppwMember(ppwHalfPriceGroup, "00"+string(rune('0'+i)), 1, float64(i), float64(2*i)))
	}
	db := ppwSeedDB(t, nil, members)

	stdout, stderr, err := ppwRunCLI(t, "specials-diff", "--group", ppwHalfPriceGroup, "--db", db, "--limit", "2", "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("specials-diff returned error: %v", err)
	}
	rows := ppwDecodeRows[specialsDiffRow](t, stdout)
	if len(rows) != 1 {
		t.Fatalf("expected 1 diff row, got %d", len(rows))
	}
	if rows[0].Entered != 5 {
		t.Fatalf("entered count = %d, want 5 (the count must survive --limit)", rows[0].Entered)
	}
	if len(rows[0].EnteredItems) != 2 {
		t.Fatalf("entered items = %d, want 2 under --limit 2", len(rows[0].EnteredItems))
	}
	if !rows[0].Truncated {
		t.Fatalf("truncation was not flagged")
	}
}

// TestSpecialsDiffJSONFidelityOnMissingMirror covers the missing-mirror guard.
func TestSpecialsDiffJSONFidelityOnMissingMirror(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.db")
	stdout, stderr, err := ppwRunCLI(t, "specials-diff", "--db", missing, "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("missing mirror returned error: %v", err)
	}
	rows := ppwDecodeRows[specialsDiffRow](t, stdout)
	if len(rows) != 0 {
		t.Fatalf("expected [], got %d rows", len(rows))
	}
	if !strings.Contains(stderr, "no local mirror at") {
		t.Fatalf("missing-mirror guidance absent from stderr:\n%s", stderr)
	}
}

func TestSpecialsDiffDryRun(t *testing.T) {
	stdout, stderr, err := ppwRunCLI(t, "specials-diff", "--dry-run", "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err != nil {
		t.Fatalf("dry run returned error: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("dry-run envelope is not JSON: %v\nraw:\n%s", err, stdout)
	}
	if envelope["dry_run"] != true || envelope["action"] != "specials-diff" {
		t.Fatalf("unexpected dry-run envelope: %v", envelope)
	}
}

// TestSpecialsDiffRejectsLiveDataSource: without --refresh this command has no
// live equivalent.
func TestSpecialsDiffRejectsLiveDataSource(t *testing.T) {
	db := ppwSeedDB(t, nil, []store.SpecialsMember{ppwMember(ppwHalfPriceGroup, "100", 0, 3, 6)})
	_, _, err := ppwRunCLI(t, "specials-diff", "--db", db, "--data-source", "live", "--json")
	if err == nil {
		t.Fatalf("--data-source live was accepted without --refresh; it must be refused")
	}
	if !strings.Contains(err.Error(), "no live equivalent") {
		t.Fatalf("error = %q, want it to say there is no live equivalent", err.Error())
	}
}

// TestSpecialsDiffRefreshConflictsWithLocalDataSource: --refresh IS the network
// fetch, so it cannot quietly override an explicit --data-source local.
func TestSpecialsDiffRefreshConflictsWithLocalDataSource(t *testing.T) {
	db := ppwSeedDB(t, nil, []store.SpecialsMember{ppwMember(ppwHalfPriceGroup, "100", 0, 3, 6)})
	stdout, stderr, err := ppwRunCLI(t, "specials-diff", "--db", db, "--refresh", "--data-source", "local", "--json")
	t.Logf("stdout:\n%s", stdout)
	t.Logf("stderr:\n%s", stderr)
	if err == nil {
		t.Fatalf("--refresh with --data-source local was accepted; the two conflict")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2 for a usage error (err: %v)", code, err)
	}
	if !strings.Contains(err.Error(), "--refresh conflicts with --data-source local") {
		t.Fatalf("error = %q, want it to name the conflict", err.Error())
	}
}

func TestSpecialsDiffRejectsPositionalArgument(t *testing.T) {
	_, _, err := ppwRunCLI(t, "specials-diff", "specialsgroup.3676", "--json")
	if err == nil {
		t.Fatalf("a stray positional argument was silently accepted")
	}
	if code := ExitCode(err); code != 2 {
		t.Fatalf("exit code = %d, want 2 for a usage error", code)
	}
}
