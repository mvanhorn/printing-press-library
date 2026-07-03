package insurance

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNextBusinessDay(t *testing.T) {
	cases := []struct {
		name string
		from string
		want string
	}{
		{"friday rolls to monday", "2026-06-26", "2026-06-29"},
		{"saturday rolls to monday", "2026-06-27", "2026-06-29"},
		{"sunday rolls to monday", "2026-06-28", "2026-06-29"},
		{"midweek is next day", "2026-06-24", "2026-06-25"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, _ := time.Parse("2006-01-02", c.from)
			got := NextBusinessDay(from).Format("2006-01-02")
			if got != c.want {
				t.Errorf("NextBusinessDay(%s) = %s, want %s", c.from, got, c.want)
			}
		})
	}
}

func TestDefaultProfile(t *testing.T) {
	now, _ := time.Parse("2006-01-02", "2026-06-26") // a Friday
	p := DefaultProfile(now)
	if p.GLPerOccurrence != "$1,000,000" || p.GLAggregate != "$2,000,000" {
		t.Errorf("default GL limits = %q / %q", p.GLPerOccurrence, p.GLAggregate)
	}
	if !p.ProductsCompletedOps {
		t.Errorf("products-completed ops should default on")
	}
	if p.EffectiveDate != "2026-06-29" {
		t.Errorf("default effective date = %q, want next business day 2026-06-29", p.EffectiveDate)
	}
}

func TestSaveLoadProfile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	in := importerProfile()
	if err := SaveProfile(path, in); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	out, err := LoadProfile(path)
	if err != nil {
		t.Fatalf("LoadProfile: %v", err)
	}
	if out.LegalName != in.LegalName || out.Importer != in.Importer ||
		out.GLPerOccurrence != in.GLPerOccurrence || len(out.CountriesOfOrigin) != 1 {
		t.Errorf("round-trip mismatch: %+v", out)
	}
	if out.SavedAt == "" {
		t.Errorf("SaveProfile should stamp SavedAt")
	}
}

func TestProfileValidate(t *testing.T) {
	if problems := importerProfile().Validate(); len(problems) != 0 {
		t.Errorf("complete importer profile should validate clean, got: %v", problems)
	}
	empty := Profile{}
	problems := empty.Validate()
	if len(problems) == 0 {
		t.Errorf("empty profile should report missing required fields")
	}
}

// A manufacturer-only business is an importer class (deemed manufacturer), so it
// gets the country-of-origin check - but the message must name the manufacturer
// class, not just "importer/private-label".
func TestProfileValidate_ManufacturerOnlyCountryMessage(t *testing.T) {
	p := manufacturerOnlyProfile()
	p.CountriesOfOrigin = nil

	var msg string
	for _, m := range p.Validate() {
		if containsFold(m, "country of origin") {
			msg = m
		}
	}
	if msg == "" {
		t.Fatalf("manufacturer-only profile with no country of origin should report a problem")
	}
	if !containsFold(msg, "manufacturer") {
		t.Errorf("country-of-origin problem should name the manufacturer class, got: %q", msg)
	}
}
