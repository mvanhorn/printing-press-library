// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored shared helpers for the Ride with GPS transcendence commands
// (export, gear, stats, records, dedup, audit, events routes). Units, geometry,
// detail-response parsing, and GPX/TCX/CSV serialization live here so each
// novel command file stays small. This file is hand-authored and preserved
// across regeneration (no generated-file header).
package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	metersPerKM   = 1000.0
	metersPerMile = 1609.344
)

func metersToKM(m float64) float64    { return m / metersPerKM }
func metersToMiles(m float64) float64 { return m / metersPerMile }

// roundN rounds to n decimal places for display.
func roundN(v float64, n int) float64 {
	p := math.Pow(10, float64(n))
	return math.Round(v*p) / p
}

// secondsToHMS renders a duration in seconds as H:MM:SS.
func secondsToHMS(sec float64) string {
	if sec <= 0 {
		return "0:00:00"
	}
	d := time.Duration(sec) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}

// haversineMeters returns the great-circle distance between two lat/lng points.
func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusM = 6371000.0
	toRad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// trackPoint mirrors the Ride with GPS track-point shape. Pointers distinguish
// "field absent" from "value is zero" — critical for elevation/sensor fields
// that are legitimately 0 on some points.
type trackPoint struct {
	X *float64 `json:"x"` // longitude
	Y *float64 `json:"y"` // latitude
	E *float64 `json:"e"` // elevation, meters
	D *float64 `json:"d"` // distance from start, meters
	T *float64 `json:"t"` // unix epoch seconds (trips only)
	S *float64 `json:"s"` // speed, km/h
	H *float64 `json:"h"` // heart rate, bpm
	C *float64 `json:"c"` // cadence, rpm
	P *float64 `json:"p"` // power, watts
}

// coursePoint mirrors the Ride with GPS cue-sheet shape.
type coursePoint struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
	D *float64 `json:"d"`
	T string   `json:"t"` // cue type, e.g. "Right"
	N string   `json:"n"` // cue text
}

// gearObj is the embedded gear object on a trip.
type gearObj struct {
	ID    json.Number `json:"id"`
	Make  string      `json:"make"`
	Model string      `json:"model"`
}

// assetDetail is a route or trip detail response, tolerant of both the v1
// envelope ({"route": {...}} / {"trip": {...}}) and the legacy unwrapped shape.
type assetDetail struct {
	ID            json.Number   `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Distance      float64       `json:"distance"`
	ElevationGain float64       `json:"elevation_gain"`
	DepartedAt    string        `json:"departed_at"`
	TrackPoints   []trackPoint  `json:"track_points"`
	CoursePoints  []coursePoint `json:"course_points"`
	Gear          *gearObj      `json:"gear"`
}

// unwrapAssetDetail extracts the asset object from a detail response, handling
// the v1 singular-key envelope and the legacy top-level shape.
func unwrapAssetDetail(raw json.RawMessage, key string) (*assetDetail, error) {
	// Try the envelope first: {"route": {...}} or {"trip": {...}}.
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err == nil {
		if inner, ok := env[key]; ok && len(inner) > 0 {
			var d assetDetail
			if err := json.Unmarshal(inner, &d); err != nil {
				return nil, err
			}
			return &d, nil
		}
	}
	// Fall back to the unwrapped (legacy) top-level object.
	var d assetDetail
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// xmlEscape escapes a string for safe inclusion in GPX/TCX element text.
func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// buildGPX serializes track points (and optional cue waypoints) into a GPX 1.1
// document. Trip track points carry timestamps; route track points do not.
func buildGPX(name string, tps []trackPoint, cps []coursePoint) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<gpx version="1.1" creator="ridewithgps-pp-cli" xmlns="http://www.topografix.com/GPX/1/1">` + "\n")
	b.WriteString("  <metadata><name>" + xmlEscape(name) + "</name></metadata>\n")
	// Cue sheet as routepoints/waypoints so head units keep turn-by-turn.
	for _, cp := range cps {
		if cp.Y == nil || cp.X == nil {
			continue
		}
		label := strings.TrimSpace(cp.N)
		if label == "" {
			label = strings.TrimSpace(cp.T)
		}
		b.WriteString(fmt.Sprintf("  <wpt lat=\"%g\" lon=\"%g\"><name>%s</name><type>%s</type></wpt>\n",
			*cp.Y, *cp.X, xmlEscape(label), xmlEscape(cp.T)))
	}
	b.WriteString("  <trk><name>" + xmlEscape(name) + "</name><trkseg>\n")
	for _, tp := range tps {
		if tp.Y == nil || tp.X == nil {
			continue
		}
		b.WriteString(fmt.Sprintf("    <trkpt lat=\"%g\" lon=\"%g\">", *tp.Y, *tp.X))
		if tp.E != nil {
			b.WriteString(fmt.Sprintf("<ele>%g</ele>", *tp.E))
		}
		if tp.T != nil {
			b.WriteString("<time>" + time.Unix(int64(*tp.T), 0).UTC().Format(time.RFC3339) + "</time>")
		}
		b.WriteString("</trkpt>\n")
	}
	b.WriteString("  </trkseg></trk>\n</gpx>\n")
	return b.String()
}

// buildTCX serializes trip track points into a minimal Garmin TCX document.
func buildTCX(name string, tps []trackPoint, startUnix float64) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<TrainingCenterDatabase xmlns="http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2">` + "\n")
	b.WriteString("  <Activities><Activity Sport=\"Biking\">\n")
	startTime := time.Now().UTC()
	if startUnix > 0 {
		startTime = time.Unix(int64(startUnix), 0).UTC()
	} else if len(tps) > 0 && tps[0].T != nil {
		startTime = time.Unix(int64(*tps[0].T), 0).UTC()
	}
	b.WriteString("    <Id>" + startTime.Format(time.RFC3339) + "</Id>\n")
	b.WriteString("    <Lap StartTime=\"" + startTime.Format(time.RFC3339) + "\"><Track>\n")
	for _, tp := range tps {
		b.WriteString("      <Trackpoint>")
		if tp.T != nil {
			b.WriteString("<Time>" + time.Unix(int64(*tp.T), 0).UTC().Format(time.RFC3339) + "</Time>")
		}
		if tp.Y != nil && tp.X != nil {
			b.WriteString(fmt.Sprintf("<Position><LatitudeDegrees>%g</LatitudeDegrees><LongitudeDegrees>%g</LongitudeDegrees></Position>", *tp.Y, *tp.X))
		}
		if tp.E != nil {
			b.WriteString(fmt.Sprintf("<AltitudeMeters>%g</AltitudeMeters>", *tp.E))
		}
		if tp.D != nil {
			b.WriteString(fmt.Sprintf("<DistanceMeters>%g</DistanceMeters>", *tp.D))
		}
		if tp.H != nil {
			b.WriteString(fmt.Sprintf("<HeartRateBpm><Value>%g</Value></HeartRateBpm>", *tp.H))
		}
		if tp.C != nil {
			b.WriteString(fmt.Sprintf("<Cadence>%g</Cadence>", *tp.C))
		}
		b.WriteString("</Trackpoint>\n")
	}
	b.WriteString("    </Track></Lap></Activity></Activities>\n</TrainingCenterDatabase>\n")
	return b.String()
}

// buildCSV serializes track points to CSV. Columns present depend on the data;
// absent sensor fields render as empty cells.
func buildCSV(tps []trackPoint) string {
	var b strings.Builder
	b.WriteString("lat,lng,elevation_m,distance_m,time,speed_kmh,heart_rate,cadence,power_w\n")
	cell := func(p *float64) string {
		if p == nil {
			return ""
		}
		return fmt.Sprintf("%g", *p)
	}
	for _, tp := range tps {
		ts := ""
		if tp.T != nil {
			ts = time.Unix(int64(*tp.T), 0).UTC().Format(time.RFC3339)
		}
		b.WriteString(strings.Join([]string{
			cell(tp.Y), cell(tp.X), cell(tp.E), cell(tp.D), ts,
			cell(tp.S), cell(tp.H), cell(tp.C), cell(tp.P),
		}, ",") + "\n")
	}
	return b.String()
}
