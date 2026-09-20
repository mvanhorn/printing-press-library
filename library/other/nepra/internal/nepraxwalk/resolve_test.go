// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	if err := Validate(); err != nil {
		t.Fatalf("embedded crosswalk.json is invalid: %v", err)
	}
	if got, want := len(Rows()), 133; got != want {
		t.Errorf("crosswalk rows: got %d, want %d", got, want)
	}
	if got, want := LatestObservedFY(), "2023-24"; got != want {
		t.Errorf("LatestObservedFY() = %q, want %q", got, want)
	}
	if got, want := strings.Join(ObservedFYs(), ","), "2017-18,2023-24"; got != want {
		t.Errorf("ObservedFYs() = %q, want %q", got, want)
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"nbsp padding both ends", " Kot Addu Power Company (KAPCO)", "Kot Addu Power Company (KAPCO)"},
		{"excel newline plus indent", "Narowal\n  Energy Ltd. (HUBCO)", "Narowal Energy Ltd. (HUBCO)"},
		{"nbsp inside the name", "Tarbela  Ext. 04 Hydropower Project (WAPDA)", "Tarbela Ext. 04 Hydropower Project (WAPDA)"},
		{"nbsp run as trailing pad", " Chashma Nuclear Power Plant 1\n  (CHASHNUPP-I)  ", "Chashma Nuclear Power Plant 1 (CHASHNUPP-I)"},
		{"leading newline then wide nbsp tail", "\n    Three Gorges Second Wind Farm (Private) Ltd. (TGS)", "Three Gorges Second Wind Farm (Private) Ltd. (TGS)"},
		{"html entity decoded", "Sapphire Electric &amp; Co", "Sapphire Electric & Co"},
		{"tabs collapse", "GTPS\t\tFaisalabad", "GTPS Faisalabad"},
		{"nbsp only cell becomes empty", " ", ""},
		{"case and letters untouched", "AES Lalpir power limited.", "AES Lalpir power limited."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name          string
		in            string
		wantOK        bool
		wantCanonical string
		wantKind      MatchKind
		wantTicker    string
		wantParent    string
	}{
		{
			name:          "verbatim published name with nbsp and newline",
			in:            " Kot Addu Power Company (KAPCO)",
			wantOK:        true,
			wantCanonical: "Kot Addu Power Company (KAPCO)",
			wantKind:      MatchCanonical,
			wantTicker:    "KAPCO",
			wantParent:    "Kot Addu Power Company Limited",
		},
		{
			name:          "FY2023-24 form of the drifted Narowal name",
			in:            "Narowal Energy Ltd.",
			wantOK:        true,
			wantCanonical: "Narowal Energy Ltd.",
			wantKind:      MatchCanonical,
			wantTicker:    "HUBC",
			wantParent:    "The Hub Power Company Limited",
		},
		{
			name:          "FY2017-18 form of the drifted Narowal name resolves to the same plant",
			in:            "Narowal\n  Energy Ltd. (HUBCO)",
			wantOK:        true,
			wantCanonical: "Narowal Energy Ltd.",
			wantKind:      MatchAlias,
			wantTicker:    "HUBC",
			wantParent:    "The Hub Power Company Limited",
		},
		{
			name:          "case folded and trailing period dropped",
			in:            "  narowal energy ltd  ",
			wantOK:        true,
			wantCanonical: "Narowal Energy Ltd.",
			wantKind:      MatchCanonical,
			wantTicker:    "HUBC",
			wantParent:    "The Hub Power Company Limited",
		},
		{
			name:          "unregistered parenthetical suffix stripped when exactly one plant remains",
			in:            "Kot Addu Power Company (KAPCO Genco)",
			wantOK:        true,
			wantCanonical: "Kot Addu Power Company (KAPCO)",
			wantKind:      MatchParenStripped,
			wantTicker:    "KAPCO",
			wantParent:    "Kot Addu Power Company Limited",
		},
		{
			name:          "the KEL plant is Kohinoor, ticker KOHE not KEL",
			in:            " Kohinoor Energy Limited. (KEL)",
			wantOK:        true,
			wantCanonical: "Kohinoor Energy Limited. (KEL)",
			wantKind:      MatchCanonical,
			wantTicker:    "KOHE",
			wantParent:    "Kohinoor Energy Limited",
		},
		{
			name:          "AES-era name maps to the renamed listed company",
			in:            "AES Lalpir power\n  limited.",
			wantOK:        true,
			wantCanonical: "AES Lalpir power limited.",
			wantKind:      MatchCanonical,
			wantTicker:    "LPL",
			wantParent:    "Lalpir Power Limited",
		},
		{
			name:          "Nishat Power is not Nishat Chunian Power",
			in:            "Nishat Power Ltd\n  (NPL)",
			wantOK:        true,
			wantCanonical: "Nishat Power Ltd (NPL)",
			wantKind:      MatchCanonical,
			wantTicker:    "NPL",
			wantParent:    "Nishat Power Limited",
		},
		{
			name:          "Nishat Chunian Power is not Nishat Power",
			in:            "Nishat Chunian Power\n  Ltd (NCPL)",
			wantOK:        true,
			wantCanonical: "Nishat Chunian Power Ltd (NCPL)",
			wantKind:      MatchCanonical,
			wantTicker:    "NCPL",
			wantParent:    "Nishat Chunian Power Limited",
		},
		{
			name:          "state hydel plant resolves with no ticker",
			in:            "Allai Khwar\n  Hydropower Project (WAPDA)",
			wantOK:        true,
			wantCanonical: "Allai Khwar Hydropower Project (WAPDA)",
			wantKind:      MatchCanonical,
			wantTicker:    "",
			wantParent:    "Pakistan Water and Power Development Authority (WAPDA)",
		},
		// --- refusals ---------------------------------------------------
		{
			name:   "both Foundation Wind farms share this base string, so refuse",
			in:     "Foundation Wind Energy-I Ltd.",
			wantOK: false,
		},
		{
			name:   "K-Electric's own plant is not in the dataset",
			in:     "Bin Qasim Power Station BQPS-1",
			wantOK: false,
		},
		{
			name:   "the PSX spelling of HUBCO's name is not a published plant name",
			in:     "The Hub Power Company Limited",
			wantOK: false,
		},
		{
			name:   "a KAPCO block is not a published row here",
			in:     "Kot Addu Power Company Block-I",
			wantOK: false,
		},
		{
			name:   "a typo gets no fuzzy rescue",
			in:     "Narowul Energy Ltd.",
			wantOK: false,
		},
		{
			name:   "a substring of a real plant is not the plant",
			in:     "Narowal",
			wantOK: false,
		},
		{
			name:   "a bare acronym is not a plant name",
			in:     "(NPPCL)",
			wantOK: false,
		},
		{
			name:   "an NBSP-only cell resolves to nothing",
			in:     " ",
			wantOK: false,
		},
		{
			name:   "empty string resolves to nothing",
			in:     "",
			wantOK: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, ok := Resolve(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("Resolve(%q) ok = %v, want %v (matched %q)", tc.in, ok, tc.wantOK, m.Row.CanonicalName)
			}
			if !tc.wantOK {
				return
			}
			if m.Row.CanonicalName != tc.wantCanonical {
				t.Errorf("canonical name = %q, want %q", m.Row.CanonicalName, tc.wantCanonical)
			}
			if m.Kind != tc.wantKind {
				t.Errorf("match kind = %q, want %q", m.Kind, tc.wantKind)
			}
			if m.Row.PSXTicker != tc.wantTicker {
				t.Errorf("psx ticker = %q, want %q", m.Row.PSXTicker, tc.wantTicker)
			}
			if m.Row.ParentName != tc.wantParent {
				t.Errorf("parent name = %q, want %q", m.Row.ParentName, tc.wantParent)
			}
			if m.Row.Evidence == "" {
				t.Error("resolved row carries no evidence")
			}
			if m.NormalizedQuery != Normalize(tc.in) {
				t.Errorf("NormalizedQuery = %q, want %q", m.NormalizedQuery, Normalize(tc.in))
			}
		})
	}
}

// TestNarowalDriftIsOnePlant is the cross-year join this package exists to
// make possible: two published spellings, one plant, one HUBC attribution,
// with both fiscal years' observations on the same row.
func TestNarowalDriftIsOnePlant(t *testing.T) {
	fy18 := "Narowal\n  Energy Ltd. (HUBCO)"
	fy24 := "Narowal Energy Ltd."

	a, okA := Resolve(fy18)
	b, okB := Resolve(fy24)
	if !okA || !okB {
		t.Fatalf("both spellings must resolve: FY2017-18 ok=%v, FY2023-24 ok=%v", okA, okB)
	}
	if a.Row.CanonicalName != b.Row.CanonicalName {
		t.Fatalf("drift split into two plants: %q vs %q", a.Row.CanonicalName, b.Row.CanonicalName)
	}
	if a.Kind != MatchAlias {
		t.Errorf("FY2017-18 spelling matched as %q, want %q (it should be a recorded alias, not a stripped guess)", a.Kind, MatchAlias)
	}
	if b.Kind != MatchCanonical {
		t.Errorf("FY2023-24 spelling matched as %q, want %q", b.Kind, MatchCanonical)
	}
	if got, want := a.Row.PSXTicker, "HUBC"; got != want {
		t.Errorf("ticker = %q, want %q; the whole point is that HUBC's subsidiary is not dropped", got, want)
	}

	row := a.Row
	if got, want := len(row.Observed), 2; got != want {
		t.Fatalf("observations on the merged row: got %d, want %d", got, want)
	}
	for _, tc := range []struct {
		fy       string
		sno      int
		capacity float64
	}{
		{"2017-18", 11, 225},
		{"2023-24", 58, 225},
	} {
		o, ok := row.ObservedIn(tc.fy)
		if !ok {
			t.Fatalf("no FY%s observation on the merged row", tc.fy)
		}
		if o.SNo != tc.sno {
			t.Errorf("FY%s S.No = %d, want %d", tc.fy, o.SNo, tc.sno)
		}
		mw, reported := o.InstalledCapacityMW()
		if !reported || mw != tc.capacity {
			t.Errorf("FY%s capacity = (%v, %v), want (%v, true)", tc.fy, mw, reported, tc.capacity)
		}
	}
	if !row.IsOpenEnded() {
		t.Errorf("ValidToFY = %q, want an open interval (the plant is still published in FY2023-24)", row.ValidToFY)
	}
	if got, want := row.ValidFromFY, "2017-18"; got != want {
		t.Errorf("ValidFromFY = %q, want %q", got, want)
	}
	// The merge is recorded for human review rather than applied invisibly.
	var found bool
	for _, pd := range PossibleDuplicates() {
		if pd.Verdict == "merged" && len(pd.Names) == 2 {
			for _, n := range pd.Names {
				if strings.Contains(n, "Narowal") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("the Narowal merge is not recorded in possible_duplicates; a merge must be reviewable")
	}
}

// TestFWELCollisionRefused proves the parenthetical-strip fallback cannot fuse
// two genuinely different plants. NEPRA publishes both Foundation Wind farms
// with the base string "Foundation Wind Energy-I Ltd."; only the parenthetical
// tells them apart.
func TestFWELCollisionRefused(t *testing.T) {
	one, ok1 := Resolve("Foundation Wind Energy-I Ltd. (FWEL-I)")
	two, ok2 := Resolve("Foundation Wind\n  Energy-I Ltd. (FWEL-II)")
	if !ok1 || !ok2 {
		t.Fatalf("both fully-qualified names must resolve: FWEL-I ok=%v, FWEL-II ok=%v", ok1, ok2)
	}
	if one.Row.CanonicalName == two.Row.CanonicalName {
		t.Fatalf("FWEL-I and FWEL-II collapsed onto one plant (%q); they are two 50 MW farms in the same workbook", one.Row.CanonicalName)
	}
	if _, ok := Resolve("Foundation Wind Energy-I Ltd."); ok {
		t.Error(`the shared base string "Foundation Wind Energy-I Ltd." must be refused, not resolved to one of the two farms`)
	}
	var recorded bool
	for _, pd := range PossibleDuplicates() {
		if len(pd.Names) == 2 && strings.Contains(pd.Names[0], "FWEL-I") && strings.Contains(pd.Names[1], "FWEL-II") {
			recorded = true
			if pd.Verdict != "kept_separate" {
				t.Errorf("FWEL pair verdict = %q, want %q", pd.Verdict, "kept_separate")
			}
		}
	}
	if !recorded {
		t.Error("the FWEL-I / FWEL-II near-collision is not recorded in possible_duplicates")
	}
}

func TestResolveInFY(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		fy     string
		wantOK bool
	}{
		{"plant valid in its first observed year", "Narowal Energy Ltd.", "2017-18", true},
		{"plant valid in the latest observed year", "Narowal Energy Ltd.", "2023-24", true},
		{"FY2023-24 arrival is not back-dated to FY2017-18", "Lucky Electric Power Company Limited (LEPCL)", "2017-18", false},
		{"FY2023-24 arrival resolves in its own year", "Lucky Electric Power Company Limited (LEPCL)", "2023-24", true},
		{"empty fiscal year is refused", "Narowal Energy Ltd.", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := ResolveInFY(tc.in, tc.fy); ok != tc.wantOK {
				t.Errorf("ResolveInFY(%q, %q) ok = %v, want %v", tc.in, tc.fy, ok, tc.wantOK)
			}
		})
	}
}

func TestRowByCanonicalName(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantOK     bool
		wantTicker string
	}{
		{"exact canonical name", "Kohinoor Energy Limited. (KEL)", true, "KOHE"},
		{"canonical name, fold-insensitive", "kohinoor energy limited (kel)", true, "KOHE"},
		{"an alias is not a canonical name", "Narowal Energy Ltd. (HUBCO)", false, ""},
		{"unknown name", "Karachi Electric Supply Company", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, ok := RowByCanonicalName(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("RowByCanonicalName(%q) ok = %v, want %v", tc.in, ok, tc.wantOK)
			}
			if ok && r.PSXTicker != tc.wantTicker {
				t.Errorf("ticker = %q, want %q", r.PSXTicker, tc.wantTicker)
			}
		})
	}
}

func TestStripTrailingParen(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Narowal Energy Ltd. (HUBCO)", "Narowal Energy Ltd."},
		{"Tricon Boston Consulting Corporation (Private) Ltd-A (TBCCPL-A)", "Tricon Boston Consulting Corporation (Private) Ltd-A"},
		{"Thermal Power Station Guddu 747", "Thermal Power Station Guddu 747"},
		{"JDW Sugar Mills Limited. (Unit-II) Rahim Yar Khan", "JDW Sugar Mills Limited. (Unit-II) Rahim Yar Khan"},
		{"(NPPMCL) - Balloki", "(NPPMCL) - Balloki"},
		{"Lucky Renewables (Pvt.) Limited (Tricom Wind Power (Pvt.) Limited)", "Lucky Renewables (Pvt.) Limited"},
	}
	for _, tc := range tests {
		if got := stripTrailingParen(tc.in); got != tc.want {
			t.Errorf("stripTrailingParen(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
