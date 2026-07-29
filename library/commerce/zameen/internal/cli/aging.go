// aging: longest-standing inventory by days-on-market, derived from each
// synced listing's update timestamp. Novel feature.
// pp:data-source local
package cli

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
)

type agingRow struct {
	types.Listing
	DaysOnMarket int `json:"days_on_market"`
}

func newNovelAgingCmd(flags *rootFlags) *cobra.Command {
	var flagCity, flagArea string
	var minDays int
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "aging",
		Short: "Lists the longest-standing inventory by days-on-market, derived from each listing's update timestamp.",
		Long: "Rank synced listings by days-on-market (now minus the listing's last-updated time). " +
			"Long-standing inventory signals sellers more likely to negotiate.\n\n" +
			"Run 'zameen-pp-cli pull ...' first to populate the local store.",
		Example:     "  zameen-pp-cli aging --city Islamabad --days 90",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list aging inventory from the local store")
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

			now := time.Now().Unix()
			rows := make([]agingRow, 0, len(listings))
			for _, l := range listings {
				ref := int64(l.UpdatedAt)
				if ref == 0 {
					ref = int64(l.CreatedAt)
				}
				if ref == 0 {
					continue
				}
				days := int((now - ref) / 86400)
				if days < 0 {
					days = 0
				}
				if days < minDays {
					continue
				}
				rows = append(rows, agingRow{Listing: l, DaysOnMarket: days})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].DaysOnMarket > rows[j].DaysOnMarket })
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no stored listings at least %d day(s) old for city=%q; run pull first or lower --days\n", minDays, flagCity)
			}
			return emitObject(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&flagCity, "city", "", "Filter stored listings to this city")
	cmd.Flags().StringVar(&flagArea, "area", "", "Filter stored listings to this area/society")
	cmd.Flags().IntVar(&minDays, "days", 30, "Minimum days-on-market to include")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum listings to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
