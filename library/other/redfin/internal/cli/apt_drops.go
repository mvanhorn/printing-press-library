// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// dropRow is one ranked drop record.
type dropRow struct {
	URL         string  `json:"url"`
	FirstPrice  int     `json:"first_price"`
	LatestPrice int     `json:"latest_price"`
	DropPercent float64 `json:"drop_percent"`
	DOM         int     `json:"dom,omitempty"`
	FirstSeen   string  `json:"first_seen,omitempty"`
	LatestSeen  string  `json:"latest_seen,omitempty"`
}

func newDropsCmd(flags *rootFlags) *cobra.Command {
	var regionID int64
	var regionType int
	var since string
	var minPct float64
	var domMin int

	cmd := &cobra.Command{
		Use:   "drops",
		Short: "Find listings whose price dropped or that have aged on market.",
		Long: `Reads listing_snapshots within --since for the given region (joined to
the homes table) and computes per-URL first-vs-latest snapshot. Returns
records where the drop is at least --min-pct OR the latest DOM exceeds
--dom-min.`,
		Example:     `  redfin-pp-cli drops --region-id 30772 --region-type 6 --since 14d --min-pct 3 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), []dropRow{}, flags)
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			window := 14 * 24 * time.Hour
			if since != "" {
				if d, perr := parseDurationLoose(since); perr == nil {
					window = d
				}
			}
			cutoff := time.Now().Add(-window)
			q := `SELECT listing_url, observed_at, COALESCE(price,0), COALESCE(dom,0)
			      FROM listing_snapshots
			      WHERE observed_at >= ?
			      ORDER BY listing_url, observed_at ASC`
			rows, qerr := s.DB().QueryContext(cmd.Context(), q, cutoff)
			if qerr != nil {
				return qerr
			}
			defer rows.Close()

			type acc struct {
				firstPrice  int
				latestPrice int
				latestDOM   int
				firstSeen   time.Time
				latestSeen  time.Time
			}
			byURL := map[string]*acc{}
			for rows.Next() {
				var url string
				var observed time.Time
				var price, dom int
				if err := rows.Scan(&url, &observed, &price, &dom); err != nil {
					return err
				}
				a := byURL[url]
				if a == nil {
					a = &acc{firstPrice: price, latestPrice: price, latestDOM: dom, firstSeen: observed, latestSeen: observed}
					byURL[url] = a
					continue
				}
				a.latestPrice = price
				a.latestDOM = dom
				a.latestSeen = observed
			}
			out := []dropRow{}
			for url, a := range byURL {
				if a.firstPrice <= 0 || a.latestPrice <= 0 {
					continue
				}
				pct := 0.0
				if a.firstPrice > 0 {
					pct = (float64(a.firstPrice-a.latestPrice) / float64(a.firstPrice)) * 100
				}
				keepDrop := pct >= minPct && a.latestPrice < a.firstPrice
				keepDOM := domMin > 0 && a.latestDOM >= domMin
				if !keepDrop && !keepDOM {
					continue
				}
				out = append(out, dropRow{
					URL:         url,
					FirstPrice:  a.firstPrice,
					LatestPrice: a.latestPrice,
					DropPercent: pct,
					DOM:         a.latestDOM,
					FirstSeen:   a.firstSeen.UTC().Format(time.RFC3339),
					LatestSeen:  a.latestSeen.UTC().Format(time.RFC3339),
				})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].DropPercent > out[j].DropPercent })
			// Optionally filter by region using the homes table.
			if regionID != 0 {
				keep := map[string]bool{}
				rs, err := listingsFromHomesTable(cmd.Context(), s.DB(), regionID, regionType)
				if err == nil {
					for _, l := range rs {
						keep[l.URL] = true
					}
					filtered := out[:0]
					for _, r := range out {
						if keep[r.URL] {
							filtered = append(filtered, r)
						}
					}
					out = filtered
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "drops region %d: %d match(es) after filter\n", regionID, len(out))
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "drops: %d match(es) across full snapshot history\n", len(out))
			}
			if out == nil {
				out = []dropRow{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Int64Var(&regionID, "region-id", 0, "Optional region filter")
	cmd.Flags().IntVar(&regionType, "region-type", 6, "Region type")
	cmd.Flags().StringVar(&since, "since", "14d", "Window: 14d, 2w, 24h")
	cmd.Flags().Float64Var(&minPct, "min-pct", 3, "Minimum drop percent")
	cmd.Flags().IntVar(&domMin, "dom-min", 0, "Minimum days-on-market (alternative trigger)")
	return cmd
}
