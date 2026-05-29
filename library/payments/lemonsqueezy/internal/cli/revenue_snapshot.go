// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/store"
	"github.com/spf13/cobra"
)

type revenueStoreRow struct {
	StoreID          string  `json:"store_id"`
	StoreName        string  `json:"store_name"`
	Currency         string  `json:"currency"`
	ThirtyDayRevenue float64 `json:"thirty_day_revenue"`
	ThirtyDaySales   int     `json:"thirty_day_sales"`
	TotalRevenue     float64 `json:"total_revenue"`
	TotalSales       int     `json:"total_sales"`
	LocalOrders      int     `json:"local_orders"`
	LocalGrossUSD    float64 `json:"local_gross_usd"`
	LocalRefundedUSD float64 `json:"local_refunded_usd"`
	LocalNetUSD      float64 `json:"local_net_usd"`
}

type revenueSnapshotView struct {
	Stores            []revenueStoreRow `json:"stores"`
	TotalThirtyDayUSD float64           `json:"total_thirty_day_usd_estimate"`
	TotalLifetimeUSD  float64           `json:"total_lifetime_usd_estimate"`
	TotalLocalNetUSD  float64           `json:"total_local_net_usd"`
	StoreCount        int               `json:"store_count"`
	Note              string            `json:"note,omitempty"`
}

func newNovelRevenueSnapshotCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "revenue-snapshot",
		Short: "Point-in-time revenue rollup: 30-day + lifetime store counters with refund-adjusted local-orders net",
		Long: `Point-in-time revenue rollup combining Lemon Squeezy's denormalized 30-day and
lifetime revenue/sales counters from the local 'stores' mirror with refund-adjusted
net revenue computed from synced 'orders'.

Use this command for a one-number snapshot of how the store is doing right now.
Do NOT use this for week-over-week MRR movement; use 'mrr-trend' instead.

Data source: local. Run 'sync --resources stores,orders' first.`,
		Example: "  lemonsqueezy-pp-cli sync --resources stores,orders\n  lemonsqueezy-pp-cli revenue-snapshot --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "stores") {
				hintIfStale(cmd, db, "stores", flags.maxAge)
			}

			view, err := buildRevenueSnapshot(cmd.Context(), db)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func buildRevenueSnapshot(_ context.Context, db *store.Store) (revenueSnapshotView, error) {
	view := revenueSnapshotView{Stores: []revenueStoreRow{}}

	storeRows, err := db.List("stores", 1000)
	if err != nil {
		return view, fmt.Errorf("querying stores: %w", err)
	}

	for _, raw := range storeRows {
		row, perr := parseStoreRevenueRow(raw)
		if perr != nil {
			continue
		}
		gross, refunded, count, oerr := sumLocalOrdersForStore(db, row.StoreID)
		if oerr == nil {
			row.LocalOrders = count
			row.LocalGrossUSD = gross
			row.LocalRefundedUSD = refunded
			row.LocalNetUSD = gross - refunded
		}
		view.Stores = append(view.Stores, row)
		view.TotalThirtyDayUSD += row.ThirtyDayRevenue
		view.TotalLifetimeUSD += row.TotalRevenue
		view.TotalLocalNetUSD += row.LocalNetUSD
	}
	view.StoreCount = len(view.Stores)
	if view.StoreCount == 0 {
		view.Note = "no stores in local mirror; run 'lemonsqueezy-pp-cli sync --resources stores' first"
	}
	return view, nil
}

func parseStoreRevenueRow(raw json.RawMessage) (revenueStoreRow, error) {
	var envelope struct {
		ID         string `json:"id"`
		Attributes struct {
			Name             string `json:"name"`
			Currency         string `json:"currency"`
			ThirtyDayRevenue any    `json:"thirty_day_revenue"`
			ThirtyDaySales   any    `json:"thirty_day_sales"`
			TotalRevenue     any    `json:"total_revenue"`
			TotalSales       any    `json:"total_sales"`
		} `json:"attributes"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return revenueStoreRow{}, err
	}
	return revenueStoreRow{
		StoreID:          envelope.ID,
		StoreName:        envelope.Attributes.Name,
		Currency:         envelope.Attributes.Currency,
		ThirtyDayRevenue: toFloatLS(envelope.Attributes.ThirtyDayRevenue) / 100.0,
		ThirtyDaySales:   int(toFloatLS(envelope.Attributes.ThirtyDaySales)),
		TotalRevenue:     toFloatLS(envelope.Attributes.TotalRevenue) / 100.0,
		TotalSales:       int(toFloatLS(envelope.Attributes.TotalSales)),
	}, nil
}

func sumLocalOrdersForStore(db *store.Store, storeID string) (gross, refunded float64, count int, err error) {
	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'orders' LIMIT 5000`,
	)
	if err != nil {
		return 0, 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		var env struct {
			Attributes struct {
				StoreID     any `json:"store_id"`
				TotalUSD    any `json:"total_usd"`
				Total       any `json:"total"`
				Refunded    any `json:"refunded"`
				RefundedAmt any `json:"refunded_amount_usd"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(data.String), &env); err != nil {
			continue
		}
		oStoreID := toStringLS(env.Attributes.StoreID)
		if storeID != "" && oStoreID != "" && oStoreID != storeID {
			continue
		}
		amount := toFloatLS(env.Attributes.TotalUSD)
		if amount == 0 {
			amount = toFloatLS(env.Attributes.Total)
		}
		gross += amount / 100.0
		ref := toFloatLS(env.Attributes.RefundedAmt)
		if ref == 0 && toBoolLS(env.Attributes.Refunded) {
			ref = amount
		}
		refunded += ref / 100.0
		count++
	}
	return gross, refunded, count, nil
}
