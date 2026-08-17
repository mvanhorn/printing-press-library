// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: pressing value ranker for a master release.
//
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/cliutil"

	"github.com/spf13/cobra"
)

type pressingRow struct {
	ReleaseID  int64    `json:"release_id"`
	Title      string   `json:"title,omitempty"`
	Format     string   `json:"format,omitempty"`
	Country    string   `json:"country,omitempty"`
	Year       string   `json:"year,omitempty"`
	Lowest     *float64 `json:"lowest_price"`
	NumForSale *int     `json:"num_for_sale,omitempty"`
}

type pressingsView struct {
	MasterID      int64          `json:"master_id"`
	Pressings     []pressingRow  `json:"pressings"`
	Scanned       int            `json:"scanned"`
	FetchFailures []fetchFailure `json:"fetch_failures"`
	Note          string         `json:"note,omitempty"`
}

func newNovelPressingsCmd(flags *rootFlags) *cobra.Command {
	var currency string
	var maxScan int
	var limit int

	cmd := &cobra.Command{
		Use:   "pressings <master_id>",
		Short: "Rank all versions/pressings of a master release by value and liquidity so you know which pressing to buy, want, or sell.",
		Long: "Expands a master release into its versions (pressings) and fetches the marketplace lowest price and " +
			"number for sale for each, ranked by value.\n\nUse this to choose among pressings of an album. For the " +
			"condition breakdown of one specific pressing, use 'comps'.",
		Example:     "  discogs-pp-cli pressings 96559 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "master_id=96559"},
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
				return usageErr(fmt.Errorf("<master_id> is required"))
			}
			masterID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("master_id must be a number, got %q", args[0]))
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

			pages := 2
			if cliutil.IsDogfoodEnv() {
				pages = 1
			}
			versions, err := syncPaginate(ctx, c, fmt.Sprintf("/masters/%d/versions", masterID), "versions", pages)
			if err != nil {
				return fmt.Errorf("fetching versions for master %d: %w", masterID, err)
			}
			view := pressingsView{MasterID: masterID, Pressings: make([]pressingRow, 0), FetchFailures: make([]fetchFailure, 0)}
			if len(versions) == 0 {
				view.Note = "no versions found for that master id"
				return emitPressings(cmd, flags, view)
			}

			type ver struct {
				id      int64
				title   string
				format  string
				country string
				year    string
			}
			var vers []ver
			for _, raw := range versions {
				var v struct {
					ID       int64  `json:"id"`
					Title    string `json:"title"`
					Released string `json:"released"`
					Country  string `json:"country"`
					Format   string `json:"format"`
				}
				if json.Unmarshal(raw, &v) != nil || v.ID == 0 {
					continue
				}
				vers = append(vers, ver{id: v.ID, title: v.Title, format: v.Format, country: v.Country, year: v.Released})
			}

			scanMax := len(vers)
			if maxScan > 0 && maxScan < scanMax {
				scanMax = maxScan
			}
			if cliutil.IsDogfoodEnv() && scanMax > 3 {
				scanMax = 3
			}
			for i := 0; i < scanMax; i++ {
				v := vers[i]
				view.Scanned++
				stats, serr := captureSnapshot(ctx, c, st, v.id, currency)
				if serr != nil {
					view.FetchFailures = append(view.FetchFailures, fetchFailure{ReleaseID: v.id, Error: serr.Error()})
					continue
				}
				row := pressingRow{ReleaseID: v.id, Title: v.title, Format: v.format, Country: v.country, Year: v.year}
				if stats.NumForSale != nil {
					n := *stats.NumForSale
					row.NumForSale = &n
				}
				if stats.LowestPrice != nil && stats.LowestPrice.Value != nil {
					low := *stats.LowestPrice.Value
					row.Lowest = &low
				}
				view.Pressings = append(view.Pressings, row)
			}

			// Rank by value (highest lowest-price first); priced ahead of unpriced.
			sort.Slice(view.Pressings, func(i, j int) bool {
				li, lj := view.Pressings[i].Lowest, view.Pressings[j].Lowest
				if li == nil && lj == nil {
					return false
				}
				if li == nil {
					return false
				}
				if lj == nil {
					return true
				}
				return *li > *lj
			})
			if limit > 0 && len(view.Pressings) > limit {
				view.Pressings = view.Pressings[:limit]
			}
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d version fetches failed; ranking over the remaining %d\n",
					len(view.FetchFailures), view.Scanned, view.Scanned-len(view.FetchFailures))
			}
			return emitPressings(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for prices (e.g. USD)")
	cmd.Flags().IntVar(&maxScan, "max", 0, "Maximum versions to price-check (0 = all fetched)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum ranked results to return (0 = all)")
	return cmd
}

func emitPressings(cmd *cobra.Command, flags *rootFlags, view pressingsView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if len(view.Pressings) == 0 {
		if view.Note != "" {
			fmt.Fprintln(cmd.OutOrStdout(), view.Note)
		}
		return nil
	}
	for _, p := range view.Pressings {
		low := "—"
		if p.Lowest != nil {
			low = fmt.Sprintf("%.2f", *p.Lowest)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-36s %-10s %-4s  lowest %s\n", truncate(p.Title, 36), truncate(p.Format, 10), p.Country, low)
	}
	return nil
}
