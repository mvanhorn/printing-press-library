// Copyright 2026 markvandeven. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Locatieserver shared response types. Solr-shaped: {response: {numFound, docs:
// [...]}, highlighting?: {...}}. Each doc is a flat map of nullable fields.

type lsResponse struct {
	Response     lsResponseBody             `json:"response"`
	Highlighting map[string]json.RawMessage `json:"highlighting,omitempty"`
}

type lsResponseBody struct {
	NumFound int               `json:"numFound"`
	Start    int               `json:"start"`
	MaxScore float64           `json:"maxScore"`
	Docs     []json.RawMessage `json:"docs"`
}

// lsDoc is the lightly-typed shape used by novel commands. Every field is
// optional because Locatieserver returns different fields per `type:`.
type lsDoc struct {
	ID             string          `json:"id,omitempty"`
	Identificatie  string          `json:"identificatie,omitempty"`
	Weergavenaam   string          `json:"weergavenaam,omitempty"`
	Type           string          `json:"type,omitempty"`
	Bron           string          `json:"bron,omitempty"`
	Score          float64         `json:"score,omitempty"`
	Afstand        float64         `json:"afstand,omitempty"`
	Straatnaam     string          `json:"straatnaam,omitempty"`
	Huisnummer     int             `json:"huisnummer,omitempty"`
	Huisletter     string          `json:"huisletter,omitempty"`
	Huisnummertv   string          `json:"huisnummertoevoeging,omitempty"`
	Postcode       string          `json:"postcode,omitempty"`
	Woonplaatsnaam string          `json:"woonplaatsnaam,omitempty"`
	Woonplaatscode string          `json:"woonplaatscode,omitempty"`
	Gemeentenaam   string          `json:"gemeentenaam,omitempty"`
	Gemeentecode   string          `json:"gemeentecode,omitempty"`
	Provincienaam  string          `json:"provincienaam,omitempty"`
	Provinciecode  string          `json:"provinciecode,omitempty"`
	CentroideLLRaw string          `json:"centroide_ll_wkt,omitempty"`
	CentroideRDRaw string          `json:"centroide_rd_wkt,omitempty"`
	CentroideLL    *coord          `json:"centroide_ll,omitempty"`
	CentroideRD    *rdCoord        `json:"centroide_rd,omitempty"`
	GeometrieLL    any             `json:"geometrie_ll,omitempty"`
	GeometrieRD    any             `json:"geometrie_rd,omitempty"`
	Raw            json.RawMessage `json:"_raw,omitempty"`
}

type coord struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

type rdCoord struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// wktPointRE matches WKT POINT(lon lat) or POINT(x y). Coordinates may be
// negative or use scientific notation.
var wktPointRE = regexp.MustCompile(`^POINT\s*\(\s*(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\s+(-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)\s*\)\s*$`)

// parseWKTPoint parses a WKT POINT literal and returns the (x, y) (or (lon,
// lat)) pair. Returns ok=false on unparseable input.
func parseWKTPoint(s string) (x, y float64, ok bool) {
	m := wktPointRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, 0, false
	}
	var err error
	x, err = strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, 0, false
	}
	y, err = strconv.ParseFloat(m[2], 64)
	if err != nil {
		return 0, 0, false
	}
	return x, y, true
}

// enrichLSDoc parses a raw Locatieserver doc, converting WKT centroid strings
// into typed {lon,lat} and {x,y} numbers while keeping the raw WKT under
// CentroideLLRaw / CentroideRDRaw. Highlighting `<b>...</b>` markup in
// weergavenaam (from suggest) is stripped unless keepHighlights is true.
func enrichLSDoc(raw json.RawMessage, keepHighlights bool) lsDoc {
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	var d lsDoc
	d.Raw = raw

	get := func(k string) string {
		if v, ok := m[k]; ok {
			var s string
			if json.Unmarshal(v, &s) == nil {
				return s
			}
		}
		return ""
	}
	getInt := func(k string) int {
		if v, ok := m[k]; ok {
			var n float64
			if json.Unmarshal(v, &n) == nil {
				return int(n)
			}
		}
		return 0
	}
	getFloat := func(k string) float64 {
		if v, ok := m[k]; ok {
			var n float64
			if json.Unmarshal(v, &n) == nil {
				return n
			}
		}
		return 0
	}
	d.ID = get("id")
	d.Identificatie = get("identificatie")
	d.Weergavenaam = get("weergavenaam")
	if !keepHighlights {
		d.Weergavenaam = stripHighlights(d.Weergavenaam)
	}
	d.Type = get("type")
	d.Bron = get("bron")
	d.Score = getFloat("score")
	d.Afstand = getFloat("afstand")
	d.Straatnaam = get("straatnaam")
	d.Huisnummer = getInt("huisnummer")
	d.Huisletter = get("huisletter")
	d.Huisnummertv = get("huisnummertoevoeging")
	d.Postcode = get("postcode")
	d.Woonplaatsnaam = get("woonplaatsnaam")
	d.Woonplaatscode = get("woonplaatscode")
	d.Gemeentenaam = get("gemeentenaam")
	d.Gemeentecode = get("gemeentecode")
	d.Provincienaam = get("provincienaam")
	d.Provinciecode = get("provinciecode")

	if wkt := get("centroide_ll"); wkt != "" {
		d.CentroideLLRaw = wkt
		if lon, lat, ok := parseWKTPoint(wkt); ok {
			d.CentroideLL = &coord{Lon: lon, Lat: lat}
		}
	}
	if wkt := get("centroide_rd"); wkt != "" {
		d.CentroideRDRaw = wkt
		if x, y, ok := parseWKTPoint(wkt); ok {
			d.CentroideRD = &rdCoord{X: x, Y: y}
		}
	}
	if v, ok := m["geometrie_ll"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			d.GeometrieLL = s
		}
	}
	if v, ok := m["geometrie_rd"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			d.GeometrieRD = s
		}
	}
	return d
}

var (
	highlightOpenRE  = regexp.MustCompile(`<b>`)
	highlightCloseRE = regexp.MustCompile(`</b>`)
)

func stripHighlights(s string) string {
	s = highlightOpenRE.ReplaceAllString(s, "")
	s = highlightCloseRE.ReplaceAllString(s, "")
	return s
}

// --------- RD <-> WGS84 (Rijksdriehoek <-> EPSG:4326) -----------------
//
// Polynomial approximation per Wikipedia / Rijksdriehoekstelsel reference. Good
// to roughly 0.25m across the Netherlands. The reference point is the
// Onze Lieve Vrouwe Toren in Amersfoort (155000, 463000 RD; 52.156160°N
// 5.387200°E).

const (
	rdRefX   = 155000.0
	rdRefY   = 463000.0
	wgsRefLa = 52.15517440
	wgsRefLo = 5.38720621
)

// rdToWGS84 converts RD (X, Y) in meters to WGS84 (lon, lat) in decimal
// degrees. Uses the standard polynomial documented by Schreutelkamp & van
// Hees (Stichting De Koepel, 2001) and reproduced on Wikipedia. The sums
// below are accumulated in arc-seconds and added to the reference lat/lon
// (in degrees) after dividing by 3600. Accurate to ~0.25m across the
// Netherlands.
func rdToWGS84(x, y float64) (lon, lat float64) {
	dx := (x - rdRefX) * 1e-5
	dy := (y - rdRefY) * 1e-5

	sumN := 3235.65389*dy +
		-32.58297*dx*dx +
		-0.24750*dy*dy +
		-0.84978*dx*dx*dy +
		-0.06550*dy*dy*dy +
		-0.01709*dx*dx*dy*dy +
		-0.00738*dx +
		0.00530*dx*dx*dx*dx +
		-0.00039*dx*dx*dy*dy*dy +
		0.00033*dx*dx*dx*dx*dy +
		-0.00012*dx*dy

	sumE := 5260.52916*dx +
		105.94684*dx*dy +
		2.45656*dx*dy*dy +
		-0.81885*dx*dx*dx +
		0.05594*dx*dy*dy*dy +
		-0.05607*dx*dx*dx*dy +
		0.01199*dy +
		-0.00256*dx*dx*dx*dy*dy +
		0.00128*dx*dy*dy*dy*dy +
		0.00022*dy*dy +
		-0.00022*dx*dx +
		0.00026*dx*dx*dx*dx*dx

	lat = wgsRefLa + sumN/3600.0
	lon = wgsRefLo + sumE/3600.0
	return lon, lat
}

// wgs84ToRD converts WGS84 (lon, lat) to RD (X, Y) in meters using the same
// Schreutelkamp/van Hees polynomial in the inverse direction. dPhi/dLam are
// scaled by 0.36 (working in tens-of-arc-seconds for numerical conditioning).
func wgs84ToRD(lon, lat float64) (x, y float64) {
	dPhi := 0.36 * (lat - wgsRefLa)
	dLam := 0.36 * (lon - wgsRefLo)

	x = rdRefX +
		190094.945*dLam +
		-11832.228*dPhi*dLam +
		-114.221*dLam*dPhi*dPhi +
		-32.391*dLam*dLam*dLam +
		-0.705*dPhi +
		-2.340*dPhi*dPhi*dPhi*dLam +
		-0.608*dPhi*dLam*dLam*dLam +
		-0.008*dLam*dLam +
		0.148*dPhi*dPhi*dPhi*dPhi*dLam

	y = rdRefY +
		309056.544*dPhi +
		3638.893*dLam*dLam +
		73.077*dPhi*dPhi +
		-157.984*dPhi*dLam*dLam +
		59.788*dPhi*dPhi*dPhi +
		0.433*dLam +
		-6.439*dPhi*dPhi*dLam*dLam +
		-0.032*dPhi*dLam +
		0.092*dPhi*dPhi*dPhi*dPhi*dPhi +
		-0.054*dPhi*dPhi*dPhi*dPhi*dPhi*dLam*dLam

	return x, y
}

// distMeters approximates the great-circle distance between two WGS84 points
// in meters via the haversine formula. Used for nearest-gemeente fallback when
// the gazetteer is loaded.
func distMeters(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000.0
	toRad := func(d float64) float64 { return d * math.Pi / 180.0 }
	a := math.Sin(toRad(lat2-lat1) / 2)
	a = a*a + math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(toRad(lon2-lon1)/2)*math.Sin(toRad(lon2-lon1)/2)
	return 2 * r * math.Asin(math.Min(1, math.Sqrt(a)))
}

// --------- WKT <-> GeoJSON ---------------------------------------------

// wktToGeoJSON converts a subset of WKT geometries (POINT, MULTIPOINT,
// LINESTRING, MULTILINESTRING, POLYGON, MULTIPOLYGON) to a GeoJSON geometry
// object. Other types return an error.
func wktToGeoJSON(wkt string) (map[string]any, error) {
	wkt = strings.TrimSpace(wkt)
	upper := strings.ToUpper(wkt)
	switch {
	case strings.HasPrefix(upper, "POINT"):
		body := wktBody(wkt)
		coords, err := parseCoordList(body)
		if err != nil || len(coords) != 1 {
			return nil, fmt.Errorf("invalid POINT: %s", wkt)
		}
		return map[string]any{"type": "Point", "coordinates": coords[0]}, nil
	case strings.HasPrefix(upper, "MULTIPOINT"):
		body := wktBody(wkt)
		coords, err := parseCoordList(strings.ReplaceAll(strings.ReplaceAll(body, "(", ""), ")", ""))
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "MultiPoint", "coordinates": coords}, nil
	case strings.HasPrefix(upper, "LINESTRING"):
		body := wktBody(wkt)
		coords, err := parseCoordList(body)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "LineString", "coordinates": coords}, nil
	case strings.HasPrefix(upper, "MULTILINESTRING"):
		body := wktBody(wkt)
		parts, err := parseRings(body)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "MultiLineString", "coordinates": parts}, nil
	case strings.HasPrefix(upper, "POLYGON"):
		body := wktBody(wkt)
		rings, err := parseRings(body)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "Polygon", "coordinates": rings}, nil
	case strings.HasPrefix(upper, "MULTIPOLYGON"):
		body := wktBody(wkt)
		polys, err := parseMultiPolygonBody(body)
		if err != nil {
			return nil, err
		}
		return map[string]any{"type": "MultiPolygon", "coordinates": polys}, nil
	}
	return nil, fmt.Errorf("unsupported WKT geometry type: %s", strings.SplitN(wkt, " ", 2)[0])
}

// wktBody extracts everything between the outermost parentheses.
func wktBody(wkt string) string {
	i := strings.Index(wkt, "(")
	j := strings.LastIndex(wkt, ")")
	if i < 0 || j <= i {
		return ""
	}
	return wkt[i+1 : j]
}

// parseCoordList parses "x y, x y, x y" (single ring / linestring body) into
// a [][float64] coordinate slice.
func parseCoordList(s string) ([][]float64, error) {
	parts := strings.Split(s, ",")
	out := make([][]float64, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields := strings.Fields(p)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid coord pair: %q", p)
		}
		x, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid x: %q", fields[0])
		}
		y, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid y: %q", fields[1])
		}
		out = append(out, []float64{x, y})
	}
	return out, nil
}

// parseRings parses "(x y, x y), (x y, x y)" (polygon rings or
// multilinestring components) into [][][]float64.
func parseRings(s string) ([][][]float64, error) {
	out := [][][]float64{}
	depth := 0
	start := -1
	for i, r := range s {
		switch r {
		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ')':
			depth--
			if depth == 0 && start >= 0 {
				body := s[start:i]
				ring, err := parseCoordList(body)
				if err != nil {
					return nil, err
				}
				out = append(out, ring)
				start = -1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parentheses in WKT")
	}
	return out, nil
}

// parseMultiPolygonBody parses "((x y,x y),(x y,x y)),((x y,x y))" into
// [][][][]float64 — one polygon per top-level group, each polygon a slice of
// rings.
func parseMultiPolygonBody(s string) ([][][][]float64, error) {
	out := [][][][]float64{}
	depth := 0
	start := -1
	for i, r := range s {
		switch r {
		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ')':
			depth--
			if depth == 0 && start >= 0 {
				rings, err := parseRings(s[start:i])
				if err != nil {
					return nil, err
				}
				out = append(out, rings)
				start = -1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parentheses in MULTIPOLYGON")
	}
	return out, nil
}

// geoJSONToWKT does the reverse for Point/LineString/Polygon/MultiPolygon
// geometries; other types return an error.
func geoJSONToWKT(g map[string]any) (string, error) {
	typ, _ := g["type"].(string)
	coordsRaw, ok := g["coordinates"]
	if !ok {
		return "", fmt.Errorf("missing coordinates")
	}
	switch typ {
	case "Point":
		c, _ := coordsRaw.([]any)
		if len(c) < 2 {
			return "", fmt.Errorf("Point needs 2 coords")
		}
		return fmt.Sprintf("POINT(%s %s)", numToWKT(c[0]), numToWKT(c[1])), nil
	case "LineString":
		c, _ := coordsRaw.([]any)
		return fmt.Sprintf("LINESTRING(%s)", joinCoords(c)), nil
	case "Polygon":
		c, _ := coordsRaw.([]any)
		return fmt.Sprintf("POLYGON(%s)", joinRings(c)), nil
	case "MultiPoint":
		c, _ := coordsRaw.([]any)
		return fmt.Sprintf("MULTIPOINT(%s)", joinCoords(c)), nil
	case "MultiLineString":
		c, _ := coordsRaw.([]any)
		return fmt.Sprintf("MULTILINESTRING(%s)", joinRings(c)), nil
	case "MultiPolygon":
		c, _ := coordsRaw.([]any)
		parts := make([]string, 0, len(c))
		for _, p := range c {
			rings, _ := p.([]any)
			parts = append(parts, "("+joinRings(rings)+")")
		}
		return "MULTIPOLYGON(" + strings.Join(parts, ",") + ")", nil
	}
	return "", fmt.Errorf("unsupported GeoJSON type: %s", typ)
}

func numToWKT(v any) string {
	switch n := v.(type) {
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	case int:
		return strconv.Itoa(n)
	}
	return fmt.Sprintf("%v", v)
}

func joinCoords(coords []any) string {
	out := make([]string, 0, len(coords))
	for _, c := range coords {
		pair, _ := c.([]any)
		if len(pair) < 2 {
			continue
		}
		out = append(out, fmt.Sprintf("%s %s", numToWKT(pair[0]), numToWKT(pair[1])))
	}
	return strings.Join(out, ",")
}

func joinRings(rings []any) string {
	out := make([]string, 0, len(rings))
	for _, r := range rings {
		ring, _ := r.([]any)
		out = append(out, "("+joinCoords(ring)+")")
	}
	return strings.Join(out, ",")
}
