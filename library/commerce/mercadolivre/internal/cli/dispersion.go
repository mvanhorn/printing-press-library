// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: price dispersion stats over local snapshots + listings.
// pp:data-source local

package cli

import (
	"math"
	"sort"

	"github.com/spf13/cobra"
)

func newNovelDispersionCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dispersion <catalog_id>",
		Short: "Report min/max/median/mean/stddev of prices observed for a catalog product",
		Long: "Compute price dispersion statistics for one catalog product from the local store, " +
			"drawing on every recorded price snapshot and any stored listing rows sharing the catalog id.",
		Example:     "  mercadolivre-pp-cli dispersion MLB51764304 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 || args[0] == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			s, err := openLocalStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			catalogID := args[0]
			prices, err := s.CatalogPrices(catalogID)
			if err != nil {
				return err
			}

			stats := priceStats(catalogID, prices)
			return printJSONFiltered(cmd.OutOrStdout(), stats, flags)
		},
	}
	return cmd
}

// dispersionStats is the emitted stats object. Count==0 signals an honest
// empty result (no local prices for the id) rather than fabricated zeros.
type dispersionStats struct {
	CatalogID string   `json:"catalog_id"`
	Count     int      `json:"count"`
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Median    *float64 `json:"median"`
	Mean      *float64 `json:"mean"`
	Stddev    *float64 `json:"stddev"`
	Note      string   `json:"note,omitempty"`
}

func priceStats(catalogID string, prices []float64) dispersionStats {
	if len(prices) == 0 {
		return dispersionStats{
			CatalogID: catalogID,
			Count:     0,
			Note:      "no local price data for this catalog_id; run 'products get' or 'listings' to populate it",
		}
	}
	sorted := append([]float64(nil), prices...)
	sort.Float64s(sorted)

	minv := sorted[0]
	maxv := sorted[len(sorted)-1]

	sum := 0.0
	for _, p := range sorted {
		sum += p
	}
	mean := sum / float64(len(sorted))

	// Population standard deviation.
	var sq float64
	for _, p := range sorted {
		d := p - mean
		sq += d * d
	}
	stddev := math.Sqrt(sq / float64(len(sorted)))

	var median float64
	n := len(sorted)
	if n%2 == 1 {
		median = sorted[n/2]
	} else {
		median = (sorted[n/2-1] + sorted[n/2]) / 2
	}

	return dispersionStats{
		CatalogID: catalogID,
		Count:     n,
		Min:       &minv,
		Max:       &maxv,
		Median:    &median,
		Mean:      &mean,
		Stddev:    &stddev,
	}
}
