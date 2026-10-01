package evidence

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

var cityIDRE = regexp.MustCompile(`/tenki/[^/]+/([0-9]+)/`)
var mountainPathRE = regexp.MustCompile(`^/mountain/[a-z]+/[0-9]+/$`)

func (c *Client) Places(ctx context.Context, query string, limit, offset int) (map[string]any, error) {
	query = strings.TrimSpace(query)
	if query == "" || len([]rune(query)) > 100 {
		return nil, fail(2, "--query requires 1–100 characters")
	}
	if e := bounds(limit, offset); e != nil {
		return nil, e
	}
	u := "https://weathernews.jp/onebox/api_search.cgi?" + url.Values{"callback": {""}, "query": {query}, "lang": {"ja"}}.Encode()
	f, e := c.Get(ctx, u, 24*time.Hour)
	if e != nil {
		return nil, e
	}
	var raw []map[string]any
	if json.Unmarshal(f.Body, &raw) != nil || raw == nil {
		return nil, fail(5, "Weathernews location schema changed")
	}
	items := make([]any, 0)
	for _, r := range raw {
		la, lo := num(r["lat"]), num(r["lon"])
		if text(r["loc"]) == nil || la == nil || lo == nil {
			return nil, fail(5, "Weathernews place identity missing")
		}
		if text(r["url"]) == nil {
			return nil, fail(5, "Weathernews place source URL is missing")
		}
		if e := validCoords(*la, *lo); e != nil {
			return nil, fail(5, "Weathernews place coordinates are invalid for Japan coverage")
		}
		uri, e := url.Parse(str(r["url"]))
		if e != nil {
			return nil, fail(5, "Weathernews place URL invalid")
		}
		u := (&url.URL{Scheme: "https", Host: "weathernews.jp"}).ResolveReference(uri)
		u.RawQuery = ""
		u.Fragment = ""
		onebox := strings.HasPrefix(u.Path, "/onebox/") && u.Path != "/onebox/"
		if allowed(u) != nil || (!onebox && !mountainPathRE.MatchString(u.Path)) {
			return nil, fail(5, "Weathernews place URL is not first-party")
		}
		id := "path:" + u.Path
		kind := "place"
		if m := cityIDRE.FindStringSubmatch(u.Path); len(m) == 2 {
			id = "city:" + m[1]
			kind = "city"
		}
		items = append(items, map[string]any{"id": id, "name_ja": str(r["loc"]), "kind": kind, "latitude": *la, "longitude": *lo, "elevation_m": nil, "elevation_status": "not_provided", "url": u.String()})
	}
	page, next := paginate(items, limit, offset)
	return map[string]any{"query": query, "items": page, "total": len(items), "next_offset": next, "coverage": "public source search matches; choose exact identity and coordinates", "source": provenance(f)}, nil
}
func bounds(limit, offset int) error {
	if limit < 1 || limit > 50 || offset < 0 || offset > 10000 {
		return fail(2, "--limit must be 1–50 and --offset 0–10000")
	}
	return nil
}
func paginate(items []any, limit, offset int) ([]any, any) {
	if offset >= len(items) {
		return []any{}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	var next any
	if end < len(items) {
		next = end
	}
	return items[offset:end], next
}
func validCoords(lat, lon float64) error {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.IsInf(lat, 0) || math.IsInf(lon, 0) || lat < 20 || lat > 46 || lon < 122 || lon > 154 {
		return fail(2, "coordinates must be finite within Japan bounding box (lat 20–46, lon 122–154); source verifies Japan coverage")
	}
	return nil
}
func ParseDate(s string) (time.Time, error) {
	d, e := time.ParseInLocation("2006-01-02", s, JST)
	if e != nil {
		return time.Time{}, fail(2, "--date requires a real YYYY-MM-DD date in JST")
	}
	return d, nil
}
func unix(v any) any {
	n := num(v)
	if n == nil || *n <= 0 {
		return nil
	}
	return stamp(time.Unix(int64(*n), 0))
}
func issue(s string) any {
	t, e := time.Parse("2006-01-02T15:04 MST", s)
	if e != nil {
		return nil
	}
	return stamp(t)
}

type ForecastOptions struct {
	Lat, Lon    float64
	Name, Date  string
	Hours, Days int
}

func (c *Client) Forecast(ctx context.Context, o ForecastOptions) (map[string]any, error) {
	if e := validCoords(o.Lat, o.Lon); e != nil {
		return nil, e
	}
	if o.Hours < 0 || o.Hours > 72 || o.Days < 0 || o.Days > 14 || o.Hours+o.Days == 0 {
		return nil, fail(2, "--hours must be 0–72 and --days 0–14, with at least one row requested")
	}
	if o.Date != "" {
		if _, e := ParseDate(o.Date); e != nil {
			return nil, e
		}
	}
	raw := "https://site.weathernews.jp/lba/wxdata/api_data_ss1?" + url.Values{"lat": {fmt.Sprintf("%.6f", o.Lat)}, "lon": {fmt.Sprintf("%.6f", o.Lon)}}.Encode()
	f, e := c.Get(ctx, raw, 10*time.Minute)
	if e != nil {
		return nil, e
	}
	var data map[string]any
	if json.Unmarshal(f.Body, &data) != nil {
		return nil, fail(5, "Weathernews forecast schema changed")
	}
	if e := validateForecastIdentity(data); e != nil {
		return nil, e
	}
	srf, ok := data["srf"].([]any)
	mrf, ok2 := data["mrf"].([]any)
	if !ok || !ok2 || len(srf) == 0 || len(mrf) == 0 {
		return nil, fail(5, "Weathernews forecast horizon is missing")
	}
	hourly := []any{}
	daily := []any{}
	for _, v := range srf {
		r := object(v)
		at := unix(r["tm"])
		if at == nil {
			return nil, fail(5, "hourly valid time missing")
		}
		if o.Date != "" && !strings.HasPrefix(str(at), o.Date) {
			continue
		}
		if len(hourly) >= o.Hours {
			continue
		}
		hourly = append(hourly, map[string]any{"kind": "forecast", "valid_at": at, "issued_at": nil, "temperature_c": number(r["AIRTMP"]), "precipitation_mm": number(r["PREC"]), "precipitation_interval_hours": number(object(data["srf_attr"])["stride"]), "wind_speed_m_s": number(r["WNDSPD"]), "wind_direction_code": number(r["WNDDIR"]), "humidity_percent": number(r["RHUM"]), "weather_code": number(r["WX"])})
	}
	for _, v := range mrf {
		r := object(v)
		at := unix(r["tm"])
		if at == nil {
			return nil, fail(5, "daily valid time missing")
		}
		date := str(at)[:10]
		if o.Date != "" && date != o.Date {
			continue
		}
		if len(daily) >= o.Days {
			continue
		}
		daily = append(daily, map[string]any{"kind": "forecast", "valid_date": date, "timezone": "Asia/Tokyo", "issued_at": nil, "max_temperature_c": number(r["MAXT"]), "min_temperature_c": number(r["MINT"]), "precipitation_probability_percent": number(r["POP"]), "precipitation_mm": number(r["PREC"]), "weather_code": number(r["WX"])})
	}
	// Source stride is seconds; preserve amount units while exposing hours.
	for _, v := range hourly {
		m := object(v)
		if n := num(m["precipitation_interval_hours"]); n != nil {
			m["precipitation_interval_hours"] = *n / 3600
		}
		m["precipitation_interval_alignment"] = "source_hour_label"
	}
	obs := object(data["observation"])
	var observed any
	if len(obs) > 0 {
		observed = map[string]any{"kind": "observation", "valid_at": issue(str(obs["ISSUE"])), "issued_at": nil, "name_ja": text(obs["LNAME"]), "temperature_c": number(obs["AIRTMP"]), "precipitation_mm": number(obs["PREC"]), "precipitation_interval_hours": nil, "wind_speed_m_s": number(obs["WNDSPD"]), "humidity_percent": number(obs["RHUM"]), "weather_code": number(obs["WX"]), "coverage": "public source observation; measurement interval and station elevation not supplied"}
	}
	geo := object(data["geo"])
	location := map[string]any{"id": "jcode:" + str(geo["JCODE"]), "name_ja": text(obs["LNAME"]), "latitude": number(geo["lat"]), "longitude": number(geo["lon"]), "elevation_m": nil, "elevation_status": "not_provided", "grid_resolution_m": nil, "requested_name": text(o.Name), "requested_latitude": o.Lat, "requested_longitude": o.Lon, "url": fmt.Sprintf("https://weathernews.jp/onebox/%.6f/%.6f/", o.Lat, o.Lon)}
	status := "available"
	if o.Date != "" && len(hourly) == 0 && len(daily) == 0 {
		status = "out_of_horizon"
	}
	return map[string]any{"location": location, "status": status, "requested_date": text(o.Date), "timezone": "Asia/Tokyo", "issued_at": nil, "issue_time_status": "not_provided_by_public_forecast", "hourly": hourly, "daily": daily, "observation": observed, "horizon": map[string]any{"hourly_first": unix(object(srf[0])["tm"]), "hourly_last": unix(object(srf[len(srf)-1])["tm"]), "daily_first": str(unix(object(mrf[0])["tm"]))[:10], "daily_last": str(unix(object(mrf[len(mrf)-1])["tm"]))[:10]}, "coverage": "public forecast only; member radar and extended products excluded", "source": provenance(f)}, nil
}

type Point struct {
	Name     string
	Lat, Lon float64
}

func ParsePoint(s string) (Point, error) {
	name, coord, ok := strings.Cut(s, "=")
	if !ok || strings.TrimSpace(name) == "" {
		return Point{}, fail(2, "--point requires name=latitude,longitude (e.g. Kyoto=35.01167,135.76806)")
	}
	parts := strings.Split(coord, ",")
	if len(parts) != 2 {
		return Point{}, fail(2, "--point requires exactly two coordinates")
	}
	var p Point
	p.Name = name
	la, lo := num(parts[0]), num(parts[1])
	if la == nil || lo == nil {
		return p, fail(2, "--point has invalid coordinates")
	}
	p.Lat, p.Lon = *la, *lo
	return p, validCoords(p.Lat, p.Lon)
}

type Criteria struct{ MaxPOP, MaxTemp, MinTemp *float64 }

func CheckDaily(d map[string]any, c Criteria) (string, []any) {
	checks := []any{}
	unknown, failed := false, false
	for _, x := range []struct {
		name, field, op string
		threshold       *float64
	}{{"max_pop_percent", "precipitation_probability_percent", "<=", c.MaxPOP}, {"max_temperature_c", "max_temperature_c", "<=", c.MaxTemp}, {"min_temperature_c", "min_temperature_c", ">=", c.MinTemp}} {
		if x.threshold == nil {
			continue
		}
		v := num(d[x.field])
		var pass any
		if v == nil {
			unknown = true
		} else {
			p := *v <= *x.threshold
			if x.op == ">=" {
				p = *v >= *x.threshold
			}
			pass = p
			if !p {
				failed = true
			}
		}
		checks = append(checks, map[string]any{"criterion": x.name, "operator": x.op, "threshold": *x.threshold, "value": number(d[x.field]), "passes": pass})
	}
	if failed {
		return "does_not_meet", checks
	}
	if unknown {
		return "unknown", checks
	}
	return "meets", checks
}
func (c *Client) CompareWeather(ctx context.Context, points []Point, date string, criteria Criteria) (map[string]any, error) {
	if len(points) < 2 || len(points) > 5 {
		return nil, fail(2, "weather compare requires 2–5 --point candidates")
	}
	if _, e := ParseDate(date); e != nil {
		return nil, e
	}
	if criteria.MaxPOP == nil && criteria.MaxTemp == nil && criteria.MinTemp == nil {
		return nil, fail(2, "supply at least one criterion: --max-pop, --max-temp, --min-temp")
	}
	if criteria.MaxPOP != nil && (*criteria.MaxPOP < 0 || *criteria.MaxPOP > 100 || math.IsNaN(*criteria.MaxPOP)) {
		return nil, fail(2, "--max-pop must be 0–100 percent")
	}
	for _, n := range []*float64{criteria.MaxTemp, criteria.MinTemp} {
		if n != nil && (math.IsNaN(*n) || math.IsInf(*n, 0) || *n < -80 || *n > 60) {
			return nil, fail(2, "temperature thresholds must be finite between -80 and 60 C")
		}
	}
	if criteria.MinTemp != nil && criteria.MaxTemp != nil && *criteria.MinTemp > *criteria.MaxTemp {
		return nil, fail(2, "--min-temp exceeds --max-temp")
	}
	rows := []any{}
	partial := false
	for _, p := range points {
		f, e := c.Forecast(ctx, ForecastOptions{p.Lat, p.Lon, p.Name, date, 0, 1})
		if e != nil {
			partial = true
			rows = append(rows, map[string]any{"name": p.Name, "status": "error", "error": e.Error()})
			continue
		}
		ds := f["daily"].([]any)
		status := "out_of_horizon"
		checks := []any{}
		var day any
		if len(ds) > 0 {
			day = ds[0]
			status, checks = CheckDaily(object(day), criteria)
		}
		rows = append(rows, map[string]any{"name": p.Name, "location": f["location"], "status": status, "checks": checks, "daily": day, "horizon": f["horizon"], "source": f["source"]})
	}
	// Retain input order. Comparison criteria are independent, never a score.
	return map[string]any{"date": date, "timezone": "Asia/Tokyo", "items": rows, "partial": partial, "comparison_basis": "daily source forecast against caller thresholds; no destination score"}, nil
}

// Stable source-ID order makes offset pagination reproducible within a snapshot.
func sortItems(items []any) {
	sort.Slice(items, func(i, j int) bool { return str(object(items[i])["id"]) < str(object(items[j])["id"]) })
}

func validateForecastIdentity(data map[string]any) error {
	coverage, ok := data["is_japan"].(float64)
	if !ok || (coverage != 0 && coverage != 1) {
		return fail(5, "Weathernews schema changed: Japan coverage discriminator missing or invalid")
	}
	if coverage == 0 {
		return fail(3, "Weathernews says coordinates are outside Japan forecast coverage")
	}
	geo := object(data["geo"])
	if geo == nil {
		return fail(5, "Weathernews schema changed: source location is missing")
	}
	code, ok := geo["JCODE"].(string)
	if !ok || !jcodeRE.MatchString(code) {
		return fail(5, "Weathernews schema changed: source JCODE missing or invalid")
	}
	la, lo := num(geo["lat"]), num(geo["lon"])
	if la == nil || lo == nil {
		return fail(5, "Weathernews schema changed: source coordinates missing or invalid")
	}
	if validCoords(*la, *lo) != nil {
		return fail(5, "Weathernews schema changed: source coordinates outside Japan coverage")
	}
	return nil
}

var jcodeRE = regexp.MustCompile(`^[0-9]{5}$`)
