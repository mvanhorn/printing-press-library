// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelDedupeCmd(flags *rootFlags) *cobra.Command {
	var mk, model, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "dedupe [make-or-model]",
		Short: "Collapse the same physical VIN listed on multiple marketplaces into one row with every source and price, cheapest first.",
		Example: strings.Trim(`
  autotempest-pp-cli dedupe --json
  autotempest-pp-cli dedupe "civic" --json
  autotempest-pp-cli dedupe --make honda --model civic --select vin,min_price,sources.source,sources.price --json`, "\n"),
		// Positional is a free-text make/model filter against the local store;
		// any string is valid, so the invalid-arg probe does not apply.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// The positional is an AMBIGUOUS make-or-model token; keep it
			// separate from --make/--model so it (and only it) OR-matches both
			// columns. --make/--model each bind to their own column.
			var term string
			if len(args) > 0 {
				term = args[0]
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, ok, err := guardLocalNovel(ctx, cmd, flags, dbPath, mk, model, "")
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			rows, err := dedupeRows(ctx, db.DB(), mk, model, term, limit)
			if err != nil {
				return err
			}
			return emitNovel(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&mk, "make", "", "Filter by make")
	cmd.Flags().StringVar(&model, "model", "", "Filter by model")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to emit")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	return cmd
}

// dedupeSourceEntry is one source's listing of a VIN.
type dedupeSourceEntry struct {
	Source string `json:"source"`
	Price  string `json:"price"`
	URL    string `json:"url"`
	cents  int64
}

func dedupeRows(ctx context.Context, sqlDB *sql.DB, mk, model, term string, limit int) ([]map[string]any, error) {
	where := []string{"vin IS NOT NULL", "vin != ''"}
	var argv []any
	where, argv = appendMakeModelTermFilter(where, argv, mk, model, term)
	// #nosec G202 -- where clauses are constant literals (slugColExpr takes a trusted
	// column name); every user value is bound through argv via ? placeholders below.
	query := `SELECT vin, title, make, model, year, price_cents, source, url
		FROM at_listings WHERE ` + strings.Join(where, " AND ")

	dbRows, err := sqlDB.QueryContext(ctx, query, argv...)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	type group struct {
		vin     string
		title   string
		make    string
		model   string
		year    int
		entries []dedupeSourceEntry
		minP    int64
	}
	groups := map[string]*group{}
	order := []string{}
	for dbRows.Next() {
		var vin, title, mkv, modelv, source, url sql.NullString
		var year sql.NullInt64
		var price sql.NullInt64
		if err := dbRows.Scan(&vin, &title, &mkv, &modelv, &year, &price, &source, &url); err != nil {
			return nil, err
		}
		v := vin.String
		g, ok := groups[v]
		if !ok {
			g = &group{vin: v, title: title.String, make: mkv.String, model: modelv.String, year: int(year.Int64), minP: -1}
			groups[v] = g
			order = append(order, v)
		}
		cents := int64(-1)
		if price.Valid {
			cents = price.Int64
		}
		g.entries = append(g.entries, dedupeSourceEntry{
			Source: source.String, Price: centsDisplay(cents), URL: url.String, cents: cents,
		})
		if cents >= 0 && (g.minP < 0 || cents < g.minP) {
			g.minP = cents
		}
	}
	if err := dbRows.Err(); err != nil {
		return nil, err
	}

	out := make([]*group, 0, len(order))
	for _, v := range order {
		out = append(out, groups[v])
	}
	// Sort rows by n_sources desc, then min_price asc.
	sort.SliceStable(out, func(i, j int) bool {
		ni, nj := len(out[i].entries), len(out[j].entries)
		if ni != nj {
			return ni > nj
		}
		return priceLess(out[i].minP, out[j].minP)
	})

	rows := make([]map[string]any, 0, len(out))
	for _, g := range out {
		// Sort each VIN's sources by price asc (unknown last).
		sort.SliceStable(g.entries, func(i, j int) bool {
			return priceLess(g.entries[i].cents, g.entries[j].cents)
		})
		rows = append(rows, map[string]any{
			"vin":       g.vin,
			"title":     g.title,
			"make":      g.make,
			"model":     g.model,
			"year":      g.year,
			"sources":   g.entries,
			"min_price": centsDisplay(g.minP),
			"n_sources": len(g.entries),
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows, nil
}

// priceLess orders by price ascending, placing unknown (-1) prices last.
func priceLess(a, b int64) bool {
	if a < 0 {
		return false
	}
	if b < 0 {
		return true
	}
	return a < b
}
