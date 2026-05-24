// Copyright 2026 bobeglz. Licensed under Apache-2.0. See LICENSE.
// pp:client-call — real HTTP via fetchStoreListPage -> rappi.Client.FetchHTML.
package cli

import (
	"context"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/rappi/internal/source/rappi"

	"github.com/spf13/cobra"
)

func newStoresAdjacencyCmd(flags *rootFlags) *cobra.Command {
	var (
		typeA       string
		typeB       string
		within      float64
		city        string
		fetchDetail bool
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "adjacency",
		Short: "Stores of type A within a Haversine radius of stores of type B",
		Long: `Cross-store-type proximity query: find every store of --type A
	within --within-km kilometers of any store of --of-type B. Useful
	for concierge-style "one-stop trip" planning (e.g., a pharmacy
	within 1km of a supermarket).

	Rappi's SSR pages return store position+name+url without geo
	coordinates, so this command needs each store's detail page
	fetched to obtain lat/lng. Set --fetch-detail to fetch those detail
	pages before applying --within-km.`,
		Example:     "  rappi-pp-cli stores adjacency --type farmatodo --of-type market --within-km 1 --city ciudad-de-mexico --fetch-detail --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if typeA == "" || typeB == "" {
				if !flags.dryRun {
					return cmd.Help()
				}
			}
			if dryRunOK(flags) {
				return nil
			}
			if !fetchDetail {
				// PATCH: Require detail-page coordinates before running proximity filters.
				return fmt.Errorf("--fetch-detail is required because /tiendas/tipo pages do not include store coordinates")
			}
			// PATCH: Reuse the configured Rappi client across list and detail fetches.
			rappiClient := newRappiHTMLFetcher(flags)
			a, err := fetchStoreListPage(cmd.Context(), rappiClient, typeA)
			if err != nil {
				return err
			}
			b, err := fetchStoreListPage(cmd.Context(), rappiClient, typeB)
			if err != nil {
				return err
			}
			a = fetchStoreDetailsForAdjacency(cmd.Context(), rappiClient, a, typeA, city)
			b = fetchStoreDetailsForAdjacency(cmd.Context(), rappiClient, b, typeB, city)
			type adj struct {
				StoreA     string  `json:"store_a"`
				StoreB     string  `json:"store_b"`
				URLA       string  `json:"url_a"`
				URLB       string  `json:"url_b"`
				DistanceKm float64 `json:"distance_km"`
			}
			out := []adj{}
			for _, sa := range a {
				if !storeHasCoordinates(sa) {
					continue
				}
				for _, sb := range b {
					if sa.URL == sb.URL {
						continue
					}
					if !storeHasCoordinates(sb) {
						continue
					}
					d := haversineKm(sa.Latitude, sa.Longitude, sb.Latitude, sb.Longitude)
					if d > within {
						continue
					}
					out = append(out, adj{
						StoreA: sa.Name, StoreB: sb.Name,
						URLA: sa.URL, URLB: sb.URL,
						DistanceKm: d,
					})
				}
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].DistanceKm < out[j].DistanceKm })
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return emitNovelJSON(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Adjacent stores (%s within %.2fkm of %s):\n", typeA, within, typeB)
			for _, p := range out {
				fmt.Fprintf(w, "  %5.2fkm  %s ↔ %s\n", p.DistanceKm, p.StoreA, p.StoreB)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&typeA, "type", "farmatodo", "Primary store type (e.g. farmatodo, market)")
	cmd.Flags().StringVar(&typeB, "of-type", "market", "Reference store type to be adjacent to")
	cmd.Flags().Float64Var(&within, "within-km", 1.0, "Maximum Haversine distance in kilometers")
	cmd.Flags().StringVar(&city, "city", "ciudad-de-mexico", "City slug for detail-page tagging")
	cmd.Flags().BoolVar(&fetchDetail, "fetch-detail", false, "Fetch each store's detail page to read geo coordinates")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max pairs to return")
	return cmd
}

func fetchStoreDetailsForAdjacency(ctx context.Context, client rappiHTMLFetcher, stores []rappi.Store, storeType, city string) []rappi.Store {
	out := make([]rappi.Store, 0, len(stores))
	for _, s := range stores {
		idSlug := idSlugFromURL(s.URL)
		if idSlug == "" {
			continue
		}
		detail, err := fetchStoreDetail(ctx, client, idSlug, storeType, city)
		if err != nil {
			stderrf("warning: detail fetch failed for %s: %v\n", s.URL, err)
			continue
		}
		if detail.Name == "" {
			detail.Name = s.Name
		}
		if detail.URL == "" {
			detail.URL = s.URL
		}
		if detail.ID == "" {
			detail.ID = s.ID
		}
		out = append(out, *detail)
	}
	return out
}

func storeHasCoordinates(s rappi.Store) bool {
	return s.Latitude != 0 || s.Longitude != 0
}
