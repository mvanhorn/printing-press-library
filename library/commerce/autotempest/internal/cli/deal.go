// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelDealCmd(flags *rootFlags) *cobra.Command {
	var mk, model, dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "deal [model]",
		Short: "Rank listings by mechanical price delta from the median of comparable cars (same model, year, mileage band)",
		Example: strings.Trim(`
  autotempest-pp-cli deal "camry" --json
  autotempest-pp-cli deal --make toyota --model tacoma --select title,price,deal_score --json`, "\n"),
		// Positional is a free-text model filter against the local store: any
		// string is valid (an empty store or no-match is exit 0 with the
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

			rows, err := dealRows(ctx, db.DB(), mk, model, term, limit)
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

const mileageBand = 25000

type dealRow struct {
	ID      string
	Title   string
	Price   int64
	Mileage int64
	Year    int
	Make    string
	Model   string
	Source  string
	URL     string
	bucket  string
}

func dealRows(ctx context.Context, sqlDB *sql.DB, mk, model, term string, limit int) ([]map[string]any, error) {
	where := []string{"price_cents >= 0"}
	var argv []any
	where, argv = appendMakeModelTermFilter(where, argv, mk, model, term)
	// #nosec G202 -- where clauses are constant literals (slugColExpr takes a trusted
	// column name); every user value is bound through argv via ? placeholders below.
	query := `SELECT listing_id, title, price_cents, mileage, year, make, model, source, url
		FROM at_listings WHERE ` + strings.Join(where, " AND ")

	dbRows, err := sqlDB.QueryContext(ctx, query, argv...)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	var listings []dealRow
	buckets := map[string][]int64{}
	for dbRows.Next() {
		var id, title, mkv, modelv, source, url sql.NullString
		var price, mileage sql.NullInt64
		var year sql.NullInt64
		if err := dbRows.Scan(&id, &title, &price, &mileage, &year, &mkv, &modelv, &source, &url); err != nil {
			return nil, err
		}
		mileVal := int64(-1)
		if mileage.Valid {
			mileVal = mileage.Int64
		}
		d := dealRow{
			ID: id.String, Title: title.String, Price: price.Int64, Mileage: mileVal,
			Year: int(year.Int64), Make: mkv.String, Model: modelv.String,
			Source: source.String, URL: url.String,
		}
		band := int64(-1)
		if mileVal >= 0 {
			band = mileVal / mileageBand
		}
		d.bucket = fmt.Sprintf("%s|%s|%d|%d", strings.ToLower(mkv.String), strings.ToLower(modelv.String), d.Year, band)
		listings = append(listings, d)
		buckets[d.bucket] = append(buckets[d.bucket], price.Int64)
	}
	if err := dbRows.Err(); err != nil {
		return nil, err
	}

	medians := map[string]int64{}
	for b, prices := range buckets {
		if len(prices) >= 3 {
			medians[b] = medianInt64(prices)
		}
	}

	type scored struct {
		row    dealRow
		score  int
		median int64
	}
	var out []scored
	for _, d := range listings {
		median, ok := medians[d.bucket]
		if !ok || median <= 0 {
			continue
		}
		score := int(float64(median-d.Price) / float64(median) * 100.0)
		out = append(out, scored{row: d, score: score, median: median})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].score > out[j].score
	})

	rows := make([]map[string]any, 0, len(out))
	for _, s := range out {
		rows = append(rows, map[string]any{
			"title":        s.row.Title,
			"price":        centsDisplay(s.row.Price),
			"mileage":      milesDisplay(s.row.Mileage),
			"year":         s.row.Year,
			"deal_score":   s.score,
			"median_price": centsDisplay(s.median),
			"source":       s.row.Source,
			"url":          s.row.URL,
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows, nil
}

// medianInt64 returns the median of a non-empty slice (lower-middle for even
// length, matching a simple integer median).
func medianInt64(v []int64) int64 {
	c := append([]int64(nil), v...)
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	n := len(c)
	if n == 0 {
		return 0
	}
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}
