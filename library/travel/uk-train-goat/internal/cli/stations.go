// uk-train-goat hand-authored: offline UK station search backed by SQLite FTS5.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/openldbws"
	"github.com/mvanhorn/printing-press-library/library/travel/uk-train-goat/internal/store"

	"github.com/spf13/cobra"
)

func newStationsCmd(flags *rootFlags) *cobra.Command {
	var (
		search string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "stations",
		Short: "Search the local UK station list (offline CRS lookup)",
		Long: `Search the local SQLite station list for a CRS code by partial name.
On first invocation, populates the local store from the wrapper's built-in
station map (~2,580 entries) — no network call required.`,
		Example: strings.Trim(`
  uk-train-goat-pp-cli stations --search paddington
  uk-train-goat-pp-cli stations --search "kings" --json
  uk-train-goat-pp-cli stations --search euston --limit 3 --select results.crs,results.name
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dbPath := defaultDBPath("uk-train-goat-pp-cli")
			s, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer s.Close()

			// Auto-populate on first call so `stations --search` Just Works.
			if err := ensureStationsPopulated(s); err != nil {
				return fmt.Errorf("populating stations: %w", err)
			}

			if search == "" {
				// Without --search, return total count + a usage hint.
				count, _ := s.Count("station")
				payload := map[string]any{
					"total_stations": count,
					"hint":           "Pass --search <query> to filter; use Paddington / KGX / partial names.",
				}
				data, _ := json.Marshal(payload)
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}

			if limit == 0 {
				limit = 20
			}
			results, err := searchStations(s, search, limit)
			if err != nil {
				return fmt.Errorf("searching stations: %w", err)
			}
			payload := map[string]any{
				"query":   search,
				"results": results,
			}
			data, _ := json.Marshal(payload)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Filter stations by partial name or CRS code")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum results (default 20)")
	return cmd
}

// ensureStationsPopulated checks whether the station table has rows; if
// empty, bulk-imports from the wrapper's built-in StationCodeToNameMap.
func ensureStationsPopulated(s *store.Store) error {
	count, _ := s.Count("station")
	if count > 0 {
		return nil
	}
	items := make([]json.RawMessage, 0, len(openldbws.StationCodeToNameMap))
	for crs, name := range openldbws.StationCodeToNameMap {
		row := map[string]string{
			"crs":  string(crs),
			"name": name,
		}
		data, _ := json.Marshal(row)
		items = append(items, data)
	}
	_, _, err := s.UpsertBatch("station", items)
	return err
}

// searchStations performs case-insensitive substring matching over the
// local station table. The store's generic Search uses FTS5; this
// wrapper falls back to a simple LIKE query so partial matches like
// "kings" work even without explicit FTS tokenization.
func searchStations(s *store.Store, query string, limit int) ([]map[string]string, error) {
	q := strings.ToLower(query)
	rows, err := s.Query(`
		SELECT data FROM resources
		WHERE resource_type = 'station'
		  AND (LOWER(json_extract(data, '$.name')) LIKE ?
		   OR LOWER(json_extract(data, '$.crs')) LIKE ?)
		ORDER BY json_extract(data, '$.name') LIMIT ?`,
		"%"+q+"%", "%"+q+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]string
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var row map[string]string
		if err := json.Unmarshal(raw, &row); err != nil {
			continue
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
