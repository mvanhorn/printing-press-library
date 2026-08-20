// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: fee-aware sell router.
//
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/discogs/internal/cliutil"

	"github.com/spf13/cobra"
)

type sellRow struct {
	ReleaseID   int64    `json:"release_id"`
	Title       string   `json:"title,omitempty"`
	Price       float64  `json:"price"`
	Fee         *float64 `json:"discogs_fee,omitempty"`
	NetProceeds *float64 `json:"net_proceeds,omitempty"`
	NumForSale  *int     `json:"num_for_sale,omitempty"`
	Currency    string   `json:"currency,omitempty"`
}

type fetchFailure struct {
	ReleaseID int64  `json:"release_id"`
	Error     string `json:"error"`
}

type sellPlanView struct {
	Ranked        []sellRow      `json:"ranked"`
	Source        string         `json:"source"`
	Scanned       int            `json:"scanned"`
	FetchFailures []fetchFailure `json:"fetch_failures"`
	FeeFailures   int            `json:"fee_failures,omitempty"`
	Note          string         `json:"note,omitempty"`
}

func newNovelSellPlanCmd(flags *rootFlags) *cobra.Command {
	var source string
	var currency string
	var maxScan int
	var limit int

	cmd := &cobra.Command{
		Use:   "sell-plan",
		Short: "Rank your inventory or collection by net-after-fee proceeds and liquidity so you know what to sell now.",
		Long: "For each item in your inventory (or collection) fetches the marketplace lowest price and the Discogs " +
			"fee, then ranks by net-after-fee proceeds with the number for sale as a liquidity signal.\n\n" +
			"Use this to decide what to list this week. To price a single release, use 'comps'.",
		Example:     "  discogs-pp-cli sell-plan --source collection --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := checkDataSource(flags, "live"); err != nil {
				return err
			}
			if source != "inventory" && source != "collection" {
				return usageErr(fmt.Errorf("--source must be inventory or collection, got %q", source))
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

			type target struct {
				id    int64
				title string
			}
			var targets []target
			rows, err := st.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = ?`, source)
			if err != nil {
				return fmt.Errorf("reading %s: %w", source, err)
			}
			seen := map[int64]bool{}
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					continue
				}
				rid, title := releaseIDAndTitle(source, data)
				if rid == 0 || seen[rid] {
					continue
				}
				seen[rid] = true
				targets = append(targets, target{id: rid, title: title})
			}
			_ = rows.Err()
			_ = rows.Close()

			view := sellPlanView{Ranked: make([]sellRow, 0), Source: source, FetchFailures: make([]fetchFailure, 0)}
			if len(targets) == 0 {
				view.Note = fmt.Sprintf("no %s synced — run 'sync --resources %s'", source, source)
				return emitSellPlan(cmd, flags, view)
			}

			scanMax := len(targets)
			if maxScan > 0 && maxScan < scanMax {
				scanMax = maxScan
			}
			if cliutil.IsDogfoodEnv() && scanMax > 3 {
				scanMax = 3
			}

			for i := 0; i < scanMax; i++ {
				t := targets[i]
				view.Scanned++
				stats, serr := fetchStats(ctx, c, t.id, currency)
				if serr != nil {
					view.FetchFailures = append(view.FetchFailures, fetchFailure{ReleaseID: t.id, Error: serr.Error()})
					continue
				}
				row := sellRow{ReleaseID: t.id, Title: t.title, Currency: currency}
				if stats.NumForSale != nil {
					n := *stats.NumForSale
					row.NumForSale = &n
				}
				if stats.LowestPrice == nil || stats.LowestPrice.Value == nil {
					// Nothing for sale => no market price to plan against; skip from ranking.
					continue
				}
				price := *stats.LowestPrice.Value
				row.Price = price
				if stats.LowestPrice.Currency != "" {
					row.Currency = stats.LowestPrice.Currency
				}
				if fee, ferr := fetchFee(ctx, c, price, row.Currency); ferr == nil {
					row.Fee = &fee
					net := price - fee
					row.NetProceeds = &net
				} else {
					view.FeeFailures++
				}
				view.Ranked = append(view.Ranked, row)
			}

			// Rank by net proceeds (fallback to price), scarcity as tie-break.
			sort.Slice(view.Ranked, func(i, j int) bool {
				ni, nj := view.Ranked[i].Price, view.Ranked[j].Price
				if view.Ranked[i].NetProceeds != nil {
					ni = *view.Ranked[i].NetProceeds
				}
				if view.Ranked[j].NetProceeds != nil {
					nj = *view.Ranked[j].NetProceeds
				}
				if ni != nj {
					return ni > nj
				}
				return numForSaleVal(view.Ranked[i]) < numForSaleVal(view.Ranked[j])
			})
			if limit > 0 && len(view.Ranked) > limit {
				view.Ranked = view.Ranked[:limit]
			}
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d fetches failed; ranking computed over the remaining %d\n",
					len(view.FetchFailures), view.Scanned, view.Scanned-len(view.FetchFailures))
			}
			if view.FeeFailures > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d item(s) had no fee estimate and are ranked by gross price\n", view.FeeFailures)
			}
			return emitSellPlan(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&source, "source", "inventory", "What to plan from: inventory or collection")
	cmd.Flags().StringVar(&currency, "currency", "", "Currency for prices/fees (e.g. USD)")
	cmd.Flags().IntVar(&maxScan, "max", 0, "Maximum items to scan (0 = all)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum ranked results to return (0 = all)")
	return cmd
}

func numForSaleVal(r sellRow) int {
	if r.NumForSale != nil {
		return *r.NumForSale
	}
	return 1 << 30
}

// releaseIDAndTitle extracts a release id + display title from a stored
// inventory listing or collection item.
func releaseIDAndTitle(source string, data []byte) (int64, string) {
	if source == "collection" {
		var it collectionItem
		if json.Unmarshal(data, &it) == nil && it.BasicInfo.ID != 0 {
			return it.BasicInfo.ID, it.BasicInfo.display()
		}
		return 0, ""
	}
	// inventory listing
	var l struct {
		Release struct {
			ID          int64  `json:"id"`
			Description string `json:"description"`
		} `json:"release"`
	}
	if json.Unmarshal(data, &l) == nil && l.Release.ID != 0 {
		return l.Release.ID, l.Release.Description
	}
	return 0, ""
}

// fetchFee calls GET /marketplace/fee/{price}[/{currency}] and returns the fee.
func fetchFee(ctx context.Context, c *client.Client, price float64, currency string) (float64, error) {
	path := fmt.Sprintf("/marketplace/fee/%s", strconv.FormatFloat(price, 'f', 2, 64))
	if currency != "" {
		path = fmt.Sprintf("%s/%s", path, currency)
	}
	data, err := c.Get(ctx, path, nil)
	if err != nil {
		return 0, err
	}
	var pv discogsPriceValue
	if err := json.Unmarshal(data, &pv); err != nil || pv.Value == nil {
		return 0, fmt.Errorf("parsing fee response")
	}
	return *pv.Value, nil
}

func emitSellPlan(cmd *cobra.Command, flags *rootFlags, view sellPlanView) error {
	if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), view, flags)
	}
	if len(view.Ranked) == 0 {
		if view.Note != "" {
			fmt.Fprintln(cmd.OutOrStdout(), view.Note)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "nothing to rank (no market prices found)")
		}
		return nil
	}
	for _, r := range view.Ranked {
		net := "—"
		if r.NetProceeds != nil {
			net = fmt.Sprintf("%.2f", *r.NetProceeds)
		}
		title := r.Title
		if title == "" {
			title = fmt.Sprintf("release %d", r.ReleaseID)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%-40s  price %.2f  net %s %s\n", truncate(title, 40), r.Price, net, r.Currency)
	}
	return nil
}
