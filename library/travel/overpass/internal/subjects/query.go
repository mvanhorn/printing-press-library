// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package subjects

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Area is where to search: either a radius around a point, or a bounding box.
type Area struct {
	Lat, Lon float64
	RadiusM  float64
	// BBoxes are searched as a union. More than one is used when a corridor
	// crosses the antimeridian and cannot be expressed as a single box.
	BBoxes []BBox
}

// LargeRadiusWarnM is where public Overpass starts refusing common types.
//
// 🔴 Deliberately a WARNING THRESHOLD, not a ceiling. What decides whether a
// big radius survives is the density of the TYPE, not the radius. Measured
// from Santa Monica, 2026-07-26:
//
//	lighthouse   644 km  -> 80 results in 37 s        (sparse: fine)
//	observatory  644 km  -> 87 results in 17 s        (sparse: fine)
//	water_tower  150 km  -> 164 results in 11 s
//	water_tower  300 km  -> every mirror 504 after 92 s
//	ruins / silo / windmill at 644 km -> all dead
//
// Refusing on radius alone would block the lighthouse query that worked. So
// the CLI warns and sends. The cost of staying silent is the failure this came
// from: the flag is accepted, two minutes of retries burn across three
// mirrors, and nothing in the error mentions the radius.
const LargeRadiusWarnM = 200_000

// LargeRadiusWarning returns a caution for an ambitious radius, or "" when
// there is nothing to say. Returned rather than printed so the decision to
// warn is testable and the I/O stays at the CLI layer.
func LargeRadiusWarning(radiusM float64) string {
	if radiusM <= LargeRadiusWarnM {
		return ""
	}
	return fmt.Sprintf(
		"radius %.0f km is large for public Overpass. Dense types "+
			"(water_tower, ruins, windmill) usually time out above ~%.0f km; "+
			"sparse ones (lighthouse, observatory) often survive. "+
			"If every mirror refuses, tile the area into smaller searches.",
		radiusM/1000, float64(LargeRadiusWarnM)/1000)
}

// BBox is a geographic bounding box.
type BBox struct {
	South, West, North, East float64
}

// Valid reports whether the box is well-formed.
func (b BBox) Valid() bool {
	return b.South < b.North && b.West < b.East &&
		b.South >= -90 && b.North <= 90 && b.West >= -180 && b.East <= 180
}

// BuildQuery renders an Overpass QL program for one subject type in one area.
//
// nwr matches nodes, ways, and relations together: a water tower may be mapped
// as any of the three, and querying only nodes silently loses most of them.
// `out center` gives ways and relations a single representative coordinate so
// every result can be plotted the same way.
func BuildQuery(t Type, a Area, timeoutSec, limit int) (string, error) {
	if len(t.Tags) == 0 {
		return "", fmt.Errorf("subject type %q has no tags", t.Name)
	}
	if timeoutSec <= 0 {
		timeoutSec = 25
	}

	var scopes []string
	switch {
	case len(a.BBoxes) > 0:
		for _, bb := range a.BBoxes {
			if !bb.Valid() {
				return "", fmt.Errorf("bounding box is not valid: %+v", bb)
			}
			scopes = append(scopes, fmt.Sprintf("(%.6f,%.6f,%.6f,%.6f)", bb.South, bb.West, bb.North, bb.East))
		}
	case a.RadiusM > 0:
		if a.Lat < -90 || a.Lat > 90 || a.Lon < -180 || a.Lon > 180 {
			return "", fmt.Errorf("coordinates out of range: %.4f,%.4f", a.Lat, a.Lon)
		}
		scopes = []string{fmt.Sprintf("(around:%.0f,%.6f,%.6f)", a.RadiusM, a.Lat, a.Lon)}
	default:
		return "", fmt.Errorf("give the search an area: a radius around a point, or a bounding box")
	}

	// Each alternative tag becomes its own statement; Overpass unions the
	// results of consecutive statements in a block.
	var b strings.Builder
	fmt.Fprintf(&b, "[out:json][timeout:%d];\n(\n", timeoutSec)
	for _, tag := range t.Tags {
		for _, scope := range scopes {
			fmt.Fprintf(&b, "  nwr%s%s;\n", scope, tag.Selector())
		}
	}
	b.WriteString(");\n")
	if limit > 0 {
		fmt.Fprintf(&b, "out center %d;", limit)
	} else {
		b.WriteString("out center;")
	}
	return b.String(), nil
}

// Element is one raw Overpass result.
type Element struct {
	Type   string  `json:"type"`
	ID     int64   `json:"id"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Center *struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	} `json:"center"`
	Tags map[string]string `json:"tags"`
}

// Subject is a cleaned-up result.
type Subject struct {
	OSMType   string  `json:"osm_type"`
	OSMID     int64   `json:"osm_id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	// DistanceKM is filled in when the search had a centre point.
	DistanceKM float64           `json:"distance_km,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	URL        string            `json:"url"`
}

// ParseElements converts an Overpass response body into subjects.
//
// Nodes carry lat/lon directly while ways and relations carry a `center`
// object instead; reading only lat/lon drops every way and relation with no
// error, which looks exactly like "there is nothing there".
func ParseElements(body []byte, origin *Area) ([]Subject, string, error) {
	var resp struct {
		Elements []Element `json:"elements"`
		Remark   string    `json:"remark"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("parsing Overpass response: %w", err)
	}
	if resp.Remark != "" && len(resp.Elements) == 0 {
		return nil, "", fmt.Errorf("overpass reported: %s", resp.Remark)
	}

	out := make([]Subject, 0, len(resp.Elements))
	for _, e := range resp.Elements {
		lat, lon := e.Lat, e.Lon
		if e.Center != nil {
			lat, lon = e.Center.Lat, e.Center.Lon
		}
		if lat == 0 && lon == 0 {
			continue
		}
		s := Subject{
			OSMType: e.Type, OSMID: e.ID,
			Name:      e.Tags["name"],
			Latitude:  lat,
			Longitude: lon,
			Tags:      e.Tags,
			URL:       fmt.Sprintf("https://www.openstreetmap.org/%s/%d", e.Type, e.ID),
		}
		if s.Name == "" {
			s.Name = "(unnamed)"
		}
		if origin != nil && origin.RadiusM > 0 {
			s.DistanceKM = HaversineKM(origin.Lat, origin.Lon, lat, lon)
		}
		out = append(out, s)
	}
	// A remark alongside results means the server truncated the query. Losing
	// it renders a partial answer as a complete one.
	return out, resp.Remark, nil
}

// HaversineKM returns the great-circle distance between two points.
func HaversineKM(lat1, lon1, lat2, lon2 float64) float64 {
	const earthKM = 6371.0
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLon := rad(lat2-lat1), rad(lon2-lon1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthKM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// CorridorBBox returns a bounding box covering a straight line between two
// points, widened by padKM on every side.
//
// This is a rectangle around the whole line, not a true buffered corridor, so
// a long diagonal drive sweeps in area well off the road. Callers should say
// so rather than implying the results are all near the route.
func CorridorBBox(lat1, lon1, lat2, lon2, padKM float64) []BBox {
	latPad := padKM / 111.0
	// Longitude degrees shrink with latitude; use the wider of the two ends
	// so the pad is never narrower than requested.
	c := math.Cos(math.Pi / 180 * math.Max(math.Abs(lat1), math.Abs(lat2)))
	if c < 0.01 {
		c = 0.01
	}
	lonPad := padKM / (111.0 * c)

	south := math.Max(math.Min(lat1, lat2)-latPad, -90)
	north := math.Min(math.Max(lat1, lat2)+latPad, 90)

	// If the two points are closer going the other way round the globe, the
	// route crosses the antimeridian. Taking min/max of the raw longitudes
	// there produces a box spanning nearly 360 degrees: two Fijian islands
	// 31 km apart yielded West -180 / East 180, and the "corridor" then
	// contained Brazil.
	west, east := math.Min(lon1, lon2), math.Max(lon1, lon2)
	if east-west > 180 {
		// Crossing: the corridor runs east from the larger longitude, over
		// +/-180, to the smaller one. Express it as two boxes.
		left := BBox{South: south, North: north, West: math.Max(east-lonPad, -180), East: 180}
		right := BBox{South: south, North: north, West: -180, East: math.Min(west+lonPad, 180)}
		var out []BBox
		if left.Valid() {
			out = append(out, left)
		}
		if right.Valid() {
			out = append(out, right)
		}
		if len(out) > 0 {
			return out
		}
	}
	return []BBox{{
		South: south, North: north,
		West: math.Max(west-lonPad, -180),
		East: math.Min(east+lonPad, 180),
	}}
}

// GeoJSON renders subjects as a FeatureCollection.
func GeoJSON(subs []Subject) ([]byte, error) {
	return GeoJSONWithRemark(subs, "")
}

// GeoJSONWithRemark is GeoJSON with Overpass's truncation remark attached as a
// FeatureCollection foreign member. The warning printed to stderr at query time
// is gone the moment the file is opened again a week later, and a truncated
// export is indistinguishable from a complete one on the map; RFC 7946 allows
// foreign members, and readers that do not know the key ignore it.
func GeoJSONWithRemark(subs []Subject, remark string) ([]byte, error) {
	type feature struct {
		Type     string         `json:"type"`
		Geometry map[string]any `json:"geometry"`
		Props    map[string]any `json:"properties"`
	}
	feats := make([]feature, 0, len(subs))
	for _, s := range subs {
		props := map[string]any{
			"name": s.Name, "osm_type": s.OSMType, "osm_id": s.OSMID, "url": s.URL,
		}
		if s.DistanceKM > 0 {
			props["distance_km"] = math.Round(s.DistanceKM*100) / 100
		}
		for k, v := range s.Tags {
			if k != "name" {
				props["osm:"+k] = v
			}
		}
		feats = append(feats, feature{
			Type:     "Feature",
			Geometry: map[string]any{"type": "Point", "coordinates": []float64{s.Longitude, s.Latitude}},
			Props:    props,
		})
	}
	// json.Marshal escapes &, < and > into & etc. by default, which is a
	// safety measure for JSON embedded in HTML. GeoJSON is a map file, not a
	// script tag, and the escaping surfaced as "The Tiffany & Co.
	// Foundation Overlook" in an export a human is expected to read.
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	doc := map[string]any{
		"type": "FeatureCollection", "features": feats,
	}
	if remark != "" {
		doc["partial"] = true
		doc["partial_remark"] = remark
	}
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	// Encode appends a newline; callers print or write the bytes as-is.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// ParseDistance turns "25km", "10mi", or a bare number of kilometres into
// metres.
func ParseDistance(s string) (float64, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		return 0, fmt.Errorf("give a distance, e.g. 25km or 10mi")
	}
	// Unit must be recognised. Falling through to kilometres meant "25miles"
	// silently became 25 km — 62% of the area asked for, presented as a
	// complete result.
	mult := 1000.0
	switch {
	case strings.HasSuffix(v, "km"):
		v = strings.TrimSuffix(v, "km")
	case strings.HasSuffix(v, "mi"):
		v, mult = strings.TrimSuffix(v, "mi"), 1609.344
	case strings.HasSuffix(v, "m"):
		v, mult = strings.TrimSuffix(v, "m"), 1.0
	default:
		if strings.ContainsFunc(v, func(r rune) bool {
			return (r >= 'a' && r <= 'z') || r == '+'
		}) {
			return 0, fmt.Errorf("unrecognised distance %q; use km, mi, or m (e.g. 25km, 10mi, 500m)", s)
		}
	}
	var n float64
	if _, err := fmt.Sscanf(strings.TrimSpace(v), "%g", &n); err != nil {
		return 0, fmt.Errorf("could not read %q as a distance; try 25km or 10mi", s)
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, fmt.Errorf("distance must be a finite number, got %q", s)
	}
	if n <= 0 {
		return 0, fmt.Errorf("distance must be positive, got %q", s)
	}
	return n * mult, nil
}
