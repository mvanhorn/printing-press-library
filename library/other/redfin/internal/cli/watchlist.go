// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/store"
)

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "watchlist",
		Short:       "Save named search criteria and check for new listings or changes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Save a named search and check it later for new listings, price drops,
or status changes. Criteria are stored in the local database.

Subcommands:
  add    Save a new watchlist entry
  list   Show all saved watchlists
  check  Check a watchlist for new matches
  rm     Remove a watchlist entry`,
	}

	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistCheckCmd(flags))
	cmd.AddCommand(newWatchlistRmCmd(flags))

	return cmd
}

type watchlistCriteria struct {
	Zip      string  `json:"zip,omitempty"`
	MaxPrice float64 `json:"max_price,omitempty"`
	MinBeds  int     `json:"min_beds,omitempty"`
	MinBaths float64 `json:"min_baths,omitempty"`
	MaxDOM   int     `json:"max_dom,omitempty"`
	PropType string  `json:"property_type,omitempty"`
	SavedAt  string  `json:"saved_at"`
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var zip string
	var maxPrice float64
	var minBeds int
	var minBaths float64
	var maxDOM int
	var propType string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Save a named search watchlist",
		Example: `  redfin-pp-cli watchlist add myhood --zip 94102 --max-price 1500000 --min-beds 2
  redfin-pp-cli watchlist add condos --zip 94103 --property-type condo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("watchlist name is required"))
			}
			name := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			criteria := watchlistCriteria{
				Zip:      zip,
				MaxPrice: maxPrice,
				MinBeds:  minBeds,
				MinBaths: minBaths,
				MaxDOM:   maxDOM,
				PropType: propType,
				SavedAt:  time.Now().UTC().Format(time.RFC3339),
			}
			data, err := json.Marshal(criteria)
			if err != nil {
				return err
			}
			if err := db.Upsert("watchlist", name, data); err != nil {
				return fmt.Errorf("saving watchlist: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved watchlist %q. Run 'redfin-pp-cli watchlist check %s' to see matches.\n", name, name)
			return nil
		},
	}

	cmd.Flags().StringVar(&zip, "zip", "", "Zip code to watch")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum list price")
	cmd.Flags().IntVar(&minBeds, "min-beds", 0, "Minimum bedrooms")
	cmd.Flags().Float64Var(&minBaths, "min-baths", 0, "Minimum bathrooms")
	cmd.Flags().IntVar(&maxDOM, "max-dom", 0, "Maximum days on market (0 = any)")
	cmd.Flags().StringVar(&propType, "property-type", "", "Property type filter (e.g. condo, single_family)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all saved watchlists",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			items, err := db.List("watchlist", 0)
			if err != nil {
				if strings.Contains(err.Error(), "no such table") {
					fmt.Fprintln(cmd.OutOrStdout(), "No watchlists saved yet. Use 'watchlist add' to create one.")
					return nil
				}
				return fmt.Errorf("listing watchlists: %w", err)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No watchlists saved yet. Use 'watchlist add' to create one.")
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchlistCheckCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "check <name>",
		Short:       "Check a watchlist for new matches since last look",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  redfin-pp-cli watchlist check myhood
  redfin-pp-cli watchlist check myhood --since 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("watchlist name is required"))
			}
			name := args[0]
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'redfin-pp-cli sync' first.", err)
			}
			defer db.Close()

			rawCriteria, err := db.Get("watchlist", name)
			if err != nil || rawCriteria == nil {
				return fmt.Errorf("watchlist %q not found — use 'watchlist add %s ...' to create it", name, name)
			}

			var criteria watchlistCriteria
			if err := json.Unmarshal(rawCriteria, &criteria); err != nil {
				return fmt.Errorf("parsing watchlist criteria: %w", err)
			}

			var sinceThreshold time.Time
			if since != "" {
				t, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since: %w", err))
				}
				sinceThreshold = t
			} else {
				sinceThreshold = time.Now().Add(-7 * 24 * time.Hour)
			}

			results, err := applyWatchlistCriteria(cmd, db, criteria, sinceThreshold)
			if err != nil {
				return err
			}

			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No new matches for watchlist %q since %s.\n", name, sinceThreshold.Format("2006-01-02"))
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "How far back to look (e.g. 7d, 24h). Default: 7d")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func applyWatchlistCriteria(cmd *cobra.Command, db *store.Store, c watchlistCriteria, since time.Time) ([]map[string]any, error) {
	conditions := []string{"resource_type = 'listings'", "synced_at >= ?"}
	args := []any{since.UTC().Format("2006-01-02T15:04:05Z")}

	if c.Zip != "" {
		conditions = append(conditions, "json_extract(data, '$.address.zip') = ?")
		args = append(args, c.Zip)
	}
	if c.MaxPrice > 0 {
		conditions = append(conditions, "CAST(json_extract(data, '$.price.value') AS REAL) <= ?")
		args = append(args, c.MaxPrice)
	}
	if c.MinBeds > 0 {
		conditions = append(conditions, "CAST(json_extract(data, '$.beds') AS INTEGER) >= ?")
		args = append(args, c.MinBeds)
	}
	if c.MinBaths > 0 {
		conditions = append(conditions, "CAST(json_extract(data, '$.baths') AS REAL) >= ?")
		args = append(args, c.MinBaths)
	}
	if c.MaxDOM > 0 {
		conditions = append(conditions, "CAST(json_extract(data, '$.dom.value') AS INTEGER) <= ?")
		args = append(args, c.MaxDOM)
	}

	query := "SELECT data FROM resources WHERE " + strings.Join(conditions, " AND ") + " ORDER BY synced_at DESC LIMIT 200"
	rows, err := db.DB().QueryContext(cmd.Context(), query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, fmt.Errorf("no data — run 'redfin-pp-cli sync' first")
		}
		return nil, fmt.Errorf("querying watchlist: %w", err)
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var dataJSON []byte
		if err := rows.Scan(&dataJSON); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(dataJSON, &obj); err != nil {
			continue
		}
		results = append(results, obj)
	}
	return results, rows.Err()
}

func newWatchlistRmCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a saved watchlist",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usageErr(fmt.Errorf("watchlist name is required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("redfin-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			_, execErr := db.DB().ExecContext(cmd.Context(),
				"DELETE FROM resources WHERE resource_type = 'watchlist' AND id = ?", args[0])
			if execErr != nil {
				return fmt.Errorf("removing watchlist: %w", execErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed watchlist %q.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
