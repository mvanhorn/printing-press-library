// Package haven implements local, location-scoped menu observations and queries.
package haven

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"
)

type Location struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	City      string   `json:"city"`
	State     string   `json:"state"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}
type Item struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	PriceCents int64  `json:"price_cents"`
	Available  *bool  `json:"available"`
	Disabled   bool   `json:"disabled"`
	Hidden     bool   `json:"hidden"`
}
type Snapshot struct {
	ID         int64     `json:"id"`
	LocationID int64     `json:"location_id"`
	FetchedAt  time.Time `json:"fetched_at"`
	Complete   bool      `json:"complete"`
	Items      []Item    `json:"items"`
}

func normalized(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func validCoord(lat, lon float64) bool {
	return !math.IsNaN(lat) && !math.IsNaN(lon) && !math.IsInf(lat, 0) && !math.IsInf(lon, 0) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
func ParseLocations(raw []byte) ([]Location, error) {
	var env struct {
		Locations json.RawMessage `json:"locations"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode locations: %w", err)
	}
	if len(env.Locations) == 0 || bytes.Equal(bytes.TrimSpace(env.Locations), []byte("null")) {
		return nil, fmt.Errorf("locations response requires a locations array")
	}
	result := []Location{}
	if err := json.Unmarshal(env.Locations, &result); err != nil {
		return nil, fmt.Errorf("decode locations array: %w", err)
	}
	seen := map[int64]bool{}
	for _, l := range result {
		if l.ID <= 0 || strings.TrimSpace(l.Name) == "" || seen[l.ID] {
			return nil, fmt.Errorf("invalid or duplicate location id %d", l.ID)
		}
		seen[l.ID] = true
		if l.Latitude != nil && l.Longitude != nil && !validCoord(*l.Latitude, *l.Longitude) {
			return nil, fmt.Errorf("invalid coordinates for location %d", l.ID)
		}
	}
	return result, nil
}
func priceCents(raw json.RawMessage) (int64, error) {
	// Rat avoids rounding binary floating point and rejects fractional cents.
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == '"' || bytes.Equal(raw, []byte("null")) {
		return 0, fmt.Errorf("price must be a nonnegative numeric amount")
	}
	r, ok := new(big.Rat).SetString(string(raw))
	if !ok || r.Sign() < 0 {
		return 0, fmt.Errorf("invalid price %s", raw)
	}
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() || !r.Num().IsInt64() {
		return 0, fmt.Errorf("price must fit integer cents without rounding")
	}
	return r.Num().Int64(), nil
}
func ParseMenu(raw []byte, locationID int64, at time.Time) (Snapshot, error) {
	out := Snapshot{LocationID: locationID, FetchedAt: at.UTC(), Items: []Item{}}
	if locationID <= 0 || at.IsZero() {
		return out, fmt.Errorf("positive location id and observation time required")
	}
	var env struct {
		Categories json.RawMessage `json:"categories"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return out, fmt.Errorf("decode menu: %w", err)
	}
	if len(env.Categories) == 0 || bytes.Equal(bytes.TrimSpace(env.Categories), []byte("null")) {
		return out, fmt.Errorf("menu requires a complete categories array")
	}
	var cats []struct {
		Hidden bool            `json:"hidden"`
		Items  json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(env.Categories, &cats); err != nil {
		return out, err
	}
	seen := map[int64]int{}
	for _, cat := range cats {
		if len(cat.Items) == 0 || bytes.Equal(bytes.TrimSpace(cat.Items), []byte("null")) {
			return out, fmt.Errorf("category missing items array; refusing incomplete menu")
		}
		var items []struct {
			ID        int64           `json:"id"`
			Name      string          `json:"name"`
			Price     json.RawMessage `json:"price"`
			Available *bool           `json:"available_now"`
			Disabled  bool            `json:"is_disabled"`
		}
		if err := json.Unmarshal(cat.Items, &items); err != nil {
			return out, err
		}
		for _, v := range items {
			if v.ID <= 0 || normalized(v.Name) == "" {
				return out, fmt.Errorf("invalid menu item id or name")
			}
			cents, err := priceCents(v.Price)
			if err != nil {
				return out, fmt.Errorf("item %d: %w", v.ID, err)
			}
			item := Item{ID: v.ID, Name: v.Name, PriceCents: cents, Available: v.Available, Disabled: v.Disabled, Hidden: cat.Hidden}
			if pos, ok := seen[item.ID]; ok {
				prev := &out.Items[pos]
				if prev.Name != item.Name || prev.PriceCents != item.PriceCents || prev.Disabled != item.Disabled || !sameBool(prev.Available, item.Available) {
					return out, fmt.Errorf("conflicting duplicate item %d", item.ID)
				}
				// Featured and ordinary categories can contain the same item. A visible occurrence wins.
				prev.Hidden = prev.Hidden && item.Hidden
			} else {
				seen[item.ID] = len(out.Items)
				out.Items = append(out.Items, item)
			}
		}
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].ID < out.Items[j].ID })
	out.Complete = true
	return out, nil
}
func sameBool(a, b *bool) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func usable(i Item) bool       { return !i.Hidden && !i.Disabled && i.Available != nil && *i.Available }
func validateSnapshot(s Snapshot) error {
	if s.LocationID <= 0 || s.FetchedAt.IsZero() || s.Items == nil {
		return fmt.Errorf("invalid snapshot location, timestamp or items")
	}
	seen := map[int64]bool{}
	for _, i := range s.Items {
		if i.ID <= 0 || normalized(i.Name) == "" || i.PriceCents < 0 || seen[i.ID] {
			return fmt.Errorf("invalid or duplicate item %d in snapshot", i.ID)
		}
		seen[i.ID] = true
	}
	return nil
}
