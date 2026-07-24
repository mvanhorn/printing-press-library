// Copyright 2026 togorashi45 and contributors. Licensed under Apache-2.0. See LICENSE.

package arcgis

import (
	"encoding/json"
	"fmt"
)

// esriGeom is the subset of esri geometry shapes we convert to GeoJSON.
type esriGeom struct {
	X      *float64        `json:"x"`
	Y      *float64        `json:"y"`
	Points json.RawMessage `json:"points"`
	Paths  json.RawMessage `json:"paths"`
	Rings  json.RawMessage `json:"rings"`
}

// ToGeoJSONGeometry converts a raw esri geometry (as returned by /query) to a
// GeoJSON geometry object. Returns nil for empty/absent geometry.
func ToGeoJSONGeometry(raw json.RawMessage, esriGeometryType string) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var g esriGeom
	if err := json.Unmarshal(raw, &g); err != nil {
		return nil, err
	}
	switch {
	case g.X != nil && g.Y != nil:
		return map[string]any{"type": "Point", "coordinates": []float64{*g.X, *g.Y}}, nil
	case len(g.Points) > 0:
		var pts [][]float64
		if err := json.Unmarshal(g.Points, &pts); err != nil {
			return nil, err
		}
		return map[string]any{"type": "MultiPoint", "coordinates": pts}, nil
	case len(g.Paths) > 0:
		var paths [][][]float64
		if err := json.Unmarshal(g.Paths, &paths); err != nil {
			return nil, err
		}
		if len(paths) == 1 {
			return map[string]any{"type": "LineString", "coordinates": paths[0]}, nil
		}
		return map[string]any{"type": "MultiLineString", "coordinates": paths}, nil
	case len(g.Rings) > 0:
		var rings [][][]float64
		if err := json.Unmarshal(g.Rings, &rings); err != nil {
			return nil, err
		}
		// Esri packs all rings (outer + holes) into one array. GeoJSON needs
		// them grouped per polygon by winding order. Group outer rings
		// (clockwise) with the holes (counter-clockwise) that follow them.
		polys := groupRings(rings)
		if len(polys) == 1 {
			return map[string]any{"type": "Polygon", "coordinates": polys[0]}, nil
		}
		return map[string]any{"type": "MultiPolygon", "coordinates": polys}, nil
	}
	return nil, nil
}

// groupRings splits esri rings into GeoJSON polygons. Esri outer rings are
// clockwise (positive signed area in screen coords => negative in math coords);
// holes are counter-clockwise. Each new outer ring starts a new polygon.
func groupRings(rings [][][]float64) [][][][]float64 {
	var polys [][][][]float64
	for _, ring := range rings {
		if isClockwise(ring) || len(polys) == 0 {
			polys = append(polys, [][][]float64{ring})
		} else {
			polys[len(polys)-1] = append(polys[len(polys)-1], ring)
		}
	}
	return polys
}

func isClockwise(ring [][]float64) bool {
	var sum float64
	for i := 0; i < len(ring)-1; i++ {
		if len(ring[i]) < 2 || len(ring[i+1]) < 2 {
			continue
		}
		sum += (ring[i+1][0] - ring[i][0]) * (ring[i+1][1] + ring[i][1])
	}
	return sum > 0
}

// FeatureToGeoJSON converts a Feature to a GeoJSON Feature object.
func FeatureToGeoJSON(f Feature, esriGeometryType string) (map[string]any, error) {
	geom, err := ToGeoJSONGeometry(f.Geometry, esriGeometryType)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type":       "Feature",
		"properties": f.Attributes,
		"geometry":   geom,
	}, nil
}

// FieldNames returns the attribute column names in a stable order derived from
// the layer fields, falling back to the first feature's keys.
func FieldNames(fields []Field, sample map[string]any) []string {
	if len(fields) > 0 {
		names := make([]string, 0, len(fields))
		for _, f := range fields {
			names = append(names, f.Name)
		}
		return names
	}
	names := make([]string, 0, len(sample))
	for k := range sample {
		names = append(names, k)
	}
	return names
}

// AttrToString renders an attribute value for CSV output.
func AttrToString(v any) string {
	if v == nil {
		return ""
	}
	switch n := v.(type) {
	case string:
		return n
	case float64:
		// Render whole numbers without a trailing .0
		if n == float64(int64(n)) {
			return fmt.Sprintf("%d", int64(n))
		}
		return fmt.Sprintf("%g", n)
	case bool:
		if n {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}
