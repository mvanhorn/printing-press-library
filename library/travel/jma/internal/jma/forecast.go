package jma

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
)

type ForecastArea struct {
	Area struct {
		Name string `json:"name"`
		Code string `json:"code"`
	} `json:"area"`
	WeatherCodes  []string `json:"weatherCodes"`
	Weathers      []string `json:"weathers"`
	Winds         []string `json:"winds"`
	Waves         []string `json:"waves"`
	Pops          []string `json:"pops"`
	Temps         []string `json:"temps"`
	Reliabilities []string `json:"reliabilities"`
	TempsMin      []string `json:"tempsMin"`
	TempsMax      []string `json:"tempsMax"`
	MinUpper      []string `json:"tempsMinUpper"`
	MinLower      []string `json:"tempsMinLower"`
	MaxUpper      []string `json:"tempsMaxUpper"`
	MaxLower      []string `json:"tempsMaxLower"`
}
type ForecastSeries struct {
	Times []string       `json:"timeDefines"`
	Areas []ForecastArea `json:"areas"`
}
type ForecastBulletin struct {
	Issued string           `json:"reportDatetime"`
	Office string           `json:"publishingOffice"`
	Series []ForecastSeries `json:"timeSeries"`
}

func at(a []string, n int) any {
	if n >= len(a) || a[n] == "" || a[n] == "-" {
		return nil
	}
	return a[n]
}
func numericAt(a []string, n int) (any, error) {
	x := at(a, n)
	if x == nil {
		return nil, nil
	}
	v, e := strconv.ParseFloat(fmt.Sprint(x), 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, failure(5, "format", "invalid source number %q", x)
	}
	return v, nil
}
func num(s any) any {
	if s == nil {
		return nil
	}
	v, e := strconv.ParseFloat(fmt.Sprint(s), 64)
	if e != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return v
}
func forecastPath(office string) string {
	switch office {
	case "014030":
		return "014100"
	case "460040":
		return "460100"
	}
	return office
}
func arrLength(n int, arrays ...[]string) error {
	for _, a := range arrays {
		if len(a) > 0 && len(a) != n {
			return failure(5, "incomplete", "forecast values (%d) do not match timeDefines (%d)", len(a), n)
		}
	}
	return nil
}
func inSet(s string, set map[string]bool) bool { return set[s] }
func (c *Client) Forecast(ctx context.Context, query, period, station string, days int) (Envelope, error) {
	if days < 1 || days > 7 {
		return Envelope{}, failure(2, "usage", "--days must be 1..7")
	}
	if period != "short" && period != "week" && period != "all" {
		return Envelope{}, failure(2, "usage", "--period must be short, week or all")
	}
	inv, e := c.Inventory(ctx, false)
	if e != nil {
		return Envelope{}, e
	}
	place, e := inv.Resolve(query)
	if e != nil {
		return Envelope{}, e
	}
	districts := map[string]bool{}
	for _, s := range place.ForecastDistrictIDs {
		districts[s] = true
	}
	shortStations := map[string]bool{}
	for _, rows := range inv.Short {
		for _, r := range rows {
			if districts[r.Class10] {
				for _, id := range r.Amedas {
					shortStations[id] = true
				}
			}
		}
	}
	weekRegions, weekStations := map[string]bool{}, map[string]bool{}
	for district := range districts {
		for _, region := range inv.WeekEligible[district] {
			weekRegions[region] = true
		}
	}
	for _, r := range inv.Week[place.OfficeID] {
		if weekRegions[r.Week] {
			weekStations[r.Station] = true
		}
	}
	if station != "" && (period == "short" && !shortStations[station] || period == "week" && !weekStations[station] || period == "all" && !shortStations[station] && !weekStations[station]) {
		return Envelope{}, failure(2, "resolution", "--station %s is not a forecast reference for area %s; run stations search", station, place.ID)
	}
	var bs []ForecastBulletin
	path := "/forecast/data/forecast/" + forecastPath(place.OfficeID) + ".json"
	if e = c.Get(ctx, path, 5*time.Minute, &bs); e != nil {
		return Envelope{}, e
	}
	if len(bs) != 2 {
		return Envelope{}, failure(5, "incomplete", "expected short and weekly forecast bulletins, got %d", len(bs))
	}
	out := []map[string]any{}
	notes := []string{"Temperature values are forecasts at reference stations; no observation data.", "Null means missing in source. Weekly reliability codes and temperature ranges are JMA source values.", "Weekly regions/stations may be broader than the selected district, as authorized by JMA week_area05 eligibility; source IDs retain this coverage."}
	coverage := "resolved district/station forecast"
	missing := []string{}
	seenWeek := map[string]bool{}
	seenShort := map[string]bool{}
	for bidx, b := range bs {
		if period == "short" && bidx == 1 || period == "week" && bidx == 0 {
			continue
		}
		issue, err := parseTime(b.Issued)
		if err != nil {
			return Envelope{}, err
		}
		if b.Office == "" || len(b.Series) < 2 {
			return Envelope{}, failure(5, "incomplete", "forecast bulletin missing office/timeSeries")
		}
		anchor := issue.Truncate(0)
		anchor = time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, JST)
		if bidx == 1 {
			anchor = anchor.AddDate(0, 0, 1)
		}
		end := anchor.AddDate(0, 0, days)
		for _, ts := range b.Series {
			if len(ts.Times) == 0 || len(ts.Areas) == 0 {
				return Envelope{}, failure(5, "incomplete", "empty forecast series")
			}
			for _, a := range ts.Areas {
				isTemp := len(a.Temps) > 0 || len(a.TempsMin) > 0 || len(a.TempsMax) > 0
				allowed := districts
				unitKind := "daily_weather"
				if bidx == 1 {
					allowed = weekRegions
					unitKind = "weekly_weather"
				}
				if isTemp {
					allowed = shortStations
					unitKind = "short_temperature"
					if bidx == 1 {
						allowed = weekStations
						unitKind = "weekly_temperature"
					}
					if station != "" && a.Area.Code != station {
						continue
					}
				}
				if !inSet(a.Area.Code, allowed) {
					continue
				}
				if a.Area.Name == "" {
					return Envelope{}, failure(5, "incomplete", "forecast area missing name")
				}
				if !isTemp {
					if bidx == 1 {
						seenWeek[a.Area.Code] = true
					} else if len(a.WeatherCodes) > 0 {
						seenShort[a.Area.Code] = true
					}
				}
				if e = arrLength(len(ts.Times), a.WeatherCodes, a.Weathers, a.Winds, a.Waves, a.Pops, a.Temps, a.Reliabilities, a.TempsMin, a.TempsMax, a.MinUpper, a.MinLower, a.MaxUpper, a.MaxLower); e != nil {
					return Envelope{}, e
				}
				if bidx == 0 && len(a.Pops) > 0 && len(a.WeatherCodes) == 0 {
					unitKind = "precipitation_probability_6h"
				}
				points := []map[string]any{}
				for n, raw := range ts.Times {
					start, err := parseTime(raw)
					if err != nil {
						return Envelope{}, err
					}
					if !start.Before(end) || start.Before(anchor) {
						continue
					}
					p := map[string]any{"valid_at": start.Format(time.RFC3339)}
					if len(a.WeatherCodes) > 0 {
						until := time.Date(start.Year(), start.Month(), start.Day()+1, 0, 0, 0, 0, JST)
						p["valid_until"] = until.Format(time.RFC3339)
						p["weather_code"] = at(a.WeatherCodes, n)
						p["weather_ja"] = at(a.Weathers, n)
						if bidx == 0 {
							p["wind_ja"] = at(a.Winds, n)
							p["wave_ja"] = at(a.Waves, n)
							p["wave_unit"] = "m (source Japanese text)"
						}
					}
					if len(a.Pops) > 0 {
						value, err := numericAt(a.Pops, n)
						if err != nil {
							return Envelope{}, err
						}
						if value != nil && (value.(float64) < 0 || value.(float64) > 100) {
							return Envelope{}, failure(5, "format", "precipitation probability outside 0..100")
						}
						p["precipitation_probability_pct"] = value
						if bidx == 0 {
							p["valid_until"] = start.Add(6 * time.Hour).Format(time.RFC3339)
						}
					}
					if len(a.Reliabilities) > 0 {
						p["reliability"] = at(a.Reliabilities, n)
					}
					if len(a.Temps) > 0 {
						v, err := numericAt(a.Temps, n)
						if err != nil {
							return Envelope{}, err
						}
						p["temperature_c"] = v
						role := temperatureRole(issue.Hour(), n)
						if role == "unused_source_marker" {
							continue
						}
						if role == "unknown" {
							return Envelope{}, failure(5, "incomplete", "unknown short temperature index %d for issue hour %d", n, issue.Hour())
						}
						p["temperature_type"] = role
						p["valid_until"] = time.Date(start.Year(), start.Month(), start.Day()+1, 0, 0, 0, 0, JST).Format(time.RFC3339)
						p["validity_note"] = "source timeDefines marker for daily minimum/maximum; not an instantaneous temperature"
					}
					if len(a.TempsMin) > 0 || len(a.TempsMax) > 0 {
						for key, a := range map[string][]string{"minimum_c": a.TempsMin, "maximum_c": a.TempsMax, "minimum_upper_c": a.MinUpper, "minimum_lower_c": a.MinLower, "maximum_upper_c": a.MaxUpper, "maximum_lower_c": a.MaxLower} {
							v, err := numericAt(a, n)
							if err != nil {
								return Envelope{}, err
							}
							p[key] = v
						}
						p["valid_until"] = start.AddDate(0, 0, 1).Format(time.RFC3339)
					}
					points = append(points, p)
				}
				if len(points) == 0 {
					continue
				}
				nameEN := nullable(inv.Areas["class10s"][a.Area.Code].EnName)
				if bidx == 1 {
					nameEN = nullable(inv.WeekNames[a.Area.Code]["en"])
				}
				if isTemp {
					nameEN = nullable(inv.StationNames[a.Area.Code])
				}
				out = append(out, map[string]any{"kind": unitKind, "source_id": a.Area.Code, "name_ja": a.Area.Name, "name_en": nameEN, "issued_at": issue.Format(time.RFC3339), "issue_age_hours": ageHours(c.now(), b.Issued), "publisher": b.Office, "points": points})
			}
		}
	}
	if period != "week" {
		for d := range districts {
			if !seenShort[d] {
				missing = append(missing, "short district "+d)
			}
		}
	}
	if period != "short" {
		if len(seenWeek) == 0 {
			missing = append(missing, "weekly region for resolved district")
		}
	}
	if len(out) == 0 {
		return Envelope{}, failure(5, "incomplete", "no forecast values for resolved area/period; source coverage unavailable")
	}
	if len(missing) > 0 {
		coverage = "partial"
		sort.Strings(missing)
		notes = append(notes, "Missing source coverage: "+fmt.Sprint(missing))
	}
	for _, b := range bs {
		if t, err := parseTime(b.Issued); err == nil && c.now().Sub(t) > 24*time.Hour {
			coverage = "stale"
			notes = append(notes, "Source forecast issued more than 24 hours ago; retrieval success does not establish freshness.")
		}
	}
	return c.Envelope(map[string]any{"area": place, "period": period, "days": days, "station_filter": nullable(station), "series": out, "url": canonical("forecast", "offices", place.OfficeID)}, coverage, notes...), nil
}

func temperatureRole(hour, index int) string {
	evening := hour < 5 || hour >= 17
	if evening {
		switch index {
		case 0:
			return "minimum"
		case 1:
			return "maximum"
		}
	} else {
		switch index {
		case 0:
			return "maximum"
		case 1:
			return "unused_source_marker"
		case 2:
			return "minimum"
		case 3:
			return "maximum"
		}
	}
	return "unknown"
}
