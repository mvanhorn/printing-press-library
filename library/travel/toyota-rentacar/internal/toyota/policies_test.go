package toyota

import (
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestRecordedFirstPartyPolicyFees(t *testing.T) {
	out, err := ParseOptions(fixture(t, "option.html"), fixture(t, "insurance.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		code  string
		fee   int
		basis string
	}{
		{"etc_card", 550, "per_rental"}, {"jaf", 550, "per_rental"}, {"child_or_infant_seat", 1650, "per_rental_per_seat"},
		{"booster", 1100, "per_rental_per_seat"}, {"model_selection", 2200, "per_rental"}, {"model_selection_c5_w3_suv4", 6600, "per_rental"},
		{"4wd_passenger", 1650, "per_24_hours"}, {"4wd_bus", 2750, "per_24_hours"}, {"winter_passenger_van", 2200, "per_24_hours"},
		{"winter_wagon_suv", 3300, "per_24_hours"}, {"waiver", 1100, "per_24_hours"}, {"double_protection", 1650, "per_24_hours"}, {"noc_increment", 550, "per_24_hours"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			o, err := out.Find(tc.code)
			if err != nil || o.FeeJPY != tc.fee || o.Basis != tc.basis || !o.TaxIncluded {
				t.Fatalf("option=%+v err=%v", o, err)
			}
		})
	}
	if _, err := out.Find("missing"); err == nil {
		t.Fatal("missing option found")
	}
	changed := strings.ReplaceAll(string(fixture(t, "option.html")), "2,200 yen/trip", "unquoted")
	if _, err := ParseOptions([]byte(changed), fixture(t, "insurance.html")); err == nil {
		t.Fatal("missing live rate was replaced with a fabricated default")
	}
}

func TestEligibilityIsSourceGuidanceOnly(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		ok   bool
	}{
		{"source", fixture(t, "drive.html"), true}, {"changed treaty", []byte(strings.ReplaceAll(string(fixture(t, "drive.html")), "1949", "1968")), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseEligibility(tc.data)
			if (err == nil) != tc.ok {
				t.Fatalf("err=%v", err)
			}
			if tc.ok && (out.IndividualEligibility != "not_assessed" || len(out.Paths) != 3 || len(out.SourceURLs) != 2) {
				t.Fatalf("guidance=%+v", out)
			}
		})
	}
}
