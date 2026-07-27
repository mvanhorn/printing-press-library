// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package subjects

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestLookupResolvesNamesAndAliases(t *testing.T) {
	for _, in := range []string{"water_tower", "WATER_TOWER", "water-tower", "watertower", " water tower "} {
		got, err := Lookup(in)
		if err != nil {
			t.Errorf("Lookup(%q) errored: %v", in, err)
			continue
		}
		if got.Name != "water_tower" {
			t.Errorf("Lookup(%q) = %q, want water_tower", in, got.Name)
		}
	}
}

// An unknown type must fail loudly. Falling back to a guess would return an
// empty result that is indistinguishable from a real empty result.
func TestLookupUnknownFailsWithTheList(t *testing.T) {
	_, err := Lookup("spaceship")
	if err == nil {
		t.Fatal("expected an error for an unknown type")
	}
	if !strings.Contains(err.Error(), "water_tower") {
		t.Errorf("error should list known types, got %v", err)
	}
	if _, err := Lookup("  "); err == nil {
		t.Error("expected an error for a blank type")
	}
}

func TestEveryTypeIsWellFormed(t *testing.T) {
	seen := map[string]string{}
	for _, ty := range All() {
		if ty.Name == "" || ty.Group == "" || ty.Description == "" {
			t.Errorf("incomplete type: %+v", ty)
		}
		if len(ty.Tags) == 0 {
			t.Errorf("type %q has no tags", ty.Name)
		}
		for _, tg := range ty.Tags {
			if tg.Key == "" {
				t.Errorf("type %q has a tag with no key", ty.Name)
			}
		}
		// Names and aliases must not collide, or Lookup becomes ambiguous.
		if prev, ok := seen[ty.Name]; ok {
			t.Errorf("duplicate type name %q (also %q)", ty.Name, prev)
		}
		seen[ty.Name] = ty.Name
		for _, a := range ty.Aliases {
			if prev, ok := seen[a]; ok {
				t.Errorf("alias %q on %q collides with %q", a, ty.Name, prev)
			}
			seen[a] = ty.Name
		}
	}
}

func TestGroupsAndInGroup(t *testing.T) {
	groups := Groups()
	if len(groups) == 0 {
		t.Fatal("expected at least one group")
	}
	var total int
	for _, g := range groups {
		in := InGroup(g)
		if len(in) == 0 {
			t.Errorf("group %q is empty", g)
		}
		total += len(in)
	}
	if total != len(All()) {
		t.Errorf("groups cover %d types but the catalogue has %d", total, len(All()))
	}
	if len(InGroup("nope")) != 0 {
		t.Error("an unknown group should return nothing")
	}
}

func TestBuildQueryRadius(t *testing.T) {
	ty, _ := Lookup("water_tower")
	q, err := BuildQuery(ty, Area{Lat: 34.05, Lon: -118.24, RadiusM: 25000}, 25, 10)
	if err != nil {
		t.Fatalf("BuildQuery: %v", err)
	}
	for _, want := range []string{"[out:json][timeout:25]", "around:25000", "34.050000", "-118.240000",
		`"man_made"="water_tower"`, `"building"="water_tower"`, "out center 10;"} {
		if !strings.Contains(q, want) {
			t.Errorf("query missing %q:\n%s", want, q)
		}
	}
	// nwr, not node: ways and relations carry most large structures.
	if strings.Count(q, "nwr") != 2 {
		t.Errorf("expected one nwr statement per tag, got:\n%s", q)
	}
}

func TestBuildQueryBBox(t *testing.T) {
	ty, _ := Lookup("lighthouse")
	q, err := BuildQuery(ty, Area{BBoxes: []BBox{{South: 33, West: -119, North: 35, East: -117}}}, 30, 0)
	if err != nil {
		t.Fatalf("BuildQuery: %v", err)
	}
	if !strings.Contains(q, "(33.000000,-119.000000,35.000000,-117.000000)") {
		t.Errorf("bbox not rendered:\n%s", q)
	}
	if !strings.HasSuffix(strings.TrimSpace(q), "out center;") {
		t.Errorf("limit 0 should emit an unbounded out:\n%s", q)
	}
}

func TestBuildQueryRejectsBadInput(t *testing.T) {
	ty, _ := Lookup("pier")
	if _, err := BuildQuery(ty, Area{}, 25, 5); err == nil {
		t.Error("expected an error when no area was given")
	}
	if _, err := BuildQuery(ty, Area{Lat: 91, Lon: 0, RadiusM: 100}, 25, 5); err == nil {
		t.Error("expected an error for latitude out of range")
	}
	if _, err := BuildQuery(ty, Area{BBoxes: []BBox{{South: 35, West: -117, North: 33, East: -119}}}, 25, 5); err == nil {
		t.Error("expected an error for an inverted bbox")
	}
	if _, err := BuildQuery(Type{Name: "empty"}, Area{Lat: 0, Lon: 0, RadiusM: 100}, 25, 5); err == nil {
		t.Error("expected an error for a type with no tags")
	}
}

func TestTagSelector(t *testing.T) {
	if got := (Tag{Key: "man_made", Value: "pier"}).Selector(); got != `["man_made"="pier"]` {
		t.Errorf("selector = %s", got)
	}
	if got := (Tag{Key: "name"}).Selector(); got != `["name"]` {
		t.Errorf("key-only selector = %s", got)
	}
	if got := (Tag{Key: "building", Value: "^(ruins|abandoned)$", Regex: true}).Selector(); !strings.Contains(got, "~") {
		t.Errorf("regex selector should use ~, got %s", got)
	}
}

// Ways and relations carry their coordinate in `center`, not lat/lon. Reading
// only lat/lon drops them silently, which looks like an empty area.
func TestParseElementsHandlesWayCenters(t *testing.T) {
	body := []byte(`{"elements":[
	  {"type":"node","id":1,"lat":34.1,"lon":-118.2,"tags":{"name":"A Tower"}},
	  {"type":"way","id":2,"center":{"lat":34.2,"lon":-118.3},"tags":{"name":"A Building"}},
	  {"type":"relation","id":3,"center":{"lat":34.3,"lon":-118.4},"tags":{}},
	  {"type":"node","id":4,"lat":0,"lon":0,"tags":{"name":"No Location"}}
	]}`)
	origin := &Area{Lat: 34.05, Lon: -118.24, RadiusM: 50000}
	got, _, err := ParseElements(body, origin)
	if err != nil {
		t.Fatalf("ParseElements: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d subjects, want 3 (the 0,0 element should be dropped)", len(got))
	}
	if got[1].Latitude != 34.2 || got[1].Longitude != -118.3 {
		t.Errorf("way center not used: %+v", got[1])
	}
	if got[2].Name != "(unnamed)" {
		t.Errorf("untagged element should read as unnamed, got %q", got[2].Name)
	}
	for _, s := range got {
		if s.DistanceKM <= 0 {
			t.Errorf("distance not computed for %+v", s)
		}
		if !strings.HasPrefix(s.URL, "https://www.openstreetmap.org/") {
			t.Errorf("bad url %q", s.URL)
		}
	}
}

func TestParseElementsSurfacesRemark(t *testing.T) {
	body := []byte(`{"elements":[],"remark":"runtime error: query timed out"}`)
	if _, _, err := ParseElements(body, nil); err == nil {
		t.Error("an Overpass remark with no elements should surface as an error, not an empty result")
	}
	// A remark alongside real results means the server truncated. It must be
	// returned so the caller can say the answer is incomplete.
	body = []byte(`{"elements":[{"type":"node","id":1,"lat":1,"lon":1,"tags":{}}],"remark":"Query timed out"}`)
	subs, remark, err := ParseElements(body, nil)
	if err != nil {
		t.Errorf("results with a remark should still parse: %v", err)
	}
	if len(subs) != 1 {
		t.Errorf("expected the partial results to be returned, got %d", len(subs))
	}
	if remark == "" {
		t.Error("the truncation remark was dropped; a partial answer would render as complete")
	}
}

func TestParseElementsBadJSON(t *testing.T) {
	if _, _, err := ParseElements([]byte("<html>error</html>"), nil); err == nil {
		t.Error("expected an error for a non-JSON body (Overpass returns HTML on overload)")
	}
}

func TestHaversine(t *testing.T) {
	// Los Angeles to San Pedro is roughly 35 km.
	d := HaversineKM(34.0522, -118.2437, 33.7358, -118.2923)
	if d < 30 || d > 40 {
		t.Errorf("LA to San Pedro = %.1f km, expected roughly 35", d)
	}
	if got := HaversineKM(10, 20, 10, 20); got != 0 {
		t.Errorf("distance to self = %f, want 0", got)
	}
}

func TestCorridorBBoxCoversBothEnds(t *testing.T) {
	boxes := CorridorBBox(34.05, -118.24, 33.35, -115.85, 15)
	if len(boxes) != 1 {
		t.Fatalf("a corridor that does not cross the antimeridian should be one box, got %d", len(boxes))
	}
	b := boxes[0]
	if !b.Valid() {
		t.Fatalf("corridor bbox invalid: %+v", b)
	}
	for _, p := range [][2]float64{{34.05, -118.24}, {33.35, -115.85}} {
		if p[0] < b.South || p[0] > b.North || p[1] < b.West || p[1] > b.East {
			t.Errorf("endpoint %v outside corridor %+v", p, b)
		}
	}
	if b.North-34.05 < 0.1 {
		t.Errorf("corridor not padded to the north: %+v", b)
	}
	// A short drive must not sweep a large slice of the planet.
	if b.East-b.West > 10 {
		t.Errorf("corridor spans %.1f degrees of longitude for a ~230km drive: %+v", b.East-b.West, b)
	}
}

// Two points either side of the antimeridian are close together, and the
// corridor between them must stay small. Taking min/max of the raw longitudes
// produced a box from -180 to +180 — a band around the whole planet — and the
// previous test passed it because it only checked the box stayed inside world
// bounds.
func TestCorridorBBoxAcrossTheAntimeridian(t *testing.T) {
	// Taveuni to Vanua Levu, Fiji: about 31 km apart, either side of 180.
	boxes := CorridorBBox(-16.85, 179.97, -16.60, -179.88, 15)

	if len(boxes) != 2 {
		t.Fatalf("an antimeridian crossing needs two boxes, got %d: %+v", len(boxes), boxes)
	}
	var spanned float64
	for _, b := range boxes {
		if !b.Valid() {
			t.Errorf("box invalid: %+v", b)
		}
		spanned += b.East - b.West
	}
	if spanned > 5 {
		t.Errorf("a 31km crossing spans %.1f degrees of longitude across %d boxes: %+v", spanned, len(boxes), boxes)
	}
	// Each endpoint must fall inside one of the boxes.
	for _, p := range [][2]float64{{-16.85, 179.97}, {-16.60, -179.88}} {
		var covered bool
		for _, b := range boxes {
			if p[0] >= b.South && p[0] <= b.North && p[1] >= b.West && p[1] <= b.East {
				covered = true
			}
		}
		if !covered {
			t.Errorf("endpoint %v is in none of the corridor boxes %+v", p, boxes)
		}
	}
}

func TestCorridorBBoxClampsToWorld(t *testing.T) {
	for _, boxes := range [][]BBox{
		CorridorBBox(89.9, 179.9, 89.8, 179.8, 500),
		CorridorBBox(-89.9, 10, -89.8, 11, 500),
	} {
		for _, b := range boxes {
			if b.North > 90 || b.East > 180 || b.South < -90 || b.West < -180 {
				t.Errorf("corridor escaped world bounds: %+v", b)
			}
		}
	}
}

func TestGeoJSON(t *testing.T) {
	subs := []Subject{{
		OSMType: "node", OSMID: 1, Name: "A Tower",
		Latitude: 34.1, Longitude: -118.2, DistanceKM: 12.345,
		Tags: map[string]string{"name": "A Tower", "man_made": "water_tower"},
		URL:  "https://www.openstreetmap.org/node/1",
	}}
	raw, err := GeoJSON(subs)
	if err != nil {
		t.Fatalf("GeoJSON: %v", err)
	}
	var fc struct {
		Type     string `json:"type"`
		Features []struct {
			Geometry struct {
				Type        string    `json:"type"`
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties map[string]any `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &fc); err != nil {
		t.Fatalf("output is not valid GeoJSON: %v", err)
	}
	if fc.Type != "FeatureCollection" || len(fc.Features) != 1 {
		t.Fatalf("unexpected shape: %+v", fc)
	}
	// GeoJSON is longitude-first; getting this backwards puts every point in
	// the wrong hemisphere.
	c := fc.Features[0].Geometry.Coordinates
	if len(c) != 2 || c[0] != -118.2 || c[1] != 34.1 {
		t.Errorf("coordinates = %v, want [lon, lat] = [-118.2, 34.1]", c)
	}
	if fc.Features[0].Properties["osm:man_made"] != "water_tower" {
		t.Errorf("osm tags not namespaced into properties: %+v", fc.Features[0].Properties)
	}
}

func TestGeoJSONEmpty(t *testing.T) {
	raw, err := GeoJSON(nil)
	if err != nil {
		t.Fatalf("GeoJSON(nil): %v", err)
	}
	if !strings.Contains(string(raw), `"features": []`) {
		t.Errorf("empty collection should still render a features array: %s", raw)
	}
}

func TestParseDistance(t *testing.T) {
	cases := map[string]float64{
		"25km": 25000, "25": 25000, "10mi": 16093.44, "500m": 500, " 3.5km ": 3500,
	}
	for in, want := range cases {
		got, err := ParseDistance(in)
		if err != nil {
			t.Errorf("ParseDistance(%q): %v", in, err)
			continue
		}
		if math.Abs(got-want) > 0.5 {
			t.Errorf("ParseDistance(%q) = %.2f, want %.2f", in, got, want)
		}
	}
	for _, bad := range []string{"", "abc", "-5km", "0", "25miles", "3feet", "inf", "+Inf", "NaN"} {
		if _, err := ParseDistance(bad); err == nil {
			t.Errorf("ParseDistance(%q) should have failed", bad)
		}
	}
}

// GeoJSON is a map file, not a script tag. Go's default HTML escaping turned
// "Tiffany & Co." into "Tiffany & Co." in an export a human reads.
func TestGeoJSONDoesNotHTMLEscapeNames(t *testing.T) {
	out, err := GeoJSON([]Subject{{
		Name: "The Tiffany & Co. Foundation Overlook",
		Tags: map[string]string{"description": "<b>viewpoint</b>"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	// The escaped forms Go emits by default, written as source escapes so the
	// literals cannot be mangled in transit: &, <, >.
	for _, escaped := range []string{"\\u0026", "\\u003c", "\\u003e"} {
		if strings.Contains(got, escaped) {
			t.Errorf("GeoJSON HTML-escaped its content (%s present):\n%s", escaped, got)
		}
	}
	if !strings.Contains(got, "Tiffany & Co.") {
		t.Errorf("literal ampersand missing:\n%s", got)
	}
	// Still valid JSON.
	var round map[string]any
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

// A large radius warns rather than being refused. What decides whether a big
// search survives is the density of the TYPE, not the radius: lighthouse at
// 644 km answered in 37 s while water_tower at 300 km had every mirror time
// out. Refusing on radius alone would block the query that worked.
func TestLargeRadiusWarning(t *testing.T) {
	if w := LargeRadiusWarning(40_000); w != "" {
		t.Fatalf("40 km should be silent, got %q", w)
	}
	if w := LargeRadiusWarning(LargeRadiusWarnM); w != "" {
		t.Fatalf("exactly at the threshold should be silent, got %q", w)
	}
	w := LargeRadiusWarning(644_000)
	if w == "" {
		t.Fatal("644 km should warn")
	}
	for _, want := range []string{"644 km", "200 km", "tile"} {
		if !strings.Contains(w, want) {
			t.Errorf("warning should mention %q, got %q", want, w)
		}
	}
	// A large radius must still BUILD a query — warning, not ceiling.
	ty, err := Lookup("lighthouse")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildQuery(ty, Area{Lat: 34.02, Lon: -118.49, RadiusM: 644_000}, 90, 0); err != nil {
		t.Fatalf("644 km must still build a query, got %v", err)
	}
}
