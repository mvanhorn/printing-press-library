// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type priceBand struct {
	Month       string  `json:"month"`
	Median      float64 `json:"median"`
	Min         float64 `json:"min"`
	Max         float64 `json:"max"`
	SampleCount int     `json:"sample_count"`
}

func newDestinationsPriceBandCmd(flags *rootFlags) *cobra.Command {
	var query string
	var year, nights int
	cmd := &cobra.Command{
		Use:         "price-band",
		Short:       "Aggregate local price history by destination and month",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]priceBand, 0))
			}
			if query == "" || year == 0 || nights <= 0 {
				return cmd.Help()
			}
			st, err := openBookingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("destinations price-band: %w", err)
			}
			defer st.Close()
			like := "%" + strings.ToLower(query) + "%"
			rows, err := st.DB().QueryContext(cmd.Context(), `SELECT substr(ph.checkin,1,7), ph.price FROM price_history ph WHERE substr(ph.checkin,1,4)=? AND julianday(ph.checkout)-julianday(ph.checkin)=? AND ph.slug IN (SELECT DISTINCT COALESCE(json_extract(data,'$.slug'), id) FROM resources WHERE resource_type='hotels' AND lower(data) LIKE ?)`, fmt.Sprintf("%04d", year), nights, like)
			if err != nil {
				return fmt.Errorf("destinations price-band: %w", err)
			}
			defer rows.Close()
			byMonth := map[string][]float64{}
			for rows.Next() {
				var month string
				var price float64
				if err := rows.Scan(&month, &price); err != nil {
					return fmt.Errorf("destinations price-band: %w", err)
				}
				byMonth[month] = append(byMonth[month], price)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("destinations price-band: %w", err)
			}
			out := make([]priceBand, 0)
			for month, vals := range byMonth {
				sort.Float64s(vals)
				out = append(out, priceBand{Month: month, Median: medianFloat(append([]float64(nil), vals...)), Min: vals[0], Max: vals[len(vals)-1], SampleCount: len(vals)})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Month < out[j].Month })
			if len(out) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local price_history samples for %q; run price sweeps first\n", query)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "Destination city")
	cmd.Flags().IntVar(&year, "year", 0, "Year")
	cmd.Flags().IntVar(&nights, "nights", 0, "Number of nights")
	return cmd
}
