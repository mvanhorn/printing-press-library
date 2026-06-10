// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelDropsCmd(flags *rootFlags) *cobra.Command {
	var flagSince, dbPath string
	var minDrop, limit int

	cmd := &cobra.Command{
		Use:   "drops [name]",
		Short: "Surface listings whose price fell since a prior sync, biggest drop first.",
		Example: strings.Trim(`
  autotempest-pp-cli drops --since 7d --min-drop 500 --json
  autotempest-pp-cli drops mysearch --since 14d --json`, "\n"),
		// Optional positional scopes to a saved-search name; an unknown name (or
		// empty store) yields exit 0 with no rows / the missing-mirror hint, not
		// an error, so the invalid-arg probe does not apply.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			var since time.Duration
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(err)
				}
				since = d
			}

			var savedName string
			if len(args) > 0 {
				savedName = args[0]
			}

			db, ok, err := guardLocalNovel(ctx, cmd, flags, dbPath, "", "", "")
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			rows, err := dropRows(ctx, db.DB(), since, int64(minDrop)*100, savedName, limit)
			if err != nil {
				return err
			}
			return emitNovel(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only compare snapshots within this window (e.g. 7d, 24h, 1w)")
	cmd.Flags().IntVar(&minDrop, "min-drop", 0, "Minimum price drop in dollars")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to emit")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	return cmd
}

// dropRows compares each listing's earliest snapshot within the --since window
// against its latest, emitting rows where the price fell by >= minDropCents.
func dropRows(ctx context.Context, sqlDB *sql.DB, since time.Duration, minDropCents int64, savedName string, limit int) ([]map[string]any, error) {
	var sinceTs int64
	if since > 0 {
		sinceTs = time.Now().Add(-since).Unix()
	}

	// Pull snapshots (optionally windowed), ordered so we can pick earliest and
	// latest per listing. Filter price_cents >= 0 to ignore unknown prices.
	query := `SELECT listing_id, ts, price_cents FROM at_price_snapshots
		WHERE price_cents >= 0`
	var argv []any
	if sinceTs > 0 {
		query += ` AND ts >= ?`
		argv = append(argv, sinceTs)
	}
	query += ` ORDER BY listing_id, ts ASC, id ASC`

	snapRows, err := sqlDB.QueryContext(ctx, query, argv...)
	if err != nil {
		return nil, err
	}
	defer snapRows.Close()

	type window struct {
		earliest int64
		latest   int64
		seen     bool
	}
	windows := map[string]*window{}
	order := []string{}
	for snapRows.Next() {
		var id string
		var ts, price int64
		if err := snapRows.Scan(&id, &ts, &price); err != nil {
			return nil, err
		}
		w, ok := windows[id]
		if !ok {
			w = &window{earliest: price, latest: price, seen: true}
			windows[id] = w
			order = append(order, id)
			continue
		}
		// Rows arrive in ascending ts, so the last one wins as latest.
		w.latest = price
	}
	if err := snapRows.Err(); err != nil {
		return nil, err
	}

	type dropResult struct {
		id   string
		old  int64
		new  int64
		drop int64
	}
	var drops []dropResult
	for _, id := range order {
		w := windows[id]
		drop := w.earliest - w.latest
		if drop >= minDropCents && drop > 0 {
			drops = append(drops, dropResult{id: id, old: w.earliest, new: w.latest, drop: drop})
		}
	}
	sort.SliceStable(drops, func(i, j int) bool { return drops[i].drop > drops[j].drop })

	rows := make([]map[string]any, 0, len(drops))
	for _, d := range drops {
		// Join listing display fields; scope by saved search name if provided.
		var vin, title, mk, model, source, url sql.NullString
		var year sql.NullInt64
		q := `SELECT vin, title, make, model, year, source, url FROM at_listings WHERE listing_id = ?`
		qargs := []any{d.id}
		if savedName != "" {
			q += ` AND search_name = ?`
			qargs = append(qargs, savedName)
		}
		err := sqlDB.QueryRowContext(ctx, q, qargs...).Scan(&vin, &title, &mk, &model, &year, &source, &url)
		if err == sql.ErrNoRows {
			continue // filtered out by saved-search scope
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, map[string]any{
			"listing_id": d.id,
			"vin":        vin.String,
			"title":      title.String,
			"make":       mk.String,
			"model":      model.String,
			"year":       int(year.Int64),
			"old_price":  centsDisplay(d.old),
			"new_price":  centsDisplay(d.new),
			"drop":       centsDisplay(d.drop),
			"source":     source.String,
			"url":        url.String,
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows, nil
}
