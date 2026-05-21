// `verdict` — classify a hotel's most recent recorded price against its own
// observed low/median/high as cheap, typical, or expensive. Pure percentile
// math over the local store, no model. Hand-authored (survives
// `generate --force`).
package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newVerdictCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verdict <hotel>",
		Short: "Judge whether a hotel's latest recorded price is cheap, typical, or expensive",
		Long: "Compare a hotel's most recently recorded nightly price against the full distribution of prices " +
			"you've recorded for it, and classify it: at or below the 25th percentile is cheap, at or above the " +
			"75th is expensive, otherwise typical. This needs a few prior observations to be meaningful — run " +
			"deals or watch for the area several times first. Matches the hotel by case-insensitive name substring.",
		Example: strings.Trim(`
  hotel-tonight-pp-cli verdict "Argonaut Hotel"
  hotel-tonight-pp-cli verdict argonaut --agent
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

			stats, ok, err := hotelPriceStats(cmd.Context(), s.DB(), name)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"No recorded prices for %q yet. Run 'deals' or 'watch' for that area first.\n", name)
				return emitJSONOrTable(cmd, flags, map[string]any{
					"hotel": name, "observations": 0, "verdict": "unknown",
				}, []string{"Hotel", "Verdict"}, [][]string{{name, "unknown (no data)"}})
			}

			// Current = most recent observation (history is newest-first).
			current := stats.Median
			if hist, _ := hotelHistory(cmd.Context(), s.DB(), name, 0); len(hist) > 0 && hist[0].Price > 0 {
				current = hist[0].Price
			}

			verdict := classifyPrice(current, stats)
			confident := stats.Observations >= 3
			result := map[string]any{
				"hotel":         stats.HotelName,
				"current_price": current,
				"verdict":       verdict,
				"confident":     confident,
				"observations":  stats.Observations,
				"low":           stats.Low,
				"median":        stats.Median,
				"high":          stats.High,
			}
			if !confident {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"Only %d observation(s) recorded — verdict is low-confidence. Record more by running 'deals'/'watch' over time.\n",
					stats.Observations)
			}
			rows := [][]string{{
				stats.HotelName, money(current), verdict,
				fmt.Sprintf("%s / %s / %s", money(stats.Low), money(stats.Median), money(stats.High)),
			}}
			return emitJSONOrTable(cmd, flags, result,
				[]string{"Hotel", "Current", "Verdict", "Low/Med/High"}, rows)
		},
	}
	return cmd
}
