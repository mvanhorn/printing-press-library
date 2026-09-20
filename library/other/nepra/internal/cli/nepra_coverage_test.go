// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Tests for `coverage`.
//
// Each assertion pins a way the join or the gate can be silently wrong, not a
// restatement of the code. They run offline; coverage makes no request.

package cli

import (
	"strings"
	"testing"
	"time"
)

// coverageNow is a fixed clock so the gate's boundary behaviour is testable
// without waiting 90 days.
var coverageNow = time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC)

// TestCoverageJoinsOnPathNotID is the central trap. verify and sources BOTH
// name the FY2023-24 workbook "gen-2023-24" while carrying different byte
// floors, so joining on the id fuses two different claims into one row and
// silently drops one of them. Joining on the path keeps both and records
// which catalogue owns the floor.
func TestCoverageJoinsOnPathNotID(t *testing.T) {
	rows, disagreements := coverageBuild(coverageNow, coverageStaleAfterDefault)

	paths := map[string]int{}
	for _, r := range rows {
		paths[r.Path]++
	}
	for p, n := range paths {
		if n != 1 {
			t.Fatalf("path %q appears %d times; the join is not deduplicating", p, n)
		}
	}
	// sources 63 + verify 11 + events 28 + licence 21 = 123 catalogue entries,
	// less the 19 paths that appear in two catalogues = 104 distinct surfaces.
	//
	// The licence family was ADDED after an output review found coverage
	// claiming to describe "every surface this CLI can serve" while omitting
	// all 21 register pages — 83 rows against a real 104, with served_by
	// never naming `licence`. A ledger that silently drops a whole command
	// family is the exact failure it exists to prevent, so this count is the
	// assertion that keeps it honest.
	if len(rows) != 104 {
		t.Fatalf("joined %d surfaces, want the measured 104 (sources 63 + verify 11 + events 28 + licence 21, less 19 shared paths)", len(rows))
	}

	// The seven generation workbooks are the id collision. Each must survive
	// as ONE row carrying BOTH catalogues and verify's more specific floor.
	shared := 0
	for _, r := range rows {
		if len(r.Catalogues) < 2 {
			continue
		}
		shared++
		if r.Kind == "gen" && r.FloorOwner != "verify" {
			t.Fatalf("%s is in both catalogues but floor_owner = %q; verify's per-year floor is the more specific claim",
				r.ID, r.FloorOwner)
		}
	}
	if shared != 19 {
		t.Fatalf("%d rows carry more than one catalogue, want 19 (8 sources+verify, 11 sources+events)", shared)
	}
	if len(disagreements) != 7 {
		t.Fatalf("%d floor disagreements recorded, want 7 — one per generation workbook whose two catalogues carry different floors",
			len(disagreements))
	}
}

// TestCoverageOverlapsMatchTheCatalogues pins the three pairwise overlaps
// independently, so a catalogue growing or shrinking cannot quietly change
// what the ledger covers.
func TestCoverageOverlapsMatchTheCatalogues(t *testing.T) {
	rows, _ := coverageBuild(coverageNow, coverageStaleAfterDefault)
	pair := map[string]int{}
	for _, r := range rows {
		pair[strings.Join(r.Catalogues, "+")]++
	}
	for combo, want := range map[string]int{
		"sources+verify": 8,
		"sources+events": 11,
	} {
		if got := pair[combo]; got != want {
			t.Fatalf("%s overlap = %d, want %d", combo, got, want)
		}
	}
	// events INTERSECT verify must be empty, and it is asserted by counting
	// ROWS rather than by looking up a "verify+events" key. Catalogues are
	// appended in a fixed order (sources, verify, licence, events), so that
	// key can never occur whatever the data does — an expectation of 0
	// against it is unfalsifiable and would pass even if every determination
	// page were also a verify surface.
	both := 0
	for _, r := range rows {
		hasV, hasE := false, false
		for _, c := range r.Catalogues {
			switch c {
			case "verify":
				hasV = true
			case "events":
				hasE = true
			}
		}
		if hasV && hasE {
			both++
		}
	}
	if both != 0 {
		t.Fatalf("%d surfaces appear in BOTH the verify and events catalogues, want 0: they describe "+
			"different families (frozen workbook/sheet expectations versus the live determination feed) "+
			"and an overlap means one of them has grown into the other", both)
	}
}

// TestCoverageNeverFabricatesAnHTTPStatus pins the constraint the whole CLI
// is built on: the generated client returns the body without the status code,
// so a recorded "200" would assert a value never observed. A status may
// appear ONLY where a non-2xx was actually seen.
func TestCoverageNeverFabricatesAnHTTPStatus(t *testing.T) {
	rows, _ := coverageBuild(coverageNow, coverageStaleAfterDefault)
	seen := 0
	for _, r := range rows {
		if r.HTTPStatusAsOf == nil {
			continue
		}
		seen++
		if *r.HTTPStatusAsOf >= 200 && *r.HTTPStatusAsOf < 400 {
			t.Fatalf("%s records http_status_as_of %d; a success status is never observable by this client",
				r.ID, *r.HTTPStatusAsOf)
		}
	}
	if seen == 0 {
		t.Fatal("no row carries an observed non-2xx status; the catalogue records several 404s and this test would then assert nothing")
	}
}

// TestCoverageProvenanceDeclaresItsOmission pins row 2's contract: the
// omission has to be stated WITH its reason, because an absent field with no
// explanation is indistinguishable from an oversight.
func TestCoverageProvenanceDeclaresItsOmission(t *testing.T) {
	rows, _ := coverageBuild(coverageNow, coverageStaleAfterDefault)
	prov := coverageBuildProvenance(rows, "https://nepra.org.pk")
	if len(prov) != len(rows) {
		t.Fatalf("provenance rows = %d, want one per surface (%d)", len(prov), len(rows))
	}
	p := prov[0]
	for _, f := range []string{"url", "bytes", "sha256_raw", "sha256_content", "fetched_at"} {
		found := false
		for _, got := range p.RecordsFields {
			if got == f {
				found = true
			}
		}
		if !found {
			t.Fatalf("provenance does not declare that it records %q", f)
		}
	}
	reason, ok := p.OmitsFields["http_status"]
	if !ok {
		t.Fatal("provenance does not declare that http_status is omitted")
	}
	if !strings.Contains(reason, "never observed") {
		t.Fatalf("the http_status omission reason %q does not say the value was never observed", reason)
	}
	// Only the content hash may be diffed between two fetches.
	if p.ComparableAcrossFetches != "sha256_content" {
		t.Fatalf("comparable_across_fetches = %q, want sha256_content", p.ComparableAcrossFetches)
	}
	if !strings.Contains(p.NotComparableReason, "data-cfemail") {
		t.Fatalf("the not-comparable reason does not name the rewriting mechanism: %q", p.NotComparableReason)
	}
	if !strings.HasPrefix(p.URL, "https://nepra.org.pk/") {
		t.Fatalf("provenance URL %q is not absolute against the client base", p.URL)
	}
}

// TestCoverageStalenessBoundary pins the window's edge in both directions. An
// off-by-one here is a gate that fires a day early or a day late forever.
func TestCoverageStalenessBoundary(t *testing.T) {
	rows, _ := coverageBuild(coverageNow, coverageStaleAfterDefault)
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	var measured string
	for _, r := range rows {
		if r.AgeDays != nil {
			measured = r.MeasuredAt
			break
		}
	}
	if measured == "" {
		t.Fatal("no row carries a readable measurement date")
	}
	base, err := time.Parse("2006-01-02", measured)
	if err != nil {
		t.Fatalf("parsing %q: %v", measured, err)
	}

	// Exactly at the window: NOT stale.
	atEdge, _ := coverageBuild(base.AddDate(0, 0, 90), 90)
	// One day past: stale.
	pastEdge, _ := coverageBuild(base.AddDate(0, 0, 91), 90)

	findAge := func(rs []coverageRow) (int, bool) {
		for _, r := range rs {
			if r.MeasuredAt == measured && r.AgeDays != nil && r.Stale != nil {
				return *r.AgeDays, *r.Stale
			}
		}
		t.Fatalf("row measured at %q vanished from the rebuild", measured)
		return 0, false
	}
	if age, stale := findAge(atEdge); stale {
		t.Fatalf("a surface exactly %d days old was reported stale against a %d-day window", age, 90)
	}
	if age, stale := findAge(pastEdge); !stale {
		t.Fatalf("a surface %d days old was NOT reported stale against a %d-day window", age, 90)
	}
}

// TestCoverageUnagedIsNeverFresh pins the one asymmetry that matters. A
// surface whose measurement date cannot be read has an UNKNOWN age, and an
// unknown age silently becoming "0 days old" would read as perfectly fresh —
// which is the failure mode a staleness gate exists to prevent.
func TestCoverageUnagedIsNeverFresh(t *testing.T) {
	if _, ok := verifyDaysBetween("not-a-date", coverageNow); ok {
		t.Fatal("an unparseable date was accepted as an age")
	}
	r := coverageRow{MeasuredAt: "not-a-date"}
	if days, ok := verifyDaysBetween(r.MeasuredAt, coverageNow); ok {
		t.Fatalf("unparseable date yielded %d days", days)
	}
	// Drive the REAL ageing path with an unreadable date. Walking the
	// shipped catalogues cannot do this: every row in them carries a
	// readable date, so the branch under test never fires and the assertion
	// below would be vacuous.
	for _, bad := range []string{"not-a-date", "", "2026-13-45", "10 Sep 2026"} {
		row := coverageRow{ID: "synthetic", MeasuredAt: bad}
		coverageApplyAge(&row, coverageNow, coverageStaleAfterDefault)
		if row.AgeDays != nil {
			t.Fatalf("measured_at %q yielded age_days %d; an unreadable date has an UNKNOWN age", bad, *row.AgeDays)
		}
		if row.Stale != nil {
			t.Fatalf("measured_at %q yielded stale=%v; an unaged surface must not claim a staleness verdict, and false would read as fresh",
				bad, *row.Stale)
		}
	}
	// A readable date must still produce both, or the guard above is just
	// disabling the feature.
	good := coverageRow{ID: "synthetic", MeasuredAt: "2026-01-01"}
	coverageApplyAge(&good, coverageNow, coverageStaleAfterDefault)
	if good.AgeDays == nil || good.Stale == nil {
		t.Fatal("a readable date produced no age or no verdict")
	}
	if !*good.Stale {
		t.Fatalf("a surface %d days old was not stale against a %d-day window", *good.AgeDays, coverageStaleAfterDefault)
	}

	rows, _ := coverageBuild(coverageNow, coverageStaleAfterDefault)
	for _, row := range rows {
		if row.AgeDays == nil && row.Stale != nil {
			t.Fatalf("%s has no age but Stale = %v; an unaged surface must not claim a staleness verdict",
				row.ID, *row.Stale)
		}
		if row.AgeDays != nil && row.Stale == nil {
			t.Fatalf("%s has an age but no staleness verdict", row.ID)
		}
	}
}

// TestCoverageEverySurfaceIsServed pins that the ledger cannot list a surface
// with no command behind it. A blank served_by is a finding about the CLI,
// and this test is what turns it into a failure instead of a shrug.
func TestCoverageEverySurfaceIsServed(t *testing.T) {
	rows, _ := coverageBuild(coverageNow, coverageStaleAfterDefault)
	for _, r := range rows {
		if r.ServedBy == "" {
			t.Fatalf("surface %s (kind %q) is catalogued but no command serves it", r.ID, r.Kind)
		}
	}
}

// TestCoverageFloorDisagreementsAreRecordedNotResolved pins that the more
// specific floor wins WITHOUT the weaker claim disappearing. Silently
// preferring one and dropping the other is how a catalogue conflict becomes
// invisible.
func TestCoverageFloorDisagreementsAreRecordedNotResolved(t *testing.T) {
	_, disagreements := coverageBuild(coverageNow, coverageStaleAfterDefault)
	if len(disagreements) == 0 {
		t.Fatal("no floor disagreement recorded; the two catalogues do carry different floors for the generation workbooks")
	}
	for _, d := range disagreements {
		if !strings.Contains(d, "sources carries byte_floor") || !strings.Contains(d, "verify carries") {
			t.Fatalf("disagreement %q does not name BOTH floors", d)
		}
	}
}

// TestCoverageIsRegisteredAndAnnotated pins the wiring and the four required
// annotations.
func TestCoverageIsRegisteredAndAnnotated(t *testing.T) {
	cmd, rest, err := RootCmd().Find([]string{"coverage"})
	if err != nil {
		t.Fatalf("Find(coverage): %v", err)
	}
	if len(rest) != 0 || cmd.Name() != "coverage" {
		t.Fatalf("coverage did not resolve: name=%q rest=%v", cmd.Name(), rest)
	}
	for _, ann := range []string{"mcp:read-only", "pp:happy-args", "pp:typed-exit-codes", "pp:novel-hand-coded"} {
		if cmd.Annotations[ann] == "" {
			t.Fatalf("coverage is missing the %s annotation", ann)
		}
	}
	if cmd.Annotations["pp:novel-scaffold"] != "" {
		t.Fatal("coverage carries the novel-scaffold annotation")
	}
	// Every declared exit code must be documented in Long, or the
	// declaration is a claim nobody can check.
	for _, code := range strings.Split(cmd.Annotations["pp:typed-exit-codes"], ",") {
		if !strings.Contains(cmd.Long, "  "+code+" ") {
			t.Fatalf("exit code %q is declared but not documented in Long", code)
		}
	}
}

// TestCoverageCoversEveryCommandFamily pins the claim coverage's own help
// makes: that it describes every surface this CLI can serve.
//
// It exists because an output review caught the ledger omitting all 21 licence
// register pages while asserting exactly that — 83 rows against a real 104,
// and no row whose served_by named `licence`. The failure mode is silent: a
// caller reading the ledger would conclude the register is not served at all.
func TestCoverageCoversEveryCommandFamily(t *testing.T) {
	rows, _ := coverageBuild(coverageNow, coverageStaleAfterDefault)

	// Every catalogue this build ships must contribute rows.
	byCatalogue := map[string]int{}
	for _, r := range rows {
		for _, c := range r.Catalogues {
			byCatalogue[c]++
		}
	}
	for _, want := range []string{"sources", "verify", "events", "licence"} {
		if byCatalogue[want] == 0 {
			t.Fatalf("catalogue %q contributes no rows to the coverage ledger, but coverage's help claims "+
				"it describes every surface this CLI can serve", want)
		}
	}
	if got := byCatalogue["licence"]; got != len(licenceSurfaces) {
		t.Fatalf("%d licence rows in the ledger, want all %d register pages", got, len(licenceSurfaces))
	}

	// Every data-serving command must appear as a server of something. A
	// command absent from served_by is a command the ledger hides.
	servers := map[string]bool{}
	for _, r := range rows {
		for _, name := range strings.Split(r.ServedBy, ",") {
			servers[strings.TrimSpace(name)] = true
		}
	}
	for _, cmd := range []string{"gen", "generation plants", "generation monthly", "capacity", "verify",
		"disco", "conflicts", "reliability", "fca", "events", "licence", "sir", "tariff", "sources"} {
		if !servers[cmd] {
			t.Fatalf("no coverage row names %q as a server; the ledger would tell a caller that command "+
				"serves nothing", cmd)
		}
	}
}

// TestCoverageFloorsDistinguishAbsentFromZero pins the nullity of the two
// floor fields.
//
// An output review found 72 rows emitting byte_floor: 0 to mean "no floor was
// declared" while the sibling bytes_as_of on the SAME row correctly said null.
// That is the blank-is-not-zero error this CLI refuses everywhere in NEPRA's
// data, committed in its own ledger. The distinction is not academic: three
// surfaces carry a DECLARED floor of zero because they are published,
// reachable and genuinely empty, and merging them with "unknown" loses the
// only interesting thing about them.
func TestCoverageFloorsDistinguishAbsentFromZero(t *testing.T) {
	rows, _ := coverageBuild(coverageNow, coverageStaleAfterDefault)

	declaredZero := 0
	for _, r := range rows {
		if r.ByteFloor != nil && *r.ByteFloor == 0 {
			t.Fatalf("%s declares byte_floor 0; a byte floor of zero asserts nothing and must be nil", r.ID)
		}
		if r.RowFloor != nil && *r.RowFloor == 0 {
			declaredZero++
		}
		// A floor must never be attributed to nobody.
		if r.ByteFloor != nil && r.FloorOwner == "" {
			t.Fatalf("%s carries a byte floor with no floor_owner", r.ID)
		}
		if r.ByteFloor == nil && r.FloorOwner != "" {
			t.Fatalf("%s names floor_owner %q with no floor", r.ID, r.FloorOwner)
		}
	}
	// The genuinely-empty surfaces: two licence register pages and one
	// determination page. If this count changes, a measured zero has either
	// been lost or invented.
	if declaredZero != 3 {
		t.Fatalf("%d surfaces declare a measured row floor of zero, want 3 (licence Sindh, licence "+
			"Net-Metering, events ipp-waste). A measured zero must survive as 0, and an unmeasured "+
			"floor must stay nil", declaredZero)
	}
	// And at least some rows must be genuinely unfloored, or the test above
	// is asserting nothing.
	unfloored := 0
	for _, r := range rows {
		if r.ByteFloor == nil {
			unfloored++
		}
	}
	if unfloored == 0 {
		t.Fatal("every row carries a byte floor; the nil case is unexercised")
	}
}
