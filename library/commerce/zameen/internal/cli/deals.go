// deals: rank synced listings by how far under their area's price-per-Marla
// they sit and flag below-market files. Novel feature.
// pp:data-source local
package cli

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

type dealRow struct {
	types.Listing
	PricePerMarla      int     `json:"price_per_marla"`
	AreaMedianPPM      int     `json:"area_median_price_per_marla"`
	PctBelowAreaMedian float64 `json:"pct_below_area_median"`
	BelowMarket        bool    `json:"below_market"`
}

func newNovelDealsCmd(flags *rootFlags) *cobra.Command {
	var flagCity, flagArea, flagType string
	var dbPath string
	var limit int
	var belowOnly bool
	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Ranks scanned listings by how far under the area's price-per-Marla they sit and flags below-market files.",
		Long: "For each synced listing, compute its price-per-Marla and compare it to the median " +
			"price-per-Marla of its area, then rank by how far below the area median it sits.\n\n" +
			"Use this command to find individual below-market listings (plot files, houses). " +
			"Do NOT use it for area summary stats; use 'comps' for medians and inventory.\n\n" +
			"Run 'zameen-pp-cli pull ...' first to populate the local store.",
		Example:     "  zameen-pp-cli deals --city Islamabad --area DHA_Defence --type Plots --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank below-market listings from the local store")
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
			if t := strings.TrimSpace(flagType); t != "" {
				kept := listings[:0]
				for _, l := range listings {
					if strings.Contains(strings.ToLower(l.PropertyType), strings.ToLower(t)) {
						kept = append(kept, l)
					}
				}
				listings = kept
			}

			// Median price-per-Marla per area.
			areaPPM := map[string][]int{}
			for _, l := range listings {
				if p := pricePerMarla(l); p > 0 {
					areaPPM[l.Location] = append(areaPPM[l.Location], int(p))
				}
			}
			areaMedian := map[string]int{}
			for area, vals := range areaPPM {
				areaMedian[area] = medianInt(vals)
			}

			rows := make([]dealRow, 0, len(listings))
			for _, l := range listings {
				ppm := pricePerMarla(l)
				if ppm <= 0 {
					continue
				}
				med := areaMedian[l.Location]
				if med <= 0 {
					continue
				}
				pct := math.Round((float64(med)-ppm)/float64(med)*1000) / 10
				row := dealRow{
					Listing:            l,
					PricePerMarla:      int(ppm),
					AreaMedianPPM:      med,
					PctBelowAreaMedian: pct,
					BelowMarket:        pct > 0,
				}
				if belowOnly && !row.BelowMarket {
					continue
				}
				rows = append(rows, row)
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].PctBelowAreaMedian > rows[j].PctBelowAreaMedian })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no priced listings in the local store for city=%q area=%q type=%q; run pull first or widen the query\n", flagCity, flagArea, flagType)
			}
			return emitObject(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "Filter stored listings to this city")
	cmd.Flags().StringVar(&flagArea, "area", "", "Filter stored listings to this area/society (e.g. DHA_Defence)")
	cmd.Flags().StringVar(&flagType, "type", "", "Filter to a property type (Homes, Plots, Commercial)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum listings to return")
	cmd.Flags().BoolVar(&belowOnly, "below-only", false, "Only show listings priced below their area median")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
