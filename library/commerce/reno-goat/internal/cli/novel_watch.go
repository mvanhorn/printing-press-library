// Copyright 2026 h179922. Licensed under Apache-2.0. See LICENSE.
// Novel command: price watch — track product prices and alert on drops.

package cli

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Track product prices and get alerts when they drop",
		Long:  "Track product prices and get alerts when they drop. Add products to a watch list, check for price changes, and view price history.",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchCheckCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchHistoryCmd(flags))

	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var price float64
	var title string

	cmd := &cobra.Command{
		Use:     "add <product-url>",
		Short:   "Add a product to the price watch list",
		Example: "  reno-goat-pp-cli watch add https://www.westelm.com/products/mid-century-sofa --threshold 15 --price 1299",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			productURL := args[0]
			source := inferSource(productURL)

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			// Store current_price and original_price when provided so that
			// watch check can record price_history continuity entries and
			// watch history returns meaningful data from the start.
			var priceVal, origVal any
			if price > 0 {
				priceVal = price
				origVal = price
			}
			var titleVal any
			if title != "" {
				titleVal = title
			}
			// Only pass threshold as non-nil when explicitly set, so the
			// ON CONFLICT clause preserves the existing value on re-add
			// (e.g. `watch add URL --title "x"` shouldn't reset threshold).
			var thresholdVal any
			if cmd.Flags().Changed("threshold") {
				thresholdVal = threshold
			}

			_, err = db.Exec(
				`INSERT INTO watches (product_url, source, threshold_pct, current_price, original_price, title)
				 VALUES (?, ?, COALESCE(?, 10.0), ?, ?, ?)
				 ON CONFLICT(product_url) DO UPDATE SET
				   threshold_pct = COALESCE(excluded.threshold_pct, watches.threshold_pct),
				   current_price = COALESCE(excluded.current_price, watches.current_price),
				   title = COALESCE(excluded.title, watches.title)`,
				productURL, source, thresholdVal, priceVal, origVal, titleVal,
			)
			if err != nil {
				return fmt.Errorf("adding watch: %w", err)
			}

			// Seed price_history with the initial price so watch history
			// has a baseline entry from the start.
			if price > 0 {
				// Get the watch ID (may be from INSERT or existing row after conflict).
				var watchID int64
				if err := db.QueryRow(`SELECT id FROM watches WHERE product_url = ?`, productURL).Scan(&watchID); err == nil {
					_, _ = db.Exec(`INSERT INTO price_history (watch_id, price) VALUES (?, ?)`, watchID, price)
				}
			}

			result := map[string]any{
				"status":      "added",
				"product_url": productURL,
				"source":      source,
				"threshold":   threshold,
			}
			if price > 0 {
				result["price"] = price
			}
			if title != "" {
				result["title"] = title
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 10.0, "Price drop percentage to trigger an alert")
	cmd.Flags().Float64Var(&price, "price", 0, "Current product price (seeds price history for tracking)")
	cmd.Flags().StringVar(&title, "title", "", "Product title for display in watch list")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all watched products with current vs original price",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.Query(
				`SELECT id, product_url, source, title, threshold_pct,
				        original_price, current_price, created_at, last_checked
				 FROM watches ORDER BY created_at DESC`)
			if err != nil {
				return fmt.Errorf("listing watches: %w", err)
			}
			defer rows.Close()

			var watches []map[string]any
			for rows.Next() {
				var (
					id                          int64
					productURL, source          string
					title                       sql.NullString
					threshold                   float64
					originalPrice, currentPrice sql.NullFloat64
					createdAt                   string
					lastChecked                 sql.NullString
				)
				if err := rows.Scan(&id, &productURL, &source, &title, &threshold,
					&originalPrice, &currentPrice, &createdAt, &lastChecked); err != nil {
					return fmt.Errorf("scanning watch row: %w", err)
				}
				w := map[string]any{
					"id":          id,
					"product_url": productURL,
					"source":      source,
					"threshold":   threshold,
					"created_at":  createdAt,
				}
				if title.Valid {
					w["title"] = title.String
				}
				if originalPrice.Valid {
					w["original_price"] = originalPrice.Float64
				}
				if currentPrice.Valid {
					w["current_price"] = currentPrice.Float64
				}
				if lastChecked.Valid {
					w["last_checked"] = lastChecked.String
				}
				if originalPrice.Valid && currentPrice.Valid && originalPrice.Float64 > 0 {
					change := ((currentPrice.Float64 - originalPrice.Float64) / originalPrice.Float64) * 100
					w["price_change_pct"] = fmt.Sprintf("%.1f%%", change)
				}
				watches = append(watches, w)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating watches: %w", err)
			}

			if len(watches) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"watches": []any{},
					"count":   0,
				}, flags)
			}

			return printJSONFiltered(cmd.OutOrStdout(), watches, flags)
		},
	}
}

func newWatchCheckCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Poll all watched products for price changes (cron-friendly)",
		Long:  "Iterates all watched products, checks if URLs are still reachable, and reports price alerts. Exit 0 always (cron-friendly). Alerts are printed to stderr.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := db.Query(
				`SELECT id, product_url, source, title, threshold_pct, original_price, current_price
				 FROM watches`)
			if err != nil {
				return fmt.Errorf("querying watches: %w", err)
			}

			type watchRow struct {
				id            int64
				productURL    string
				source        string
				title         sql.NullString
				threshold     float64
				originalPrice sql.NullFloat64
				currentPrice  sql.NullFloat64
			}
			var watchList []watchRow
			for rows.Next() {
				var w watchRow
				if err := rows.Scan(&w.id, &w.productURL, &w.source, &w.title,
					&w.threshold, &w.originalPrice, &w.currentPrice); err != nil {
					rows.Close()
					return fmt.Errorf("scanning watch: %w", err)
				}
				watchList = append(watchList, w)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating watches: %w", err)
			}

			if len(watchList) == 0 {
				fmt.Fprintln(os.Stderr, "no watches configured")
				return nil
			}

			httpClient := &http.Client{Timeout: flags.timeout}
			var results []map[string]any

			for _, w := range watchList {
				now := time.Now().UTC().Format(time.RFC3339)
				result := map[string]any{
					"id":          w.id,
					"product_url": w.productURL,
					"source":      w.source,
				}
				if w.title.Valid {
					result["title"] = w.title.String
				}

				// Check if URL is reachable
				headReq, headErr := http.NewRequestWithContext(cmd.Context(), "HEAD", w.productURL, nil)
				if headErr != nil {
					result["reachable"] = false
					result["status"] = "error"
					result["checked_at"] = now
					results = append(results, result)
					continue
				}
				resp, err := httpClient.Do(headReq)
				reachable := err == nil && resp != nil && resp.StatusCode < 400
				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}
				result["reachable"] = reachable
				result["checked_at"] = now

				if !reachable {
					result["status"] = "unreachable"
					if err != nil {
						fmt.Fprintf(os.Stderr, "ALERT: %s is unreachable: %v\n", w.productURL, err)
					} else if resp != nil {
						fmt.Fprintf(os.Stderr, "ALERT: %s returned HTTP %d\n", w.productURL, resp.StatusCode)
					}
				} else {
					result["status"] = "ok"
				}

				// Report the current price in the check result when available.
				// price_history is seeded by watch add --price; v1 cannot
				// scrape live prices, so we don't insert duplicate continuity
				// rows here. When v2 adds source-specific price parsing,
				// this block should INSERT the newly fetched price and UPDATE
				// watches.current_price.
				if w.currentPrice.Valid {
					result["price"] = w.currentPrice.Float64
				}

				// Update last_checked timestamp
				_, _ = db.Exec(`UPDATE watches SET last_checked = ? WHERE id = ?`, now, w.id)

				// Check threshold alert
				if w.originalPrice.Valid && w.currentPrice.Valid && w.originalPrice.Float64 > 0 {
					drop := ((w.originalPrice.Float64 - w.currentPrice.Float64) / w.originalPrice.Float64) * 100
					if drop >= w.threshold {
						result["alert"] = true
						result["drop_pct"] = fmt.Sprintf("%.1f%%", drop)
						title := w.productURL
						if w.title.Valid {
							title = w.title.String
						}
						fmt.Fprintf(os.Stderr, "PRICE ALERT: %s dropped %.1f%% (was $%.2f, now $%.2f)\n",
							title, drop, w.originalPrice.Float64, w.currentPrice.Float64)
					}
				}

				results = append(results, result)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <product-url>",
		Short:   "Remove a product from the watch list",
		Example: "  reno-goat-pp-cli watch remove https://www.westelm.com/products/mid-century-sofa",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			productURL := args[0]

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			// Get the watch ID first to clean up price_history
			var watchID int64
			err = db.QueryRow(`SELECT id FROM watches WHERE product_url = ?`, productURL).Scan(&watchID)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("no watch found for %q", productURL))
			}
			if err != nil {
				return fmt.Errorf("looking up watch: %w", err)
			}

			// Delete price history first (foreign key)
			_, _ = db.Exec(`DELETE FROM price_history WHERE watch_id = ?`, watchID)
			_, err = db.Exec(`DELETE FROM watches WHERE id = ?`, watchID)
			if err != nil {
				return fmt.Errorf("removing watch: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":      "removed",
				"product_url": productURL,
			}, flags)
		},
	}
}

func newWatchHistoryCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "history <product-url>",
		Short:   "Show price history for a watched product",
		Example: "  reno-goat-pp-cli watch history https://www.westelm.com/products/mid-century-sofa",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			productURL := args[0]

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			var watchID int64
			var source string
			var title sql.NullString
			err = db.QueryRow(
				`SELECT id, source, title FROM watches WHERE product_url = ?`,
				productURL,
			).Scan(&watchID, &source, &title)
			if err == sql.ErrNoRows {
				return notFoundErr(fmt.Errorf("no watch found for %q", productURL))
			}
			if err != nil {
				return fmt.Errorf("looking up watch: %w", err)
			}

			rows, err := db.Query(
				`SELECT price, on_sale, checked_at FROM price_history
				 WHERE watch_id = ? ORDER BY checked_at DESC`,
				watchID,
			)
			if err != nil {
				return fmt.Errorf("querying price history: %w", err)
			}
			defer rows.Close()

			var history []map[string]any
			for rows.Next() {
				var (
					price     float64
					onSale    bool
					checkedAt string
				)
				if err := rows.Scan(&price, &onSale, &checkedAt); err != nil {
					return fmt.Errorf("scanning history row: %w", err)
				}
				history = append(history, map[string]any{
					"price":      price,
					"on_sale":    onSale,
					"checked_at": checkedAt,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating history: %w", err)
			}

			result := map[string]any{
				"product_url": productURL,
				"source":      source,
				"history":     history,
				"count":       len(history),
			}
			if title.Valid {
				result["title"] = title.String
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
}
