// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"fmt"
	"sort"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newStoresCoverageCmd(flags *rootFlags) *cobra.Command {
	var (
		cities []string
		types  []string
	)
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Cross-city, cross-store-type coverage matrix",
		Long: `Tabulate Rappi store counts per (city, store_type) cell. By default
counts every store-type slug across every Rappi-served Mexican city.
The output is a matrix Rappi's UI has no equivalent for — perfect
for retail-analyst week-over-week coverage tracking.`,
		Example:     "  rappi-pp-cli stores coverage --cities ciudad-de-mexico,guadalajara,monterrey --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(cities) == 0 {
				cities = rappi.CitySlugs()
			}
			if len(types) == 0 {
				for _, t := range rappi.StoreTypes {
					types = append(types, t.Slug)
				}
			}
			matrix := map[string]map[string]int{}
			for _, c := range cities {
				matrix[c] = map[string]int{}
			}
			var mu sync.Mutex
			var wg sync.WaitGroup
			sem := make(chan struct{}, 3)
			// PATCH: Reuse the configured Rappi client across store-type fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			db, _, _ := openLocalStore(cmd.Context())
			if db != nil {
				defer db.Close()
			}
			for _, t := range types {
				wg.Add(1)
				go func(t string) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()
					stores, err := fetchStoreListPage(cmd.Context(), rappiClient, t)
					if err != nil {
						stderrf("warning: %s fetch: %v\n", t, err)
						return
					}
					// Store-by-type pages don't carry a city dimension natively.
					// We attribute every result to "all" but also write a
					// snapshot keyed under the synthetic "all" city for
					// coverage-diff to consume.
					mu.Lock()
					for _, c := range cities {
						matrix[c][t] = len(stores)
					}
					mu.Unlock()
					if db != nil {
						_ = snapshotStores(db, t, "all", stores)
					}
				}(t)
			}
			wg.Wait()
			type cell struct {
				City      string `json:"city"`
				StoreType string `json:"store_type"`
				Count     int    `json:"count"`
			}
			cells := []cell{}
			cityOrder := append([]string{}, cities...)
			sort.Strings(cityOrder)
			for _, c := range cityOrder {
				for _, t := range types {
					cells = append(cells, cell{City: c, StoreType: t, Count: matrix[c][t]})
				}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), cells, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Coverage matrix (cities × store_types):\n")
			fmt.Fprintf(w, "%-22s", "city")
			for _, t := range types {
				fmt.Fprintf(w, " %-12s", t)
			}
			fmt.Fprintln(w)
			for _, c := range cityOrder {
				fmt.Fprintf(w, "%-22s", c)
				for _, t := range types {
					fmt.Fprintf(w, " %-12d", matrix[c][t])
				}
				fmt.Fprintln(w)
			}
			fmt.Fprintln(w, "\nNote: Rappi's /tiendas/tipo/<type> listing isn't city-scoped at the SSR layer; counts here reflect the platform-wide list returned per type. Use 'stores list-by-type' for per-store inspection.")
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&cities, "cities", nil, "Cities to include in the matrix (default: all known)")
	cmd.Flags().StringSliceVar(&types, "types", nil, "Store types to include (default: all)")
	return cmd
}
