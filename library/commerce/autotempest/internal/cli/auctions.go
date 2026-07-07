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

func newNovelAuctionsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, sortBy string
	var limit int

	cmd := &cobra.Command{
		Use:   "auctions",
		Short: "Filter to eBay auction listings with live current bid and bid count, sortable by bid.",
		Example: strings.Trim(`
  autotempest-pp-cli auctions --select title,current_bid,bids,url --json
  autotempest-pp-cli auctions --sort bid --limit 25 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, ok, err := guardLocalNovel(ctx, cmd, flags, dbPath, "", "", "")
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			rows, err := auctionRows(ctx, db.DB(), sortBy, limit)
			if err != nil {
				return err
			}
			return emitNovel(cmd, flags, rows)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max rows to emit")
	cmd.Flags().StringVar(&sortBy, "sort", "bid", "Sort order: bid|end")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path")
	return cmd
}

func auctionRows(ctx context.Context, sqlDB *sql.DB, sortBy string, limit int) ([]map[string]any, error) {
	query := `SELECT title, current_bid_cents, bids, year, mileage, source, url, last_seen
		FROM at_listings
		WHERE listing_type = 'auction' OR current_bid_cents > 0`

	dbRows, err := sqlDB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer dbRows.Close()

	type arow struct {
		title    string
		bidCents int64
		bids     int64
		year     int
		mileage  int64
		source   string
		url      string
		lastSeen int64
	}
	var arows []arow
	for dbRows.Next() {
		var title, source, url sql.NullString
		var bidCents, bids, year, mileage, lastSeen sql.NullInt64
		if err := dbRows.Scan(&title, &bidCents, &bids, &year, &mileage, &source, &url, &lastSeen); err != nil {
			return nil, err
		}
		arows = append(arows, arow{
			title: title.String, bidCents: nullOrNeg(bidCents), bids: nullOrNeg(bids),
			year: int(year.Int64), mileage: nullOrNeg(mileage),
			source: source.String, url: url.String, lastSeen: lastSeen.Int64,
		})
	}
	if err := dbRows.Err(); err != nil {
		return nil, err
	}

	switch sortBy {
	case "end":
		// No end-date column persisted; approximate "ending soon" by most
		// recently seen first as a stable proxy.
		sort.SliceStable(arows, func(i, j int) bool { return arows[i].lastSeen > arows[j].lastSeen })
	default: // "bid"
		sort.SliceStable(arows, func(i, j int) bool { return arows[i].bidCents > arows[j].bidCents })
	}

	rows := make([]map[string]any, 0, len(arows))
	for _, a := range arows {
		// Emit null (not the -1 sentinel) when eBay did not provide a bid
		// count or current bid, so callers see a clean absence instead of a
		// nonsensical "-1".
		var bidsOut any
		if a.bids >= 0 {
			bidsOut = a.bids
		}
		var currentBidOut any
		if a.bidCents >= 0 {
			currentBidOut = centsDisplay(a.bidCents)
		}
		rows = append(rows, map[string]any{
			"title":       a.title,
			"current_bid": currentBidOut,
			"bids":        bidsOut,
			"year":        a.year,
			"mileage":     milesDisplay(a.mileage),
			"source":      a.source,
			"url":         a.url,
		})
		if limit > 0 && len(rows) >= limit {
			break
		}
	}
	return rows, nil
}

func nullOrNeg(v sql.NullInt64) int64 {
	if !v.Valid {
		return -1
	}
	return v.Int64
}
