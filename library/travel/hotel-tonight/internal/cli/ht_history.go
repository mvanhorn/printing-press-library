// `history` — show the recorded price/% off trajectory for one hotel from the
// local store. Reads only; the data exists because deals/watch/datescan
// recorded snapshots over time. Hand-authored (survives `generate --force`).
package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	var days int

	cmd := &cobra.Command{
		Use:   "history <hotel>",
		Short: "Show the recorded price and % off history for one hotel",
		Long: "Read the local price-snapshot store and show the observed nightly price and % off for a hotel " +
			"over time. HotelTonight erases prior prices, so this history only contains what you've recorded by " +
			"running deals, watch, datescan, or daily-drop for that area. Matches the hotel by case-insensitive " +
			"name substring.",
		Example: strings.Trim(`
  hotel-tonight-pp-cli history "Argonaut Hotel" --days 30
  hotel-tonight-pp-cli history argonaut --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := strings.TrimSpace(strings.Join(args, " "))
			s, err := openPriceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			rows, err := hotelHistory(cmd.Context(), s.DB(), name, days)
			if err != nil {
				return err
			}
			result := map[string]any{
				"hotel":        name,
				"days":         days,
				"observations": len(rows),
				"history":      rows,
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"No recorded history for %q yet. Run 'deals' or 'watch' for that area first to start recording prices.\n", name)
			}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				pct := ""
				if r.PctOff > 0 {
					pct = strconv.Itoa(r.PctOff) + "%"
				}
				tableRows = append(tableRows, []string{r.ObservedAt, r.CheckIn, money(r.Price), pct, r.DealType})
			}
			return emitJSONOrTable(cmd, flags, result,
				[]string{"Observed", "Check-in", "Price", "Off", "Deal"}, tableRows)
		},
	}
	cmd.Flags().IntVar(&days, "days", 30, "Only include observations from the last N days (0 = all)")
	return cmd
}
