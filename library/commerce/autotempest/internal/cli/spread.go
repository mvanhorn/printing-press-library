// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/autotempest"

	"github.com/spf13/cobra"
)

func newNovelSpreadCmd(flags *rootFlags) *cobra.Command {
	var mk, model, dbPath string

	cmd := &cobra.Command{
		Use:   "spread [make-or-model]",
		Short: "Report min, median, and max price per marketplace for a model so you see which sources run cheap or expensive.",
		Example: strings.Trim(`
  autotempest-pp-cli spread "f-150" --json
  autotempest-pp-cli spread --make honda --model civic --json`, "\n"),
		// Positional is a free-text make/model filter against the local store:
		// any string is valid (no-match / empty store is exit 0 with the
		// missing-mirror hint, not an error), so the invalid-arg probe does not apply.
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

			rows, err := spreadRows(ctx, db.DB(), mk, model, term)
			if err != nil {
				return err
			}
			return emitNovel(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&mk, "make", "", "Filter by make")
	cmd.Flags().StringVar(&model, "model", "", "Filter by model")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	return cmd
}

func spreadRows(ctx context.Context, sqlDB *sql.DB, mk, model, term string) ([]map[string]any, error) {
	where := []string{"price_cents >= 0", "source IS NOT NULL", "source != ''"}
	var argv []any
	where, argv = appendMakeModelTermFilter(where, argv, mk, model, term)
	// #nosec G202 -- where clauses are constant literals (slugColExpr takes a trusted
	// column name); every user value is bound through argv via ? placeholders below.
	query := `SELECT source, price_cents FROM at_listings WHERE ` + strings.Join(where, " AND ")

	dbRows, err := sqlDB.QueryContext(ctx, query, argv...)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	prices := map[string][]int64{}
	order := []string{}
	for dbRows.Next() {
		var source sql.NullString
		var price sql.NullInt64
		if err := dbRows.Scan(&source, &price); err != nil {
			return nil, err
		}
		s := source.String
		if _, ok := prices[s]; !ok {
			order = append(order, s)
		}
		prices[s] = append(prices[s], price.Int64)
	}
	if err := dbRows.Err(); err != nil {
		return nil, err
	}

	type srow struct {
		source string
		n      int
		minP   int64
		medP   int64
		maxP   int64
	}
	var srows []srow
	for _, s := range order {
		p := prices[s]
		sort.Slice(p, func(i, j int) bool { return p[i] < p[j] })
		srows = append(srows, srow{
			source: s, n: len(p),
			minP: p[0], medP: medianInt64(p), maxP: p[len(p)-1],
		})
	}
	sort.SliceStable(srows, func(i, j int) bool { return srows[i].medP < srows[j].medP })

	rows := make([]map[string]any, 0, len(srows))
	for _, r := range srows {
		rows = append(rows, map[string]any{
			"source":       r.source,
			"source_name":  autotempest.SourceName(r.source),
			"n":            r.n,
			"min_price":    centsDisplay(r.minP),
			"median_price": centsDisplay(r.medP),
			"max_price":    centsDisplay(r.maxP),
		})
	}
	return rows, nil
}
