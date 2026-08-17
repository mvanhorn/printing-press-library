// analytics: group-by rollups over synced listings in the local store.
// Fulfils absorbed feature "analytics / group-by aggregates".
// pp:data-source local
package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

type analyticsRow struct {
	Group       string `json:"group"`
	Count       int    `json:"count"`
	MedianPrice int    `json:"median_price"`
	MinPrice    int    `json:"min_price"`
	MaxPrice    int    `json:"max_price"`
}

func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var dbPath, groupBy string
	var limit int
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Group-by rollups (count, median/min/max price) over synced listings",
		Long: "Aggregate the local store by a dimension (city, location, agency, purpose, type) into " +
			"count and median/min/max price. Run 'zameen-pp-cli pull ...' first to populate it.",
		Example:     "  zameen-pp-cli analytics --group-by city",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate the local store")
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
			key := func(l types.Listing) string {
				switch groupBy {
				case "location", "area":
					return l.Location
				case "agency":
					return l.Agency
				case "purpose":
					return l.Purpose
				case "type", "property_type":
					return l.PropertyType
				default:
					return l.City
				}
			}
			groups := map[string][]int{}
			for _, l := range listings {
				g := key(l)
				if g == "" {
					g = "(unspecified)"
				}
				groups[g] = append(groups[g], l.Price)
			}
			rows := make([]analyticsRow, 0, len(groups))
			for g, prices := range groups {
				minP, maxP := 0, 0
				for i, p := range prices {
					if i == 0 || p < minP {
						minP = p
					}
					if p > maxP {
						maxP = p
					}
				}
				rows = append(rows, analyticsRow{
					Group: g, Count: len(prices), MedianPrice: medianInt(prices),
					MinPrice: minP, MaxPrice: maxP,
				})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return emitObject(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&groupBy, "group-by", "city", "Dimension to group by: city, location, agency, purpose, type")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum groups to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
