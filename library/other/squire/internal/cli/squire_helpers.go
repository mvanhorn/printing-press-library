// Copyright 2026 Dev Basu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored transcendence helpers for squire-pp-cli (Phase 3). Safe to edit.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/squire/internal/client"
)

// --- defensive map accessors (JSON numbers decode to float64 in map[string]any) ---

func sqString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func sqInt(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
	case int:
		return v
	}
	return 0
}

func sqFloat(m map[string]any, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
	case int:
		return float64(v)
	}
	return 0
}

func sqMap(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

// --- HTTP fetch helpers (all GET, no auth) ---

func sqGetObject(ctx context.Context, c *client.Client, path string, params map[string]string) (map[string]any, error) {
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("decode object: %w", err)
	}
	return obj, nil
}

func sqGetArray(ctx context.Context, c *client.Client, path string, params map[string]string) ([]map[string]any, error) {
	raw, err := c.Get(ctx, path, params)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode array: %w", err)
	}
	return out, nil
}

// resolveShop fetches /v1/shop/{idOrRoute}/details. The endpoint accepts a slug
// OR a uuid; the response carries the canonical uuid `id`, which the v2 service
// and reviews endpoints require.
func resolveShop(ctx context.Context, c *client.Client, idOrRoute string) (uuid, name, route string, barberCount, bookingFee int, raw map[string]any, err error) {
	path := replacePathParam("/v1/shop/{shop_id}/details", "shop_id", idOrRoute)
	raw, err = sqGetObject(ctx, c, path, nil)
	if err != nil {
		return "", "", "", 0, 0, nil, err
	}
	return sqString(raw, "id"), sqString(raw, "name"), sqString(raw, "route"),
		sqInt(raw, "barberCount"), sqInt(raw, "customerBookingFee"), raw, nil
}

func fetchServices(ctx context.Context, c *client.Client, shopUUID string) ([]map[string]any, error) {
	path := replacePathParam("/v2/shop/{shop_id}/service", "shop_id", shopUUID)
	return sqGetArray(ctx, c, path, nil)
}

func fetchProfessionals(ctx context.Context, c *client.Client, idOrRoute string) ([]map[string]any, error) {
	path := replacePathParam("/v1/shop/{shop_id}/details/professional", "shop_id", idOrRoute)
	return sqGetArray(ctx, c, path, nil)
}

func fetchReviewMeta(ctx context.Context, c *client.Client, shopUUID string) (avg float64, num int, summary string, err error) {
	path := replacePathParam("/v1/reviews/shop/{shop_id}", "shop_id", shopUUID)
	obj, err := sqGetObject(ctx, c, path, map[string]string{"limit": "1"})
	if err != nil {
		return 0, 0, "", err
	}
	return sqFloat(obj, "averageRating"), sqInt(obj, "numberOfRatings"), sqString(obj, "summary"), nil
}

// fetchCityShops returns the `entity` map for each shop in a city discovery page
// (https://getsquire.com/discover/api/shops?cityId=&lat=&lon=&page=).
func fetchCityShops(ctx context.Context, c *client.Client, cityID, lat, lon string, page int) ([]map[string]any, error) {
	params := map[string]string{"cityId": cityID, "lat": lat, "lon": lon, "page": fmt.Sprintf("%d", page)}
	obj, err := sqGetObject(ctx, c, "https://getsquire.com/discover/api/shops", params)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0)
	list, _ := obj["list"].([]any)
	for _, it := range list {
		row, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if ent := sqMap(row, "entity"); ent != nil {
			out = append(out, ent)
		} else {
			out = append(out, row)
		}
	}
	return out, nil
}

// --- pure logic (unit-tested in squire_helpers_test.go) ---

// serviceMatches reports whether a service name or any of its category names
// contains term (case-insensitive, whitespace-trimmed).
func serviceMatches(name string, categories []any, term string) bool {
	t := strings.ToLower(strings.TrimSpace(term))
	if t == "" {
		return true
	}
	if strings.Contains(strings.ToLower(strings.TrimSpace(name)), t) {
		return true
	}
	for _, c := range categories {
		if cm, ok := c.(map[string]any); ok {
			if strings.Contains(strings.ToLower(strings.TrimSpace(sqString(cm, "name"))), t) {
				return true
			}
		}
	}
	return false
}

// rosterScore weights a rating by review-volume confidence: rating * ln(count+1).
func rosterScore(rating float64, count int) float64 {
	if count < 0 {
		count = 0
	}
	return rating * math.Log(float64(count)+1)
}

// PriceChange records a per-service price move (integer cents).
type PriceChange struct {
	Service  string `json:"service"`
	OldCents int    `json:"old_cents"`
	NewCents int    `json:"new_cents"`
}

// diffServicePrices compares two service-name -> cents maps.
func diffServicePrices(old, current map[string]int) (changes []PriceChange, added, removed []string) {
	changes = make([]PriceChange, 0)
	added = make([]string, 0)
	removed = make([]string, 0)
	for name, newCents := range current {
		if oldCents, ok := old[name]; ok {
			if oldCents != newCents {
				changes = append(changes, PriceChange{Service: name, OldCents: oldCents, NewCents: newCents})
			}
		} else {
			added = append(added, name)
		}
	}
	for name := range old {
		if _, ok := current[name]; !ok {
			removed = append(removed, name)
		}
	}
	return changes, added, removed
}
