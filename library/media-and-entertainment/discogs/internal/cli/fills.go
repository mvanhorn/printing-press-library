// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: wantlist limit-order fills. Flagship feature.
//
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/cliutil"

	"github.com/spf13/cobra"
)

type fillRow struct {
	ReleaseID   int64    `json:"release_id"`
	Title       string   `json:"title,omitempty"`
	MaxPrice    float64  `json:"max_price"`
	Currency    string   `json:"currency"`
	Lowest      *float64 `json:"lowest_price"`
	NumForSale  *int     `json:"num_for_sale,omitempty"`
	Filled      bool     `json:"filled"`
	ChangeSince *float64 `json:"change_since_last,omitempty"`
}

type fillsView struct {
	Fills         []fillRow `json:"fills"`
	Watching      []fillRow `json:"watching,omitempty"`
	ScannedLimits int       `json:"scanned_limits"`
	Note          string    `json:"note,omitempty"`
}

func newNovelFillsCmd(flags *rootFlags) *cobra.Command {
	var currency string
	var showAll bool
	var maxScan int

	cmd := &cobra.Command{
		Use:   "fills",
		Short: "Treat each wantlist entry as a standing limit order and surface the ones now selling at or below the max price you set.",
		Long: "Checks the live marketplace lowest asking for every release you've set a limit on " +
			"(see 'limit set') and reports the ones that have reached your price — a fill — with the " +
			"change since the last check. Records a price snapshot each run so history accrues.\n\n" +
			"Use this for wantlist items hitting a limit you set. To find market-wide mispricing with no " +
			"preset limit, use 'undervalued'.",
		Example:     "  discogs-pp-cli fills --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "live"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			st, err := openDiscogsStore(ctx, flags)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := st.DB().QueryContext(ctx,
				`SELECT release_id, max_price, COALESCE(currency,'') FROM wantlist_limits ORDER BY set_at DESC`)
			if err != nil {
				return fmt.Errorf("reading limits: %w", err)
			}
			type lim struct {
				id   int64
				max  float64
				curr string
			}
			var limits []lim
			for rows.Next() {
				var l lim
				if err := rows.Scan(&l.id, &l.max, &l.curr); err != nil {
					continue
				}
				limits = append(limits, l)
			}
			_ = rows.Err()
			_ = rows.Close()

			view := fillsView{Fills: make([]fillRow, 0), Watching: make([]fillRow, 0)}
			if len(limits) == 0 {
				view.Note = "no limits set — use 'limit set <release_id> <max_price>' to start"
				return emitFills(cmd, flags, view)
			}

			scanMax := len(limits)
			if maxScan > 0 && maxScan < scanMax {
				scanMax = maxScan
			}
			if cliutil.IsDogfoodEnv() && scanMax > 3 {
				scanMax = 3
			}

			for i := 0; i < scanMax; i++ {
				l := limits[i]
				cur := currency
				if cur == "" {
					cur = l.curr
				}
				prior, hasPrior := priorLowest(ctx, st, l.id)
				stats, cerr := captureSnapshot(ctx, c, st, l.id, cur)
				view.ScannedLimits++
				if cerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not check release %d: %v\n", l.id, cerr)
					continue
				}
				row := fillRow{ReleaseID: l.id, MaxPrice: l.max, Currency: cur}
				if raw, gerr := st.Get("wantlist", strconv.FormatInt(l.id, 10)); gerr == nil && len(raw) > 0 {
					var w wantItem
					if json.Unmarshal(raw, &w) == nil {
						row.Title = w.BasicInfo.display()
					}
				}
				if stats.NumForSale != nil {
					n := *stats.NumForSale
					row.NumForSale = &n
				}
				if stats.LowestPrice != nil && stats.LowestPrice.Value != nil {
					low := *stats.LowestPrice.Value
					row.Lowest = &low
					if hasPrior {
						d := low - prior
						row.ChangeSince = &d
					}
					if low <= l.max {
						row.Filled = true
						view.Fills = append(view.Fills, row)
					} else if showAll {
						view.Watching = append(view.Watching, row)
					}
				} else if showAll {
					view.Watching = append(view.Watching, row)
				}
			}
			if len(view.Fills) == 0 && view.Note == "" {
				view.Note = fmt.Sprintf("no fills right now across %d limit(s); nothing at or below your price", view.ScannedLimits)
			}
			return emitFills(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for marketplace lowest-price checks (e.g. USD)")
	cmd.Flags().BoolVar(&showAll, "all", false, "Include limits that have not filled (watching) in the output")
	cmd.Flags().IntVar(&maxScan, "max", 0, "Maximum number of limits to check (0 = all)")
	return cmd
}

func emitFills(cmd *cobra.Command, flags *rootFlags, view fillsView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	for _, f := range view.Fills {
		low := "—"
		if f.Lowest != nil {
			low = fmt.Sprintf("%.2f", *f.Lowest)
		}
		title := f.Title
		if title == "" {
			title = fmt.Sprintf("release %d", f.ReleaseID)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "FILL  %s  lowest %s %s (<= %.2f)\n", title, low, f.Currency, f.MaxPrice)
	}
	if view.Note != "" {
		fmt.Fprintln(cmd.OutOrStdout(), view.Note)
	}
	return nil
}
