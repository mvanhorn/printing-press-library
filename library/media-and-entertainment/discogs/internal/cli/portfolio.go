// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: portfolio value + history from local snapshots.
//
// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/store"

	"github.com/spf13/cobra"
)

type moverRow struct {
	ReleaseID int64   `json:"release_id"`
	Title     string  `json:"title,omitempty"`
	Copies    int     `json:"copies"`
	Current   float64 `json:"current_value"`
	Baseline  float64 `json:"baseline_value"`
	Change    float64 `json:"change"`
}

type portfolioView struct {
	ItemCount      int        `json:"item_count"`
	PricedCount    int        `json:"priced_count"`
	CurrentValue   float64    `json:"current_value"`
	CostBasis      float64    `json:"cost_basis"`
	ProfitLoss     float64    `json:"profit_loss"`
	Window         string     `json:"window,omitempty"`
	BaselineValue  float64    `json:"baseline_value,omitempty"`
	ChangeInWindow float64    `json:"change_in_window,omitempty"`
	Currency       string     `json:"currency,omitempty"`
	TopMovers      []moverRow `json:"top_movers,omitempty"`
	Note           string     `json:"note,omitempty"`
}

func newNovelPortfolioCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var limit int

	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Chart your collection's value over time with per-record contribution to the change and cost-basis P&L.",
		Long: "Values your synced collection from locally recorded price snapshots. Because the Discogs API " +
			"keeps no price history, this reads the snapshots the CLI captured over time (run " +
			"'sync --snapshot collection' periodically). Cost basis defaults to each record's first recorded price.",
		Example:     "  discogs-pp-cli portfolio --since 90d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "local"); err != nil {
				return err
			}
			ctx := cmd.Context()

			var cutoff string
			if flagSince != "" {
				d, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				cutoff = time.Now().UTC().Add(-d).Format("2006-01-02T15:04:05Z07:00")
			}

			st, err := openDiscogsStore(ctx, flags)
			if err != nil {
				return err
			}
			defer st.Close()

			// Distinct collection releases + copy counts + titles (drain-first).
			rows, err := st.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'collection'`)
			if err != nil {
				return fmt.Errorf("reading collection: %w", err)
			}
			type agg struct {
				title  string
				copies int
			}
			releases := map[int64]*agg{}
			itemCount := 0
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					continue
				}
				var it collectionItem
				if json.Unmarshal(data, &it) != nil || it.BasicInfo.ID == 0 {
					continue
				}
				itemCount++
				rid := it.BasicInfo.ID
				if releases[rid] == nil {
					releases[rid] = &agg{title: it.BasicInfo.display()}
				}
				releases[rid].copies++
			}
			_ = rows.Err()
			_ = rows.Close()

			view := portfolioView{ItemCount: itemCount, Currency: ""}
			if flagSince != "" {
				view.Window = flagSince
			}
			if itemCount == 0 {
				view.Note = "no collection synced — run 'sync --resources collection --snapshot collection'"
				return emitPortfolio(cmd, flags, view)
			}

			var movers []moverRow
			for rid, a := range releases {
				cur, hasCur := latestLowestValue(ctx, st, rid)
				if !hasCur {
					continue
				}
				first, _ := earliestLowestValue(ctx, st, rid)
				base := first
				if cutoff != "" {
					if v, ok := valueAtOrBefore(ctx, st, rid, cutoff); ok {
						base = v
					}
				}
				copies := float64(a.copies)
				view.CurrentValue += cur * copies
				view.CostBasis += first * copies
				view.PricedCount++
				if cutoff != "" {
					view.BaselineValue += base * copies
				}
				movers = append(movers, moverRow{
					ReleaseID: rid, Title: a.title, Copies: a.copies,
					Current: cur * copies, Baseline: base * copies, Change: (cur - base) * copies,
				})
			}
			view.ProfitLoss = view.CurrentValue - view.CostBasis
			if cutoff != "" {
				view.ChangeInWindow = view.CurrentValue - view.BaselineValue
			}
			if view.PricedCount == 0 {
				view.Note = "no price snapshots yet — run 'sync --snapshot collection' periodically to build history"
				return emitPortfolio(cmd, flags, view)
			}

			sort.Slice(movers, func(i, j int) bool {
				ai, aj := movers[i].Change, movers[j].Change
				if ai < 0 {
					ai = -ai
				}
				if aj < 0 {
					aj = -aj
				}
				return ai > aj
			})
			if limit <= 0 {
				limit = 10
			}
			if len(movers) > limit {
				movers = movers[:limit]
			}
			view.TopMovers = movers
			return emitPortfolio(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Window for the change view, e.g. 30d, 12w, 90d")
	cmd.Flags().IntVar(&limit, "limit", 10, "How many top movers to show")
	return cmd
}

func emitPortfolio(cmd *cobra.Command, flags *rootFlags, view portfolioView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if view.Note != "" && view.PricedCount == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), view.Note)
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "collection value  %.2f  (cost basis %.2f, P&L %+.2f) over %d priced of %d items\n",
		view.CurrentValue, view.CostBasis, view.ProfitLoss, view.PricedCount, view.ItemCount)
	if view.Window != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "change over %s: %+.2f\n", view.Window, view.ChangeInWindow)
	}
	for _, m := range view.TopMovers {
		fmt.Fprintf(cmd.OutOrStdout(), "  %-40s %+.2f\n", truncate(m.Title, 40), m.Change)
	}
	return nil
}

// latestLowestValue / earliestLowestValue / valueAtOrBefore read the local
// price_snapshots table (all local, no network).
func latestLowestValue(ctx context.Context, st *store.Store, releaseID int64) (float64, bool) {
	return scanLowest(ctx, st, `SELECT lowest_price FROM price_snapshots
		WHERE release_id = ? AND lowest_price IS NOT NULL ORDER BY captured_at DESC LIMIT 1`, releaseID)
}

func earliestLowestValue(ctx context.Context, st *store.Store, releaseID int64) (float64, bool) {
	return scanLowest(ctx, st, `SELECT lowest_price FROM price_snapshots
		WHERE release_id = ? AND lowest_price IS NOT NULL ORDER BY captured_at ASC LIMIT 1`, releaseID)
}

func valueAtOrBefore(ctx context.Context, st *store.Store, releaseID int64, cutoff string) (float64, bool) {
	if v, ok := scanLowest(ctx, st, `SELECT lowest_price FROM price_snapshots
		WHERE release_id = ? AND lowest_price IS NOT NULL AND captured_at <= ? ORDER BY captured_at DESC LIMIT 1`, releaseID, cutoff); ok {
		return v, true
	}
	// No snapshot before the cutoff: fall back to the earliest we have.
	return earliestLowestValue(ctx, st, releaseID)
}

func scanLowest(ctx context.Context, st *store.Store, query string, args ...any) (float64, bool) {
	var v sql.NullFloat64
	if err := st.DB().QueryRowContext(ctx, query, args...).Scan(&v); err != nil || !v.Valid {
		return 0, false
	}
	return v.Float64, true
}
