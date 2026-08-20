// Copyright 2026 Hamza Qazi and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command. Safe to edit.
//
// pp:data-source live

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type priceChange struct {
	ItemID   string  `json:"itemId"`
	Name     string  `json:"name"`
	OldPrice float64 `json:"oldPrice"`
	NewPrice float64 `json:"newPrice"`
	DeltaPct float64 `json:"deltaPct"`
	URL      string  `json:"url"`
}

type sinceOut struct {
	Query           string        `json:"query"`
	Baseline        bool          `json:"baseline"`
	PreviousChecked string        `json:"previousChecked,omitempty"`
	Scanned         int           `json:"scanned"`
	NewListings     []itemBrief   `json:"newListings"`
	PriceDrops      []priceChange `json:"priceDrops"`
	PriceIncreases  []priceChange `json:"priceIncreases"`
	Message         string        `json:"message"`
}

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var maxScan int

	cmd := &cobra.Command{
		Use:         "since <query>",
		Short:       "Diff a saved search against its last local snapshot to show new listings and price moves since you last checked.",
		Long:        "Diff a search against its last local snapshot to show new listings and price moves since you last checked, then record a fresh snapshot.\n\nUse this command to monitor a market over time. For a fresh listing, use 'products' instead. The first run records a baseline; run it again later to see changes.",
		Example:     "  daraz-pp-cli since \"gaming mouse\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search query is required, e.g. since \"gaming mouse\""))
			}
			query := strings.Join(args, " ")
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			items, _, scanned, err := scanSearch(ctx, c, query, "", "", maxScan, 0)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			s, err := openDarazStore(ctx, flags)
			if err != nil {
				return err
			}
			defer s.Close()

			prevTs, prev, err := loadLastSearchSnapshot(ctx, s, query)
			if err != nil {
				return fmt.Errorf("reading previous snapshot: %w", err)
			}
			// Always record the current view + snapshot for next time.
			recordProducts(ctx, s, items)
			recordSearchSnapshot(ctx, s, query, items)

			out := sinceOut{Query: query, Scanned: scanned, NewListings: []itemBrief{}, PriceDrops: []priceChange{}, PriceIncreases: []priceChange{}}
			if prevTs == 0 {
				out.Baseline = true
				out.Message = fmt.Sprintf("Baseline recorded for %q (%d items). Run 'since %q' again later to see what changed.", query, len(items), query)
				return emitDaraz(cmd, flags, out)
			}
			out.PreviousChecked = time.Unix(prevTs, 0).Format("2006-01-02 15:04")
			for _, p := range items {
				old, seen := prev[p.ItemID]
				if !seen {
					out.NewListings = append(out.NewListings, *briefOf(p))
					continue
				}
				np := p.priceF()
				if old.price > 0 && np > 0 && np != old.price {
					delta := math.Round((np-old.price)/old.price*1000) / 10
					ch := priceChange{ItemID: p.ItemID, Name: p.Name, OldPrice: old.price, NewPrice: np, DeltaPct: delta, URL: p.fullURL()}
					if np < old.price {
						out.PriceDrops = append(out.PriceDrops, ch)
					} else {
						out.PriceIncreases = append(out.PriceIncreases, ch)
					}
				}
			}
			sort.SliceStable(out.PriceDrops, func(i, j int) bool { return out.PriceDrops[i].DeltaPct < out.PriceDrops[j].DeltaPct })
			sort.SliceStable(out.PriceIncreases, func(i, j int) bool { return out.PriceIncreases[i].DeltaPct > out.PriceIncreases[j].DeltaPct })
			out.Message = fmt.Sprintf("Since %s: %d new, %d price drops, %d increases.", out.PreviousChecked, len(out.NewListings), len(out.PriceDrops), len(out.PriceIncreases))
			return emitDaraz(cmd, flags, out)
		},
	}
	cmd.Flags().IntVar(&maxScan, "max-scan-pages", 2, "maximum search pages to scan (40 items per page)")
	return cmd
}
