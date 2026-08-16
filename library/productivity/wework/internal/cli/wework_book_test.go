package cli

import "testing"

func TestLocalToUTC(t *testing.T) {
	// Austin (America/Chicago) is CDT (-05:00) on 2026-08-18.
	s, e, off, err := localToUTC("2026-08-18", "08:30", "17:00", "America/Chicago")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "2026-08-18T13:30:00Z" || e != "2026-08-18T22:00:00Z" || off != "-05:00" {
		t.Fatalf("got start=%q end=%q offset=%q", s, e, off)
	}
	// Unknown tz falls back to UTC (offset +00:00).
	if _, _, off2, err := localToUTC("2026-08-18", "08:30", "17:00", ""); err != nil || off2 != "+00:00" {
		t.Fatalf("utc fallback: offset=%q err=%v", off2, err)
	}
	// Bad time is an error.
	if _, _, _, err := localToUTC("2026-08-18", "nope", "17:00", "UTC"); err == nil {
		t.Fatal("expected error on bad start time")
	}
}

func TestPickBuilding(t *testing.T) {
	bs := []weworkBuilding{
		{LocationID: "id-a", Name: "801 Barton Springs Rd", Line1: "801 Barton Springs"},
		{LocationID: "id-b", Name: "600 Congress Ave"},
		{LocationID: "id-c", Name: "Barton Creek Mall"},
	}
	if b, err := pickBuilding(bs, "id-b", ""); err != nil || b.Name != "600 Congress Ave" {
		t.Fatalf("by id: %+v err=%v", b, err)
	}
	if b, err := pickBuilding(bs, "", "801 barton springs"); err != nil || b.LocationID != "id-a" {
		t.Fatalf("by unique name: %+v err=%v", b, err)
	}
	// Ambiguous "barton" matches two -> error.
	if _, err := pickBuilding(bs, "", "barton"); err == nil {
		t.Fatal("expected ambiguity error for 'barton'")
	}
	// No match.
	if _, err := pickBuilding(bs, "", "nowhere"); err == nil {
		t.Fatal("expected no-match error")
	}
	// Unknown id.
	if _, err := pickBuilding(bs, "id-zzz", ""); err == nil {
		t.Fatal("expected unknown-id error")
	}
}

func TestTZOffset(t *testing.T) {
	// Austin (America/Chicago) is CDT (-05:00) on 2026-08-18.
	if got := tzOffset("2026-08-18", "America/Chicago"); got != "-05:00" {
		t.Fatalf("Chicago summer: got %q, want -05:00", got)
	}
	// ...and CST (-06:00) in January.
	if got := tzOffset("2026-01-15", "America/Chicago"); got != "-06:00" {
		t.Fatalf("Chicago winter: got %q, want -06:00", got)
	}
	// Unknown/blank zone falls back to UTC.
	if got := tzOffset("2026-08-18", ""); got != "+00:00" {
		t.Fatalf("blank tz: got %q, want +00:00", got)
	}
	if got := tzOffset("2026-08-18", "Not/AZone"); got != "+00:00" {
		t.Fatalf("bad tz: got %q, want +00:00", got)
	}
}

func TestCityNameOnly(t *testing.T) {
	if got := cityNameOnly("Austin, TX"); got != "Austin" {
		t.Fatalf("got %q", got)
	}
	if got := cityNameOnly("Austin"); got != "Austin" {
		t.Fatalf("got %q", got)
	}
}

func TestOrDefault(t *testing.T) {
	if orDefault("", "USD") != "USD" || orDefault("EUR", "USD") != "EUR" || orDefault("  ", "USD") != "USD" {
		t.Fatal("orDefault wrong")
	}
}
