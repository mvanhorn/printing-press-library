package irailref

import "testing"

func TestFold(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercases", "Leuven", "leuven"},
		{"strips hyphen", "Gent-Sint-Pieters", "gentsintpieters"},
		{"strips accents", "Liège-Guillemins", "liegeguillemins"},
		{"strips spaces", "Brussels Airport - Zaventem", "brusselsairportzaventem"},
		{"handles umlaut", "Brüssel-Nord", "brusselnord"},
		{"empty stays empty", "   ", ""},
		{"keeps digits", "008813003", "008813003"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Fold(tc.in); got != tc.want {
				t.Fatalf("Fold(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLookupResolvesAliasesAndCodes(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string // expected canonical Name
	}{
		{"exact name", "Gent-Sint-Pieters", "Gent-Sint-Pieters"},
		{"french alias", "Gand-Saint-Pierre", "Gent-Sint-Pieters"},
		{"telegraph code", "FGSP", "Gent-Sint-Pieters"},
		{"telegraph code lowercase", "fgsp", "Gent-Sint-Pieters"},
		{"french alias for leuven", "Louvain", "Leuven"},
		{"german alias for liege", "Lüttich-Guillemins", "Liège-Guillemins"},
		{"half of bilingual name", "Bruxelles-Midi", "Brussel-Zuid/Bruxelles-Midi"},
		{"numeric id", "008813003", "Brussel-Centraal/Bruxelles-Central"},
		{"be.nmbs id", "BE.NMBS.008813003", "Brussel-Centraal/Bruxelles-Central"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, ok := Lookup(tc.query)
			if !ok {
				t.Fatalf("Lookup(%q) found nothing", tc.query)
			}
			if st.Name != tc.want {
				t.Fatalf("Lookup(%q).Name = %q, want %q", tc.query, st.Name, tc.want)
			}
		})
	}

	if _, ok := Lookup("Kings Cross"); ok {
		t.Fatal("Lookup matched a non-Belgian station that should be absent")
	}
	if _, ok := Lookup(""); ok {
		t.Fatal("Lookup matched the empty query")
	}
}

func TestSearchRanksBusiestFirst(t *testing.T) {
	got := Search("brussel", 5)
	if len(got) == 0 {
		t.Fatal("Search(\"brussel\") returned nothing")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].AvgStopTimes < got[i].AvgStopTimes {
			t.Fatalf("results not sorted busiest-first: %q(%v) before %q(%v)",
				got[i-1].Name, got[i-1].AvgStopTimes, got[i].Name, got[i].AvgStopTimes)
		}
	}
	if len(got) > 5 {
		t.Fatalf("Search ignored limit: got %d results, want <= 5", len(got))
	}

	// A query matching nothing must return nothing, not everything.
	if res := Search("zzzzznotastation", 5); len(res) != 0 {
		t.Fatalf("Search on a nonsense query returned %d results, want 0", len(res))
	}
}

func TestTransferSecondsDistinguishesMissingFromZero(t *testing.T) {
	secs, ok := TransferSecondsFor("Gent-Sint-Pieters")
	if !ok {
		t.Fatal("expected an official transfer time for Gent-Sint-Pieters")
	}
	if secs <= 0 {
		t.Fatalf("transfer seconds = %d, want a positive value", secs)
	}

	// Brussels Airport carries a much longer official transfer time than a
	// standard station; this guards against silently reading the wrong column.
	airport, ok := TransferSecondsFor("Brussels Airport - Zaventem")
	if !ok {
		t.Fatal("expected an official transfer time for Brussels Airport")
	}
	if airport <= secs {
		t.Fatalf("airport transfer %ds should exceed standard station transfer %ds", airport, secs)
	}

	if _, ok := TransferSecondsFor("zzzzznotastation"); ok {
		t.Fatal("unknown station reported a transfer time")
	}
}

func TestFacilitiesLookupAndStepFree(t *testing.T) {
	st, ok := Lookup("Gent-Sint-Pieters")
	if !ok {
		t.Fatal("Gent-Sint-Pieters not found")
	}
	f, ok := FacilitiesFor(st)
	if !ok {
		t.Fatal("expected facilities for Gent-Sint-Pieters")
	}
	if !f.StepFree() {
		t.Fatal("Gent-Sint-Pieters should report step-free access")
	}
	if len(f.SalesHours) == 0 {
		t.Fatal("expected ticket-desk sales hours for Gent-Sint-Pieters")
	}
	for _, w := range f.SalesHours {
		if w.Day == "" || (w.Open == "" && w.Close == "") {
			t.Fatalf("malformed sales window: %+v", w)
		}
	}

	// A nil station must not panic and must not claim step-free access.
	var nilFac *Facilities
	if nilFac.StepFree() {
		t.Fatal("nil facilities reported step-free access")
	}
	if _, ok := FacilitiesFor(nil); ok {
		t.Fatal("FacilitiesFor(nil) reported a hit")
	}
}

func TestAllLoadsEmbeddedDataset(t *testing.T) {
	all := All()
	if len(all) < 600 {
		t.Fatalf("embedded stations dataset looks truncated: %d rows", len(all))
	}
	var withTelegraph, withTransfer int
	for _, st := range all {
		if st.Telegraph != "" {
			withTelegraph++
		}
		if st.HasTransfer {
			withTransfer++
		}
	}
	if withTelegraph < 500 {
		t.Fatalf("expected 500+ telegraph codes, got %d", withTelegraph)
	}
	if withTransfer < 500 {
		t.Fatalf("expected 500+ official transfer times, got %d", withTransfer)
	}
}
