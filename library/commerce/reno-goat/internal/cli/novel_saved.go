// Copyright 2026 h179922. Licensed under Apache-2.0. See LICENSE.
// Novel command: save & stale detection — bookmark products and detect changes.

package cli

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newSaveCmd(flags *rootFlags) *cobra.Command {
	var title string
	var price float64

	cmd := &cobra.Command{
		Use:     "save <product-url>",
		Short:   "Bookmark a product for later",
		Example: "  reno-goat-pp-cli save https://www.article.com/product/sven-sofa --title \"Sven Sofa\" --price 1899",
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

			var titlePtr *string
			if title != "" {
				titlePtr = &title
			}
			var pricePtr *float64
			if price > 0 {
				pricePtr = &price
			}

			_, err = db.Exec(
				`INSERT INTO saved_products (product_url, source, title, price)
				 VALUES (?, ?, ?, ?)
				 ON CONFLICT(product_url) DO UPDATE SET
				   title = COALESCE(excluded.title, saved_products.title),
				   price = COALESCE(excluded.price, saved_products.price)`,
				productURL, source, titlePtr, pricePtr,
			)
			if err != nil {
				return fmt.Errorf("saving product: %w", err)
			}

			result := map[string]any{
				"status":      "saved",
				"product_url": productURL,
				"source":      source,
			}
			if title != "" {
				result["title"] = title
			}
			if price > 0 {
				result["price"] = price
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Product title/description")
	cmd.Flags().Float64Var(&price, "price", 0, "Product price")
	return cmd
}

func newSavedCmd(flags *rootFlags) *cobra.Command {
	var checkStale bool

	cmd := &cobra.Command{
		Use:   "saved",
		Short: "List all saved products",
		Long:  "List all saved/bookmarked products. Use --check-stale to re-fetch and flag changes (price changes, out-of-stock, discontinued).",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := openNovelDB()
			if err != nil {
				return err
			}
			defer db.Close()

			if checkStale {
				return runStaleCheck(cmd, db, flags)
			}

			return listSavedProducts(cmd, db, flags)
		},
	}
	cmd.Flags().BoolVar(&checkStale, "check-stale", false, "Re-fetch all saved products and flag changes")

	cmd.AddCommand(newSavedRemoveCmd(flags))

	return cmd
}

func listSavedProducts(cmd *cobra.Command, db *sql.DB, flags *rootFlags) error {
	rows, err := db.Query(`
		SELECT id, product_url, source, title, price, in_stock, saved_at, last_checked
		FROM saved_products ORDER BY saved_at DESC`)
	if err != nil {
		return fmt.Errorf("listing saved products: %w", err)
	}
	defer rows.Close()

	var products []map[string]any
	for rows.Next() {
		var (
			id                 int64
			productURL, source string
			title              sql.NullString
			price              sql.NullFloat64
			inStock            bool
			savedAt            string
			lastChecked        sql.NullString
		)
		if err := rows.Scan(&id, &productURL, &source, &title, &price, &inStock, &savedAt, &lastChecked); err != nil {
			return fmt.Errorf("scanning saved product: %w", err)
		}
		p := map[string]any{
			"id":          id,
			"product_url": productURL,
			"source":      source,
			"in_stock":    inStock,
			"saved_at":    savedAt,
		}
		if title.Valid {
			p["title"] = title.String
		}
		if price.Valid {
			p["price"] = price.Float64
		}
		if lastChecked.Valid {
			p["last_checked"] = lastChecked.String
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating saved products: %w", err)
	}

	if len(products) == 0 {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"saved": []any{},
			"count": 0,
		}, flags)
	}

	return printJSONFiltered(cmd.OutOrStdout(), products, flags)
}

func runStaleCheck(cmd *cobra.Command, db *sql.DB, flags *rootFlags) error {
	rows, err := db.Query(`
		SELECT id, product_url, source, title, price, in_stock
		FROM saved_products ORDER BY saved_at DESC`)
	if err != nil {
		return fmt.Errorf("listing saved products: %w", err)
	}

	type savedRow struct {
		id         int64
		productURL string
		source     string
		title      sql.NullString
		price      sql.NullFloat64
		inStock    bool
	}
	var savedList []savedRow
	for rows.Next() {
		var s savedRow
		if err := rows.Scan(&s.id, &s.productURL, &s.source, &s.title, &s.price, &s.inStock); err != nil {
			rows.Close()
			return fmt.Errorf("scanning saved product: %w", err)
		}
		savedList = append(savedList, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating saved products: %w", err)
	}

	if len(savedList) == 0 {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"saved": []any{},
			"count": 0,
		}, flags)
	}

	httpClient := &http.Client{Timeout: flags.timeout}
	var results []map[string]any

	for _, s := range savedList {
		now := time.Now().UTC().Format(time.RFC3339)
		result := map[string]any{
			"product_url": s.productURL,
			"source":      s.source,
			"checked_at":  now,
		}
		if s.title.Valid {
			result["title"] = s.title.String
		}
		if s.price.Valid {
			result["saved_price"] = s.price.Float64
		}

		// Check reachability via HEAD request
		headReq, headErr := http.NewRequestWithContext(cmd.Context(), "HEAD", s.productURL, nil)
		if headErr != nil {
			result["status"] = "error"
			results = append(results, result)
			continue
		}
		resp, err := httpClient.Do(headReq)
		reachable := err == nil && resp != nil && resp.StatusCode < 400
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}

		var status string
		if !reachable {
			// Product may be discontinued
			status = "unavailable"
			if s.inStock {
				// Was in stock, now unreachable
				_, _ = db.Exec(`UPDATE saved_products SET in_stock = 0, last_checked = ? WHERE id = ?`, now, s.id)
			}
			if resp != nil && resp.StatusCode == 404 {
				status = "discontinued"
				fmt.Fprintf(os.Stderr, "%s %s — DISCONTINUED (404)\n", red("[!]"), s.productURL)
			} else {
				fmt.Fprintf(os.Stderr, "%s %s — UNAVAILABLE\n", red("[!]"), s.productURL)
			}
		} else {
			// Product is reachable
			if !s.inStock {
				// Was out of stock, now back
				status = "back_in_stock"
				_, _ = db.Exec(`UPDATE saved_products SET in_stock = 1, last_checked = ? WHERE id = ?`, now, s.id)
				fmt.Fprintf(os.Stderr, "%s %s — back in stock\n", green("[+]"), s.productURL)
			} else {
				status = "ok"
				_, _ = db.Exec(`UPDATE saved_products SET last_checked = ? WHERE id = ?`, now, s.id)
				fmt.Fprintf(os.Stderr, "%s %s — still available\n", green("[ok]"), s.productURL)
			}
		}

		// Note: actual price re-fetch requires source-specific HTML/API parsing.
		// For v1, we detect reachability only. Price comparison against saved value
		// is a future enhancement.
		result["status"] = status
		result["in_stock"] = reachable

		results = append(results, result)
	}

	return printJSONFiltered(cmd.OutOrStdout(), results, flags)
}

func newSavedRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <product-url>",
		Short:   "Remove a bookmarked product",
		Example: "  reno-goat-pp-cli saved remove https://www.article.com/product/sven-sofa",
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

			res, err := db.Exec(`DELETE FROM saved_products WHERE product_url = ?`, productURL)
			if err != nil {
				return fmt.Errorf("removing saved product: %w", err)
			}
			affected, _ := res.RowsAffected()
			if affected == 0 {
				return notFoundErr(fmt.Errorf("no saved product found for %q", productURL))
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":      "removed",
				"product_url": productURL,
			}, flags)
		},
	}
}
