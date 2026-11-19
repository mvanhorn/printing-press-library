// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newNovelListingsDeadWeightCmd(flags *rootFlags) *cobra.Command {
	var flagSince, dbPath string

	cmd := &cobra.Command{
		Use:         "dead-weight",
		Short:       "Find active listings with no sales in a window, with favorite counts.",
		Example:     "  etsy-pp-cli listings dead-weight --since 90d --agent",
		Long:        "Find active listings with no sales in a window, with favorite counts.",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "listings dead-weight")
			}
			window, err := shopxParseWindow("since", flagSince)
			if err != nil {
				return err
			}
			db, err := shopxOpenDB(cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			familyNotes := shopxCommandNotes(db, "listings", "transactions")
			shop := shopxShopID(cmd)
			rows, notes := shopxDeadWeight(db, shop, shopxNow(), window)
			notes = append(familyNotes, notes...)
			if shop != "" && !shopxFamilyHasData(db, shopxTransactionTypes) {
				notes = append(notes, "transactions not synced (needs OAuth transactions_r: run 'etsy-pp-cli auth login' then 'etsy-pp-cli sync --resources transactions'); cannot tell which listings had no sales")
				return shopxEmit(cmd, flags, []shopxDeadWeightRow{}, notes)
			}
			for i := range rows {
				rows[i].Window = flagSince
			}
			return shopxEmit(cmd, flags, rows, notes)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "90d", "Sales lookback window")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// shopxDeadWeightRow contains listing metrics only, never buyer information.
type shopxDeadWeightRow struct {
	ListingID   int64   `json:"listing_id"`
	Title       string  `json:"title"`
	NumFavorers *int64  `json:"num_favorers"`
	Views       *int64  `json:"views"`
	LastSaleAt  *string `json:"last_sale_at"`
	Window      string  `json:"window"`
}

func shopxDeadWeight(db *store.Store, shop string, now time.Time, window time.Duration) ([]shopxDeadWeightRow, []string) {
	if shop == "" {
		return []shopxDeadWeightRow{}, []string{shopxNoteShopUnset}
	}
	notes := make([]string, 0)
	listings, dropped := shopxActiveListings(db, shop)
	if dropped > 0 {
		notes = append(notes, shopxForeignNote(dropped))
	}
	last := make(map[int64]time.Time)
	seen := make(map[string]bool)
	for _, tx := range shopxRows(db, shopxTransactionTypes...) {
		id := shopxLinkedListingID(tx)
		ts, ok := shopxFirstTS(tx, "paid_timestamp", "created_timestamp", "create_timestamp")
		if !ok {
			continue
		}
		key := ""
		if tid := shopxStr(tx["transaction_id"]); tid != "" {
			key = "tx:" + tid
		} else if rid := shopxStr(tx["receipt_id"]); rid != "" {
			key = fmt.Sprintf("receipt:%s:%d", rid, id)
		}
		if key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		if ts.After(last[id]) {
			last[id] = ts
		}
	}
	rows := make([]shopxDeadWeightRow, 0)
	for _, listing := range listings {
		id := shopxListingID(listing)
		ts, sold := last[id]
		if sold && !ts.Before(now.Add(-window)) {
			continue
		}
		row := shopxDeadWeightRow{ListingID: id, Title: shopxStr(listing["title"]), Window: window.String()}
		if n, ok := shopxInt(listing["num_favorers"]); ok {
			row.NumFavorers = &n
		}
		if n, ok := shopxInt(listing["views"]); ok {
			row.Views = &n
		}
		if sold {
			formatted := ts.UTC().Format(time.RFC3339)
			row.LastSaleAt = &formatted
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		var a, b int64
		if rows[i].NumFavorers != nil {
			a = *rows[i].NumFavorers
		}
		if rows[j].NumFavorers != nil {
			b = *rows[j].NumFavorers
		}
		if a != b {
			return a > b
		}
		return rows[i].ListingID < rows[j].ListingID
	})
	return rows, notes
}
