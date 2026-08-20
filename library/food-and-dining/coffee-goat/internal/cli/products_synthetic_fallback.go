// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

// Synthetic-anchor fallback for the spec-emitted `products` command.
// coffee-goat is a synthetic CLI (no live API at base_url); the typed
// `products` command falls through to the locally-synced
// `roaster_products` table when the generic API path fails.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
)

// fallbackToRoasterProducts reads the locally-synced corpus and returns
// a JSON array of products. Empty result is success (not error).
func fallbackToRoasterProducts(ctx context.Context, roaster, origin, process string, inStockOnly bool, limit int) ([]byte, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath("coffee-goat-pp-cli"))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	defer db.Close()

	var where []string
	var args []any
	if roaster != "" {
		where = append(where, "roaster_slug = ?")
		args = append(args, roaster)
	}
	if origin != "" {
		where = append(where, "LOWER(origin) LIKE ?")
		args = append(args, "%"+strings.ToLower(origin)+"%")
	}
	if process != "" {
		where = append(where, "LOWER(process) LIKE ?")
		args = append(args, "%"+strings.ToLower(process)+"%")
	}
	if inStockOnly {
		where = append(where, "in_stock = 1")
	}
	q := `SELECT roaster_slug, handle, title, origin, process, price_cents, currency, in_stock, url
	      FROM roaster_products`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY title"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	} else {
		q += " LIMIT 100"
	}

	rows, err := db.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query roaster_products: %w", err)
	}
	defer rows.Close()

	type product struct {
		Roaster    string `json:"roaster"`
		Handle     string `json:"handle"`
		Title      string `json:"title"`
		Origin     string `json:"origin,omitempty"`
		Process    string `json:"process,omitempty"`
		PriceCents int    `json:"price_cents"`
		Currency   string `json:"currency"`
		InStock    bool   `json:"in_stock"`
		URL        string `json:"url"`
	}
	var out []product
	for rows.Next() {
		var p product
		var origin, process *string
		var inStock int
		if err := rows.Scan(&p.Roaster, &p.Handle, &p.Title, &origin, &process, &p.PriceCents, &p.Currency, &inStock, &p.URL); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if origin != nil {
			p.Origin = *origin
		}
		if process != nil {
			p.Process = *process
		}
		p.InStock = inStock == 1
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate roaster_products rows: %w", err)
	}
	if out == nil {
		out = []product{}
	}
	return json.Marshal(out)
}
