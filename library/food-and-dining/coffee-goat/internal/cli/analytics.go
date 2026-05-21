// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// newAnalyticsCmd is the vision-canonical analytics entry-point. Each
// subcommand answers an aggregation question against the synced corpus
// or personal brew log. All read-only.
func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Roll-up aggregations across the synced corpus and personal brew log (origins, roasters, descriptors, monthly trends)",
		Example: `  coffee-goat-pp-cli analytics origins
  coffee-goat-pp-cli analytics roasters --limit 20
  coffee-goat-pp-cli analytics descriptors
  coffee-goat-pp-cli analytics brews-by-month`,
	}
	cmd.Annotations = map[string]string{"mcp:read-only": "true"}
	cmd.AddCommand(newAnalyticsOriginsCmd(flags))
	cmd.AddCommand(newAnalyticsRoastersCmd(flags))
	cmd.AddCommand(newAnalyticsDescriptorsCmd(flags))
	cmd.AddCommand(newAnalyticsBrewsByMonthCmd(flags))
	return cmd
}

func newAnalyticsOriginsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "origins",
		Short: "Top origins in the synced cross-roaster corpus by product count",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ensureFreshForResources(ctx, flags, "products")
			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(ctx,
				`SELECT COALESCE(NULLIF(origin,''),'(unknown)') AS origin, COUNT(*) AS n,
				        SUM(CASE WHEN in_stock=1 THEN 1 ELSE 0 END) AS in_stock_n
				 FROM roaster_products
				 GROUP BY origin
				 ORDER BY n DESC
				 LIMIT ?`, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type row struct {
				Origin  string `json:"origin"`
				Total   int    `json:"total"`
				InStock int    `json:"in_stock"`
			}
			var data []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Origin, &r.Total, &r.InStock); err != nil {
					continue
				}
				data = append(data, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate origins rows: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"origins": data, "count": len(data)}, flags)
			}
			headers := []string{"origin", "total", "in_stock"}
			out := make([][]string, 0, len(data))
			for _, r := range data {
				out = append(out, []string{r.Origin, fmt.Sprintf("%d", r.Total), fmt.Sprintf("%d", r.InStock)})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max origins")
	return cmd
}

func newAnalyticsRoastersCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "roasters",
		Short: "Top roasters by product count, with average price and in-stock share",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ensureFreshForResources(ctx, flags, "products")
			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(ctx,
				`SELECT roaster_slug AS roaster, COUNT(*) AS n,
				        SUM(CASE WHEN in_stock=1 THEN 1 ELSE 0 END) AS in_stock_n,
				        ROUND(AVG(CASE WHEN price_cents > 0 THEN price_cents/100.0 ELSE NULL END), 2) AS avg_price
				 FROM roaster_products
				 GROUP BY roaster_slug
				 ORDER BY n DESC
				 LIMIT ?`, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type row struct {
				Roaster  string          `json:"roaster"`
				Total    int             `json:"total"`
				InStock  int             `json:"in_stock"`
				AvgPrice sql.NullFloat64 `json:"avg_price_usd,omitempty"`
			}
			var data []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Roaster, &r.Total, &r.InStock, &r.AvgPrice); err != nil {
					continue
				}
				data = append(data, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate roasters rows: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"roasters": data, "count": len(data)}, flags)
			}
			headers := []string{"roaster", "total", "in_stock", "avg_price_usd"}
			out := make([][]string, 0, len(data))
			for _, r := range data {
				price := "—"
				if r.AvgPrice.Valid {
					price = fmt.Sprintf("%.2f", r.AvgPrice.Float64)
				}
				out = append(out, []string{r.Roaster, fmt.Sprintf("%d", r.Total), fmt.Sprintf("%d", r.InStock), price})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 30, "Max roasters")
	return cmd
}

func newAnalyticsDescriptorsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "descriptors",
		Short: "Most common flavor descriptors across the synced reviews corpus",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ensureFreshForResources(ctx, flags, "reviews")
			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			// reviews.descriptors_json may be a JSON array of strings; use
			// json_each to unnest.
			rows, err := db.DB().QueryContext(ctx,
				`SELECT LOWER(j.value) AS d, COUNT(*) AS n
				 FROM reviews, json_each(NULLIF(reviews.descriptors_json,'')) AS j
				 WHERE j.value IS NOT NULL AND TRIM(j.value) != ''
				 GROUP BY d
				 ORDER BY n DESC
				 LIMIT ?`, limit)
			if err != nil {
				return err
			}
			defer rows.Close()
			type row struct {
				Descriptor string `json:"descriptor"`
				Count      int    `json:"count"`
			}
			var data []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Descriptor, &r.Count); err != nil {
					continue
				}
				data = append(data, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate descriptors rows: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"descriptors": data, "count": len(data)}, flags)
			}
			headers := []string{"descriptor", "count"}
			out := make([][]string, 0, len(data))
			for _, r := range data {
				out = append(out, []string{r.Descriptor, fmt.Sprintf("%d", r.Count)})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 30, "Max descriptors")
	return cmd
}

func newAnalyticsBrewsByMonthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brews-by-month",
		Short: "Personal brew count and average rating per month",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.DB().QueryContext(ctx,
				`SELECT strftime('%Y-%m', brewed_at) AS month,
				        COUNT(*) AS brews,
				        ROUND(AVG(CASE WHEN rating > 0 THEN rating ELSE NULL END), 2) AS avg_rating
				 FROM brews
				 WHERE brewed_at IS NOT NULL
				 GROUP BY month
				 ORDER BY month DESC`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type row struct {
				Month     string          `json:"month"`
				Brews     int             `json:"brews"`
				AvgRating sql.NullFloat64 `json:"avg_rating,omitempty"`
			}
			var data []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.Month, &r.Brews, &r.AvgRating); err != nil {
					continue
				}
				data = append(data, r)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate brews-by-month rows: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"months": data, "count": len(data)}, flags)
			}
			headers := []string{"month", "brews", "avg_rating"}
			out := make([][]string, 0, len(data))
			for _, r := range data {
				avg := "—"
				if r.AvgRating.Valid {
					avg = fmt.Sprintf("%.2f", r.AvgRating.Float64)
				}
				out = append(out, []string{r.Month, fmt.Sprintf("%d", r.Brews), avg})
			}
			return flags.printTable(cmd, headers, out)
		},
	}
	return cmd
}
