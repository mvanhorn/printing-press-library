// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"fmt"
	"math"
	"time"
	_ "time/tzdata" // --tz works on hosts without a zoneinfo database
)

// JST is Japan Standard Time. Japan has no daylight saving time.
var JST = time.FixedZone("JST", 9*60*60)

// Date is a calendar date in JST.
type Date struct{ Y, M, D int }

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (Date, error) {
	t, err := time.ParseInLocation("2006-01-02", s, JST)
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: use YYYY-MM-DD", s)
	}
	return DateOf(t), nil
}

// DateOf returns the JST calendar date of t.
func DateOf(t time.Time) Date {
	j := t.In(JST)
	return Date{j.Year(), int(j.Month()), j.Day()}
}

// Time returns the JST time on date d.
func (d Date) Time(hour, min int) time.Time {
	return time.Date(d.Y, time.Month(d.M), d.D, hour, min, 0, 0, JST)
}

func (d Date) String() string { return fmt.Sprintf("%04d-%02d-%02d", d.Y, d.M, d.D) }

// AddDays returns d shifted by n days.
func (d Date) AddDays(n int) Date { return DateOf(d.Time(12, 0).AddDate(0, 0, n)) }

// Before reports whether d is before o.
func (d Date) Before(o Date) bool { return d.Time(0, 0).Before(o.Time(0, 0)) }

// Month returns "YYYY-MM".
func (d Date) Month() string { return fmt.Sprintf("%04d-%02d", d.Y, d.M) }

// DaysUntil returns the number of days from d to o (negative when o is earlier).
func (d Date) DaysUntil(o Date) int {
	return int(math.Round(o.Time(12, 0).Sub(d.Time(12, 0)).Hours() / 24))
}

// InferYear returns the year for a month named without a year, choosing the
// occurrence within six months before or five months after the reference
// month. Sources write "1月のチケット" or "10/1" without a year.
func InferYear(month, refYear, refMonth int) int {
	switch diff := month - refMonth; {
	case diff < -6:
		return refYear + 1
	case diff > 5:
		return refYear - 1
	}
	return refYear
}

// DateRange lists every date from a to b inclusive. Callers bound the range
// first (see MaxRangeDays).
func DateRange(a, b Date) []Date {
	out := make([]Date, 0)
	for d := a; !b.Before(d); d = d.AddDays(1) {
		out = append(out, d)
	}
	return out
}

// FormatJST formats t as RFC3339 in JST.
func FormatJST(t time.Time) string { return t.In(JST).Format(time.RFC3339) }

// ShibuyaLat and ShibuyaLon locate SHIBUYA SKY (Shibuya Scramble Square).
const (
	ShibuyaLat = 35.6585
	ShibuyaLon = 139.7022
)

// Sunset returns the apparent sunset time (JST) at lat/lon on date d using the
// NOAA solar position equations (zenith 90.833 degrees). Accuracy is about
// one minute for mid latitudes.
func Sunset(d Date, lat, lon float64) (time.Time, bool) {
	// Julian day at 00:00 UTC of the calendar date.
	base := time.Date(d.Y, time.Month(d.M), d.D, 0, 0, 0, 0, time.UTC)
	jd0 := float64(base.Unix())/86400.0 + 2440587.5
	// Start from the solar noon estimate, then refine at the sunset estimate.
	minutes := 720 - 4*lon
	for i := 0; i < 3; i++ {
		jc := (jd0 + minutes/1440.0 - 2451545.0) / 36525.0
		eqTime, decl := solarParams(jc)
		ha, valid := hourAngleSunset(lat, decl)
		if !valid {
			return time.Time{}, false
		}
		// Minutes after 00:00 UTC; longitude is positive east.
		minutes = 720 - 4*lon - eqTime + 4*ha
	}
	t := base.Add(time.Duration(minutes * float64(time.Minute))).In(JST)
	return t.Truncate(time.Second), true
}

func solarParams(jc float64) (eqTime, decl float64) {
	rad := math.Pi / 180
	geomMeanLong := math.Mod(280.46646+jc*(36000.76983+jc*0.0003032), 360)
	geomMeanAnom := 357.52911 + jc*(35999.05029-0.0001537*jc)
	ecc := 0.016708634 - jc*(0.000042037+0.0000001267*jc)
	center := math.Sin(rad*geomMeanAnom)*(1.914602-jc*(0.004817+0.000014*jc)) +
		math.Sin(rad*2*geomMeanAnom)*(0.019993-0.000101*jc) +
		math.Sin(rad*3*geomMeanAnom)*0.000289
	trueLong := geomMeanLong + center
	omega := 125.04 - 1934.136*jc
	appLong := trueLong - 0.00569 - 0.00478*math.Sin(rad*omega)
	meanObliq := 23 + (26+(21.448-jc*(46.815+jc*(0.00059-jc*0.001813)))/60)/60
	obliqCorr := meanObliq + 0.00256*math.Cos(rad*omega)
	decl = math.Asin(math.Sin(rad*obliqCorr)*math.Sin(rad*appLong)) / rad
	y := math.Tan(rad*obliqCorr/2) * math.Tan(rad*obliqCorr/2)
	eqTime = 4 / rad * (y*math.Sin(2*rad*geomMeanLong) -
		2*ecc*math.Sin(rad*geomMeanAnom) +
		4*ecc*y*math.Sin(rad*geomMeanAnom)*math.Cos(2*rad*geomMeanLong) -
		0.5*y*y*math.Sin(4*rad*geomMeanLong) -
		1.25*ecc*ecc*math.Sin(2*rad*geomMeanAnom))
	return eqTime, decl
}

func hourAngleSunset(lat, decl float64) (float64, bool) {
	rad := math.Pi / 180
	cosH := math.Cos(rad*90.833)/(math.Cos(rad*lat)*math.Cos(rad*decl)) - math.Tan(rad*lat)*math.Tan(rad*decl)
	if cosH < -1 || cosH > 1 {
		return 0, false
	}
	return math.Acos(cosH) / rad, true
}
