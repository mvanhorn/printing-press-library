// agencies: agency inventory leaderboard (listing count + median asking price)
// from synced listings. Novel feature.
// pp:data-source local
package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

// normalizeAgencyKey collapses case and whitespace differences so near-duplicate
// spellings of the same agency ("Blue Rock Investments" vs "Blue rock  Investments")
// group into one leaderboard row instead of splitting rank and misidentifying the
// top supplier.
func normalizeAgencyKey(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

type agencyRow struct {
	Agency      string `json:"agency"`
	City        string `json:"city"`
	Count       int    `json:"count"`
	MedianPrice int    `json:"median_price"`
	Verified    int    `json:"verified_listings"`
}

func newNovelAgenciesCmd(flags *rootFlags) *cobra.Command {
	var flagCity, flagArea string
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "agencies",
		Short: "Ranks agencies by listing count and median asking price in an area, from your synced listings.",
		Long: "Roll up synced listings by agency into listing count, median asking price, and " +
			"verified-listing count — a supply-side view of who dominates an area.\n\n" +
			"Run 'zameen-pp-cli pull ...' first to populate the local store.",
		Example:     "  zameen-pp-cli agencies --city Karachi",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank agencies from the local store")
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
			display := map[string]string{}
			for _, l := range listings {
				name := l.Agency
				if strings.TrimSpace(name) == "" {
					name = "(no agency)"
				}
				key := normalizeAgencyKey(name)
				groups[key] = append(groups[key], l)
				if _, ok := display[key]; !ok {
					display[key] = name
				}
			}
			rows := make([]agencyRow, 0, len(groups))
			for key, ls := range groups {
				prices := make([]int, 0, len(ls))
				verified := 0
				city := ""
				for _, l := range ls {
					prices = append(prices, l.Price)
					if l.IsVerified {
						verified++
					}
					if city == "" {
						city = l.City
					}
				}
				rows = append(rows, agencyRow{
					Agency:      display[key],
					City:        city,
					Count:       len(ls),
					MedianPrice: medianInt(prices),
					Verified:    verified,
				})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Count > rows[j].Count })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no stored listings for city=%q; run pull first\n", flagCity)
			}
			return emitObject(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "Filter stored listings to this city")
	cmd.Flags().StringVar(&flagArea, "area", "", "Filter stored listings to this area/society")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum agencies to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
