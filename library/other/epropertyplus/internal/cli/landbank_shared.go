// Copyright 2026 startos00. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the hand-built novel commands (list, export, land-export,
// enrich, compare). Handles fetching the instance index, hydrating full
// property detail (live or from the synced store), and politeness rate limits.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/landbank"
	"github.com/mvanhorn/printing-press-library/library/other/epropertyplus/internal/store"

	"github.com/spf13/cobra"
)

// politeDelay is the inter-request sleep used when hydrating many properties
// live so the public API isn't hammered.
const politeDelay = 120 * time.Millisecond

// fetchIndex pulls the lightweight index for the active instance.
func fetchIndex(c *client.Client) (*landbank.IndexResponse, error) {
	data, err := c.Get("/property/searchSummaryPublicMapQuery", nil)
	if err != nil {
		return nil, err
	}
	return landbank.ParseIndex(data)
}

// hydrateProperty fetches and parses a single property's full detail live.
func hydrateProperty(c *client.Client, id string) (landbank.Property, json.RawMessage, error) {
	data, err := c.Get("/property/getPublishedProperty", map[string]string{"propertyId": id})
	if err != nil {
		return landbank.Property{}, nil, err
	}
	inner, err := landbank.ParseDetail(data)
	if err != nil {
		return landbank.Property{}, nil, err
	}
	p, err := landbank.ParseProperty(inner)
	return p, inner, err
}

// loadedProperty pairs a parsed Property with its full raw detail JSON.
type loadedProperty struct {
	Prop landbank.Property
	Raw  json.RawMessage
}

// loadProperties returns hydrated properties for the active instance,
// preferring the synced local store when it has data, otherwise hydrating live
// up to limit (limit <= 0 means a sane default cap to avoid hammering the full
// catalogue). It returns the source ("local" or "live") for provenance.
//
// kindFilter ("structure"|"lot"|"all"|"") is applied after hydration so the
// returned slice already matches the requested classification.
func loadProperties(ctx context.Context, c *client.Client, flags *rootFlags, limit int, kindFilter string) ([]loadedProperty, string, error) {
	slug := flags.activeSlug()

	// Try the local store first unless the user forced --data-source=live.
	if flags.dataSource != "live" {
		if loaded, ok, err := loadFromStore(ctx, slug, limit, kindFilter); err != nil {
			return nil, "", err
		} else if ok {
			return loaded, "local", nil
		}
		if flags.dataSource == "local" {
			return nil, "", fmt.Errorf("no synced data for instance %q. Run 'epropertyplus-pp-cli --instance %s sync' first", slug, slug)
		}
	}

	// Live hydration.
	idx, err := fetchIndex(c)
	if err != nil {
		return nil, "", err
	}
	cap := limit
	if cap <= 0 {
		cap = 200
	}
	var out []loadedProperty
	hydrated := 0
	for _, row := range idx.Rows {
		if hydrated >= cap {
			break
		}
		id := row.ID.String()
		p, raw, err := hydrateProperty(c, id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping property %s: %v\n", id, err)
			continue
		}
		hydrated++
		if !p.MatchesKind(kindFilter) {
			continue
		}
		out = append(out, loadedProperty{Prop: p, Raw: raw})
		time.Sleep(politeDelay)
	}
	return out, "live", nil
}

// loadFromStore reads hydrated properties from the synced store for the given
// instance slug. Returns (rows, true, nil) when the store has at least one
// hydrated property; (nil, false, nil) when the store is empty/absent.
func loadFromStore(ctx context.Context, slug string, limit int, kindFilter string) ([]loadedProperty, bool, error) {
	dbPath := defaultDBPath("epropertyplus-pp-cli")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, false, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, false, fmt.Errorf("opening local store: %w", err)
	}
	defer db.Close()

	n, err := db.CountFullProperties(slug)
	if err != nil {
		return nil, false, err
	}
	if n == 0 {
		return nil, false, nil
	}

	// Read all (limit applied after kind filtering so a kind subset isn't
	// starved by a pre-filter cap).
	rows, err := db.ListFullProperties(slug, 0)
	if err != nil {
		return nil, false, err
	}
	var out []loadedProperty
	for _, r := range rows {
		p, err := landbank.ParseProperty(r.Data)
		if err != nil {
			continue
		}
		if !p.MatchesKind(kindFilter) {
			continue
		}
		out = append(out, loadedProperty{Prop: p, Raw: r.Data})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, true, nil
}

// emitProperties renders a slice of loaded properties through the standard
// output pipeline (so --json/--csv/--select/--compact all work). The projected
// objects use GeoProps() field names for stable, agent-friendly keys.
func emitProperties(cmd *cobra.Command, flags *rootFlags, loaded []loadedProperty) error {
	out := make([]map[string]any, 0, len(loaded))
	for _, lp := range loaded {
		out = append(out, lp.Prop.GeoProps())
	}
	return printJSONFiltered(cmd.OutOrStdout(), out, flags)
}
