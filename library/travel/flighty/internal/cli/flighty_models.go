// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared models and fetch helpers for the Flighty novel
// commands. Not generator-emitted; keep in its own file for regen-merge.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/flighty/internal/store"
)

// flightyBoardFlight is one row of an arrivals/departures board.
type flightyBoardFlight struct {
	ID              string            `json:"id"`
	City            string            `json:"city"`
	Status          []flightyStatusEl `json:"status,omitempty"`
	OriginalTime    *flightyTimeEl    `json:"originalTime,omitempty"`
	NewTime         *flightyTimeEl    `json:"newTime,omitempty"`
	SecondaryCorner string            `json:"secondaryCorner,omitempty"`
	Airline         flightyAirline    `json:"airline,omitempty"`
	FlightNumber    string            `json:"flightNumber,omitempty"`
	Departure       flightyBoardPoint `json:"departure,omitempty"`
	Arrival         flightyBoardPoint `json:"arrival,omitempty"`
}

type flightyStatusEl struct {
	Type  string `json:"type,omitempty"`
	Icon  string `json:"icon,omitempty"`
	Text  string `json:"text,omitempty"`
	Style string `json:"style,omitempty"`
}

type flightyTimeEl struct {
	Text  string `json:"text,omitempty"`
	Style string `json:"style,omitempty"`
}

type flightyAirline struct {
	ID   string `json:"id,omitempty"`
	IATA string `json:"iata,omitempty"`
	Name string `json:"name,omitempty"`
}

type flightyBoardPoint struct {
	IATA     string `json:"iata,omitempty"`
	Terminal string `json:"terminal,omitempty"`
	Gate     string `json:"gate,omitempty"`
	Belt     string `json:"belt,omitempty"`
	Flag     string `json:"flag,omitempty"`
}

// flightyStatusText collapses the status element list into one readable line.
func (f flightyBoardFlight) StatusText() string {
	parts := make([]string, 0, len(f.Status))
	for _, el := range f.Status {
		switch {
		case el.Text != "":
			parts = append(parts, el.Text)
		case el.Icon != "" && el.Icon != "BULLET":
			parts = append(parts, strings.ToLower(strings.ReplaceAll(el.Icon, "_", " ")))
		}
	}
	return strings.Join(parts, "; ")
}

// flightyFetchBoard resolves the airport and fetches one board live (or from
// the local mirror under --data-source local). kind is "arrivals" or
// "departures".
func flightyFetchBoard(ctx context.Context, flags *rootFlags, kind, airport string) ([]flightyBoardFlight, error) {
	slug, err := flightyResolveSlugForFetch(ctx, flags, airport)
	if err != nil {
		return nil, err
	}
	path := "/airports/" + slug + "/" + kind
	var raw json.RawMessage
	if flags.dataSource == "local" {
		data, err := flightyLocalPathJSON(path)
		if err != nil {
			return nil, err
		}
		raw = data
	} else {
		c, err := flags.newClient()
		if err != nil {
			return nil, err
		}
		fetched, err := c.GetWithHeaders(ctx, path, nil, nil)
		if err != nil {
			if flags.dataSource == "auto" {
				data, localErr := flightyLocalPathJSON(path)
				if localErr != nil {
					return nil, fmt.Errorf("API unreachable and no local data. Run 'flighty-pp-cli sync' to enable offline access.\n\nOriginal error: %w", err)
				}
				raw = data
			} else {
				return nil, err
			}
		} else {
			raw = fetched
		}
	}
	extracted, err := extractFlightyRSC("board", raw)
	if err != nil {
		return nil, err
	}
	var flights []flightyBoardFlight
	if err := json.Unmarshal(extracted, &flights); err != nil {
		return nil, fmt.Errorf("parsing %s board: %w", kind, err)
	}
	if flights == nil {
		flights = []flightyBoardFlight{}
	}
	return flights, nil
}

// flightyLocalPathJSON returns an honest error: flight boards are live-only
// surfaces (sync syncs the catalog, not per-airport boards).
func flightyLocalPathJSON(path string) (json.RawMessage, error) {
	return nil, fmt.Errorf("no local data for %s; boards are live-only — rerun without --data-source local", path)
}

// flightyFetchDetailRaw resolves the airport and returns the extracted detail
// JSON (identity + weather + today performance).
func flightyFetchDetailRaw(ctx context.Context, flags *rootFlags, airport string) (json.RawMessage, string, error) {
	slug, err := flightyResolveSlugForFetch(ctx, flags, airport)
	if err != nil {
		return nil, "", err
	}
	path := "/airports/" + slug
	c, err := flags.newClient()
	if err != nil {
		return nil, "", err
	}
	fetched, err := c.GetWithHeaders(ctx, path, nil, nil)
	if err != nil {
		return nil, "", err
	}
	if isDryRunResponseForClient(c, fetched) {
		return fetched, slug, nil
	}
	raw, err := extractFlightyRSC("detail", fetched)
	if err != nil {
		return nil, "", err
	}
	return raw, slug, nil
}

// flightyFetchDetailBySlug fetches and extracts an airport detail page for a
// slug already resolved from the catalog (no per-call resolution).
func flightyFetchDetailBySlug(ctx context.Context, flags *rootFlags, slug string) (json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	fetched, err := c.GetWithHeaders(ctx, "/airports/"+slug, nil, nil)
	if err != nil {
		return nil, err
	}
	return extractFlightyRSC("detail", fetched)
}

// flightyParsePercent converts "13%" (or "0.13") to 13.0; ok=false when absent.
func flightyParsePercent(v any) (float64, bool) {
	switch t := v.(type) {
	case string:
		s := strings.TrimSuffix(strings.TrimSpace(t), "%")
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	case float64:
		if t <= 1 {
			return t * 100, true
		}
		return t, true
	}
	return 0, false
}

// flightyCatalogAutoWithHints acquires the catalog with local preference:
// read the synced mirror (with sync hints); when the mirror has no rows,
// fall back to one live catalog fetch so the command still answers on a
// fresh machine.
func flightyCatalogAutoWithHints(cmd *cobra.Command, flags *rootFlags, dbPath string) ([]flightyCatalogAirport, error) {
	db, rows, err := flightyOpenLocalCatalog(cmd, flags, "airports", dbPath)
	if err == nil {
		defer db.Close()
		if len(rows) > 0 {
			return rows, nil
		}
	}
	// Empty or unreadable mirror: fetch live (auto strategy).
	fetched, liveErr := flightyCatalogLive(cmd.Context(), flags)
	if liveErr != nil {
		if err != nil {
			return nil, err
		}
		fmt.Fprintln(os.Stderr, "no local catalog and live fetch failed. run: flighty-pp-cli sync --resources airports --full")
		return nil, liveErr
	}
	parsed := []flightyCatalogAirport{}
	if err := json.Unmarshal(fetched, &parsed); err != nil {
		return nil, fmt.Errorf("parsing live catalog: %w", err)
	}
	fmt.Fprintln(os.Stderr, "note: local mirror empty; ranked from a live catalog fetch (run 'flighty-pp-cli sync --resources airports --full' for offline use)")
	return parsed, nil
}

// flightyOpenLocalCatalog opens the local store read-only, runs the sync
// hints, and returns the synced catalog rows (deduplicated). The caller must
// Close the returned store. resource should be "airports".
func flightyOpenLocalCatalog(cmd *cobra.Command, flags *rootFlags, resource, dbPath string) (*store.Store, []flightyCatalogAirport, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("flighty-pp-cli")
	}
	db, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening local database: %w", err)
	}
	if !hintIfUnsynced(cmd, db, resource) {
		hintIfStale(cmd, db, resource, flags.maxAge)
	}
	rows, err := db.DB().QueryContext(cmd.Context(), `
		SELECT data FROM resources
		WHERE resource_type IN ('airports', 'airports-tv')`)
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("querying local catalog: %w", err)
	}
	byIATA := map[string]flightyCatalogAirport{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			db.Close()
			return nil, nil, fmt.Errorf("scanning local catalog row: %w", err)
		}
		var ap flightyCatalogAirport
		if err := json.Unmarshal([]byte(data), &ap); err != nil {
			continue // skip flight/detail rows that share the resource table
		}
		if ap.IATA == "" && ap.Slug == "" {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(ap.IATA))
		if key == "" {
			key = ap.Slug
		}
		if existing, seen := byIATA[key]; seen && existing.Region != "" && ap.Region == "" {
			continue
		}
		byIATA[key] = ap
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		db.Close()
		return nil, nil, fmt.Errorf("iterating local catalog: %w", err)
	}
	if err := rows.Close(); err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("closing local catalog rows: %w", err)
	}
	out := make([]flightyCatalogAirport, 0, len(byIATA))
	for _, ap := range byIATA {
		out = append(out, ap)
	}
	return db, out, nil
}

// upper/lower are tiny local helpers used by the Flighty novel commands.
func upper(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }
func lower(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
