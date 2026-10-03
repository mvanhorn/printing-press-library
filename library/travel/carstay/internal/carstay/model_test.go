package carstay

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

const fixtureID = "632c59b82b614b99a252d1b2"

func boolp(v bool) *bool      { return &v }
func nump(v float64) *float64 { return &v }
func sourceFixture(extra string) json.RawMessage {
	return json.RawMessage(`{"_id":"` + fixtureID + `","name":"新名神　鈴鹿PA","nameEn":"Suzuka PA","prefecture":"三重県","region":"鈴鹿市","activityOnly":false,"approvedEn":true,"location":[136.46,34.96],"price":2200` + extra + `}`)
}
func fixtureSpot(t *testing.T) Spot {
	t.Helper()
	s, e := Normalize(sourceFixture(`,"length":8,"breadth":4,"restroom":{"exist":true},"wifi":{"exist":false,"notification":"建物内のみ"},"nonFreeShower":{"exist":false,"price":0,"notification":"200円/9分"},"privateNote":"PRIVATE MUST NOT ESCAPE"`), "2026-10-03T00:00:00Z")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestNormalizeFacilitiesAndWhitelist(t *testing.T) {
	s := fixtureSpot(t)
	if s.Name != "新名神　鈴鹿PA" {
		t.Fatalf("original Japanese spacing changed: %q", s.Name)
	}
	if s.Latitude == nil || *s.Latitude != 34.96 || s.Longitude == nil || *s.Longitude != 136.46 {
		t.Fatal("longitude-latitude reversed")
	}
	states := map[string]string{}
	for _, f := range s.Facilities {
		states[f.Key] = f.Status
	}
	for _, tc := range []struct{ key, want string }{{"restroom", "reported_present"}, {"wifi", "requires_confirmation"}, {"nonFreeShower", "requires_confirmation"}, {"electricity", "unknown"}, {"toilet", "unknown"}} {
		if states[tc.key] != tc.want {
			t.Errorf("%s=%s want%s", tc.key, states[tc.key], tc.want)
		}
	}
	b, _ := json.Marshal(s)
	if strings.Contains(string(b), "PRIVATE") || strings.Contains(string(b), "privateNote") {
		t.Fatal("non-whitelisted field leaked")
	}
	if s.Availability != "unknown" || s.VehicleAcceptance != "unknown" || s.ParkingSpace.Height != nil {
		t.Fatal("unknown state strengthened")
	}
}
func TestNormalizeRejectsChangedContracts(t *testing.T) {
	for _, tc := range []struct{ name, input string }{{"bad-id", `{"_id":"wrong","name":"x"}`}, {"unknown-prefecture", `{"_id":"` + fixtureID + `","name":"x","prefecture":"Mars"}`}, {"string-price", strings.Replace(string(sourceFixture("")), `"price":2200`, `"price":"2200"`, 1)}, {"string-boolean", strings.Replace(string(sourceFixture("")), `"activityOnly":false`, `"activityOnly":"false"`, 1)}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, e := Normalize(json.RawMessage(tc.input), "now"); e == nil {
				t.Fatal("changed shape accepted")
			}
		})
	}
}
func TestIDsPrefecturesAndFacilities(t *testing.T) {
	for _, tc := range []struct {
		id    string
		valid bool
	}{{fixtureID, true}, {"ABCDEF632c59b82b614b99a25", false}, {"../../station", false}, {"", false}} {
		if ValidID(tc.id) != tc.valid {
			t.Errorf("id %s", tc.id)
		}
	}
	for _, tc := range []struct{ input, want string }{{"Yamanashi", "山梨県"}, {"三重県", "三重県"}, {"tokyo", "東京都"}, {"", ""}} {
		got, e := Prefecture(tc.input)
		if e != nil || got != tc.want {
			t.Errorf("pref %s=%s,%v", tc.input, got, e)
		}
	}
	if _, e := Prefecture("Fuji"); e == nil {
		t.Fatal("ambiguous prefecture accepted")
	}
	for _, tc := range []struct{ pref, area string }{{"三重県", "kinki"}, {"新潟県", "hokuriku"}, {"東京都", "kanto"}} {
		if AreaFor(tc.pref) != tc.area {
			t.Errorf("area %s", tc.pref)
		}
	}
	for _, tc := range []struct {
		key string
		ok  bool
	}{{"restroom", true}, {"toilet", true}, {"electricity", true}, {"anyVanAccepted", false}} {
		if FacilityKnown(tc.key) != tc.ok {
			t.Errorf("facility %s", tc.key)
		}
	}
}
func TestMatchSummaryAndActivityExclusion(t *testing.T) {
	s := fixtureSpot(t)
	for _, tc := range []struct {
		q, pref, lang string
		want          bool
	}{{"Suzuka", "", "ja", true}, {"鈴鹿", "三重県", "ja", true}, {"unrelated-negative", "", "ja", false}, {"", "山梨県", "ja", false}, {"", "", "en", true}} {
		if Match(s, tc.q, tc.pref, tc.lang) != tc.want {
			t.Errorf("match %v", tc)
		}
	}
	s.ActivityOnly = boolp(true)
	if Overnight(s) || Match(s, "", "", "ja") {
		t.Fatal("activity used as overnight")
	}
	s.ActivityOnly = nil
	if Overnight(s) {
		t.Fatal("unknown activity accepted")
	}
	s.ActivityOnly = boolp(false)
	s.EnglishApproved = nil
	if EnglishPublished(s) || Match(s, "", "", "en") {
		t.Fatal("unknown English flag treated as approved")
	}
	s.Description = "description"
	s.Rules = "rules"
	summary := Summary(s)
	if summary.Description != "" || summary.Rules != "" || summary.Facilities != nil {
		t.Fatal("summary unbounded")
	}
	if s.Description == "" || len(s.Facilities) == 0 {
		t.Fatal("summary mutated source")
	}
}
func TestDistanceAndStableRanking(t *testing.T) {
	for _, tc := range []struct{ a, b, c, d, want float64 }{{0, 0, 0, 0, 0}, {0, 0, 0, 1, 111.195}, {0, 179.5, 0, -179.5, 111.195}} {
		if math.Abs(DistanceKM(tc.a, tc.b, tc.c, tc.d)-tc.want) > .05 {
			t.Errorf("distance %v", tc)
		}
	}
	a := fixtureSpot(t)
	a.Latitude = nump(0)
	a.Longitude = nump(0)
	b := a
	b.ID = "000000000000000000000001"
	activity := a
	activity.ActivityOnly = boolp(true)
	rank := RankNear([]Spot{a, b, activity}, 0, 0, 1)
	if len(rank) != 2 || rank[0].ID != b.ID {
		t.Fatal("unstable ties or activity retained")
	}
	if got := RankNear([]Spot{a}, 40, 140, 1); len(got) != 0 {
		t.Fatal("negative proximity returned unrelated item")
	}
}
func TestDatesAndCanonicalHandoff(t *testing.T) {
	for _, tc := range []struct {
		in, out string
		ok      bool
	}{{"", "", true}, {"2028-02-29", "2028-03-01", true}, {"2026-02-29", "2026-03-01", false}, {"2026-10-10", "", false}, {"2026-10-10", "2026-10-10", false}, {"2026-10-11", "2026-10-10", false}, {"2026-10-10T00:00:00Z", "2026-10-11", false}} {
		if (Dates(tc.in, tc.out) == nil) != tc.ok {
			t.Errorf("dates %+v", tc)
		}
	}
	s := fixtureSpot(t)
	u, e := Handoff(s, "en", "2026-10-10", "2026-10-11")
	if e != nil || !strings.Contains(u, "/en/stations/kinki/station/"+fixtureID+"/") || !strings.Contains(u, "checkIn=2026-10-10") {
		t.Fatalf("handoff %s,%v", u, e)
	}
	s.EnglishURL = ""
	u, e = Handoff(s, "en", "", "")
	if e != nil || u != s.SourceURL {
		t.Fatal("Japanese canonical fallback failed")
	}
	s.ActivityOnly = boolp(true)
	if _, e = Handoff(s, "ja", "", ""); e == nil {
		t.Fatal("activity booking handoff accepted")
	}
}
func TestAssessUnknownQualifiedAndBoundaryCases(t *testing.T) {
	s := fixtureSpot(t)
	for _, tc := range []struct {
		name string
		c    Constraints
		want string
	}{{"inside-space", Constraints{Length: 6, Width: 2}, "fits_reported_constraints_only"}, {"exact-boundary", Constraints{Length: 8, Width: 4}, "fits_reported_constraints_only"}, {"too-long", Constraints{Length: 9}, "fails_reported_constraint"}, {"missing-height", Constraints{Height: 2.5}, "requires_confirmation"}, {"qualified-wifi", Constraints{Require: []string{"wifi"}}, "requires_confirmation"}, {"unknown-power", Constraints{Require: []string{"electricity"}}, "requires_confirmation"}, {"true-restroom", Constraints{Require: []string{"restroom"}}, "fits_reported_constraints_only"}} {
		t.Run(tc.name, func(t *testing.T) {
			a := Assess(s, tc.c)
			if a.SpaceAndFacilities != tc.want || a.VehicleAcceptance != "unknown" {
				t.Fatalf("%+v", a)
			}
		})
	}
	s.Facilities = []Facility{{Key: "electricity", Status: "reported_absent"}}
	if Assess(s, Constraints{Require: []string{"electricity"}}).SpaceAndFacilities != "fails_reported_constraint" {
		t.Fatal("explicit false discarded")
	}
}
func TestAuditPreservesPriceAndAvailabilityUnknowns(t *testing.T) {
	s := fixtureSpot(t)
	a := Audit(s, time.Date(2026, 10, 3, 0, 1, 0, 0, time.UTC), "provider_date_filtered_candidate")
	if a.Availability != "unknown" || a.ObservationAgeSeconds != 60 || len(a.QualifiedFacilities) != 2 || len(a.UnknownFields) == 0 || a.UnresolvedFees == nil {
		t.Fatalf("%+v", a)
	}
	if a.DateCandidacy != "provider_date_filtered_candidate" || !strings.Contains(a.PriceBasis, "not_dated_total") {
		t.Fatal("date candidacy or starting basis strengthened")
	}
}
