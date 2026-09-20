// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Tests for the licence register shaper and command.
//
// They run offline against five committed register pages chosen because
// between them they carry every corruption class the register contains:
// the Gross Capacityy key typo, the K-Elecric header typo, an upstream
// key/value transposition, two entities with no capacity key at all, and the
// four different units the capacity cell publishes.

package cli

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func licenceFixture(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".php.gz"))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip %s: %v", name, err)
	}
	defer zr.Close()
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return b
}

func licenceParseFixture(t *testing.T, name string) []licenceEntity {
	t.Helper()
	got, err := licenceParsePage(licenceFixture(t, name), name, "https://nepra.org.pk/licensing/"+name)
	if err != nil {
		t.Fatalf("licenceParsePage(%s): %v", name, err)
	}
	return got
}

// TestLicenceSplitsAccordionsPerEntity pins the split itself. This is what
// nepraparse.BuildGrid cannot do: it counts <table> without partitioning on
// it, so the 122 tables of one register page collapse into a single flat grid
// and the <h6> entity names — the only identity there is — are never read.
func TestLicenceSplitsAccordionsPerEntity(t *testing.T) {
	for _, tc := range []struct {
		fixture string
		want    int
	}{
		{"licence-wapda-hydel", 1},
		{"licence-k-electric", 1},
		{"licence-ipps-1994", 15},
		{"licence-short-term", 2},
		{"licence-igcs", 9},
	} {
		got := licenceParseFixture(t, tc.fixture)
		if len(got) != tc.want {
			t.Fatalf("%s: %d entities, want %d", tc.fixture, len(got), tc.want)
		}
		for _, e := range got {
			if e.Name == "" {
				t.Fatalf("%s: an entity parsed with no name; identity comes only from the <h6>", tc.fixture)
			}
			if len(e.Pairs) == 0 && !strings.Contains(e.Name, "Expired") {
				t.Fatalf("%s: %q parsed with no key/value pairs", tc.fixture, e.Name)
			}
		}
	}
}

// TestLicenceRecoversTheTypoedCapacityKey is the single highest-value
// assertion here. WAPDA Hydel's key cell reads "Gross Capacityy" — split
// across a newline and tabs in the source — and it is the ONLY occurrence of
// that spelling in the register. Exact key matching alone silently drops its
// 17,367.96 MW, which is over a quarter of the register's readable capacity.
func TestLicenceRecoversTheTypoedCapacityKey(t *testing.T) {
	got := licenceParseFixture(t, "licence-wapda-hydel")
	if len(got) != 1 {
		t.Fatalf("%d entities, want 1", len(got))
	}
	e := got[0]
	if e.GrossCapacityMW == nil {
		t.Fatalf("WAPDA Hydel capacity was not recovered (raw %q, note %q). The key is \"Gross Capacityy\" "+
			"and exact matching alone drops it", e.GrossCapacityRaw, e.CapacityNote)
	}
	if *e.GrossCapacityMW != 17367.96 {
		t.Fatalf("WAPDA Hydel = %v MW, want 17367.96", *e.GrossCapacityMW)
	}
	if e.CapacityUnit != "MW" {
		t.Fatalf("unit = %q, want MW", e.CapacityUnit)
	}
	// The typo must remain VISIBLE, not silently corrected.
	found := false
	for _, p := range e.Pairs {
		if strings.Contains(p.Key, "Capacityy") {
			found = true
		}
	}
	if !found {
		t.Fatal("the published key typo \"Capacityy\" is absent from pairs[]; the evidence must survive the alias")
	}
}

// TestLicenceKeepsTheMisspeltEntityName pins that a defect in the NAME is
// reproduced and flagged rather than repaired. The K-Electric accordion
// header reads "K-Elecric" while the same page's title spells it correctly,
// so the misspelling sits in exactly the field a name lookup reads.
func TestLicenceKeepsTheMisspeltEntityName(t *testing.T) {
	got := licenceParseFixture(t, "licence-k-electric")
	if len(got) != 1 {
		t.Fatalf("%d entities, want 1", len(got))
	}
	e := got[0]
	if !strings.Contains(e.Name, "Elecric") {
		t.Fatalf("name = %q; the register publishes the misspelling and this tool reproduces it verbatim", e.Name)
	}
	if e.NameNote == "" {
		t.Fatal("the misspelling is reproduced but not flagged; a caller cannot tell it from a real name")
	}
	if e.GrossCapacityMW == nil || *e.GrossCapacityMW != 2817.114 {
		t.Fatalf("K-Electric capacity = %v, want 2817.114 MW", e.GrossCapacityMW)
	}
	// Its eleven modifications must be collected despite the spelling drift
	// in the modification keys themselves.
	if len(e.Modifications) < 11 {
		t.Fatalf("%d modifications, want at least the 11 published", len(e.Modifications))
	}
}

// TestLicenceFlagsTransposedPairsAndRefusesTheCapacity pins the corruption
// class the research probe never found. Fauji Kabirwala publishes
// Plant Type -> "170 MW" and Gross Capacity -> "Thermal (Combined Cycle)".
// The 170 MW is real but sits under the wrong key; moving it would be this
// tool inventing an attribution, and reading the capacity cell as a number
// is impossible.
func TestLicenceFlagsTransposedPairsAndRefusesTheCapacity(t *testing.T) {
	var e *licenceEntity
	for _, c := range licenceParseFixture(t, "licence-ipps-1994") {
		if strings.Contains(c.Name, "Fauji Kabirwala") {
			cc := c
			e = &cc
		}
	}
	if e == nil {
		t.Fatal("Fauji Kabirwala is absent from the 1994 page")
	}
	if !e.Transposed {
		t.Fatalf("the transposition was not detected: capacity=%q plant_type=%q", e.GrossCapacityRaw, e.PlantType)
	}
	if e.TransposedNote == "" {
		t.Fatal("transposed but unexplained")
	}
	if e.GrossCapacityMW != nil {
		t.Fatalf("a capacity of %v MW was read from a transposed cell publishing %q", *e.GrossCapacityMW, e.GrossCapacityRaw)
	}
	// And it must NOT be repaired: the megawatts stay where the source put them.
	if !strings.Contains(e.PlantType, "MW") {
		t.Fatalf("plant_type = %q; the published megawatts must stay in the cell that published them", e.PlantType)
	}
	if strings.Contains(e.GrossCapacityRaw, "MW") {
		t.Fatalf("gross_capacity_raw = %q; nothing may be moved into it", e.GrossCapacityRaw)
	}
}

// TestLicenceNoCapacityKeyIsAbsentNotZero pins the two licence-expired
// entities that publish only a determination row. Absence of a key is not a
// capacity of zero.
func TestLicenceNoCapacityKeyIsAbsentNotZero(t *testing.T) {
	got := licenceParseFixture(t, "licence-short-term")
	if len(got) != 2 {
		t.Fatalf("%d entities, want 2", len(got))
	}
	for _, e := range got {
		if e.GrossCapacityRaw != "" {
			t.Fatalf("%q published a capacity cell %q; this page's entities publish none", e.Name, e.GrossCapacityRaw)
		}
		if e.GrossCapacityMW != nil {
			t.Fatalf("%q reported %v MW with no capacity key at all", e.Name, *e.GrossCapacityMW)
		}
		if e.CapacityValue != nil {
			t.Fatalf("%q reported a capacity value with no capacity key", e.Name)
		}
		if e.CapacityNote == "" {
			t.Fatalf("%q has no capacity and no stated reason", e.Name)
		}
	}
}

// TestLicenceCapacityUnitsAreReadNotAssumed is the 1000x safety property.
//
// The register publishes four units in one cell. Reading the number and
// assuming MW would report Atlas Energy's 858.80 kW rooftop array as 858.80
// MW — larger than any plant in the country — and would fold a solar peak DC
// rating into an AC megawatt total.
func TestLicenceCapacityUnitsAreReadNotAssumed(t *testing.T) {
	cases := []struct {
		raw      string
		wantMW   *float64
		wantUnit string
		wantVal  float64
	}{
		{raw: "137 MWe", wantMW: f64p(137), wantUnit: "MWe", wantVal: 137},
		{raw: "858.80 kW", wantMW: f64p(0.8588), wantUnit: "kW", wantVal: 858.80},
		{raw: "17,367.96 MW", wantMW: f64p(17367.96), wantUnit: "MW", wantVal: 17367.96},
		// MWp is a DIFFERENT QUANTITY: parsed and labelled, never converted.
		{raw: "30.00 MWp", wantMW: nil, wantUnit: "MWp", wantVal: 30},
	}
	for _, tc := range cases {
		e := licenceEntity{GrossCapacityRaw: tc.raw}
		licenceAssign(&e)
		if e.CapacityUnit != tc.wantUnit {
			t.Fatalf("%q unit = %q, want %q", tc.raw, e.CapacityUnit, tc.wantUnit)
		}
		if e.CapacityValue == nil || *e.CapacityValue != tc.wantVal {
			t.Fatalf("%q value = %v, want %v", tc.raw, e.CapacityValue, tc.wantVal)
		}
		switch {
		case tc.wantMW == nil && e.GrossCapacityMW != nil:
			t.Fatalf("%q produced %v MW; a %s rating has no NEPRA-published conversion to MW",
				tc.raw, *e.GrossCapacityMW, tc.wantUnit)
		case tc.wantMW != nil && e.GrossCapacityMW == nil:
			t.Fatalf("%q produced no MW figure", tc.raw)
		case tc.wantMW != nil && *e.GrossCapacityMW != *tc.wantMW:
			t.Fatalf("%q = %v MW, want %v", tc.raw, *e.GrossCapacityMW, *tc.wantMW)
		}
	}
	// kW must come out SMALLER, which is the direction that matters.
	e := licenceEntity{GrossCapacityRaw: "995.60 kW"}
	licenceAssign(&e)
	if e.GrossCapacityMW == nil || *e.GrossCapacityMW >= 1 {
		t.Fatalf("995.60 kW became %v MW; a sub-megawatt array must not read as ~996 MW", e.GrossCapacityMW)
	}
}

func f64p(v float64) *float64 { return &v }

// TestLicenceRefusesCorruptCapacityCells pins the refusals individually. Each
// of these is a real cell in the register, and each would become a wrong
// number under a first-number-wins parser.
func TestLicenceRefusesCorruptCapacityCells(t *testing.T) {
	for _, raw := range []string{
		"12:00 MW", // Khairpur Sugar; a leading-number read gives 12
		"02:00 MW", // Grid Edge
		"3s 6 MW",  // Almoiz; really 36 per its own plant-detail row, so a read of 3 is out by 12x
		"110 MW Gross ISO",
		"8.5 MW (based on alternators coupled with S.Ts)",
		"Thermal (Combined Cycle)", // the transposed cell
	} {
		e := licenceEntity{GrossCapacityRaw: raw}
		licenceAssign(&e)
		if e.GrossCapacityMW != nil {
			t.Fatalf("%q was read as %v MW; it carries no clean unit and must be refused", raw, *e.GrossCapacityMW)
		}
		if e.CapacityValue != nil {
			t.Fatalf("%q produced a value %v", raw, *e.CapacityValue)
		}
		if e.CapacityNote == "" {
			t.Fatalf("%q was refused with no stated reason", raw)
		}
	}
}

// TestLicenceModificationKeysMatchByPrefix pins the normalised-prefix rule.
// A literal alias map cannot cover this key: the register uses at least eight
// spellings and one of them embeds a DATE.
func TestLicenceModificationKeysMatchByPrefix(t *testing.T) {
	for _, k := range []string{
		"Modification-I", "Licence Modification-I", "Licence Modification",
		"Licence Modification -I", "Modification - I", "Modification -I",
		"Licence Modification -II", "Licence Modification (10-01-2025)",
		"Licence Revocation", "Licence Determination", "Determination of the Authority",
	} {
		if !licenceModificationKeyRE.MatchString(licenceNormaliseKey(k)) {
			t.Fatalf("modification key %q was not matched", k)
		}
	}
	// It must not swallow the core fields.
	for _, k := range []string{"Gross Capacity", "Plant Type", "Fuel Type", "Licence No.", "Plant Detail"} {
		if licenceModificationKeyRE.MatchString(licenceNormaliseKey(k)) {
			t.Fatalf("core key %q was swallowed by the modification rule", k)
		}
	}
}

// TestLicenceFuelMatchIsByTokenNotSubstring pins the over-collection the
// survey measured: a naive substring match on "Coal" also pulls in
// "Coal Water Slurry" and "Coke Oven Gas / Blast Furnace Gas /Coal tar".
func TestLicenceFuelMatchIsByTokenNotSubstring(t *testing.T) {
	for _, published := range []string{"Coal", "coal", "Coal/Biomass", "Biomass / Coal", "RFO & Coal"} {
		if !licenceFuelMatch(published, "Coal") {
			t.Fatalf("%q should carry Coal as a token", published)
		}
	}
	for _, published := range []string{"Coal Water Slurry", "Biomass/Local Coal", "Imported Coal", "Coke Oven Gas / Blast Furnace Gas /Coal tar"} {
		if licenceFuelMatch(published, "Coal") {
			t.Fatalf("%q matched Coal exactly; it is a different published fuel", published)
		}
		if !licenceFuelNearMiss(published, "Coal") {
			t.Fatalf("%q is neither a match nor a reported near miss; it would vanish silently", published)
		}
	}
	// A near miss must never also be a match.
	if licenceFuelNearMiss("Coal/Biomass", "Coal") {
		t.Fatal("an exact-token match was also reported as a near miss")
	}
}

// TestLicenceCatalogueIsInternallyConsistent pins the catalogue's own
// arithmetic, so the two entity counts cannot drift apart.
func TestLicenceCatalogueIsInternallyConsistent(t *testing.T) {
	if got := licenceEntitiesFloor(false); got != 379 {
		t.Fatalf("catalogue total = %d, want the measured 379", got)
	}
	if got := licenceEntitiesFloor(true); got != 335 {
		t.Fatalf("probe-scope total = %d, want the probe's 335", got)
	}
	if len(licenceSurfaces) != 21 {
		t.Fatalf("%d catalogued pages, want 21", len(licenceSurfaces))
	}
	ids := map[string]struct{}{}
	for _, s := range licenceSurfaces {
		if _, dup := ids[s.ID]; dup {
			t.Fatalf("duplicate surface id %q", s.ID)
		}
		ids[s.ID] = struct{}{}
		if !strings.HasPrefix(s.Path, "/licensing/") {
			t.Fatalf("%s path %q is not a licensing path", s.ID, s.Path)
		}
		if strings.Contains(s.Path, " ") {
			t.Fatalf("%s path %q contains a literal space; the hub page's hrefs are unencoded and cannot be "+
				"used verbatim as request paths", s.ID, s.Path)
		}
	}
	// Concurrences must NOT be in the register list: same CSS, date-keyed
	// schema. Reading it with this shaper yields empty entities.
	for _, s := range licenceSurfaces {
		if strings.Contains(strings.ToLower(s.ID), "concurrence") {
			t.Fatal("Generation Concurrences.php is in the register catalogue; its keys are DATES, not field names")
		}
	}
	if len(licenceExcludedSurfaces) < 2 {
		t.Fatal("the excluded list must name Concurrences and the hub page, with reasons")
	}
}

// TestLicenceTickerIsRefusedNotAnswered pins the declared limit. The
// crosswalk resolves only 28 of 335 register names, so a --ticker that
// answered would return a partial list that LOOKS complete.
func TestLicenceTickerIsRefusedNotAnswered(t *testing.T) {
	cmd, rest, err := RootCmd().Find([]string{"licence"})
	if err != nil || len(rest) != 0 || cmd.Name() != "licence" {
		t.Fatalf("licence did not resolve: %v rest=%v", err, rest)
	}
	if cmd.Flags().Lookup("ticker") == nil {
		t.Fatal("--ticker is not declared; it must exist and refuse, so the refusal is discoverable")
	}
	for _, f := range []string{"search", "fuel", "surface", "all"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Fatalf("--%s is not declared on licence", f)
		}
	}
	for _, ann := range []string{"mcp:read-only", "pp:happy-args", "pp:typed-exit-codes", "pp:novel-hand-coded"} {
		if cmd.Annotations[ann] == "" {
			t.Fatalf("licence is missing the %s annotation", ann)
		}
	}
	// The refusal has to name the alternatives, or it is a dead end.
	for _, want := range []string{"--ticker", "--search", "28"} {
		if !strings.Contains(cmd.Long, want) {
			t.Fatalf("the command's Long help does not mention %q", want)
		}
	}
}

// TestLicenceIsTheOnlyLicenceChild pins the replacement. The generated
// command is not a novel scaffold, so addNovelCommandIfAbsent cannot replace
// it and preferImplementedNovelCommands will not drop it: without an explicit
// RemoveCommand the tree carries two `licence` children forever.
func TestLicenceIsTheOnlyLicenceChild(t *testing.T) {
	n := 0
	for _, c := range RootCmd().Commands() {
		if c.Name() == "licence" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d licence children in the command tree, want exactly 1", n)
	}
	cmd, _, _ := RootCmd().Find([]string{"licence"})
	// The replacement must be the hand-authored one, identifiable by a flag
	// the generated command never had.
	if cmd.Flags().Lookup("search") == nil {
		t.Fatal("the generated licence command is still the registered one")
	}
}

// TestLicenceAccordionsDoNotNest pins the measured property that makes the
// splitter's early return safe.
//
// licenceParsePage stops descending once it has taken an <li class="accordion">.
// Removing that return is behaviourally identical on today's register — which
// is exactly why a mutation of it survived the first test pass — and it is
// identical ONLY because no accordion contains another. If NEPRA ever nests
// them, the outer entity would swallow the inner one's rows and this test is
// what says so, pointing at the return rather than at a silent miscount.
func TestLicenceAccordionsDoNotNest(t *testing.T) {
	checked := 0
	for _, name := range []string{
		"licence-wapda-hydel", "licence-k-electric", "licence-ipps-1994",
		"licence-short-term", "licence-igcs",
	} {
		raw := licenceFixture(t, name)
		// Count the accordion markers in the source and compare against what
		// the parser returned. A nested accordion would make the source count
		// exceed the parsed count.
		markers := strings.Count(string(raw), `li class="accordion"`)
		got := licenceParseFixture(t, name)
		checked += markers
		if markers != len(got) {
			t.Fatalf("%s: %d accordion markers in the source but %d entities parsed. If the difference is "+
				"nesting, licenceParsePage's early return is now load-bearing and a nested entity is being "+
				"swallowed by its parent", name, markers, len(got))
		}
	}
	if checked == 0 {
		t.Fatal("no accordion markers found; this test would assert nothing")
	}
	t.Logf("verified %d accordions across 5 pages, none nested", checked)
}

// TestLicenceTranspositionNeedsBothSidesChecked pins that the detection reads
// BOTH cells. Deciding on the capacity cell alone would flag every entity
// whose capacity is merely unreadable — "12:00 MW", "110 MW Gross ISO" — as
// transposed, which is a different and wrong claim about the source.
func TestLicenceTranspositionNeedsBothSidesChecked(t *testing.T) {
	// The real transposition: capacity holds a technology, plant type holds MW.
	both := licenceEntity{GrossCapacityRaw: "Thermal (Combined Cycle)", PlantType: "170 MW"}
	licenceAssign(&both)
	if !both.Transposed {
		t.Fatal("a genuine transposition was not detected")
	}

	// Unreadable capacity, but the plant type is NOT megawatts: that is
	// corruption, not transposition.
	for _, tc := range []licenceEntity{
		{GrossCapacityRaw: "12:00 MW", PlantType: "Bagasse"},
		{GrossCapacityRaw: "110 MW Gross ISO", PlantType: "Thermal"},
		{GrossCapacityRaw: "3s 6 MW", PlantType: "Thermal"},
	} {
		e := tc
		licenceAssign(&e)
		if e.Transposed {
			t.Fatalf("capacity %q with plant_type %q was reported as TRANSPOSED; it is merely unreadable, "+
				"and the two are different claims about the source", tc.GrossCapacityRaw, tc.PlantType)
		}
		if e.CapacityNote == "" {
			t.Fatalf("capacity %q was refused with no reason", tc.GrossCapacityRaw)
		}
	}

	// A clean entity must be neither.
	clean := licenceEntity{GrossCapacityRaw: "660.00 MW", PlantType: "Coal"}
	licenceAssign(&clean)
	if clean.Transposed {
		t.Fatal("a clean entity was reported as transposed")
	}
	if clean.GrossCapacityMW == nil || *clean.GrossCapacityMW != 660 {
		t.Fatalf("clean capacity = %v, want 660", clean.GrossCapacityMW)
	}
}

// TestLicenceAssembleKeepsCatalogueOrder pins the determinism the concurrent
// fetch depends on.
//
// The 21 pages are read in parallel and their replies arrive in whatever
// order the network gives. Results are written into an INDEXED slice and
// reassembled by catalogue position, so the row order of the extract is a
// property of the catalogue rather than of the race. This test exists because
// a mutation that reversed the result index survived the entire suite: the
// determinism was asserted only in a comment, and an order that varied per
// run would make two otherwise-identical extracts diff against each other.
func TestLicenceAssembleKeepsCatalogueOrder(t *testing.T) {
	surfaces := []licenceSurface{
		{ID: "first"}, {ID: "second"}, {ID: "third"},
	}
	results := []licenceFetch{
		{ok: true, entities: []licenceEntity{{Name: "A1"}, {Name: "A2"}}},
		{ok: true, entities: []licenceEntity{{Name: "B1"}}},
		{ok: true, entities: []licenceEntity{{Name: "C1"}}},
	}
	entities, artifacts, failed := licenceAssemble(surfaces, results)
	if len(failed) != 0 {
		t.Fatalf("unexpected failures: %v", failed)
	}
	if len(artifacts) != 3 {
		t.Fatalf("%d artifacts, want 3", len(artifacts))
	}
	got := make([]string, 0, len(entities))
	for _, e := range entities {
		got = append(got, e.Name)
	}
	want := []string{"A1", "A2", "B1", "C1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("entity order = %v, want %v — the assembly must follow the CATALOGUE, not the reply order",
			got, want)
	}
}

// TestLicenceAssembleNamesEveryFailedPage pins that a page which failed is
// reported by NAME and contributes no rows. A silently dropped page makes a
// truncated register indistinguishable from a small one, which is the exact
// failure shape this site produces (its determination pages truncate under an
// HTTP 200).
func TestLicenceAssembleNamesEveryFailedPage(t *testing.T) {
	surfaces := []licenceSurface{{ID: "ok-page"}, {ID: "broken-page"}, {ID: "empty-result"}}
	results := []licenceFetch{
		{ok: true, entities: []licenceEntity{{Name: "kept"}}},
		{ok: false, err: errLicenceTest},
		// A worker that recorded nothing at all must still be accounted for,
		// not read as a page with no entities.
		{},
	}
	entities, _, failed := licenceAssemble(surfaces, results)
	if len(entities) != 1 || entities[0].Name != "kept" {
		t.Fatalf("entities = %v, want only the one good page's row", entities)
	}
	if len(failed) != 2 {
		t.Fatalf("%d failures reported, want 2 (the errored page and the unrecorded one)", len(failed))
	}
	joined := strings.Join(failed, " | ")
	for _, want := range []string{"broken-page", "empty-result"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("failure list %q does not name %q", joined, want)
		}
	}
	if !strings.Contains(joined, errLicenceTest.Error()) {
		t.Fatalf("failure list %q does not carry the underlying reason", joined)
	}

	// A results slice shorter than the catalogue must not silently shorten
	// the register either.
	_, _, short := licenceAssemble(surfaces, results[:1])
	if len(short) != 2 {
		t.Fatalf("%d failures for a truncated result slice, want 2", len(short))
	}
}

var errLicenceTest = errLicenceTestType{}

type errLicenceTestType struct{}

func (errLicenceTestType) Error() string { return "synthetic fetch failure" }

// TestLicenceFetchAllPairsResultsWithTheirOwnSurface pins the index
// discipline of the concurrent read, with no network.
//
// The fetcher returns content derived from the surface it was handed, so any
// mis-indexing shows up as a surface's rows landing under a different
// surface. The replies are deliberately returned out of order (later
// surfaces answer first) to prove the result slot, not the reply order,
// decides placement.
func TestLicenceFetchAllPairsResultsWithTheirOwnSurface(t *testing.T) {
	surfaces := make([]licenceSurface, 0, 12)
	for i := 0; i < 12; i++ {
		surfaces = append(surfaces, licenceSurface{ID: "surface-" + string(rune('a'+i))})
	}
	results := licenceFetchAll(surfaces, 5, func(sf licenceSurface) licenceFetch {
		return licenceFetch{
			ok:       true,
			entities: []licenceEntity{{Name: "row-of-" + sf.ID, Surface: sf.ID}},
		}
	})
	if len(results) != len(surfaces) {
		t.Fatalf("%d results for %d surfaces", len(results), len(surfaces))
	}
	for i, sf := range surfaces {
		r := results[i]
		if !r.ok || len(r.entities) != 1 {
			t.Fatalf("results[%d] (%s) is empty", i, sf.ID)
		}
		if r.entities[0].Surface != sf.ID {
			t.Fatalf("results[%d] holds %s's rows but belongs to %s — the fetch loop is mis-indexing, so "+
				"failure attribution and row order both follow the race instead of the catalogue",
				i, r.entities[0].Surface, sf.ID)
		}
	}

	// End to end through the assembler: order must be catalogue order.
	entities, _, failed := licenceAssemble(surfaces, results)
	if len(failed) != 0 {
		t.Fatalf("unexpected failures: %v", failed)
	}
	for i, e := range entities {
		if e.Surface != surfaces[i].ID {
			t.Fatalf("entity %d came from %s, want %s", i, e.Surface, surfaces[i].ID)
		}
	}

	// A per-surface failure must stay attached to that surface.
	mixed := licenceFetchAll(surfaces, 5, func(sf licenceSurface) licenceFetch {
		if sf.ID == "surface-d" {
			return licenceFetch{err: errLicenceTest}
		}
		return licenceFetch{ok: true, entities: []licenceEntity{{Name: sf.ID, Surface: sf.ID}}}
	})
	_, _, mixedFailed := licenceAssemble(surfaces, mixed)
	if len(mixedFailed) != 1 {
		t.Fatalf("%d failures, want exactly 1: %v", len(mixedFailed), mixedFailed)
	}
	if !strings.Contains(mixedFailed[0], "surface-d") {
		t.Fatalf("the failure was attributed to %q, not to surface-d", mixedFailed[0])
	}

	// Concurrency must be bounded but never change the result set.
	for _, n := range []int{0, 1, 3, 50} {
		got := licenceFetchAll(surfaces, n, func(sf licenceSurface) licenceFetch {
			return licenceFetch{ok: true, entities: []licenceEntity{{Surface: sf.ID}}}
		})
		for i, sf := range surfaces {
			if got[i].entities[0].Surface != sf.ID {
				t.Fatalf("concurrency %d mis-indexed surface %d", n, i)
			}
		}
	}
}

// TestLicenceTickerActuallyExitsTwo executes the refusal rather than just
// asserting the flag exists.
//
// This test exists because a mutation that disabled the refusal entirely —
// turning `if flagTicker != ""` into `if false` — SURVIVED the whole suite.
// TestLicenceTickerIsRefusedNotAnswered checks that the flag is declared and
// that the help text explains itself, which is necessary and nowhere near
// sufficient: the refusal IS the feature, and without it the command would
// silently ignore --ticker and return the entire register as though the
// filter had been applied.
//
// It makes no request: the refusal is the first check after --dry-run and the
// positional-argument guard, so it short-circuits before any client is built.
func TestLicenceTickerActuallyExitsTwo(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"licence", "--ticker", "HUBC"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("licence --ticker HUBC succeeded; it must refuse, because the crosswalk resolves only 28 of " +
			"335 register names and a silent partial answer looks complete")
	}
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("error %v is not a typed cliError; the exit code is the contract", err)
	}
	if ce.code != 2 {
		t.Fatalf("licence --ticker exit code = %d, want 2", ce.code)
	}
	// The refusal must be actionable: it has to name what to use instead.
	msg := err.Error()
	for _, want := range []string{"--ticker", "28", "--search"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("the refusal message does not mention %q: %s", want, msg)
		}
	}
	// And it must not have silently produced register rows.
	if strings.Contains(out.String(), "gross_capacity_mw") {
		t.Fatal("the refusal still emitted register rows")
	}
}

// TestLicencePositionalArgumentIsRefused pins the other guard on the same
// path, so the two cannot be confused for one another.
func TestLicencePositionalArgumentIsRefused(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"licence", "somepositional"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("licence accepted a positional argument")
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 2 {
		t.Fatalf("positional refusal error = %v, want a cliError with code 2", err)
	}
}

// TestLicenceModificationTrailCarriesItsLinks pins the feature the novel
// description actually promises.
//
// Before this, every modification row emitted {"key":"Modification-I",
// "value":"View"} — the anchor's visible TEXT with the href dropped — so a
// caller learned that a modification existed and had no way to reach it. The
// register's modification trail IS links: "View" is the only text those cells
// carry, so dropping the href discards the entire content of the row.
func TestLicenceModificationTrailCarriesItsLinks(t *testing.T) {
	got := licenceParseFixture(t, "licence-k-electric")
	if len(got) != 1 {
		t.Fatalf("%d entities, want 1", len(got))
	}
	e := got[0]
	if len(e.Modifications) < 11 {
		t.Fatalf("%d modifications, want at least the 11 published", len(e.Modifications))
	}
	linked := 0
	for _, m := range e.Modifications {
		if m.DocumentURL == "" {
			continue
		}
		linked++
		if !strings.HasPrefix(m.DocumentURL, "https://") && !strings.HasPrefix(m.DocumentURL, "http://") {
			t.Fatalf("%s document_url %q is not an absolute http(s) URL", m.Key, m.DocumentURL)
		}
	}
	if linked != len(e.Modifications) {
		t.Fatalf("%d of %d modification rows carry a document_url; the trail is the links, and a row whose "+
			"only visible text is \"View\" carries nothing without one", linked, len(e.Modifications))
	}
}

// TestLicenceHrefSchemeAllowlist pins that a non-fetchable scheme is dropped
// while the ROW survives — the modification is real even when its link is not.
func TestLicenceHrefSchemeAllowlist(t *testing.T) {
	const page = "https://nepra.org.pk/licensing/Generation%20K-Electric.php"
	for href, want := range map[string]string{
		"https://nepra.org.pk/a.pdf": "https://nepra.org.pk/a.pdf",
		"http://nepra.org.pk/a.pdf":  "http://nepra.org.pk/a.pdf",
		"Licences/Generation/x.pdf":  "https://nepra.org.pk/licensing/Licences/Generation/x.pdf",
		"/licensing/y.pdf":           "https://nepra.org.pk/licensing/y.pdf",
		"javascript:alert(1)":        "",
		"data:text/html,<b>x":        "",
		"file:///etc/passwd":         "",
		"#":                          "",
		"":                           "",
	} {
		if got := licenceResolveHref(href, page); got != want {
			t.Errorf("licenceResolveHref(%q) = %q, want %q", href, got, want)
		}
	}
}

// TestLicenceShortSurfacesNamesAQuietlyEmptyPage is the completeness guard for
// the failure mode that carries no error at all.
//
// A page that fails to fetch is already named in surfaces_failed. This is the
// other one: NEPRA answers HTTP 200, the body parses cleanly, and the page
// yields nothing or half the register. Nothing errors, the surface counts as
// read, and the run reports `source: live` over a short register. That is the
// exact shape the gzip defect produced across all seven HTML commands, and it
// is what this CLI's cardinal rule forbids — a measured shortfall must never
// be indistinguishable from a genuinely small register.
func TestLicenceShortSurfacesNamesAQuietlyEmptyPage(t *testing.T) {
	surfaces := []licenceSurface{
		{ID: "empty-200", EntitiesAsOf: 40},
		{ID: "half-read", EntitiesAsOf: 40},
		{ID: "complete", EntitiesAsOf: 2},
	}
	results := []licenceFetch{
		{ok: true, entities: nil},
		{ok: true, entities: make([]licenceEntity, 19)},
		{ok: true, entities: make([]licenceEntity, 2)},
	}
	short := licenceShortSurfaces(surfaces, results)
	if len(short) != 2 {
		t.Fatalf("%d short surfaces, want 2 (the empty 200 and the half read); got %v", len(short), short)
	}
	joined := strings.Join(short, " | ")
	for _, want := range []string{"empty-200", "half-read"} {
		if !strings.Contains(joined, want) {
			t.Errorf("short surfaces do not name %s: %s", want, joined)
		}
	}
	if strings.Contains(joined, "complete") {
		t.Errorf("a page that met its floor exactly was reported short: %s", joined)
	}
	// The message must carry both numbers, or a reader cannot tell how far
	// short the page fell.
	if !strings.Contains(joined, "0 entities, below the 40") {
		t.Errorf("the empty page's message does not state got-vs-floor: %s", joined)
	}
}

// TestLicenceShortSurfacesTreatsTheCountAsAFloorNotAnEquality pins the
// direction of the comparison. The register GROWS, so more entities than were
// measured is the normal healthy case and must never be reported as a defect.
func TestLicenceShortSurfacesTreatsTheCountAsAFloorNotAnEquality(t *testing.T) {
	surfaces := []licenceSurface{{ID: "grew", EntitiesAsOf: 10}}
	results := []licenceFetch{{ok: true, entities: make([]licenceEntity, 47)}}
	if short := licenceShortSurfaces(surfaces, results); len(short) != 0 {
		t.Fatalf("a page that grew past its floor was reported short: %v", short)
	}
}

// TestLicenceShortSurfacesClaimsNothingWithoutAMeasuredFloor: a page carrying
// no measured count asserts nothing about its size, so it is never reported
// short — claiming a shortfall against a number nobody measured is the same
// error as reporting an unmeasured value as zero.
//
// THIS TEST IS WRITTEN TO BE ABLE TO FAIL. An earlier version passed a page
// with no floor AND no entities, which no mutation could break: a parsed
// count is never negative, so `got < 0` is unreachable and the assertion held
// with or without the code under test. Verified by mutation — deleting the
// guard it was meant to cover left it green, which is what exposed the guard
// itself as dead code. The version below carries entities, so turning the
// floor comparison into an equality (`!=`) reports this page short and fails.
func TestLicenceShortSurfacesClaimsNothingWithoutAMeasuredFloor(t *testing.T) {
	surfaces := []licenceSurface{{ID: "never-measured", EntitiesAsOf: 0}}
	results := []licenceFetch{{ok: true, entities: make([]licenceEntity, 5)}}
	if short := licenceShortSurfaces(surfaces, results); len(short) != 0 {
		t.Fatalf("a page with no measured floor was reported short: %v", short)
	}
}

// TestLicenceShortSurfacesDoesNotDoubleCountAFailedPage: a fetch failure is
// licenceAssemble's to name. Naming it here too would report one broken page
// twice, in two different vocabularies.
func TestLicenceShortSurfacesDoesNotDoubleCountAFailedPage(t *testing.T) {
	surfaces := []licenceSurface{
		{ID: "broken", EntitiesAsOf: 40},
		{ID: "unrecorded", EntitiesAsOf: 40},
	}
	results := []licenceFetch{{ok: false, err: errLicenceTest}}
	short := licenceShortSurfaces(surfaces, results)
	if len(short) != 0 {
		t.Fatalf("failed and unrecorded pages were also reported short: %v", short)
	}
	// They must still be named by the assembler, so nothing goes unreported.
	if _, _, failed := licenceAssemble(surfaces, results); len(failed) != 2 {
		t.Fatalf("%d failures from the assembler, want 2", len(failed))
	}
}
