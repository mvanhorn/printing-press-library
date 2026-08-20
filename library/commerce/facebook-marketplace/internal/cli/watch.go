package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Manage local Marketplace watches",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchToggleCmd(flags))
	cmd.AddCommand(newWatchRunCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var row watchRow
	var minPrice, maxPrice float64
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a local Marketplace watch",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if row.Name == "" || row.Query == "" {
				return fmt.Errorf("--name and --query are required")
			}
			row.MinPriceCents = centsFromDollars(minPrice)
			row.MaxPriceCents = centsFromDollars(maxPrice)
			db, err := openLocalDB()
			if err != nil {
				return err
			}
			defer db.Close()
			now := time.Now().UTC().Format(time.RFC3339)
			res, err := db.Exec(`INSERT INTO watches (name, query, must_have_keywords, reject_keywords, min_price_cents, max_price_cents, radius_miles, enabled, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
				ON CONFLICT(name) DO UPDATE SET query = excluded.query, must_have_keywords = excluded.must_have_keywords, reject_keywords = excluded.reject_keywords,
					min_price_cents = excluded.min_price_cents, max_price_cents = excluded.max_price_cents, radius_miles = excluded.radius_miles, enabled = 1, updated_at = excluded.updated_at`,
				row.Name, row.Query, row.MustHaveKeywords, row.RejectKeywords, row.MinPriceCents, row.MaxPriceCents, row.RadiusMiles, now, now)
			if err != nil {
				return err
			}
			row.ID, _ = res.LastInsertId()
			row.Enabled = true
			row.CreatedAt = now
			row.UpdatedAt = now
			return json.NewEncoder(cmd.OutOrStdout()).Encode(row)
		},
	}
	cmd.Flags().StringVar(&row.Name, "name", "", "Watch name")
	cmd.Flags().StringVar(&row.Query, "query", "", "Marketplace search query")
	cmd.Flags().StringVar(&row.MustHaveKeywords, "must-have-keywords", "", "Comma-separated required title keywords")
	cmd.Flags().StringVar(&row.RejectKeywords, "reject-keywords", "", "Comma-separated rejected title keywords")
	cmd.Flags().Float64Var(&minPrice, "min-price", 0, "Minimum price in dollars")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum price in dollars")
	cmd.Flags().IntVar(&row.RadiusMiles, "radius", 0, "Maximum distance in miles")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List local Marketplace watches",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalDB()
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query(`SELECT id, name, query, must_have_keywords, reject_keywords, min_price_cents, max_price_cents, radius_miles, enabled, created_at, updated_at FROM watches ORDER BY name`)
			if err != nil {
				return err
			}
			defer rows.Close()
			watches, err := scanWatches(rows)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(watches)
		},
	}
}

func newWatchToggleCmd(flags *rootFlags) *cobra.Command {
	var enabled bool
	cmd := &cobra.Command{
		Use:   "toggle <name>",
		Short: "Enable or disable a local watch",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalDB()
			if err != nil {
				return err
			}
			defer db.Close()
			res, err := db.Exec(`UPDATE watches SET enabled = ?, updated_at = ? WHERE name = ?`, boolInt(enabled), time.Now().UTC().Format(time.RFC3339), args[0])
			if err != nil {
				return err
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return notFoundErr(fmt.Errorf("watch %q not found", args[0]))
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"name": args[0], "enabled": enabled})
		},
	}
	cmd.Flags().BoolVar(&enabled, "enabled", true, "Whether the watch is enabled")
	return cmd
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var listingsPath string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run deterministic watch filters against local or supplied listings",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openLocalDB()
			if err != nil {
				return err
			}
			defer db.Close()
			if listingsPath != "" {
				listings, err := loadListingsFile(listingsPath)
				if err != nil {
					return err
				}
				for _, listing := range listings {
					if err := upsertListing(db, listing); err != nil {
						return err
					}
				}
			}
			watches, err := enabledWatches(db)
			if err != nil {
				return err
			}
			listings, err := allListings(db)
			if err != nil {
				return err
			}
			created := 0
			for _, watch := range watches {
				for _, listing := range listings {
					ok, reason := deterministicMatch(watch, listing)
					if !ok {
						continue
					}
					inserted, err := insertWatchMatch(db, watch.ID, listing.ID, reason, time.Now().UTC().Format(time.RFC3339))
					if err != nil {
						return err
					}
					if inserted {
						created++
					}
				}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"watches":  len(watches),
				"listings": len(listings),
				"matches":  created,
				"filter":   "deterministic filters only",
			})
		},
	}
	cmd.Flags().StringVar(&listingsPath, "listings", "", "Optional JSON file of listings to import before matching")
	return cmd
}

func insertWatchMatch(db *sql.DB, watchID int64, listingID, reason, createdAt string) (bool, error) {
	res, err := db.Exec(`INSERT OR IGNORE INTO matches (watch_id, listing_id, deterministic_ok, llm_relevant, reason, is_new, created_at)
		VALUES (?, ?, 1, 0, ?, 1, ?)`, watchID, listingID, reason, createdAt)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func scanWatches(rows *sql.Rows) ([]watchRow, error) {
	watches := []watchRow{}
	for rows.Next() {
		var w watchRow
		var enabled int
		if err := rows.Scan(&w.ID, &w.Name, &w.Query, &w.MustHaveKeywords, &w.RejectKeywords, &w.MinPriceCents, &w.MaxPriceCents, &w.RadiusMiles, &enabled, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, err
		}
		w.Enabled = enabled == 1
		watches = append(watches, w)
	}
	return watches, rows.Err()
}

func enabledWatches(db *sql.DB) ([]watchRow, error) {
	rows, err := db.Query(`SELECT id, name, query, must_have_keywords, reject_keywords, min_price_cents, max_price_cents, radius_miles, enabled, created_at, updated_at FROM watches WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanWatches(rows)
}

func allListings(db *sql.DB) ([]listingRow, error) {
	rows, err := db.Query(`SELECT id, title, price_cents, distance_miles, url, seller_name, public_location, listed_at, updated_at, engagement_count, raw_json FROM listings ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var listings []listingRow
	for rows.Next() {
		var l listingRow
		if err := rows.Scan(&l.ID, &l.Title, &l.PriceCents, &l.DistanceMiles, &l.URL, &l.SellerName, &l.PublicLocation, &l.ListedAt, &l.UpdatedAt, &l.EngagementCount, &l.RawJSON); err != nil {
			return nil, err
		}
		listings = append(listings, l)
	}
	return listings, rows.Err()
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
