// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.

package arcgis

import (
	"encoding/json"
	"testing"
)

func TestNormalizeLayerURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://x/FeatureServer/0", "https://x/FeatureServer/0"},
		{"https://x/FeatureServer/0/", "https://x/FeatureServer/0"},
		{"https://x/FeatureServer/0/query", "https://x/FeatureServer/0"},
		{"https://x/FeatureServer/0/query/", "https://x/FeatureServer/0"},
		{"  https://x/FeatureServer/0/QUERY  ", "https://x/FeatureServer/0"},
	}
	for _, c := range cases {
		if got := NormalizeLayerURL(c.in); got != c.want {
			t.Errorf("NormalizeLayerURL(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAttrToString(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.5), "3.5"},
		{true, "true"},
		{false, "false"},
	}
	for _, c := range cases {
		if got := AttrToString(c.in); got != c.want {
			t.Errorf("AttrToString(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFeatureOID(t *testing.T) {
	f := Feature{Attributes: map[string]any{"OBJECTID": float64(7), "PIN": "123"}}
	if oid, ok := f.OID("OBJECTID"); !ok || oid != 7 {
		t.Errorf("OID OBJECTID = %d,%v want 7,true", oid, ok)
	}
	if oid, ok := f.OID("PIN"); !ok || oid != 123 {
		t.Errorf("OID PIN(string) = %d,%v want 123,true", oid, ok)
	}
	if _, ok := f.OID("MISSING"); ok {
		t.Errorf("OID MISSING should be false")
	}
}

func TestToGeoJSONGeometry_Point(t *testing.T) {
	raw := json.RawMessage(`{"x":-101.5,"y":33.2}`)
	g, err := ToGeoJSONGeometry(raw, "esriGeometryPoint")
	if err != nil {
		t.Fatal(err)
	}
	if g["type"] != "Point" {
		t.Fatalf("type = %v want Point", g["type"])
	}
	coords := g["coordinates"].([]float64)
	if coords[0] != -101.5 || coords[1] != 33.2 {
		t.Errorf("coords = %v", coords)
	}
}

func TestToGeoJSONGeometry_Polygon(t *testing.T) {
	// One clockwise outer ring => a Polygon with one ring.
	raw := json.RawMessage(`{"rings":[[[0,0],[0,1],[1,1],[1,0],[0,0]]]}`)
	g, err := ToGeoJSONGeometry(raw, "esriGeometryPolygon")
	if err != nil {
		t.Fatal(err)
	}
	if g["type"] != "Polygon" {
		t.Fatalf("type = %v want Polygon", g["type"])
	}
	rings := g["coordinates"].([][][]float64)
	if len(rings) != 1 || len(rings[0]) != 5 {
		t.Errorf("polygon shape unexpected: %v", rings)
	}
}

func TestToGeoJSONGeometry_MultiPolygon(t *testing.T) {
	// Two clockwise outer rings => a MultiPolygon with two polygons.
	raw := json.RawMessage(`{"rings":[[[0,0],[0,1],[1,1],[1,0],[0,0]],[[5,5],[5,6],[6,6],[6,5],[5,5]]]}`)
	g, err := ToGeoJSONGeometry(raw, "esriGeometryPolygon")
	if err != nil {
		t.Fatal(err)
	}
	if g["type"] != "MultiPolygon" {
		t.Fatalf("type = %v want MultiPolygon", g["type"])
	}
	polys := g["coordinates"].([][][][]float64)
	if len(polys) != 2 {
		t.Errorf("want 2 polygons, got %d", len(polys))
	}
}

func TestToGeoJSONGeometry_Empty(t *testing.T) {
	for _, raw := range []json.RawMessage{nil, json.RawMessage("null"), json.RawMessage("")} {
		g, err := ToGeoJSONGeometry(raw, "esriGeometryPolygon")
		if err != nil {
			t.Fatalf("unexpected err for %q: %v", raw, err)
		}
		if g != nil {
			t.Errorf("expected nil geometry for empty input, got %v", g)
		}
	}
}

func TestIsClockwise(t *testing.T) {
	cw := [][]float64{{0, 0}, {0, 1}, {1, 1}, {1, 0}, {0, 0}}
	ccw := [][]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}
	if !isClockwise(cw) {
		t.Error("expected clockwise ring to report clockwise")
	}
	if isClockwise(ccw) {
		t.Error("expected counter-clockwise ring to report not-clockwise")
	}
}

func TestQueryOptionsValues(t *testing.T) {
	o := QueryOptions{Where: "1=1", Geometry: "-1,-1,1,1"}
	v := o.values()
	if v.Get("geometryType") != "esriGeometryEnvelope" {
		t.Errorf("default geometryType = %q", v.Get("geometryType"))
	}
	if v.Get("spatialRel") != "esriSpatialRelIntersects" {
		t.Errorf("default spatialRel = %q", v.Get("spatialRel"))
	}
	if v.Get("outSR") != "4326" {
		t.Errorf("default outSR = %q", v.Get("outSR"))
	}
	if v.Get("inSR") != "4326" {
		t.Errorf("inSR should be set when geometry present")
	}
}
