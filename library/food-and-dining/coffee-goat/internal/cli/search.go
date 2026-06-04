// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// searchHit is one row in the search response.
type searchHit struct {
	Roaster    string `json:"roaster"`
	Handle     string `json:"handle"`
	Title      string `json:"title"`
	Origin     string `json:"origin,omitempty"`
	Process    string `json:"process,omitempty"`
	PriceCents int    `json:"price_cents,omitempty"`
	Currency   string `json:"currency,omitempty"`
	WeightG    int    `json:"weight_g,omitempty"`
	InStock    bool   `json:"in_stock"`
	URL        string `json:"url,omitempty"`
	PricePerOz string `json:"price_per_oz,omitempty"`
}

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var inStock bool
	var origin, process string
	var priceLT int
	var limit int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Cross-roaster FTS5 search over the synced roaster_products corpus",
		Long: `Search every roaster's catalog at once using SQLite FTS5. Match against
title, body_text, origin, producer, varietal, and tag fields. Filter
by in-stock, origin, process, or price ceiling.`,
		Example: `  coffee-goat-pp-cli search "ethiopia natural"
  coffee-goat-pp-cli search gesha --in-stock --limit 5
  coffee-goat-pp-cli search colombia --process washed --price-lt 3500`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return usageErr(fmt.Errorf("search requires a query"))
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			hits, err := runSearch(db, query, origin, process, inStock, priceLT, limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), hits, flags)
			}
			if len(hits) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no matches")
				return nil
			}
			for _, h := range hits {
				stockMark := ""
				if !h.InStock {
					stockMark = " [out]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s / %s — %s (%s)%s\n", h.Roaster, h.Title, h.Origin, h.Process, stockMark)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&inStock, "in-stock", false, "Only return products marked in stock")
	cmd.Flags().StringVar(&origin, "origin", "", "Filter by origin (case-insensitive substring)")
	cmd.Flags().StringVar(&process, "process", "", "Filter by process (case-insensitive substring)")
	cmd.Flags().IntVar(&priceLT, "price-lt", 0, "Maximum price in cents (e.g. 3500 for $35)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum results to return")
	return cmd
}

func runSearch(db *store.Store, query, origin, process string, inStockOnly bool, priceLT, limit int) ([]searchHit, error) {
	if limit <= 0 {
		limit = 20
	}
	// Fast path: no structural filters, just delegate to the store's
	// domain-specific FTS method. Keeps the query out of the CLI layer
	// for the common case.
	if origin == "" && process == "" && priceLT == 0 && !inStockOnly {
		hits, err := db.SearchRoasterProducts(query, limit)
		if err != nil {
			return nil, fmt.Errorf("search: %w", err)
		}
		out := make([]searchHit, 0, len(hits))
		for _, h := range hits {
			s := searchHit{
				Roaster: h.Roaster, Handle: h.Handle, Title: h.Title,
				Origin: h.Origin, Process: h.Process,
				PriceCents: h.PriceCents, Currency: h.Currency,
				WeightG: h.WeightG, InStock: h.InStock, URL: h.URL,
			}
			computePricePerOz(&s)
			out = append(out, s)
		}
		return out, nil
	}
	// FTS5 'porter unicode61' tokenizer handles case folding; pass the
	// raw user query straight in. Multi-word queries become an
	// AND-implicit match by default.
	q := `SELECT rp.roaster_slug, rp.handle, COALESCE(rp.title,''), COALESCE(rp.origin,''), COALESCE(rp.process,''),
	             COALESCE(rp.price_cents,0), COALESCE(rp.currency,''), COALESCE(rp.weight_g,0),
	             COALESCE(rp.in_stock,0), COALESCE(rp.url,'')
	      FROM roaster_products rp
	      JOIN roaster_products_fts ON roaster_products_fts.rowid = rp.rowid
	      WHERE roaster_products_fts MATCH ?`
	args := []any{query}
	if inStockOnly {
		q += ` AND rp.in_stock = 1`
	}
	if origin != "" {
		q += ` AND LOWER(rp.origin) LIKE ?`
		args = append(args, "%"+strings.ToLower(origin)+"%")
	}
	if process != "" {
		q += ` AND LOWER(rp.process) LIKE ?`
		args = append(args, "%"+strings.ToLower(process)+"%")
	}
	if priceLT > 0 {
		q += ` AND rp.price_cents > 0 AND rp.price_cents <= ?`
		args = append(args, priceLT)
	}
	q += ` ORDER BY rank LIMIT ?`
	args = append(args, limit)

	rows, err := db.DB().Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	var out []searchHit
	for rows.Next() {
		var h searchHit
		var inStockInt int
		if err := rows.Scan(&h.Roaster, &h.Handle, &h.Title, &h.Origin, &h.Process, &h.PriceCents, &h.Currency, &h.WeightG, &inStockInt, &h.URL); err != nil {
			return nil, fmt.Errorf("scan search row: %w", err)
		}
		h.InStock = inStockInt == 1
		computePricePerOz(&h)
		out = append(out, h)
	}
	if err := rows.Err(); err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return out, nil
}

// computePricePerOz fills the PricePerOz string field from PriceCents and
// WeightG. Bag weight is grams (the Shopify-shaped roaster_products column);
// 28.3495 g/oz is the standard ounce conversion. No-op when either input is
// zero or missing.
func computePricePerOz(h *searchHit) {
	if h.PriceCents <= 0 || h.WeightG <= 0 {
		return
	}
	ozs := float64(h.WeightG) / 28.3495
	h.PricePerOz = fmt.Sprintf("$%.2f/oz", float64(h.PriceCents)/100.0/ozs)
}
