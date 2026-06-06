// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"math"
	"testing"
)

func TestAOScore(t *testing.T) {
	cases := []struct {
		name string
		p    AOPlace
		min  int
		max  int
	}{
		{"rich subtitle", AOPlace{Title: "Secret Apartment", Subtitle: "A hidden flat atop the Eiffel Tower built by Gustave himself."}, 7, 10},
		{"mundane marker", AOPlace{Title: "Civil War Historical Marker", Subtitle: "A roadside plaque."}, 0, 4},
		{"bare", AOPlace{Title: "Some Place"}, 4, 6},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := aoScore(c.p)
			if got < c.min || got > c.max {
				t.Fatalf("aoScore(%q)=%d, want [%d,%d]", c.name, got, c.min, c.max)
			}
		})
	}
}

func TestHaversineMiles(t *testing.T) {
	// Paris to London is ~214 miles.
	d := haversineMiles(48.8566, 2.3522, 51.5072, -0.1276)
	if math.Abs(d-214) > 15 {
		t.Fatalf("Paris->London = %.1f mi, want ~214", d)
	}
	if haversineMiles(40, -100, 40, -100) != 0 {
		t.Fatal("identical points must be 0 miles apart")
	}
}

func TestSlugFromPlaceURL(t *testing.T) {
	cases := map[string]string{
		"/places/hou-wang-temple":                 "hou-wang-temple",
		"https://www.atlasobscura.com/places/foo": "foo",
		"/places/bar?utm=x":                       "bar",
	}
	for in, want := range cases {
		if got := slugFromPlaceURL(in); got != want {
			t.Errorf("slugFromPlaceURL(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestResolvePointLatLng(t *testing.T) {
	lat, lng, label, err := resolvePoint(context.Background(), "48.8584,2.2945")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if math.Abs(lat-48.8584) > 1e-6 || math.Abs(lng-2.2945) > 1e-6 {
		t.Fatalf("got %f,%f", lat, lng)
	}
	if label == "" {
		t.Fatal("expected a label")
	}
}

func TestExtractKBYG(t *testing.T) {
	html := `<div>preamble</div><h3>Know Before You Go</h3><p>Open Tuesday to Saturday, 10am-2pm.</p><div>Community Contributors</div><p>ignore me</p>`
	got := extractKBYG(html)
	if got == "" {
		t.Fatal("expected KBYG text")
	}
	if !contains(got, "Tuesday") {
		t.Fatalf("KBYG missing expected content: %q", got)
	}
	if contains(got, "ignore me") {
		t.Fatalf("KBYG bled past the trailer: %q", got)
	}
}

func TestSeededIndexDeterministic(t *testing.T) {
	a := seededIndex("2026-06-06", 25)
	b := seededIndex("2026-06-06", 25)
	if a != b {
		t.Fatalf("seededIndex not deterministic: %d != %d", a, b)
	}
	if a < 0 || a >= 25 {
		t.Fatalf("seededIndex out of range: %d", a)
	}
	if seededIndex("x", 0) != 0 {
		t.Fatal("seededIndex with n=0 must be 0")
	}
}

func TestHasCategory(t *testing.T) {
	cats := []string{"cemeteries", "funeral-art", "statues"}
	if !hasCategory(cats, "cemeteries") {
		t.Fatal("expected exact match")
	}
	if !hasCategory(cats, "funeral") {
		t.Fatal("expected substring match funeral-art")
	}
	if hasCategory(cats, "caves") {
		t.Fatal("unexpected match for caves")
	}
}

func TestBuildClusters(t *testing.T) {
	// Three tight places + one far outlier.
	pool := []AOPlace{
		{ID: 1, Title: "A", Lat: 55.9500, Lng: -3.1900},
		{ID: 2, Title: "B", Lat: 55.9505, Lng: -3.1890},
		{ID: 3, Title: "C", Lat: 55.9510, Lng: -3.1880},
		{ID: 4, Title: "Far", Lat: 56.5000, Lng: -3.0000},
	}
	clusters := buildClusters(pool, 0.6, 3)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].Size != 3 {
		t.Fatalf("expected cluster size 3, got %d", clusters[0].Size)
	}
}

func TestToPlace(t *testing.T) {
	e := aoSearchEntry{Title: "Foo", URL: "/places/foo-bar", ID: 42, DistanceFromQuery: "1.50"}
	e.Coordinates.Lat = 10
	e.Coordinates.Lng = 20
	p := e.toPlace()
	if p.ID != 42 || p.Slug != "foo-bar" || p.Lat != 10 || p.Lng != 20 {
		t.Fatalf("toPlace wrong: %+v", p)
	}
	if p.URL != "https://www.atlasobscura.com/places/foo-bar" {
		t.Fatalf("toPlace URL wrong: %q", p.URL)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
