// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-written: resolves --items tokens against the live restaurant menu so a
// freshly composed cart carries the numeric item id and unit price the
// postmicmeshorder endpoint requires.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/store"
)

// menuEntry is the slice of a /m/api/restaurants/{slug}/menus/full result we
// need to build an order line. The wire body's numeric item_id maps to the
// menu result's `id` field (e.g. 19001), NOT its `item_id` field, which is a
// human SKU string like "Nigiri Two Pieces04".
type menuEntry struct {
	id    string
	price float64
	name  string
}

type menuIndex struct {
	byID   map[string]menuEntry
	byName map[string]menuEntry // lowercased display name -> entry
}

// loadMenuIndex fetches and indexes a restaurant's full menu by numeric id and
// by display name.
func loadMenuIndex(c *client.Client, slug string) (*menuIndex, error) {
	path := replacePathParam("/m/api/restaurants/{slug}/menus/full", "slug", slug)
	data, err := c.Get(path, nil)
	if err != nil {
		return nil, err
	}
	results, err := menuResults(data)
	if err != nil {
		return nil, err
	}
	idx := &menuIndex{byID: map[string]menuEntry{}, byName: map[string]menuEntry{}}
	for _, r := range results {
		id := r.ID.String()
		if id == "" || id == "0" {
			continue
		}
		e := menuEntry{id: id, price: r.Price, name: r.Name}
		idx.byID[id] = e
		if name := strings.ToLower(strings.TrimSpace(r.Name)); name != "" {
			// First write wins so the lowest-id canonical item is preferred over
			// duplicate-named option rows.
			if _, exists := idx.byName[name]; !exists {
				idx.byName[name] = e
			}
		}
	}
	if len(idx.byID) == 0 {
		return nil, fmt.Errorf("menu for %q returned no items", slug)
	}
	return idx, nil
}

type menuResult struct {
	ID    json.Number `json:"id"`
	Name  string      `json:"name"`
	Price float64     `json:"price"`
}

// menuResults extracts the item array from the menu payload, tolerating both
// the observed `{"meta":...,"results":[...]}` envelope and a bare array.
func menuResults(data json.RawMessage) ([]menuResult, error) {
	var wrapped struct {
		Results []menuResult `json:"results"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && len(wrapped.Results) > 0 {
		return wrapped.Results, nil
	}
	var bare []menuResult
	if err := json.Unmarshal(data, &bare); err == nil && len(bare) > 0 {
		return bare, nil
	}
	return nil, fmt.Errorf("menu payload had no recognizable items array")
}

// lookup resolves a --items token to a menu entry. A token is matched first as
// a numeric menu id, then as a display name (case-insensitive).
func (m *menuIndex) lookup(token string) (menuEntry, bool) {
	token = strings.TrimSpace(token)
	if e, ok := m.byID[token]; ok {
		return e, true
	}
	if e, ok := m.byName[strings.ToLower(token)]; ok {
		return e, true
	}
	return menuEntry{}, false
}

// resolveCartItems rewrites each parsed --items entry in place, setting its
// numeric id, unit price, and display name from the menu. The raw user token
// is read from ItemID (where parseItemsSpec stashed it). Unknown tokens are
// collected and reported together so the caller sees every miss at once.
func resolveCartItems(idx *menuIndex, items []store.OrderItem) error {
	var unresolved []string
	for i := range items {
		token := items[i].ItemID
		if token == "" {
			token = items[i].ID
		}
		entry, ok := idx.lookup(token)
		if !ok {
			unresolved = append(unresolved, token)
			continue
		}
		items[i].ID = entry.id
		items[i].ItemID = entry.id
		items[i].Price = entry.price
		if items[i].Name == "" {
			items[i].Name = entry.name
		}
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return usageErr(fmt.Errorf("unknown menu item(s): %s. Use a numeric menu id (from `ordertogo restaurants menu --slug <slug>`) or an exact item name", strings.Join(unresolved, ", ")))
	}
	return nil
}
