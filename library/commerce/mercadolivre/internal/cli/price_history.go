// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: price history over local snapshots, with deltas.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/cliutil"
	"github.com/spf13/cobra"
)

func newNovelPriceHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "price-history <catalog_id>",
		Short: "Show how a product's price changed across repeated local snapshots, with an optional --since window",
		Long: "List the local price snapshots for one catalog product ordered oldest-first, each " +
			"annotated with its delta versus the first and the previous observation. A single " +
			"observation is reported honestly with no invented drift.",
		Example:     "  mercadolivre-pp-cli price-history MLB51764304 --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || args[0] == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			var since time.Time
			if s := flagSince; s != "" {
				dur, err := cliutil.ParseDurationLoose(s)
				if err != nil {
					return usageErr(fmt.Errorf("--since %q: %w", s, err))
				}
				since = time.Now().Add(-dur)
			}

			st, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			defer st.Close()

			snaps, err := st.PriceSnapshots(args[0], since)
			if err != nil {
				return err
			}

			type pricePoint struct {
				CapturedAt string  `json:"captured_at"`
				Price      float64 `json:"price"`
				Currency   string  `json:"currency"`
				DeltaPrev  float64 `json:"delta_prev"`
				DeltaFirst float64 `json:"delta_first"`
			}
			rows := make([]pricePoint, 0, len(snaps))
			for i, s := range snaps {
				pt := pricePoint{
					CapturedAt: s.CapturedAt.UTC().Format(time.RFC3339),
					Price:      s.Price,
					Currency:   s.Currency,
				}
				if i > 0 {
					pt.DeltaPrev = round2(s.Price - snaps[i-1].Price)
					pt.DeltaFirst = round2(s.Price - snaps[0].Price)
				}
				rows = append(rows, pt)
			}

			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no price snapshots for %s in the requested window\n", args[0])
			} else if len(rows) == 1 {
				fmt.Fprintf(cmd.ErrOrStderr(), "only one snapshot for %s; no price drift to report\n", args[0])
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "Only snapshots within this window (e.g. 30d, 720h, 4w)")
	return cmd
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
