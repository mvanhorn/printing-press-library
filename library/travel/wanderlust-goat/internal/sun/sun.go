// Package sun computes sunrise, sunset, blue-hour, and golden-hour times
// for a (lat, lng, date) using the SunCalc algorithm. Pure Go, no external
// API. Algorithm is the standard Mike Hocevar / NOAA approximation used
// by SunCalc.js — accurate to within ~1 minute for civilian use, more than
// enough for "when's blue hour tonight."
package sun

import (
	"fmt"
	"math"
	"time"
)

// Times groups the moments returned for a single day at a single location.
// Civil twilight is the standard "blue hour" boundary; golden hour starts
// when the sun is 6° above the horizon descending in the evening or rising.
type Times struct {
	Sunrise          time.Time `json:"sunrise"`
	Sunset           time.Time `json:"sunset"`
	GoldenHourMorn   Window    `json:"golden_hour_morning"`
	GoldenHourEve    Window    `json:"golden_hour_evening"`
	BlueHourMorn     Window    `json:"blue_hour_morning"`
	BlueHourEve      Window    `json:"blue_hour_evening"`
	CivilDawn        time.Time `json:"civil_dawn"`
	CivilDusk        time.Time `json:"civil_dusk"`
}

// Window represents an open interval [Start, End].
type Window struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Compute returns the sun-position times for the given date at (lat, lng).
// `date` is treated as a calendar day in UTC; the returned times are in
// UTC and the caller can translate to a local zone for display.
func Compute(date time.Time, lat, lng float64) Times {
	day := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, time.UTC)
	noon := solarNoon(day, lng)

	// Solar elevation angles in degrees:
	//   golden-hour evening starts at +6°, ends at -0.833° (sunset)
	//   blue-hour evening starts at -4°, ends at -8°
	//   civil dawn/dusk: -6°
	// Source: SunCalc.js + NOAA terminology.
	sunriseUp := timeForAltitude(day, lat, lng, -0.833, true)
	sunsetDown := timeForAltitude(day, lat, lng, -0.833, false)
	goldenStartUp := timeForAltitude(day, lat, lng, 6.0, true)
	goldenEndDown := timeForAltitude(day, lat, lng, 6.0, false)
	blueStartUp := timeForAltitude(day, lat, lng, -4.0, true)
	blueEndUp := timeForAltitude(day, lat, lng, -8.0, true)
	blueStartDown := timeForAltitude(day, lat, lng, -4.0, false)
	blueEndDown := timeForAltitude(day, lat, lng, -8.0, false)
	civilDawn := timeForAltitude(day, lat, lng, -6.0, true)
	civilDusk := timeForAltitude(day, lat, lng, -6.0, false)

	_ = noon
	return Times{
		Sunrise:        sunriseUp,
		Sunset:         sunsetDown,
		GoldenHourMorn: Window{Start: sunriseUp, End: goldenStartUp},
		GoldenHourEve:  Window{Start: goldenEndDown, End: sunsetDown},
		// Morning blue hour: dark → light, so start is the deeper -8° and end is -4°.
		BlueHourMorn: Window{Start: blueEndUp, End: blueStartUp},
		// Evening blue hour: light → dark, so start is -4° and end is -8°.
		BlueHourEve: Window{Start: blueStartDown, End: blueEndDown},
		CivilDawn:   civilDawn,
		CivilDusk:   civilDusk,
	}
}

// MarshalString returns a human-readable summary in the location's local zone.
func (t Times) MarshalString(zone *time.Location) string {
	if zone == nil {
		zone = time.UTC
	}
	fmtT := func(x time.Time) string { return x.In(zone).Format("15:04 MST") }
	return fmt.Sprintf("sunrise=%s · sunset=%s · golden_eve=%s→%s · blue_eve=%s→%s",
		fmtT(t.Sunrise), fmtT(t.Sunset),
		fmtT(t.GoldenHourEve.Start), fmtT(t.GoldenHourEve.End),
		fmtT(t.BlueHourEve.Start), fmtT(t.BlueHourEve.End))
}

// --- algorithmic helpers (NOAA / SunCalc) -----

const rad = math.Pi / 180.0

func toJulian(t time.Time) float64 {
	return float64(t.Unix())/86400.0 + 2440587.5
}

func fromJulian(j float64) time.Time {
	sec := int64((j - 2440587.5) * 86400.0)
	return time.Unix(sec, 0).UTC()
}

func toDays(t time.Time) float64 { return toJulian(t) - 2451545.0 }

func solarMeanAnomaly(d float64) float64 { return rad * (357.5291 + 0.98560028*d) }

func eclipticLongitude(M float64) float64 {
	C := rad * (1.9148*math.Sin(M) + 0.02*math.Sin(2*M) + 0.0003*math.Sin(3*M))
	const P = rad * 102.9372
	return M + C + P + math.Pi
}

func declination(L, B float64) float64 {
	const e = rad * 23.4397
	return math.Asin(math.Sin(B)*math.Cos(e) + math.Cos(B)*math.Sin(e)*math.Sin(L))
}

func solarNoon(t time.Time, lng float64) time.Time {
	d := toDays(t)
	n := math.Round(d - 0.0009 - lng/360.0)
	approxJ := 0.0009 + lng/360.0 + n
	M := solarMeanAnomaly(approxJ)
	L := eclipticLongitude(M)
	J := 2451545.0 + approxJ + 0.0053*math.Sin(M) - 0.0069*math.Sin(2*L)
	return fromJulian(J)
}

// timeForAltitude returns the time at which the sun reaches `altDeg`
// elevation. `morning` selects the ascending crossing (true) or the
// descending crossing (false). Returns the noon time if the altitude is
// never reached (polar day/night fallback).
func timeForAltitude(t time.Time, lat, lng, altDeg float64, morning bool) time.Time {
	d := toDays(t)
	n := math.Round(d - 0.0009 - lng/360.0)
	approxJ := 0.0009 + lng/360.0 + n
	M := solarMeanAnomaly(approxJ)
	L := eclipticLongitude(M)
	dec := declination(L, 0)
	noonJ := 2451545.0 + approxJ + 0.0053*math.Sin(M) - 0.0069*math.Sin(2*L)

	cosH := (math.Sin(altDeg*rad) - math.Sin(lat*rad)*math.Sin(dec)) /
		(math.Cos(lat*rad) * math.Cos(dec))
	if cosH < -1 || cosH > 1 {
		return fromJulian(noonJ)
	}
	H := math.Acos(cosH)
	dt := H / (2 * math.Pi)
	if morning {
		return fromJulian(noonJ - dt)
	}
	return fromJulian(noonJ + dt)
}
