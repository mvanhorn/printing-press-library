// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

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
			// Read-only local query — no mutation to suppress under --dry-run;
			// we still run so --dry-run --json emits a real view.
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "stores", []string{"orders"}, flags.maxAge)

			view, err := buildRevenueSnapshot(cmd.Context(), db)
			if err != nil {
				return err
			}
			return emitRevenueSnapshot(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func emitRevenueSnapshot(cmd *cobra.Command, flags *rootFlags, view revenueSnapshotView) error {
	if len(view.Stores) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Stores))
		for _, s := range view.Stores {
			items = append(items, map[string]any{
				"store":             s.StoreName,
				"store_id":          s.StoreID,
				"currency":          s.Currency,
				"thirty_day":        fmt.Sprintf("%.2f", s.ThirtyDayRevenue),
				"lifetime":          fmt.Sprintf("%.2f", s.TotalRevenue),
				"local_orders":      s.LocalOrders,
				"local_net_usd":     fmt.Sprintf("%.2f", s.LocalNetUSD),
				"refunded_usd":      fmt.Sprintf("%.2f", s.LocalRefundedUSD),
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(),
			"\nTotals: 30-day $%.2f  lifetime $%.2f  local-net $%.2f across %d store(s).\n",
			view.TotalThirtyDayUSD, view.TotalLifetimeUSD, view.TotalLocalNetUSD, view.StoreCount)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
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
		gross, refunded, count, hitCap, oerr := sumLocalOrdersForStore(db, row.StoreID)
		if oerr == nil {
			row.LocalOrders = count
			row.LocalGrossUSD = gross
			row.LocalRefundedUSD = refunded
			row.LocalNetUSD = gross - refunded
		}
		if hitCap {
			capNote := fmt.Sprintf("store %s hit the %d-order scan cap; net is a lower bound. Open an issue if a single store routinely exceeds this volume.", row.StoreID, revenueSnapshotOrdersScanCap)
			if view.Note == "" {
				view.Note = capNote
			} else {
				view.Note = view.Note + "; " + capNote
			}
		}
		view.Stores = append(view.Stores, row)
		view.TotalThirtyDayUSD += row.ThirtyDayRevenue
		view.TotalLifetimeUSD += row.TotalRevenue
		view.TotalLocalNetUSD += row.LocalNetUSD
	}
	// Stable sort: lifetime revenue desc, store_id asc tie-break.
	sort.Slice(view.Stores, func(i, j int) bool {
		if view.Stores[i].TotalRevenue != view.Stores[j].TotalRevenue {
			return view.Stores[i].TotalRevenue > view.Stores[j].TotalRevenue
		}
		return view.Stores[i].StoreID < view.Stores[j].StoreID
	})
	view.StoreCount = len(view.Stores)
	if view.StoreCount == 0 && view.Note == "" {
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

// revenueSnapshotOrdersScanCap is the per-store row scan budget. The query
// filters by store_id at SQL level (via json_extract) so this cap applies to
// orders from a single store, not the global orders table. Bump if a store
// genuinely has more than this many orders in the synced window.
const revenueSnapshotOrdersScanCap = 50000

func sumLocalOrdersForStore(db *store.Store, storeID string) (gross, refunded float64, count int, hitCap bool, err error) {
	// Filter by store_id at SQL level so multi-store accounts do not scan O(N
	// stores * cap) rows in Go. The CAST is required because Lemon Squeezy
	// returns store_id as either a JSON number or string depending on the
	// endpoint, and we always compare against a string at the Go boundary.
	rows, err := db.Query(
		`SELECT data FROM resources
		 WHERE resource_type = 'orders'
		   AND CAST(json_extract(data, '$.attributes.store_id') AS TEXT) = ?
		 LIMIT ?`,
		storeID, revenueSnapshotOrdersScanCap,
	)
	if err != nil {
		return 0, 0, 0, false, err
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
				TotalUSD    any `json:"total_usd"`
				Total       any `json:"total"`
				Refunded    any `json:"refunded"`
				RefundedAmt any `json:"refunded_amount_usd"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(data.String), &env); err != nil {
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
	hitCap = count >= revenueSnapshotOrdersScanCap
	return gross, refunded, count, hitCap, nil
}
