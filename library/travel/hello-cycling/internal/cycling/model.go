// Copyright 2026 zjsng. Licensed under Apache-2.0.
// Package cycling interprets the provider's HELLO CYCLING GBFS snapshots.
package cycling

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

const IndexURL = "https://api-public.odpt.org/api/v4/gbfs/hellocycling/gbfs.json"
const LicenseURL = "https://d1yl7kw204zjxn.cloudfront.net/gbfs/v2/public/hellocycling_gbfs_licence.txt"
const PriceURL = "https://www.hellocycling.jp/price/"
const RulesURL = "https://www.hellocycling.jp/getting-started/types-and-stations/"

type FeedMeta struct {
	URL         string `json:"url"`
	LastUpdated int64  `json:"last_updated"`
	TTL         int    `json:"ttl_seconds"`
	Version     string `json:"version"`
}
type Info struct {
	ID          string            `json:"station_id"`
	Name        string            `json:"name"`
	Address     string            `json:"address"`
	Lat         *float64          `json:"lat"`
	Lon         *float64          `json:"lon"`
	Capacity    json.RawMessage   `json:"vehicle_capacity"`
	RentalURIs  map[string]string `json:"rental_uris"`
	ParkingType string            `json:"parking_type"`
}
type TypeCount struct {
	ID    string `json:"vehicle_type_id"`
	Count *int   `json:"count"`
}
type DockCount struct {
	IDs   []string `json:"vehicle_type_ids"`
	Count *int     `json:"count"`
}
type Status struct {
	ID           string      `json:"station_id"`
	Bikes        *int        `json:"num_bikes_available"`
	Docks        *int        `json:"num_docks_available"`
	Installed    *bool       `json:"is_installed"`
	Renting      *bool       `json:"is_renting"`
	Returning    *bool       `json:"is_returning"`
	Reported     int64       `json:"last_reported"`
	Vehicles     []TypeCount `json:"vehicle_types_available"`
	VehicleDocks []DockCount `json:"vehicle_docks_available"`
}
type Vehicle struct {
	ID               string `json:"vehicle_type_id"`
	Name             string `json:"name"`
	Form             string `json:"form_factor"`
	Propulsion       string `json:"propulsion_type"`
	ReturnConstraint string `json:"return_constraint"`
}
type Snapshot struct {
	ObservedAt  time.Time           `json:"observed_at"`
	Feeds       map[string]FeedMeta `json:"feeds"`
	Information []Info              `json:"information"`
	Statuses    []Status            `json:"statuses"`
	Vehicles    []Vehicle           `json:"vehicles"`
	Warnings    []string            `json:"warnings"`
	Requests    int                 `json:"requests"`
	Bytes       int64               `json:"response_bytes"`
}
type Meta struct {
	Source      string              `json:"source"`
	ObservedAt  time.Time           `json:"observed_at"`
	EvaluatedAt time.Time           `json:"evaluated_at"`
	Timezone    string              `json:"timezone"`
	Feeds       map[string]FeedMeta `json:"feeds"`
	Warnings    []string            `json:"warnings"`
	Requests    int                 `json:"request_count"`
	Bytes       int64               `json:"response_bytes"`
	Attribution string              `json:"attribution"`
	License     string              `json:"license"`
	LicenseURL  string              `json:"license_url"`
	Caveat      string              `json:"caveat"`
}
type Station struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Address           string            `json:"address"`
	Lat               *float64          `json:"latitude"`
	Lon               *float64          `json:"longitude"`
	Capacity          *int              `json:"capacity"`
	ParkingType       string            `json:"parking_type"`
	Bikes             *int              `json:"bikes_available"`
	ReturnSpaces      *int              `json:"return_spaces"`
	VehicleType       string            `json:"selected_vehicle_type"`
	SelectedBikes     *int              `json:"selected_type_bikes"`
	SelectedSpaces    *int              `json:"selected_type_return_spaces"`
	RentalState       string            `json:"rental_state"`
	ReturnState       string            `json:"return_state"`
	Installed         *bool             `json:"is_installed"`
	Renting           *bool             `json:"is_renting"`
	Returning         *bool             `json:"is_returning"`
	ReportedAt        *time.Time        `json:"last_reported_at"`
	AgeSeconds        *int64            `json:"age_seconds"`
	Stale             bool              `json:"stale"`
	Vehicles          []TypeCount       `json:"vehicle_types_available"`
	VehicleDocks      []DockCount       `json:"vehicle_docks_available"`
	Distance          *float64          `json:"distance_meters"`
	RentalURIs        map[string]string `json:"rental_uris"`
	SourceURL         string            `json:"source_url"`
	CompatibilityNote string            `json:"compatibility_note"`
}

func (s Snapshot) Meta(source string, now time.Time) Meta {
	requests, bytes := s.Requests, s.Bytes
	if source == "local" {
		requests = 0
		bytes = 0
	}
	return Meta{source, s.ObservedAt, now, "Asia/Tokyo", s.Feeds, s.Warnings, requests, bytes, "HELLO CYCLING / OpenStreet Co., Ltd. via ODPT; normalized and locally ranked", "CC BY 4.0", LicenseURL, "Counts are source observations, not reserved availability. Confirm the actual bike, return eligibility and price in the official app; distances are straight-line, not walking or cycling routes."}
}
func safeCount(n *int) *int {
	if n == nil || *n < 0 {
		return nil
	}
	return n
}
func capacity(raw json.RawMessage) *int {
	var n int
	if json.Unmarshal(raw, &n) != nil {
		var str string
		if json.Unmarshal(raw, &str) != nil {
			return nil
		}
		var e error
		n, e = strconv.Atoi(str)
		if e != nil {
			return nil
		}
	}
	if n < 0 {
		return nil
	}
	return &n
}
func selectedCounts(st Status, typ string) (*int, *int) {
	if typ == "" {
		return safeCount(st.Bikes), safeCount(st.Docks)
	}
	var bikes, docks *int
	foundBikeType := false
	for _, r := range st.Vehicles {
		if r.ID == typ {
			foundBikeType = true
			bikes = safeCount(r.Count)
			break
		}
	}
	// A populated list is an exhaustive per-type inventory. Missing list is unknown.
	if !foundBikeType && len(st.Vehicles) > 0 {
		zero := 0
		bikes = &zero
	}
	for _, r := range st.VehicleDocks {
		for _, id := range r.IDs {
			if id == typ {
				if r.Count == nil || *r.Count < 0 {
					return bikes, nil
				}
				if docks == nil {
					zero := 0
					docks = &zero
				}
				*docks += *r.Count
				break
			}
		}
	}
	if docks == nil && len(st.VehicleDocks) > 0 {
		zero := 0
		docks = &zero
	}
	return bikes, docks
}
func directionState(present bool, installed, enabled *bool, stale bool, n *int, unavailable string) string {
	if !present {
		return "source_missing"
	}
	if installed != nil && !*installed {
		return "uninstalled"
	}
	if enabled != nil && !*enabled {
		return "closed"
	}
	if stale {
		return "stale"
	}
	if installed == nil || enabled == nil || n == nil {
		return "unknown"
	}
	if *n == 0 {
		return unavailable
	}
	return "available"
}

// Stations joins by provider ID, retaining unavailable and missing values as distinct states.
func (s Snapshot) Stations(now time.Time, maxAge time.Duration, typ string) []Station {
	index := make(map[string]Status, len(s.Statuses))
	for _, st := range s.Statuses {
		index[st.ID] = st
	}
	rows := make([]Station, 0, len(s.Information))
	for _, in := range s.Information {
		st, present := index[in.ID]
		r := Station{ID: in.ID, Name: in.Name, Address: in.Address, Lat: in.Lat, Lon: in.Lon, Capacity: capacity(in.Capacity), ParkingType: in.ParkingType, VehicleType: typ, Vehicles: st.Vehicles, VehicleDocks: st.VehicleDocks, RentalURIs: in.RentalURIs, SourceURL: in.RentalURIs["web"], CompatibilityNote: "GBFS generic vehicle class only; individual model, battery, rules and price require app confirmation."}
		if r.SourceURL == "" {
			r.SourceURL = "https://www.hellocycling.jp/map/"
		}
		r.Bikes = safeCount(st.Bikes)
		r.ReturnSpaces = safeCount(st.Docks)
		r.Installed = st.Installed
		r.Renting = st.Renting
		r.Returning = st.Returning
		r.SelectedBikes, r.SelectedSpaces = selectedCounts(st, typ)
		if st.Reported > 0 {
			t := time.Unix(st.Reported, 0).UTC()
			age := int64(now.Sub(t).Seconds())
			r.ReportedAt = &t
			r.AgeSeconds = &age
			r.Stale = age > int64(maxAge.Seconds()) || age < -60
		} else {
			r.Stale = present
		}
		fm, found := s.Feeds["station_status"]
		if present && (!found || fm.LastUpdated <= 0 || now.Sub(time.Unix(fm.LastUpdated, 0)) > maxAge || time.Unix(fm.LastUpdated, 0).After(now.Add(time.Minute))) {
			r.Stale = true
		}
		r.RentalState = directionState(present, st.Installed, st.Renting, r.Stale, r.SelectedBikes, "empty")
		r.ReturnState = directionState(present, st.Installed, st.Returning, r.Stale, r.SelectedSpaces, "full")
		if typ != "" && r.ReturnState == "full" && len(st.VehicleDocks) > 0 {
			supported := false
			for _, d := range st.VehicleDocks {
				for _, id := range d.IDs {
					if id == typ {
						supported = true
					}
				}
			}
			if !supported {
				r.ReturnState = "incompatible"
			}
		}
		rows = append(rows, r)
	}
	return rows
}
func ValidCoordinates(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && !math.IsInf(lat, 0) && !math.IsInf(lon, 0) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
func distance(lat, lon, a, b float64) float64 {
	r := math.Pi / 180
	dLat := (a - lat) * r
	dLon := (b - lon) * r
	h := math.Pow(math.Sin(dLat/2), 2) + math.Cos(lat*r)*math.Cos(a*r)*math.Pow(math.Sin(dLon/2), 2)
	return 6371008.8 * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(math.Max(0, 1-h)))
}
func fold(s string) string { return strings.ToLower(norm.NFKC.String(strings.TrimSpace(s))) }

// Find filters normalized Japanese names, addresses and stable IDs, with deterministic order.
func Find(rows []Station, q string, limit int) ([]Station, int) {
	q = fold(q)
	out := make([]Station, 0)
	for _, r := range rows {
		if strings.Contains(fold(r.Name+" "+r.Address+" "+r.ID), q) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	count := len(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, count
}

// Nearby ranks geometric station distances without transmitting coordinates to a provider.
func Nearby(rows []Station, lat, lon, radius float64, purpose string, availableOnly bool, limit int) ([]Station, int) {
	out := make([]Station, 0)
	for _, r := range rows {
		if r.Lat == nil || r.Lon == nil || !ValidCoordinates(*r.Lat, *r.Lon) {
			continue
		}
		d := distance(lat, lon, *r.Lat, *r.Lon)
		if d > radius {
			continue
		}
		state := r.RentalState
		if purpose == "return" {
			state = r.ReturnState
		}
		if availableOnly && state != "available" {
			continue
		}
		d = math.Round(d)
		r.Distance = &d
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if *out[i].Distance == *out[j].Distance {
			return out[i].ID < out[j].ID
		}
		return *out[i].Distance < *out[j].Distance
	})
	n := len(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, n
}

type Pair struct {
	Pickup         Station `json:"pickup"`
	Dropoff        Station `json:"dropoff"`
	AccessDistance float64 `json:"total_access_distance_meters"`
	Compatibility  string  `json:"compatibility"`
}

// Compare lists only pairs with fresh observed rental and compatible return capacity.
func Compare(rows []Station, fromLat, fromLon, toLat, toLon, radius float64, limit int) []Pair {
	// Include one extra endpoint candidate so excluding the same station cannot
	// turn a valid one-pair request into an empty result.
	pick, _ := Nearby(rows, fromLat, fromLon, radius, "pickup", true, limit+1)
	drop, _ := Nearby(rows, toLat, toLon, radius, "return", true, limit+1)
	out := make([]Pair, 0)
	for _, p := range pick {
		for _, d := range drop {
			if p.ID == d.ID {
				continue
			}
			out = append(out, Pair{p, d, *p.Distance + *d.Distance, "compatible_generic_vehicle_class_observed"})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AccessDistance == out[j].AccessDistance {
			return out[i].Pickup.ID+"/"+out[i].Dropoff.ID < out[j].Pickup.ID+"/"+out[j].Dropoff.ID
		}
		return out[i].AccessDistance < out[j].AccessDistance
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

type Change struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Before *Station `json:"before"`
	After  *Station `json:"after"`
}

// Changes compares two observations; it makes no inference about rides or future inventory.
func Changes(before, after []Station, q string, limit int) ([]Change, int) {
	a := map[string]Station{}
	b := map[string]Station{}
	for _, r := range before {
		a[r.ID] = r
	}
	for _, r := range after {
		b[r.ID] = r
	}
	ids := map[string]bool{}
	for id := range a {
		ids[id] = true
	}
	for id := range b {
		ids[id] = true
	}
	out := make([]Change, 0)
	for id := range ids {
		old, ok1 := a[id]
		new, ok2 := b[id]
		kind := "changed"
		name := new.Name
		if !ok1 {
			kind = "added"
		}
		if !ok2 {
			kind = "source_missing_after"
			name = old.Name
		}
		if q != "" && !strings.Contains(fold(old.Name+" "+old.Address+" "+new.Name+" "+new.Address+" "+id), fold(q)) {
			continue
		}
		if ok1 && ok2 && equalCount(old.Bikes, new.Bikes) && equalCount(old.ReturnSpaces, new.ReturnSpaces) && equalCount(old.SelectedBikes, new.SelectedBikes) && equalCount(old.SelectedSpaces, new.SelectedSpaces) && equalBool(old.Installed, new.Installed) && equalBool(old.Renting, new.Renting) && equalBool(old.Returning, new.Returning) && old.RentalState == new.RentalState && old.ReturnState == new.ReturnState && old.Stale == new.Stale {
			continue
		}
		r := Change{ID: id, Name: name, Kind: kind}
		if ok1 {
			r.Before = &old
		}
		if ok2 {
			r.After = &new
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	n := len(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out, n
}
func equalCount(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
func equalBool(a, b *bool) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
func ValidateLimit(n, max int) error {
	if n < 1 || n > max {
		return fmt.Errorf("--limit must be between 1 and %d", max)
	}
	return nil
}
