package jalan

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

func datedQuery() Query {
	return Query{CheckIn: "2026-11-10", Nights: 1, Rooms: 1, Adults: 2, Page: 1, Limit: 5}
}
func TestQueryCalendarAndUnsupportedInputs(t *testing.T) {
	cases := []struct {
		name        string
		edit        func(*Query)
		destination bool
	}{
		{"invalid calendar", func(q *Query) { q.CheckIn = "2026-11-31" }, false},
		{"past date in Tokyo", func(q *Query) { q.CheckIn = "2026-09-26" }, false},
		{"beyond client window", func(q *Query) { q.CheckIn = "2027-09-28" }, false},
		{"missing date", func(q *Query) { q.CheckIn = "" }, false},
		{"nine-plus bucket is not exact", func(q *Query) { q.Adults = 9 }, false},
		{"negative child", func(q *Query) { q.Children[2] = -1 }, false},
		{"six children exceed source UI", func(q *Query) { q.Children[1] = 6 }, false},
		{"too many nights", func(q *Query) { q.Nights = 10 }, false},
		{"too many rooms", func(q *Query) { q.Rooms = 11 }, false},
		{"oversized limit", func(q *Query) { q.Limit = 31 }, false},
		{"negative page", func(q *Query) { q.Page = -1 }, false},
		{"unsupported meals", func(q *Query) { q.Meals = "lunch" }, false},
		{"unknown destination", func(q *Query) { q.Destination = "Hakone hotel" }, true},
		{"two destinations", func(q *Query) { q.Destination = "Hakone"; q.AreaCode = "141600" }, true},
		{"unknown prefecture", func(q *Query) { q.AreaCode = "990000" }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := datedQuery()
			tc.edit(&q)
			_, err := normalizeQuery(q, tc.destination, testNow)
			var typed *Error
			if !errors.As(err, &typed) {
				t.Fatalf("expected typed validation failure, got %v", err)
			}
		})
	}
}
func TestQueryTokyoDayAndOccupancyEncoding(t *testing.T) {
	q := datedQuery()
	q.CheckIn = "2026-09-28"
	q.Rooms = 2
	q.Children[0] = 1
	q.Children[2] = 2
	q.Meals = "both"
	q.NonSmoking = true
	q.RoomOutdoorBath = true
	normalized, err := normalizeQuery(q, false, time.Date(2026, 9, 27, 15, 30, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	v := normalized.values()
	if got := v.Get("roomCrack"); got != "210200,210200" {
		t.Fatalf("room encoding=%q", got)
	}
	if v.Get("child1Num") != "1" || v.Get("child3Num") != "2" || v.Has("child2Num") || v.Has("dateUndecided") {
		t.Fatalf("unexpected occupancy parameters: %v", v)
	}
	if v.Get("mealType") != "3" || v.Get("careNsmr") != "1" || v.Get("carePribateBath") != "1" {
		t.Fatalf("native filter parameters: %v", v)
	}
	q.CheckIn = "2026-09-27"
	if _, err := normalizeQuery(q, false, time.Date(2026, 9, 27, 15, 30, 0, 0, time.UTC)); err == nil {
		t.Fatal("UTC previous calendar day accepted after JST midnight")
	}
}
func TestLocationResolutionAndApplicability(t *testing.T) {
	for alias, code := range map[string]string{"Hakone": "141600", "箱根": "141600", "TOKYO": "130000", "Kyoto": "260000", "Yufuin": "440600", "Kinosaki": "281100"} {
		q := datedQuery()
		q.Destination = alias
		out, err := normalizeQuery(q, true, testNow)
		if err != nil || out.AreaCode != code {
			t.Fatalf("%s => %+v %v", alias, out, err)
		}
	}
	q := datedQuery()
	q.Destination = "Tokyo"
	out, _ := normalizeQuery(q, true, testNow)
	if out.values().Get("kenCd") != "130000" || out.values().Get("lrgCd") != "" {
		t.Fatal("prefecture encoded as large area")
	}
	q = datedQuery()
	q.Onsen = true
	if err := validateOfferFilters(q); err == nil {
		t.Fatal("unverified property offer filter accepted")
	}
	q = datedQuery()
	q.Meals = "breakfast"
	if err := validatePlanFilters(q); err == nil {
		t.Fatal("exact plan filter silently accepted")
	}
	q = datedQuery()
	q.Meals = "breakfast"
	q.NonSmoking = true
	q.RoomOutdoorBath = true
	if err := validateOfferFilters(q); err != nil {
		t.Fatal(err)
	}
	catalogue, _ := Locations("")
	prefectures := 0
	for _, item := range catalogue.Results {
		if item.(Location).Kind == "prefecture" {
			prefectures++
		}
	}
	if prefectures != 47 {
		t.Fatalf("got%d prefectures", prefectures)
	}
	kusatsu, err := resolveLocation("Kusatsu")
	if err != nil || !strings.Contains(kusatsu.Scope, "broader") {
		t.Fatalf("scope lost: %+v %v", kusatsu, err)
	}
}
func TestExactIDValidation(t *testing.T) {
	for _, tc := range []struct {
		value, kind string
		valid       bool
	}{{"385995", "property", true}, {"38599", "property", false}, {"03912759", "plan", true}, {"0576806", "room", true}, {"../385995", "property", false}} {
		err := validateID(tc.value, tc.kind)
		if (err == nil) != tc.valid {
			t.Fatalf("%s %s: %v", tc.kind, tc.value, err)
		}
	}
}

func TestQueryExactSourceBounds(t *testing.T) {
	q := datedQuery()
	q.Adults = 8
	q.Rooms = 10
	q.Nights = 9
	q.Children = [5]int{5, 5, 5, 5, 5}
	normalized, err := normalizeQuery(q, false, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.values().Get("roomCrack") != strings.TrimSuffix(strings.Repeat("855555,", 10), ",") {
		t.Fatal("maximum exact per-room occupancy encoded incorrectly")
	}
}
