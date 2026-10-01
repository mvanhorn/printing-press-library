package jma

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

//go:embed catalog/*.json
var catalog embed.FS

const CatalogDate = "2026-10-01T03:59:36+09:00"

var inventoryPaths = map[string]string{"area": "/common/const/area.json", "short": "/forecast/const/forecast_area.json", "week": "/forecast/const/week_area.json", "week_names": "/forecast/const/week_area_name.json", "station_names": "/forecast/const/en_amedas.json", "stations": "/amedas/const/amedastable.json", "week_eligible": "/forecast/const/week_area05.json", "no_wave_tide": "/warning/const/no_wave_tide.json"}
var catalogFiles = map[string]string{"area": "area.json", "short": "forecast_area.json", "week": "week_area.json", "week_names": "week_area_name.json", "station_names": "en_amedas.json", "stations": "amedastable.json", "week_eligible": "week_area05.json", "no_wave_tide": "no_wave_tide.json"}

type Area struct {
	Name     string   `json:"name"`
	EnName   string   `json:"enName"`
	Parent   string   `json:"parent"`
	Children []string `json:"children"`
}
type ShortMap struct {
	Class10 string   `json:"class10"`
	Amedas  []string `json:"amedas"`
	Class20 string   `json:"class20"`
}
type WeekMap struct {
	Short   string `json:"srf"`
	Week    string `json:"week"`
	Station string `json:"amedas"`
}
type StationInfo struct {
	Name string `json:"kjName"`
}
type Inventory struct {
	NoWaveTide   map[string]NoHazardAreas     `json:"no_wave_tide"`
	WeekEligible map[string][]string          `json:"week_eligible"`
	CapturedAt   string                       `json:"captured_at"`
	Areas        map[string]map[string]Area   `json:"areas"`
	Short        map[string][]ShortMap        `json:"short"`
	Week         map[string][]WeekMap         `json:"week"`
	WeekNames    map[string]map[string]string `json:"week_names"`
	StationNames map[string]string            `json:"station_names"`
	Stations     map[string]StationInfo       `json:"stations"`
}
type Place struct {
	ID                  string   `json:"id"`
	NameJA              string   `json:"name_ja"`
	NameEN              any      `json:"name_en"`
	Kind                string   `json:"kind"`
	ParentID            any      `json:"parent_id"`
	OfficeID            string   `json:"office_id"`
	ForecastDistrictIDs []string `json:"forecast_district_ids"`
	URL                 string   `json:"url"`
}

var kindNames = map[string]string{"offices": "office", "class10s": "district", "class15s": "subdivision", "class20s": "municipality"}

func (i *Inventory) Validate() error {
	if len(i.Areas["offices"]) < 40 || len(i.Areas["class10s"]) < 100 || len(i.Areas["class20s"]) < 1000 || len(i.Short) < 40 || len(i.Week) < 40 || len(i.StationNames) < 100 || len(i.Stations) < 1000 || len(i.WeekNames) < 40 || len(i.WeekEligible) < 100 || len(i.NoWaveTide["wave"].Municipalities) < 40 || len(i.NoWaveTide["tide"].Municipalities) < 40 {
		return failure(5, "incomplete", "inventory is empty or missing a required source catalog")
	}
	if _, err := parseTime(i.CapturedAt); err != nil {
		return err
	}
	for _, kind := range []string{"class10s", "class15s", "class20s"} {
		for code, a := range i.Areas[kind] {
			if a.Name == "" || a.Parent == "" {
				return failure(5, "format", "inventory %s %s missing name/parent", kind, code)
			}
			if i.office(kind, code) == "" {
				return failure(5, "format", "inventory %s %s has unresolved parent", kind, code)
			}
		}
	}
	for office, rows := range i.Short {
		for _, r := range rows {
			if i.Areas["class10s"][r.Class10].Name == "" || len(r.Amedas) == 0 {
				return failure(5, "format", "short mapping %s missing district/station", office)
			}
		}
	}
	return nil
}
func embeddedInventory() (*Inventory, error) {
	i := &Inventory{CapturedAt: CatalogDate}
	dest := map[string]any{"area": &i.Areas, "short": &i.Short, "week": &i.Week, "week_names": &i.WeekNames, "station_names": &i.StationNames, "stations": &i.Stations, "week_eligible": &i.WeekEligible, "no_wave_tide": &i.NoWaveTide}
	for n, d := range dest {
		b, e := catalog.ReadFile("catalog/" + catalogFiles[n])
		if e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, d); e != nil {
			return nil, e
		}
	}
	return i, i.Validate()
}
func (c *Client) Inventory(ctx context.Context, refresh bool) (*Inventory, error) {
	path := filepath.Join(c.CacheDir, "inventory.json")
	if refresh {
		if c.Offline {
			return nil, failure(2, "usage", "inventory refresh requires live access; remove --offline")
		}
		i := &Inventory{CapturedAt: c.now().In(JST).Format(time.RFC3339)}
		dest := map[string]any{"area": &i.Areas, "short": &i.Short, "week": &i.Week, "week_names": &i.WeekNames, "station_names": &i.StationNames, "stations": &i.Stations, "week_eligible": &i.WeekEligible, "no_wave_tide": &i.NoWaveTide}
		old := c.Refresh
		c.Refresh = true
		defer func() { c.Refresh = old }()
		for _, n := range []string{"area", "short", "week", "week_names", "station_names", "stations", "week_eligible", "no_wave_tide"} {
			if e := c.Get(ctx, inventoryPaths[n], 0, dest[n]); e != nil {
				return nil, e
			}
		}
		if e := i.Validate(); e != nil {
			return nil, e
		}
		b, _ := json.Marshal(i)
		if e := atomicWrite(path, b); e != nil {
			return nil, failure(4, "cache", "save inventory: %v", e)
		}
		return i, nil
	}
	b, e := readBounded(path, 4<<20)
	if e == nil {
		var i Inventory
		if e = json.Unmarshal(b, &i); e != nil {
			return nil, failure(5, "format", "invalid inventory; run inventory refresh")
		}
		if e = i.Validate(); e != nil {
			return nil, failure(5, "incomplete", "invalid stored inventory: %v; run inventory refresh", e)
		}
		c.sources = append(c.sources, Source{URL: Origin + inventoryPaths["area"], FetchedAt: i.CapturedAt, Cache: true, AgeSeconds: inventoryAge(c.now(), i.CapturedAt), Bytes: len(b)})
		return &i, nil
	}
	if !os.IsNotExist(e) {
		return nil, failure(4, "cache", "read inventory: %v", e)
	}
	i, e := embeddedInventory()
	if e == nil {
		c.sources = append(c.sources, Source{URL: Origin + inventoryPaths["area"], FetchedAt: CatalogDate, Cache: true, AgeSeconds: inventoryAge(c.now(), CatalogDate)})
	}
	return i, e
}
func (i *Inventory) office(kind, code string) string {
	for n := 0; n < 5; n++ {
		a, ok := i.Areas[kind][code]
		if !ok {
			return ""
		}
		if kind == "offices" {
			return code
		}
		code = a.Parent
		switch kind {
		case "class20s":
			kind = "class15s"
		case "class15s":
			kind = "class10s"
		case "class10s":
			kind = "offices"
		default:
			return ""
		}
	}
	return ""
}
func (i *Inventory) descendant(kind, code, parent string) bool {
	for n := 0; n < 5; n++ {
		if code == parent {
			return true
		}
		a, ok := i.Areas[kind][code]
		if !ok {
			return false
		}
		code = a.Parent
		switch kind {
		case "class20s":
			kind = "class15s"
		case "class15s":
			kind = "class10s"
		case "class10s":
			kind = "offices"
		default:
			return false
		}
	}
	return false
}
func nullable(s string) any {
	if s == "" || s == "-" {
		return nil
	}
	return s
}
func (i *Inventory) place(kind, id string) Place {
	a := i.Areas[kind][id]
	districts := []string{}
	if kind == "offices" {
		for _, d := range a.Children {
			districts = append(districts, d)
		}
	} else if kind == "class10s" {
		districts = append(districts, id)
	} else {
		code := id
		k := kind
		for n := 0; n < 3 && k != "class10s"; n++ {
			code = i.Areas[k][code].Parent
			if k == "class20s" {
				k = "class15s"
			} else {
				k = "class10s"
			}
		}
		districts = append(districts, code)
	}
	sort.Strings(districts)
	return Place{id, a.Name, nullable(a.EnName), kindNames[kind], nullable(a.Parent), i.office(kind, id), districts, canonical("forecast", kind, id)}
}
func (i *Inventory) Search(q, kind string) []Place {
	rows := []Place{}
	q = strings.ToLower(strings.TrimSpace(q))
	for _, k := range []string{"offices", "class10s", "class15s", "class20s"} {
		if kind != "all" && kind != kindNames[k] {
			continue
		}
		for id, a := range i.Areas[k] {
			if q == "" || strings.Contains(strings.ToLower(a.Name+" "+a.EnName+" "+id), q) {
				rows = append(rows, i.place(k, id))
			}
		}
	}
	sort.Slice(rows, func(a, b int) bool {
		if rows[a].ID == rows[b].ID {
			return rows[a].Kind < rows[b].Kind
		}
		return rows[a].ID < rows[b].ID
	})
	return rows
}
func (i *Inventory) Resolve(q string) (Place, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return Place{}, failure(2, "usage", "--area requires a source ID or exact Japanese/English name; run areas search")
	}
	// An office's ID is canonical even when a district shares the same ID.
	for _, k := range []string{"offices", "class20s", "class15s", "class10s"} {
		if _, ok := i.Areas[k][q]; ok {
			return i.place(k, q), nil
		}
	}
	matches := []Place{}
	for _, p := range i.Search(q, "all") {
		if strings.EqualFold(p.NameJA, q) || p.NameEN != nil && strings.EqualFold(fmt.Sprint(p.NameEN), q) {
			matches = append(matches, p)
		}
	}
	if len(matches) != 1 {
		ids := []string{}
		for _, m := range matches {
			ids = append(ids, m.ID+" ("+m.Kind+")")
		}
		if len(ids) > 8 {
			ids = ids[:8]
		}
		return Place{}, failure(2, "resolution", "area %q has %d exact matches %v; run areas search --query and use an ID", q, len(matches), ids)
	}
	return matches[0], nil
}

type Station struct {
	ID          string   `json:"id"`
	NameJA      any      `json:"name_ja"`
	NameEN      any      `json:"name_en"`
	Role        string   `json:"role"`
	DistrictIDs []string `json:"district_ids"`
	OfficeIDs   []string `json:"office_ids"`
	URL         string   `json:"url"`
}

func unique(xs []string) []string {
	sort.Strings(xs)
	out := []string{}
	for _, s := range xs {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func (i *Inventory) SearchStations(q string) []Station {
	rows := map[string]*Station{}
	for office, ds := range i.Short {
		for _, d := range ds {
			for _, id := range d.Amedas {
				s := rows[id]
				if s == nil {
					s = &Station{ID: id, NameJA: nullable(i.Stations[id].Name), NameEN: nullable(i.StationNames[id]), Role: "forecast temperature reference station; not an observation", URL: canonical("forecast", "class10s", d.Class10)}
					rows[id] = s
				}
				s.DistrictIDs = append(s.DistrictIDs, d.Class10)
				s.OfficeIDs = append(s.OfficeIDs, office)
			}
		}
	}
	out := []Station{}
	for _, s := range rows {
		s.DistrictIDs = unique(s.DistrictIDs)
		s.OfficeIDs = unique(s.OfficeIDs)
		if q == "" || strings.Contains(strings.ToLower(s.ID+" "+fmt.Sprint(s.NameJA)+" "+fmt.Sprint(s.NameEN)), strings.ToLower(q)) {
			out = append(out, *s)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].ID < out[b].ID })
	return out
}
func pageRows[T any](rows []T, offset, limit int) ([]T, map[string]any, error) {
	if limit < 1 || limit > 100 || offset < 0 || offset > 100000 {
		return nil, nil, failure(2, "usage", "--limit must be 1..100 and --offset 0..100000")
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	start := offset
	if start > len(rows) {
		start = len(rows)
	}
	var next any
	if end < len(rows) {
		next = end
	}
	return rows[start:end], map[string]any{"total": len(rows), "offset": offset, "limit": limit, "next_offset": next}, nil
}
func (c *Client) AreaSearch(ctx context.Context, q, kind string, offset, limit int) (Envelope, error) {
	i, e := c.Inventory(ctx, false)
	if e != nil {
		return Envelope{}, e
	}
	if kind != "all" {
		ok := false
		for _, v := range kindNames {
			ok = ok || v == kind
		}
		if !ok {
			return Envelope{}, failure(2, "usage", "--kind must be all, office, district, subdivision or municipality")
		}
	}
	rows, p, e := pageRows(i.Search(q, kind), offset, limit)
	v := c.Envelope(rows, "source area inventory")
	v.Page = p
	return v, e
}
func (c *Client) StationSearch(ctx context.Context, q string, offset, limit int) (Envelope, error) {
	i, e := c.Inventory(ctx, false)
	if e != nil {
		return Envelope{}, e
	}
	rows, p, e := pageRows(i.SearchStations(q), offset, limit)
	v := c.Envelope(rows, "forecast reference stations only")
	v.Page = p
	return v, e
}
func (c *Client) ResolveArea(ctx context.Context, q string) (Envelope, error) {
	i, e := c.Inventory(ctx, false)
	if e != nil {
		return Envelope{}, e
	}
	p, e := i.Resolve(q)
	return c.Envelope(p, "source resolution"), e
}
func (c *Client) RefreshInventory(ctx context.Context) (Envelope, error) {
	i, e := c.Inventory(ctx, true)
	if e != nil {
		return Envelope{}, e
	}
	counts := map[string]int{}
	for k, v := range i.Areas {
		counts[k] = len(v)
	}
	return c.Envelope(map[string]any{"captured_at": i.CapturedAt, "area_counts": counts, "forecast_stations": len(i.SearchStations("")), "path": filepath.Join(c.CacheDir, "inventory.json")}, "inventory refreshed from JMA"), nil
}

var _ = time.Second

func inventoryAge(now time.Time, stamp string) int64 {
	t, e := parseTime(stamp)
	if e != nil {
		return 0
	}
	return int64(now.Sub(t).Seconds())
}

type NoHazardAreas struct {
	Districts      []string            `json:"class10s"`
	Subdivisions   []string            `json:"class15s"`
	Municipalities map[string][]string `json:"class20s"`
}

func (i *Inventory) noWaveTide(office, id string) bool {
	no := func(hazard string) bool {
		for _, code := range i.NoWaveTide[hazard].Municipalities[office] {
			if code == id {
				return true
			}
		}
		return false
	}
	return no("wave") && no("tide")
}
