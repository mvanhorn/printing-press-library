// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored transcendence command: client lifetime value.
//
// pp:data-source local
package cli

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type ltvRow struct {
	ClientID   string  `json:"client_id"`
	ClientName string  `json:"client_name"`
	TotalSpend float64 `json:"total_spend"`
	Purchases  int     `json:"purchases"`
	TenureDays int     `json:"tenure_days"`
}

func newNovelLtvCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "ltv",
		Short: "Rank clients by total purchase spend with tenure since signup.",
		Long: `Rank clients by lifetime purchase spend from your locally synced data.

Sums each client's purchase price and joins their signup date for tenure,
ranked highest-spend first. This per-client lifetime total is a multi-table
aggregation no single Sutra endpoint returns.

Run 'sutra-fitness-pp-cli sync' first to populate clients and purchases.

Use this command for per-client lifetime spend ranking. For period revenue totals
by plan type use 'revenue'.`,
		Example:     "  sutra-fitness-pp-cli ltv --limit 25\n  sutra-fitness-pp-cli ltv --json --select client_name,total_spend",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, ready, err := openAnalyticsStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if !ready {
				return emitAnalytics(cmd, flags, []ltvRow{})
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "purchases") {
				hintIfStale(cmd, db, "purchases", flags.maxAge)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT p.client_id,
				       COALESCE(SUM(p.price),0) AS total,
				       COUNT(*) AS n,
				       COALESCE(MAX(cl.first_name),'') AS first_name,
				       COALESCE(MAX(cl.last_name),'') AS last_name,
				       COALESCE(MAX(cl.created_at),'') AS created_at
				FROM purchases p
				LEFT JOIN clients cl ON p.client_id = cl.id
				WHERE p.client_id IS NOT NULL
				GROUP BY p.client_id`)
			if err != nil {
				return fmt.Errorf("querying purchases: %w", err)
			}
			defer rows.Close()

			now := time.Now()
			out := make([]ltvRow, 0)
			for rows.Next() {
				var clientID string
				var total float64
				var n int
				var first, last, created string
				var clientIDNull sql.NullString
				if err := rows.Scan(&clientIDNull, &total, &n, &first, &last, &created); err != nil {
					continue
				}
				clientID = clientIDNull.String
				name := strings.TrimSpace(first + " " + last)
				if name == "" {
					name = clientID
				}
				tenure := -1
				if createdTime, ok := parseLocalTime(created); ok {
					tenure = int(now.Sub(createdTime).Hours() / 24)
				}
				out = append(out, ltvRow{
					ClientID:   clientID,
					ClientName: name,
					TotalSpend: round2(total),
					Purchases:  n,
					TenureDays: tenure,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating purchases: %w", err)
			}
			sort.SliceStable(out, func(i, j int) bool {
				if out[i].TotalSpend != out[j].TotalSpend {
					return out[i].TotalSpend > out[j].TotalSpend
				}
				return out[i].ClientName < out[j].ClientName
			})
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}

			return emitAnalytics(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum clients to return (0 = all)")
	return cmd
}
