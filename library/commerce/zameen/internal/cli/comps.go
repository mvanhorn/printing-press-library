// comps: area-level price research (median price, price-per-Marla, inventory)
// computed from synced listings. Novel feature.
// pp:data-source local
package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

type areaComp struct {
	Area                string `json:"area"`
	City                string `json:"city"`
	Count               int    `json:"count"`
	MedianPrice         int    `json:"median_price"`
	MedianPricePerMarla int    `json:"median_price_per_marla"`
	MinPrice            int    `json:"min_price"`
	MaxPrice            int    `json:"max_price"`
}

func newNovelCompsCmd(flags *rootFlags) *cobra.Command {
	var flagCity, flagArea string
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "comps",
		Short: "Median price, price-per-Marla, and inventory count for an area or society, computed from your synced listings.",
		Long: "Roll up synced listings by area/society into median price, Marla-normalized " +
			"price-per-Marla, and inventory count.\n\nUse this command for area/society-level " +
			"rollups (comps sheets, market overviews). Do NOT use it to rank individual " +
			"below-market listings; use 'deals' for per-listing ranking.\n\n" +
			"Run 'zameen-pp-cli pull ...' first to populate the local store.",
		Example:     "  zameen-pp-cli comps --city Islamabad --area DHA_Defence",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute area comps from the local store")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			listings, err := loadStoredListings(cmd.Context(), dbPath)
			if err != nil {
				if errors.Is(err, errNoMirror) {
					return emitEmptyMirrorHint(cmd, flags, dbPath)
				}
				return err
			}
			listings = filterStoredByArea(listings, flagCity, flagArea)

			groups := map[string][]types.Listing{}
			cityOf := map[string]string{}
			for _, l := range listings {
				key := l.Location
				if key == "" {
					key = "(unspecified)"
				}
				groups[key] = append(groups[key], l)
				if cityOf[key] == "" {
					cityOf[key] = l.City
				}
			}
			comps := make([]areaComp, 0, len(groups))
			for area, ls := range groups {
				prices := make([]int, 0, len(ls))
				ppm := make([]int, 0, len(ls))
				minP, maxP := 0, 0
				for i, l := range ls {
					prices = append(prices, l.Price)
					if p := pricePerMarla(l); p > 0 {
						ppm = append(ppm, int(p))
					}
					if i == 0 || l.Price < minP {
						minP = l.Price
					}
					if l.Price > maxP {
						maxP = l.Price
					}
				}
				comps = append(comps, areaComp{
					Area:                area,
					City:                cityOf[area],
					Count:               len(ls),
					MedianPrice:         medianInt(prices),
					MedianPricePerMarla: medianInt(ppm),
					MinPrice:            minP,
					MaxPrice:            maxP,
				})
			}
			sort.SliceStable(comps, func(i, j int) bool { return comps[i].Count > comps[j].Count })
			if limit > 0 && len(comps) > limit {
				comps = comps[:limit]
			}
			if len(comps) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no listings in the local store match city=%q area=%q; run pull first or widen the query\n", flagCity, flagArea)
			}
			return emitObject(cmd, flags, comps)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "Filter stored listings to this city")
	cmd.Flags().StringVar(&flagArea, "area", "", "Filter stored listings to this area/society (e.g. DHA_Defence)")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum area rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
