// Copyright 2026 adam-birddog and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored transcendence command: revenue by type with prior-period comparison.
//
// pp:data-source local
package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type revenueRow struct {
	Key          string   `json:"key"`
	Revenue      float64  `json:"revenue"`
	Purchases    int      `json:"purchases"`
	PriorRevenue *float64 `json:"prior_revenue,omitempty"`
	Delta        *float64 `json:"delta,omitempty"`
	DeltaPct     *float64 `json:"delta_pct,omitempty"`
}

type revenueView struct {
	GroupBy      string       `json:"group_by"`
	WindowStart  string       `json:"window_start"`
	WindowEnd    string       `json:"window_end"`
	ComparePrior bool         `json:"compare_prior"`
	TotalRevenue float64      `json:"total_revenue"`
	PriorTotal   *float64     `json:"prior_total,omitempty"`
	Rows         []revenueRow `json:"rows"`
}

func newNovelRevenueCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var groupBy string
	var comparePrior bool
	var start, end string

	cmd := &cobra.Command{
		Use:   "revenue",
		Short: "Sum purchase revenue by type for a window and show the delta versus the prior equal window.",
		Long: `Sum purchase revenue from synced purchases, with prior-period comparison.

Totals purchase price over a window (default the last 30 days) grouped by
membership type or plan name. With --compare-prior, the prior equal-length
window is computed and per-group and total deltas are reported — the
prior-period comparison the canned vendor reports omit.

Run 'sutra-fitness-pp-cli sync' first to populate purchases.

Use this command for period revenue totals by plan type. For per-client lifetime
spend ranking use 'ltv'.`,
		Example:     "  sutra-fitness-pp-cli revenue --group-by type --compare-prior\n  sutra-fitness-pp-cli revenue --start 2026-05-01 --end 2026-06-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			groupBy = strings.ToLower(strings.TrimSpace(groupBy))
			if groupBy == "" {
				groupBy = "type"
			}
			var keyCol string
			switch groupBy {
			case "type":
				keyCol = "COALESCE(NULLIF(type,''),'(untyped)')"
			case "name":
				keyCol = "COALESCE(NULLIF(name,''),'(unnamed)')"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--group-by must be one of: type, name (purchases carry no location)"))
			}

			// Resolve the window. Default: last 30 days ending now.
			now := time.Now()
			endTime := now
			if end != "" {
				if t, ok := parseLocalTime(end); ok {
					endTime = t
				} else {
					return usageErr(fmt.Errorf("invalid --end %q (use ISO 8601)", end))
				}
			}
			startTime := endTime.AddDate(0, 0, -30)
			if start != "" {
				if t, ok := parseLocalTime(start); ok {
					startTime = t
				} else {
					return usageErr(fmt.Errorf("invalid --start %q (use ISO 8601)", start))
				}
			}
			if !startTime.Before(endTime) {
				return usageErr(fmt.Errorf("--start must be before --end"))
			}

			db, ready, err := openAnalyticsStore(cmd.Context(), cmd, dbPath)
			if err != nil {
				return err
			}
			if !ready {
				return emitAnalytics(cmd, flags, revenueView{GroupBy: groupBy, Rows: []revenueRow{}})
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "purchases") {
				hintIfStale(cmd, db, "purchases", flags.maxAge)
			}

			sumByGroup := func(winStart, winEnd time.Time) (map[string]float64, map[string]int, float64, error) {
				q := fmt.Sprintf(`
					SELECT %s AS grp, COALESCE(SUM(price),0) AS revenue, COUNT(*) AS n
					FROM purchases
					WHERE start_date >= ? AND start_date < ?
					GROUP BY grp`, keyCol)
				rows, err := db.DB().QueryContext(cmd.Context(), q,
					winStart.Format(time.RFC3339), winEnd.Format(time.RFC3339))
				if err != nil {
					return nil, nil, 0, err
				}
				defer rows.Close()
				rev := map[string]float64{}
				cnt := map[string]int{}
				total := 0.0
				for rows.Next() {
					var key string
					var revenue float64
					var n int
					if err := rows.Scan(&key, &revenue, &n); err != nil {
						continue
					}
					rev[key] = round2(revenue)
					cnt[key] = n
					total += revenue
				}
				return rev, cnt, round2(total), rows.Err()
			}

			curRev, curCnt, curTotal, err := sumByGroup(startTime, endTime)
			if err != nil {
				return fmt.Errorf("querying purchases: %w", err)
			}

			view := revenueView{
				GroupBy:      groupBy,
				WindowStart:  startTime.Format(time.RFC3339),
				WindowEnd:    endTime.Format(time.RFC3339),
				ComparePrior: comparePrior,
				TotalRevenue: curTotal,
				Rows:         []revenueRow{},
			}

			var priorRev map[string]float64
			if comparePrior {
				priorStart := startTime.Add(-endTime.Sub(startTime))
				pr, _, priorTotal, err := sumByGroup(priorStart, startTime)
				if err != nil {
					return fmt.Errorf("querying prior period: %w", err)
				}
				priorRev = pr
				view.PriorTotal = &priorTotal
			}

			// Union of keys across both windows.
			keys := map[string]bool{}
			for k := range curRev {
				keys[k] = true
			}
			for k := range priorRev {
				keys[k] = true
			}
			for k := range keys {
				row := revenueRow{Key: k, Revenue: curRev[k], Purchases: curCnt[k]}
				if comparePrior {
					prior := priorRev[k]
					delta := round2(curRev[k] - prior)
					row.PriorRevenue = &prior
					row.Delta = &delta
					if prior != 0 {
						dp := round2((curRev[k] - prior) / prior * 100)
						row.DeltaPct = &dp
					}
				}
				view.Rows = append(view.Rows, row)
			}
			sort.SliceStable(view.Rows, func(i, j int) bool {
				if view.Rows[i].Revenue != view.Rows[j].Revenue {
					return view.Rows[i].Revenue > view.Rows[j].Revenue
				}
				return view.Rows[i].Key < view.Rows[j].Key
			})

			return emitAnalytics(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&groupBy, "group-by", "type", "Group by: type or name")
	cmd.Flags().BoolVar(&comparePrior, "compare-prior", false, "Compare against the prior equal-length window")
	cmd.Flags().StringVar(&start, "start", "", "Window start (ISO 8601; default 30 days before end)")
	cmd.Flags().StringVar(&end, "end", "", "Window end (ISO 8601; default now)")
	return cmd
}
