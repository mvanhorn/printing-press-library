// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-written: resolves a restaurant slug to its numeric restaurant id, which
// the order endpoints require but no per-restaurant endpoint exposes. The slug
// is rejected by `restaurants show` and absent from the menu payload; the
// mapping only appears in the metro-wide `restaurants list`.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/store"
)

// resolveRestID resolves a restaurant slug to its numeric restid. It tries the
// offline fast path first (a prior order at this restaurant), then falls back
// to a live restaurants-list lookup for the configured location code.
func resolveRestID(ctx context.Context, c *client.Client, locationCode, slug string) (string, error) {
	if slug == "" {
		return "", usageErr(fmt.Errorf("no restaurant slug to resolve (pass --restaurant or set default_restaurant)"))
	}

	if id := restIDFromStore(ctx, slug); id != "" {
		return id, nil
	}

	if locationCode == "" {
		return "", usageErr(fmt.Errorf("cannot resolve restid for %q: set a location code with `ordertogo config set default_location_code <metro>` (e.g. sto for the Seattle area), then retry", slug))
	}

	id, err := restIDFromList(c, locationCode, slug)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", notFoundErr(fmt.Errorf("restaurant %q not found in location %q; check the slug or try a different --location-code", slug, locationCode))
	}
	return id, nil
}

func restIDFromStore(ctx context.Context, slug string) string {
	db, err := store.OpenWithContext(ctx, defaultDBPath("ordertogo-pp-cli"))
	if err != nil {
		return ""
	}
	defer db.Close()
	id, err := db.RestIDForSlug(slug)
	if err != nil {
		return ""
	}
	return id
}

// restIDFromList fetches the metro restaurant list and returns the numeric id
// whose name matches the slug.
func restIDFromList(c *client.Client, locationCode, slug string) (string, error) {
	path := replacePathParam("/m/api/restaurants/filter/{location_code}", "location_code", locationCode)
	data, err := c.Get(path, nil)
	if err != nil {
		return "", err
	}
	return matchRestID(data, slug)
}

type restListEntry struct {
	Name string      `json:"name"`
	ID   json.Number `json:"id"`
}

// matchRestID finds the numeric id whose name matches the slug in a restaurants
// list payload, tolerating both a bare array and a {"results":[...]} envelope.
// Returns ("", nil) when the slug is not present.
func matchRestID(data []byte, slug string) (string, error) {
	var bare []restListEntry
	if err := json.Unmarshal(data, &bare); err == nil && len(bare) > 0 {
		return findRestID(bare, slug), nil
	}
	var wrapped struct {
		Results []restListEntry `json:"results"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return "", fmt.Errorf("parsing restaurants list: %w", err)
	}
	return findRestID(wrapped.Results, slug), nil
}

func findRestID(entries []restListEntry, slug string) string {
	for _, r := range entries {
		if r.Name == slug {
			return r.ID.String()
		}
	}
	return ""
}
