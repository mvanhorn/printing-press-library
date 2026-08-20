// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: condition-matched comps for one release.
//
// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type compsHistory struct {
	Count         int      `json:"count"`
	Min           *float64 `json:"min,omitempty"`
	Median        *float64 `json:"median,omitempty"`
	Max           *float64 `json:"max,omitempty"`
	FirstCaptured string   `json:"first_captured,omitempty"`
}

type compsView struct {
	ReleaseID   int64              `json:"release_id"`
	Title       string             `json:"title,omitempty"`
	Suggestions map[string]float64 `json:"price_suggestions,omitempty"`
	Current     *float64           `json:"current_lowest"`
	NumForSale  *int               `json:"num_for_sale,omitempty"`
	Currency    string             `json:"currency,omitempty"`
	History     compsHistory       `json:"history"`
	Note        string             `json:"note,omitempty"`
}

func newNovelCompsCmd(flags *rootFlags) *cobra.Command {
	var currency string

	cmd := &cobra.Command{
		Use:   "comps <release_id>",
		Short: "The condition-by-condition price picture for one release, with local snapshot history the API can't provide.",
		Long: "Shows Discogs' per-condition price suggestions plus the current marketplace lowest and the local " +
			"snapshot history (min/median/max) for one release.\n\nUse this to price or value a single release. " +
			"To compare different pressings of the same album, use 'pressings'.",
		Example:     "  discogs-pp-cli comps 249504 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "release_id=249504"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "live"); err != nil {
				return err
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<release_id> is required"))
			}
			releaseID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("release_id must be a number, got %q", args[0]))
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

			view := compsView{ReleaseID: releaseID}

			// Per-condition price suggestions (needs a seller token; tolerate failure).
			if data, serr := c.Get(ctx, fmt.Sprintf("/marketplace/price_suggestions/%d", releaseID), nil); serr == nil {
				var raw map[string]discogsPriceValue
				if json.Unmarshal(data, &raw) == nil && len(raw) > 0 {
					view.Suggestions = map[string]float64{}
					for cond, pv := range raw {
						if pv.Value != nil {
							view.Suggestions[cond] = *pv.Value
							if view.Currency == "" {
								view.Currency = pv.Currency
							}
						}
					}
				}
			} else {
				view.Note = "price suggestions unavailable (they require a seller token); showing marketplace stats + local history only"
			}

			// Current stats (captures a fresh snapshot too).
			if stats, cerr := captureSnapshot(ctx, c, st, releaseID, currency); cerr == nil {
				if stats.LowestPrice != nil && stats.LowestPrice.Value != nil {
					v := *stats.LowestPrice.Value
					view.Current = &v
					if view.Currency == "" {
						view.Currency = stats.LowestPrice.Currency
					}
				}
				if stats.NumForSale != nil {
					n := *stats.NumForSale
					view.NumForSale = &n
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: marketplace stats unavailable: %v\n", cerr)
			}

			// Title from the local wantlist mirror if synced (wantlist is
			// release-keyed; collection is instance-keyed so it can't be
			// looked up by release_id here).
			if raw, gerr := st.Get("wantlist", strconv.FormatInt(releaseID, 10)); gerr == nil && len(raw) > 0 {
				var w wantItem
				if json.Unmarshal(raw, &w) == nil && w.BasicInfo.Title != "" {
					view.Title = w.BasicInfo.display()
				}
			}

			view.History = compsHistoryFor(ctx, st.DB(), releaseID)
			if view.Current != nil {
				if med, ok := trailingMedian(ctx, st, releaseID); ok {
					m := med
					view.History.Median = &m
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			title := view.Title
			if title == "" {
				title = fmt.Sprintf("release %d", releaseID)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", title)
			if view.Current != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  current lowest: %.2f %s\n", *view.Current, view.Currency)
			}
			for cond, price := range view.Suggestions {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-28s %.2f\n", cond, price)
			}
			if view.History.Count > 0 && view.History.Median != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "  history: %d snapshots, median %.2f\n", view.History.Count, *view.History.Median)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for prices (e.g. USD)")
	return cmd
}

func compsHistoryFor(ctx context.Context, db *sql.DB, releaseID int64) compsHistory {
	h := compsHistory{}
	var first sql.NullString
	_ = db.QueryRowContext(ctx, `SELECT COUNT(*), MIN(captured_at) FROM price_snapshots WHERE release_id = ? AND lowest_price IS NOT NULL`, releaseID).Scan(&h.Count, &first)
	h.FirstCaptured = first.String
	var mn, mx sql.NullFloat64
	_ = db.QueryRowContext(ctx, `SELECT MIN(lowest_price), MAX(lowest_price) FROM price_snapshots WHERE release_id = ? AND lowest_price IS NOT NULL`, releaseID).Scan(&mn, &mx)
	if mn.Valid {
		v := mn.Float64
		h.Min = &v
	}
	if mx.Valid {
		v := mx.Float64
		h.Max = &v
	}
	return h
}
