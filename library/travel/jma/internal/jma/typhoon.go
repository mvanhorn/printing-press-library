package jma

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Cyclone struct {
	ID       string `json:"tropicalCyclone"`
	Number   string `json:"typhoonNumber"`
	Category string `json:"category"`
	Issue    string `json:"issue"`
}

var cycloneID = regexp.MustCompile(`^TC[0-9]{4}$`)

func (c *Client) cycloneList(ctx context.Context) ([]Cyclone, error) {
	var xs []Cyclone
	if e := c.Get(ctx, "/typhoon/data/targetTc.json", time.Minute, &xs); e != nil {
		return nil, e
	}
	if xs == nil {
		return nil, failure(5, "incomplete", "cyclone index null; only [] means none listed")
	}
	seen := map[string]bool{}
	for _, x := range xs {
		if !cycloneID.MatchString(x.ID) || x.Category == "" || seen[x.ID] {
			return nil, failure(5, "format", "invalid/duplicate cyclone index record")
		}
		if _, e := parseTime(x.Issue); e != nil {
			return nil, e
		}
		seen[x.ID] = true
	}
	sort.Slice(xs, func(a, b int) bool { return xs[a].ID < xs[b].ID })
	return xs, nil
}
func (c *Client) TyphoonList(ctx context.Context, offset, limit int) (Envelope, error) {
	xs, e := c.cycloneList(ctx)
	if e != nil {
		return Envelope{}, e
	}
	rows := []map[string]any{}
	for _, x := range xs {
		issued, _ := timeJST(x.Issue)
		rows = append(rows, map[string]any{"id": x.ID, "number": x.Number, "category": x.Category, "issued_at": issued, "issue_age_hours": ageHours(c.now(), x.Issue), "url": Origin + "/map.html#contents=typhoon&tc_num=" + x.ID + "&lang=ja"})
	}
	items, p, e := pageRows(rows, offset, limit)
	v := c.Envelope(items, "active index only; developing tropical depressions included", "Empty results mean JMA index listed no cyclones at retrieval; retrieval errors remain failures.")
	v.Page = p
	return v, e
}

type tcTime struct {
	JST string `json:"JST"`
	UTC string `json:"UTC"`
}
type tcPoint struct {
	Part   json.RawMessage   `json:"part"`
	Issue  tcTime            `json:"issue"`
	Valid  tcTime            `json:"validtime"`
	Number string            `json:"typhoonNumber"`
	Name   map[string]string `json:"name"`
	Hours  *int              `json:"advancedHours"`
	Center []float64         `json:"center"`
	Circle *struct {
		Radius *float64 `json:"radius"`
	} `json:"probabilityCircle"`
	Track    map[string][][]float64 `json:"track"`
	Gale     any                    `json:"galeWarningArea"`
	Storm    any                    `json:"stormWarningArea"`
	Category map[string]string      `json:"category"`
	Position struct {
		Deg      []float64 `json:"deg"`
		Accuracy string    `json:"accuracy"`
	} `json:"position"`
	Pressure          any                       `json:"pressure"`
	MaximumWind       map[string]map[string]any `json:"maximumWind"`
	Speed             map[string]any            `json:"speed"`
	ProbabilityRadius map[string]any            `json:"probabilityCircleRadius"`
	Location          string                    `json:"location"`
	Course            string                    `json:"course"`
	Scale             any                       `json:"scale"`
	Intensity         any                       `json:"intensity"`
	GaleSpecs         any                       `json:"galeWarning"`
	StormSpecs        any                       `json:"stormWarning"`
}

func title(p tcPoint) bool { return string(p.Part) == `"title"` }
func sourceNumber(v any) (any, error) {
	if v == nil || fmt.Sprint(v) == "" || fmt.Sprint(v) == "-" {
		return nil, nil
	}
	x := num(v)
	if x == nil {
		return nil, failure(5, "format", "invalid cyclone numeric value %q", v)
	}
	return x, nil
}
func coordinates(xs []float64) (any, error) {
	if len(xs) != 2 || xs[0] < -90 || xs[0] > 90 || xs[1] < -180 || xs[1] > 180 {
		return nil, failure(5, "format", "invalid cyclone coordinate [latitude,longitude]")
	}
	return map[string]any{"latitude_deg": xs[0], "longitude_deg": xs[1]}, nil
}
func pointType(p tcPoint) (string, error) {
	var part map[string]string
	if e := json.Unmarshal(p.Part, &part); e != nil {
		return "", failure(5, "format", "cyclone part missing label")
	}
	en := strings.ToLower(part["en"])
	if en == "analysis" {
		return "analysis", nil
	}
	if strings.Contains(en, "estimate") {
		return "estimate", nil
	}
	if strings.Contains(en, "forecast") {
		return "forecast", nil
	}
	return "", failure(5, "format", "unknown cyclone part %s", p.Part)
}
func (c *Client) Typhoon(ctx context.Context, id string, detail bool, hours int) (Envelope, error) {
	if !cycloneID.MatchString(id) {
		return Envelope{}, failure(2, "usage", "--id must be a JMA TC identifier such as TC2632; run typhoons list")
	}
	if hours < 0 || hours > 120 {
		return Envelope{}, failure(2, "usage", "--hours must be 0..120")
	}
	xs, e := c.cycloneList(ctx)
	if e != nil {
		return Envelope{}, e
	}
	var target *Cyclone
	for n := range xs {
		if xs[n].ID == id {
			target = &xs[n]
			break
		}
	}
	if target == nil {
		return Envelope{}, failure(2, "resolution", "cyclone %s is not in current JMA index; run typhoons list --refresh", id)
	}
	var track, specs []tcPoint
	if e = c.Get(ctx, "/typhoon/data/"+id+"/forecast.json", time.Minute, &track); e != nil {
		return Envelope{}, e
	}
	if e = c.Get(ctx, "/typhoon/data/"+id+"/specifications.json", time.Minute, &specs); e != nil {
		return Envelope{}, e
	}
	if len(track) < 2 || len(specs) < 2 || !title(track[0]) || !title(specs[0]) {
		return Envelope{}, failure(5, "incomplete", "cyclone title/analysis missing")
	}
	ti, e := parseTime(track[0].Issue.JST)
	if e != nil {
		return Envelope{}, e
	}
	si, e := parseTime(specs[0].Issue.JST)
	if e != nil {
		return Envelope{}, e
	}
	ii, _ := parseTime(target.Issue)
	if !ti.Equal(si) || !ti.Equal(ii) || track[0].Number != specs[0].Number || track[0].Number != target.Number {
		return Envelope{}, failure(5, "incomplete", "cyclone documents have different issued times/identities; retry with --refresh")
	}
	index := map[string]tcPoint{}
	for _, s := range specs[1:] {
		v, e := timeJST(s.Valid.JST)
		if e != nil {
			return Envelope{}, e
		}
		if _, ok := index[v]; ok {
			return Envelope{}, failure(5, "format", "duplicate cyclone specifications valid time")
		}
		index[v] = s
	}
	rows := []map[string]any{}
	seen := map[string]bool{}
	hasAnalysis := false
	for _, t := range track[1:] {
		valid, e := timeJST(t.Valid.JST)
		if e != nil {
			return Envelope{}, e
		}
		if seen[valid] {
			return Envelope{}, failure(5, "format", "duplicate cyclone track valid time")
		}
		seen[valid] = true
		s, ok := index[valid]
		if !ok || t.Hours == nil || s.Hours == nil || *t.Hours != *s.Hours {
			return Envelope{}, failure(5, "incomplete", "cyclone geometry/intensity valid-time join failed")
		}
		kind, e := pointType(t)
		if e != nil {
			return Envelope{}, e
		}
		specKind, e := pointType(s)
		if e != nil || specKind != kind {
			return Envelope{}, failure(5, "incomplete", "cyclone geometry/intensity part mismatch")
		}
		if kind == "analysis" {
			hasAnalysis = true
		}
		pos, e := coordinates(t.Center)
		if e != nil {
			return Envelope{}, e
		}
		if len(s.Position.Deg) != 2 || s.Position.Deg[0] != t.Center[0] || s.Position.Deg[1] != t.Center[1] {
			return Envelope{}, failure(5, "incomplete", "cyclone source positions disagree")
		}
		if *t.Hours > hours {
			continue
		}
		p := map[string]any{"type": kind, "label": json.RawMessage(t.Part), "lead_hours": *t.Hours, "valid_at": valid, "position": pos, "category": s.Category, "location_ja": nullable(s.Location), "course_ja": nullable(s.Course), "position_accuracy_ja": nullable(s.Position.Accuracy), "scale_ja": missingString(s.Scale), "intensity_ja": missingString(s.Intensity)}
		nums := map[string]any{"central_pressure_hpa": s.Pressure, "maximum_sustained_wind_m_s": s.MaximumWind["sustained"]["m/s"], "maximum_sustained_wind_kt": s.MaximumWind["sustained"]["kt"], "maximum_gust_m_s": s.MaximumWind["gust"]["m/s"], "maximum_gust_kt": s.MaximumWind["gust"]["kt"], "movement_km_h": s.Speed["km/h"], "movement_kt": s.Speed["kt"], "probability_circle_radius_km": s.ProbabilityRadius["km"], "probability_circle_radius_nm": s.ProbabilityRadius["nm"]}
		for k, v := range nums {
			n, e := sourceNumber(v)
			if e != nil {
				return Envelope{}, e
			}
			p[k] = n
		}
		p["movement_note"] = s.Speed["note"]
		p["probability_circle_center_probability_pct"] = nil
		p["probability_circle_radius_m"] = nil
		if kind == "forecast" {
			if t.Circle == nil || t.Circle.Radius == nil || *t.Circle.Radius <= 0 {
				return Envelope{}, failure(5, "incomplete", "forecast is missing uncertainty radius")
			}
			p["probability_circle_center_probability_pct"] = 70
			p["probability_circle_radius_m"] = *t.Circle.Radius
		}
		if detail {
			p["analysis_track_untimed_lat_lon_deg"] = t.Track
			p["gale_area_geometry_m"] = t.Gale
			p["storm_area_geometry_m"] = t.Storm
			p["gale_area_source_km_nm"] = s.GaleSpecs
			p["storm_area_source_km_nm"] = s.StormSpecs
		}
		rows = append(rows, p)
	}
	if len(index) != len(seen) || !hasAnalysis {
		return Envelope{}, failure(5, "incomplete", "cyclone analysis/forecast coverage does not match between documents")
	}
	coverage := "coherent current cyclone analysis/forecast documents"
	if c.now().Sub(ti) > 12*time.Hour {
		coverage = "stale"
	}
	notes := []string{"Forecast circles indicate 70% probability of the center being inside at the specified time; not cyclone size. Forecast centers are not a promised path.", "Analysis is JMA's analyzed position, not a future track. Historical track coordinates have no individual timestamps. Estimate is distinct from analysis and forecast.", "Geometry radii are metres; source specification km/nm values are rounded independently. Storm warning area is conditional on the center lying within the forecast circle; hazards can occur outside it."}
	return c.Envelope(map[string]any{"id": id, "number": target.Number, "name_ja": nullable(track[0].Name["jp"]), "name_en": nullable(track[0].Name["en"]), "issued_at": ti.Format(time.RFC3339), "issue_age_hours": ageHours(c.now(), ti.Format(time.RFC3339)), "points": rows, "url": Origin + "/map.html#contents=typhoon&tc_num=" + id + "&lang=ja"}, coverage, notes...), nil
}

func missingString(v any) any {
	if v == nil {
		return nil
	}
	return nullable(fmt.Sprint(v))
}
