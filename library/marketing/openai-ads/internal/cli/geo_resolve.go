// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: resolve campaign targeting location IDs to readable names.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. This command reads the
// local store and resolves unknown location ids against the live geo lookup API,
// caching results into the geo_lookup table.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/openai-ads/internal/store"
	"github.com/spf13/cobra"
)

// geoResolveRow maps one targeting location id to its readable name. Name is
// empty and Resolved is false when the id could not be resolved — echoing the
// id back as its own name would make a total resolution failure look like a
// successful lookup.
type geoResolveRow struct {
	LocationID string   `json:"location_id"`
	Name       string   `json:"name"`
	Resolved   bool     `json:"resolved"`
	Campaigns  []string `json:"campaigns"`
}

// campaignLocationIDs extracts the targeting location ids for a campaign.
func campaignLocationIDs(data string) []string {
	var obj map[string]any
	if json.Unmarshal([]byte(data), &obj) != nil {
		return nil
	}
	targeting, ok := obj["targeting"].(map[string]any)
	if !ok {
		return nil
	}
	locations, ok := targeting["locations"].(map[string]any)
	if !ok {
		return nil
	}
	include, ok := locations["include"].([]any)
	if !ok {
		return nil
	}
	var ids []string
	for _, item := range include {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := m["id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

func newNovelGeoResolveCmd(flags *rootFlags) *cobra.Command {
	var flagRefresh bool

	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Turn the bare location IDs in campaign targeting into readable place names.",
		Long: `Read every campaign's targeting location IDs from the local store, resolve
unknown IDs via the geo lookup API, cache them in the geo_lookup table, and
render readable names. --refresh bypasses the cache and re-resolves.`,
		Example: strings.Trim(`
  openai-ads-pp-cli geo resolve --agent
  openai-ads-pp-cli geo resolve --refresh
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "geo resolve")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, defaultDBPath("openai-ads-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// Collect location id -> campaigns from the store.
			campaignNames := map[string]string{}
			idCampaigns := map[string][]string{}
			var idOrder []string
			rows, err := db.Query(`SELECT id, name, data FROM campaigns`)
			if err != nil {
				return fmt.Errorf("querying campaigns: %w", err)
			}
			for rows.Next() {
				var cid, name, data sql.NullString
				if err := rows.Scan(&cid, &name, &data); err != nil {
					_ = rows.Close()
					return err
				}
				campaignNames[cid.String] = name.String
				for _, loc := range campaignLocationIDs(data.String) {
					if _, seen := idCampaigns[loc]; !seen {
						idOrder = append(idOrder, loc)
					}
					idCampaigns[loc] = append(idCampaigns[loc], cid.String)
				}
			}
			if err := closeRows(rows); err != nil {
				return err
			}

			cache := geoCacheNames(db)
			// A missing client is not fatal: cached names still resolve. It
			// only means unresolved ids stay unresolved, which the summary
			// below reports rather than papering over.
			c, cerr := flags.newClient()
			get := func(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
				if c == nil {
					return nil, fmt.Errorf("no client available")
				}
				return c.Get(ctx, path, params)
			}

			var result []geoResolveRow
			unresolved := 0
			for _, loc := range idOrder {
				name := cache[loc]
				if name == "" || flagRefresh {
					if c != nil {
						if n, ok := resolveLocationDB(ctx, db, get, loc); ok {
							name = n
						}
					}
				}
				if name == "" {
					unresolved++
				}
				result = append(result, geoResolveRow{
					LocationID: loc,
					Name:       name,
					Resolved:   name != "",
					Campaigns:  idCampaigns[loc],
				})
			}
			if unresolved > 0 {
				reason := "no matching location in the geo lookup API"
				if c == nil {
					reason = "geo lookup API unavailable"
					if cerr != nil {
						reason = fmt.Sprintf("geo lookup API unavailable: %v", cerr)
					}
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d location IDs left unresolved (%s)\n", unresolved, len(result), reason)
			}
			sort.SliceStable(result, func(i, j int) bool { return result[i].LocationID < result[j].LocationID })
			if result == nil {
				result = make([]geoResolveRow, 0)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(result) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no campaign targeting locations to resolve")
				return nil
			}
			maps, merr := rowsToMaps(result)
			if merr != nil {
				return merr
			}
			return printAutoTable(cmd.OutOrStdout(), maps)
		},
	}
	cmd.Flags().BoolVar(&flagRefresh, "refresh", false, "Bypass the geo_lookup cache and re-resolve location IDs against the API.")
	return cmd
}

// geoCacheNames returns location id -> cached name from the geo_lookup table.
func geoCacheNames(db *store.Store) map[string]string {
	out := map[string]string{}
	rows, err := db.Query(`SELECT id, name FROM geo_lookup`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id, name sql.NullString
		if rows.Scan(&id, &name) == nil && id.Valid {
			out[id.String] = name.String
		}
	}
	return out
}

// resolveLocationDB queries the geo lookup API for a location id, caches
// every result entry into the geo_lookup table, and returns the matched name.
func resolveLocationDB(ctx context.Context, db *store.Store, get func(context.Context, string, map[string]string) (json.RawMessage, error), loc string) (string, bool) {
	if get == nil {
		return "", false
	}
	data, err := get(ctx, "/geo_lookup/search", map[string]string{"q": loc, "limit": "100"})
	if err != nil {
		return "", false
	}
	results, ok := decodeGeoResults(data)
	if !ok {
		return "", false
	}
	var matched string
	for _, entry := range results {
		key := mapString(entry, "id")
		name := mapString(entry, "name")
		if key == "" {
			continue
		}
		// cache every entry
		if raw, err := json.Marshal(entry); err == nil {
			_ = db.UpsertGeoLookup(raw)
		}
		if key == loc && matched == "" && name != "" {
			matched = name
		}
	}
	// Deliberately no results[0] fallback: the search is a text query, so the
	// first hit for a miss is an unrelated place. Labelling the id with it
	// would be worse than reporting it unresolved.
	return matched, matched != ""
}

func decodeGeoResults(data json.RawMessage) ([]map[string]any, bool) {
	if len(data) == 0 {
		return nil, false
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, false
	}
	raw, ok := obj["results"].([]any)
	if !ok {
		return nil, false
	}
	out := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		if m, ok := r.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, true
}

func mapString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
