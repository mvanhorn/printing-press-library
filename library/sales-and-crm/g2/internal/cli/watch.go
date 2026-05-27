// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/g2/internal/store"
	"github.com/spf13/cobra"
)

// newWatchCmd builds the `watch` parent + its `products` and
// `market-signals` children. The parent is a no-op shell; both leaves
// read the local SQLite mirror and emit a structured per-target row.
func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Diff locally synced G2 snapshots across a window",
		Long:        "Diff star ratings, new-review counts, and market signals across products or categories using the local store.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWatchProductsCmd(flags))
	cmd.AddCommand(newWatchMarketSignalsCmd(flags))
	return cmd
}

// productWatchRow is the per-product row emitted by `watch products`.
// Kept narrow so JSON output is stable and predictable.
type productWatchRow struct {
	ProductID                  string  `json:"product_id"`
	ProductSlug                string  `json:"product_slug"`
	ProductName                string  `json:"product_name"`
	CurrentStarRating          float64 `json:"current_star_rating"`
	ReviewCount                int64   `json:"review_count"`
	NewReviewsCountSinceCutoff int64   `json:"new_reviews_count_since_cutoff"`
}

func newWatchProductsCmd(flags *rootFlags) *cobra.Command {
	var productList string
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "products [flags]",
		Short:       "Diff star ratings and new-review counts across products since a window",
		Long:        "For each --product (comma-separated slug or ID), emit current star rating, current review count, and new reviews synced since --since.\n\nReads the local SQLite mirror only. Run 'g2-pp-cli sync --resources products,reviews' first.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  g2-pp-cli watch products --product slack,zoom --since 7d",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(productList) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--product is required"))
			}
			if strings.TrimSpace(since) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since is required"))
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}
			cutoff := time.Now().Add(-window)

			if dbPath == "" {
				dbPath = defaultDBPath("g2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			maybeEmitSyncHints(cmd, db, "products", flags.maxAge)

			rows, err := collectProductWatchRows(db.DB(), parseProductList(productList), cutoff)
			if err != nil {
				return fmt.Errorf("collecting product diff rows: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"product_id", "slug", "name", "star_rating", "review_count", "new_since_cutoff"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{
					r.ProductID,
					r.ProductSlug,
					r.ProductName,
					fmt.Sprintf("%.2f", r.CurrentStarRating),
					fmt.Sprintf("%d", r.ReviewCount),
					fmt.Sprintf("%d", r.NewReviewsCountSinceCutoff),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&productList, "product", "", "Comma-separated product slugs or IDs (required)")
	cmd.Flags().StringVar(&since, "since", "", "Window for the diff (e.g. 7d, 24h, 1w; required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/g2-pp-cli/data.db)")
	return cmd
}

// parseProductList trims and splits a comma-separated product spec.
// Exported-ish: kept lowercase so it's package-private; consumed by tests.
func parseProductList(spec string) []string {
	var out []string
	for _, p := range strings.Split(spec, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// collectProductWatchRows runs the per-product queries against the local
// `products` typed table and counts new reviews in `reviews` since cutoff.
// Reviews live under `products` because the spec marks them as the
// `products_reviews` hierarchical child; the typed table is `reviews`.
func collectProductWatchRows(db *sql.DB, products []string, cutoff time.Time) ([]productWatchRow, error) {
	rows := make([]productWatchRow, 0, len(products))
	for _, p := range products {
		var row productWatchRow

		// Look up the product. Accept slug OR id; either column may be
		// empty in the typed table on a sparse sync, so fall back to the
		// generic resources table for `data` JSON before giving up.
		var (
			id          sql.NullString
			slug        sql.NullString
			name        sql.NullString
			starRating  sql.NullFloat64
			reviewCount sql.NullInt64
		)
		err := db.QueryRow(`SELECT id, slug, name, star_rating, review_count FROM products WHERE slug = ? OR id = ? LIMIT 1`, p, p).Scan(&id, &slug, &name, &starRating, &reviewCount)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("querying products for %q: %w", p, err)
		}
		row.ProductID = id.String
		row.ProductSlug = slug.String
		row.ProductName = name.String
		row.CurrentStarRating = starRating.Float64
		row.ReviewCount = reviewCount.Int64

		// If typed columns are blank (older sync), fall back to data JSON.
		if !starRating.Valid || !reviewCount.Valid {
			var rawData sql.NullString
			_ = db.QueryRow(`SELECT data FROM products WHERE slug = ? OR id = ? LIMIT 1`, p, p).Scan(&rawData)
			if rawData.Valid {
				var obj map[string]any
				if json.Unmarshal([]byte(rawData.String), &obj) == nil {
					if attrs, ok := obj["attributes"].(map[string]any); ok {
						if !starRating.Valid {
							if v, ok := attrs["star_rating"].(float64); ok {
								row.CurrentStarRating = v
							}
						}
						if !reviewCount.Valid {
							if v, ok := attrs["review_count"].(float64); ok {
								row.ReviewCount = int64(v)
							}
						}
					}
				}
			}
		}

		// If we still don't have an ID, use the input as a synthetic
		// identifier so the row still emits a non-empty key — agents
		// reading JSON output rely on a stable per-target row.
		if row.ProductID == "" {
			row.ProductID = p
		}
		if row.ProductSlug == "" {
			row.ProductSlug = p
		}

		// Count new reviews synced since cutoff. We scope by products_id
		// (the typed parent foreign key) when we know the product's ID;
		// otherwise scope by the raw input (which the sync routine writes
		// into products_id when the parent was looked up by slug).
		scope := row.ProductID
		if id.Valid && id.String != "" {
			scope = id.String
		}
		var newCount sql.NullInt64
		if err := db.QueryRow(`SELECT COUNT(*) FROM reviews WHERE products_id = ? AND synced_at >= ?`, scope, cutoff).Scan(&newCount); err != nil {
			return nil, fmt.Errorf("counting new reviews for %q: %w", p, err)
		}
		row.NewReviewsCountSinceCutoff = newCount.Int64

		rows = append(rows, row)
	}
	return rows, nil
}

// marketSignalRow is the per-signal row emitted by `watch market-signals`.
type marketSignalRow struct {
	CategoryID    string  `json:"category_id"`
	SignalID      string  `json:"signal_id"`
	Day           string  `json:"day,omitempty"`
	IntentScore   float64 `json:"intent_score"`
	VisitsCount   int64   `json:"visits_count"`
	CompanyName   string  `json:"company_name,omitempty"`
	CompanyDomain string  `json:"company_domain,omitempty"`
}

func newWatchMarketSignalsCmd(flags *rootFlags) *cobra.Command {
	var category string
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "market-signals [flags]",
		Short:       "Emit market_signals state for a category over a window",
		Long:        "Emit market_signals rows for --category synced within --since.\n\nV1 emits the current synced state per category. Historical diff requires running sync periodically and comparing snapshots externally.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  g2-pp-cli watch market-signals --category 12345 --since 7d",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if strings.TrimSpace(category) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--category is required"))
			}
			if strings.TrimSpace(since) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--since is required"))
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
			}
			cutoff := time.Now().Add(-window)

			if dbPath == "" {
				dbPath = defaultDBPath("g2-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			maybeEmitSyncHints(cmd, db, "market_signals", flags.maxAge)

			rows, err := collectMarketSignalRows(db.DB(), category, cutoff)
			if err != nil {
				return fmt.Errorf("collecting market signal rows: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			headers := []string{"day", "category_id", "company_name", "domain", "intent_score", "visits"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{
					r.Day,
					r.CategoryID,
					r.CompanyName,
					r.CompanyDomain,
					fmt.Sprintf("%.2f", r.IntentScore),
					fmt.Sprintf("%d", r.VisitsCount),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "Category slug or ID (required)")
	cmd.Flags().StringVar(&since, "since", "", "Window for the diff (e.g. 7d, 24h, 1w; required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (defaults to ~/.local/share/g2-pp-cli/data.db)")
	return cmd
}

// collectMarketSignalRows reads market_signals state for a category from
// the local store. The spec routes market_signals through the generic
// `resources` table with `resource_type='market_signals'`; data JSON
// holds the per-signal attributes. We also look at the typed `sandbox`
// table because the sandbox surface mirrors market_signals shape under
// scope=no-auth.
func collectMarketSignalRows(db *sql.DB, category string, cutoff time.Time) ([]marketSignalRow, error) {
	var rows []marketSignalRow

	// Resources path (canonical for market_signals).
	q := `SELECT id, data, synced_at FROM resources WHERE resource_type IN ('market_signals', 'market-signals') AND synced_at >= ?`
	rs, err := db.Query(q, cutoff)
	if err != nil {
		return nil, fmt.Errorf("querying market_signals resources: %w", err)
	}
	defer rs.Close()
	for rs.Next() {
		var id, data sql.NullString
		var syncedAt sql.NullTime
		if err := rs.Scan(&id, &data, &syncedAt); err != nil {
			return nil, fmt.Errorf("scanning market_signals row: %w", err)
		}
		if !data.Valid {
			continue
		}
		row := marketSignalRow{SignalID: id.String}
		var obj map[string]any
		if err := json.Unmarshal([]byte(data.String), &obj); err == nil {
			attrs, _ := obj["attributes"].(map[string]any)
			if attrs == nil {
				attrs = obj
			}
			catID, _ := attrs["category_id"].(string)
			if catID == "" {
				if v, ok := attrs["category_id"].(float64); ok {
					catID = fmt.Sprintf("%d", int64(v))
				}
			}
			row.CategoryID = catID
			row.Day, _ = attrs["day"].(string)
			if v, ok := attrs["intent_score"].(float64); ok {
				row.IntentScore = v
			}
			if v, ok := attrs["company_intent_score"].(float64); ok && row.IntentScore == 0 {
				row.IntentScore = v
			}
			if v, ok := attrs["visits_count"].(float64); ok {
				row.VisitsCount = int64(v)
			}
			if v, ok := attrs["total_activity"].(float64); ok && row.VisitsCount == 0 {
				row.VisitsCount = int64(v)
			}
			row.CompanyName, _ = attrs["company_name"].(string)
			row.CompanyDomain, _ = attrs["company_domain"].(string)
		}
		if !matchCategory(row.CategoryID, category) {
			continue
		}
		rows = append(rows, row)
	}
	if err := rs.Err(); err != nil {
		return nil, fmt.Errorf("reading market_signals rows: %w", err)
	}

	// Sandbox path (fallback for environments where only sandbox is synced).
	sq := `SELECT id, category_id, company_name, company_domain, company_intent_score, total_activity, day FROM sandbox WHERE category_id = ? AND synced_at >= ?`
	srows, err := db.Query(sq, category, cutoff)
	if err == nil {
		defer srows.Close()
		for srows.Next() {
			var id, catID, name, domain, day sql.NullString
			var intent sql.NullFloat64
			var activity sql.NullInt64
			if err := srows.Scan(&id, &catID, &name, &domain, &intent, &activity, &day); err != nil {
				continue
			}
			rows = append(rows, marketSignalRow{
				CategoryID:    catID.String,
				SignalID:      id.String,
				Day:           day.String,
				IntentScore:   intent.Float64,
				VisitsCount:   activity.Int64,
				CompanyName:   name.String,
				CompanyDomain: domain.String,
			})
		}
	}
	return rows, nil
}

func matchCategory(rowCat, target string) bool {
	if rowCat == "" {
		return true // no category in JSON means we shouldn't filter it out
	}
	return rowCat == target
}
