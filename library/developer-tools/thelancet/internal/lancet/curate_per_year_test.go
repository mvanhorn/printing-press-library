package lancet

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"
)

// curateJSON runs Curate and returns each row as its JSON object, so the tests
// check the wire format rather than the Go field names.
func curateJSON(t *testing.T, works []decodedWork, topic, sort string, limit int) []map[string]any {
	t.Helper()
	db := wordDB(t, works...)
	rows, err := Curate(context.Background(), db, topic, "", sort, false, limit)
	if err != nil {
		t.Fatalf("Curate: %v", err)
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// sameDayOrSkip runs body and skips the test if the UTC date changed while it
// ran, since fixture dates are derived from the clock.
func sameDayOrSkip(t *testing.T, body func(today time.Time)) {
	t.Helper()
	before := time.Now().UTC()
	body(before)
	if after := time.Now().UTC(); after.Format("2006-01-02") != before.Format("2006-01-02") {
		t.Skip("UTC date changed during the test")
	}
}

func dateAgo(now time.Time, days int) string {
	return now.AddDate(0, 0, -days).Format("2006-01-02")
}

func vw(id, title, date string, year, cited int) decodedWork {
	return decodedWork{ID: id, Title: title, Topic: "Rate", Year: year, Date: date, Cited: cited}
}

func num(r map[string]any) float64 { v, _ := r["citations_per_year"].(float64); return v }

func titleOrder(rows []map[string]any) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r["title"].(string))
	}
	return out
}

func TestPerYearRecentFewerCitesBeatsOldMoreCites(t *testing.T) {
	sameDayOrSkip(t, func(now time.Time) {
		rows := curateJSON(t, []decodedWork{
			vw("1", "Old classic", dateAgo(now, 3650), now.Year()-10, 1000), // ~100/yr
			vw("2", "Recent rising", dateAgo(now, 730), now.Year()-2, 400),  // ~200/yr
		}, "rate", "per-year", 10)
		got := titleOrder(rows)
		if len(got) != 2 || got[0] != "Recent rising" {
			t.Fatalf("per-year order = %v, want Recent rising first", got)
		}
	})
}

func TestPerYearComputedBeforeLimit(t *testing.T) {
	sameDayOrSkip(t, func(now time.Time) {
		// By citations the top 1 is "Giant"; by per-year it is "Fast".
		rows := curateJSON(t, []decodedWork{
			vw("1", "Giant", dateAgo(now, 7300), now.Year()-20, 2000),
			vw("2", "Mid", dateAgo(now, 3650), now.Year()-10, 900),
			vw("3", "Fast", dateAgo(now, 365), now.Year()-1, 300),
		}, "rate", "per-year", 1)
		got := titleOrder(rows)
		if len(got) != 1 || got[0] != "Fast" {
			t.Fatalf("limit 1 per-year winner = %v, want [Fast]", got)
		}
	})
}

func TestPerYearMissingPubDateUsesJuly1(t *testing.T) {
	sameDayOrSkip(t, func(now time.Time) {
		year := now.Year() - 3
		rows := curateJSON(t, []decodedWork{vw("1", "No date", "", year, 100)}, "rate", "per-year", 10)
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}
		if rows[0]["pub_date"] != "" {
			t.Errorf("pub_date = %v, want empty", rows[0]["pub_date"])
		}
		age := now.Sub(time.Date(year, time.July, 1, 0, 0, 0, 0, time.UTC)).Hours() / 24 / 365.25
		want := 100 / age
		if got := num(rows[0]); math.Abs(got-want) > 0.2 {
			t.Errorf("citations_per_year = %v, want ~%.1f (age %.3f from July 1 %d)", got, want, age, year)
		}
	})
}

func TestPerYearAgeFloor(t *testing.T) {
	sameDayOrSkip(t, func(now time.Time) {
		rows := curateJSON(t, []decodedWork{vw("1", "Brand new", dateAgo(now, 3), now.Year(), 10)}, "rate", "per-year", 10)
		if len(rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(rows))
		}
		// Floor 0.25 years: 10 / 0.25 = 40.0
		if got := num(rows[0]); got != 40.0 {
			t.Errorf("citations_per_year = %v, want 40 (0.25 year floor)", got)
		}
	})
}

func TestPerYearExcludesRowsWithoutUsableYear(t *testing.T) {
	sameDayOrSkip(t, func(now time.Time) {
		rows := curateJSON(t, []decodedWork{
			vw("1", "No year", "", 0, 5000),
			vw("2", "Has year", dateAgo(now, 365), now.Year()-1, 10),
		}, "rate", "per-year", 10)
		if got := titleOrder(rows); len(got) != 1 || got[0] != "Has year" {
			t.Fatalf("per-year rows = %v, want only [Has year]", got)
		}
		// Other sorts still return the row.
		if got := curateJSON(t, []decodedWork{vw("1", "No year", "", 0, 5000)}, "rate", "citations", 10); len(got) != 1 {
			t.Fatalf("citations sort rows = %d, want 1", len(got))
		}
	})
}

func TestPubDateAndCitationsPerYearInEverySort(t *testing.T) {
	sameDayOrSkip(t, func(now time.Time) {
		date := dateAgo(now, 730)
		for _, sort := range []string{"citations", "date", "per-year"} {
			rows := curateJSON(t, []decodedWork{vw("1", "Paper", date, now.Year()-2, 200)}, "rate", sort, 10)
			if len(rows) != 1 {
				t.Fatalf("%s: rows = %d, want 1", sort, len(rows))
			}
			if rows[0]["pub_date"] != date {
				t.Errorf("%s: pub_date = %v, want %s", sort, rows[0]["pub_date"], date)
			}
			if v, ok := rows[0]["citations_per_year"].(float64); !ok || v < 99 || v > 101 {
				t.Errorf("%s: citations_per_year = %v, want ~100", sort, rows[0]["citations_per_year"])
			}
		}
	})
}

func TestCurateLivePerYearErrors(t *testing.T) {
	if _, err := CurateLive(context.Background(), nil, "ai", "", "per-year", false, 5); err == nil {
		t.Fatal("CurateLive per-year returned nil error, want a local-store error")
	}
}

func TestCitationsPerYearHelperRule(t *testing.T) {
	now := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)
	if got, ok := CitationsPerYear(now, "2026-05-30", 2026, 10); !ok || got != 40.0 {
		t.Errorf("floor: got %v %v, want 40 true", got, ok)
	}
	if got, ok := CitationsPerYear(now, "", 2024, 100); !ok || math.Abs(got-100/(now.Sub(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)).Hours()/24/365.25)) > 0.1 {
		t.Errorf("July 1 fallback: got %v %v", got, ok)
	}
	if _, ok := CitationsPerYear(now, "garbage", 0, 10); ok {
		t.Error("no usable year must report ok=false")
	}
}

// The SQL effective date and the Go helper must agree on which pub_date values
// are usable; anything that is not a real YYYY-MM-DD uses July 1 of pub_year.
func TestPerYearSQLMatchesGoHelperForPubDates(t *testing.T) {
	cases := []struct {
		pubDate string
		valid   bool
	}{
		{"2024-02-29", true},
		{"2023-02-29", false},
		{"2024-02-31", false},
		{"2024-02-15T10:30:00", false},
		{"2024-2-5", false},
		{"", false},
		{"garbage", false},
	}
	sameDayOrSkip(t, func(now time.Time) {
		const year, cited = 2024, 100
		july1, ok := CitationsPerYear(now, "", year, cited)
		if !ok {
			t.Fatal("July 1 fallback must be usable")
		}
		for _, c := range cases {
			want, ok := CitationsPerYear(now, c.pubDate, year, cited)
			if !ok {
				t.Fatalf("%q: Go helper reported no usable date", c.pubDate)
			}
			if !c.valid && want != july1 {
				t.Errorf("%q: Go helper = %v, want the July 1 value %v", c.pubDate, want, july1)
			}
			rows := curateJSON(t, []decodedWork{vw("1", "Dated", c.pubDate, year, cited)}, "rate", "per-year", 10)
			if len(rows) != 1 {
				t.Fatalf("%q: rows = %d, want 1", c.pubDate, len(rows))
			}
			got := num(rows[0])
			if math.Abs(got-want) > 0.1+1e-9 {
				t.Errorf("%q: SQL citations_per_year = %v, Go helper = %v", c.pubDate, got, want)
			}
			if !c.valid && math.Abs(got-july1) > 0.1+1e-9 {
				t.Errorf("%q: SQL citations_per_year = %v, want the July 1 value %v", c.pubDate, got, july1)
			}
			wantDate := ""
			if c.valid {
				wantDate = c.pubDate
			}
			if rows[0]["pub_date"] != wantDate {
				t.Errorf("%q: pub_date = %v, want %q", c.pubDate, rows[0]["pub_date"], wantDate)
			}
		}
	})
}

// Ordering uses the unrounded rate: two works whose rates round to the same
// one-decimal value must still rank by the raw rate, even when the lower raw
// rate has more citations (the old tie-break).
func TestPerYearOrdersByUnroundedRate(t *testing.T) {
	sameDayOrSkip(t, func(now time.Time) {
		const daysA, daysB = 10957, 7305 // A is older with more citations
		age := func(days int) float64 {
			d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -days)
			return now.Sub(d).Hours() / 24 / 365.25
		}
		round1 := func(v float64) float64 { return math.Round(v*10) / 10 }
		// Keep the rates away from a rounding boundary so the pair stays a
		// near-tie as the clock moves within the day.
		safe := func(v float64) bool { return math.Abs(math.Mod(v*10, 1)-0.5) > 0.05 }
		citedA, citedB := 0, 1000
		for ; citedB < 1100 && citedA == 0; citedB++ {
			rb := float64(citedB) / age(daysB)
			for a := citedB + 1; a < citedB*2; a++ {
				ra := float64(a) / age(daysA)
				if ra < rb && round1(ra) == round1(rb) && safe(ra) && safe(rb) {
					citedA = a
					break
				}
			}
		}
		citedB--
		if citedA == 0 {
			t.Fatal("no near-tie fixture found")
		}
		rows := curateJSON(t, []decodedWork{
			vw("1", "Older more cited", dateAgo(now, daysA), now.Year()-30, citedA),
			vw("2", "Newer faster", dateAgo(now, daysB), now.Year()-20, citedB),
		}, "rate", "per-year", 1)
		if len(rows) != 1 || rows[0]["title"] != "Newer faster" {
			t.Fatalf("limit 1 winner = %v, want [Newer faster] (cited %d vs %d)", titleOrder(rows), citedB, citedA)
		}
		all := curateJSON(t, []decodedWork{
			vw("1", "Older more cited", dateAgo(now, daysA), now.Year()-30, citedA),
			vw("2", "Newer faster", dateAgo(now, daysB), now.Year()-20, citedB),
		}, "rate", "per-year", 10)
		if len(all) != 2 || num(all[0]) != num(all[1]) {
			t.Fatalf("fixture is not a rounded tie: %v %v", num(all[0]), num(all[1]))
		}
	})
}
