// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"math"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/store"

	"github.com/spf13/cobra"
)

type reciprocityRow struct {
	Handle   string  `json:"handle"`
	Outgoing int     `json:"outgoing"`
	Incoming int     `json:"incoming"`
	Net      int     `json:"net"`
	Drift    float64 `json:"drift"`
}

func newEngageReciprocityCmd(flags *rootFlags) *cobra.Command {
	var handle string
	var days int

	cmd := &cobra.Command{
		Use:   "reciprocity",
		Short: "Net give/take per writer — who reciprocates, who free-rides",
		Example: strings.Trim(`
  substack-pp-cli engage reciprocity --json
  substack-pp-cli engage reciprocity --handle alice --days 60 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), []reciprocityRow{}, flags)
			}
			rows, err := computeReciprocity(flags, handle, days)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []reciprocityRow{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&handle, "handle", "", "Filter to a single handle")
	cmd.Flags().IntVar(&days, "days", 30, "Lookback window in days")
	return cmd
}

func computeReciprocity(flags *rootFlags, handleFilter string, days int) ([]reciprocityRow, error) {
	st, err := store.Open(defaultDBPath("substack-pp-cli"))
	if err != nil {
		return []reciprocityRow{}, nil
	}
	defer st.Close()

	since := "-" + intToStr(days) + " days"
	outgoing := map[string]int{}
	incoming := map[string]int{}

	if rows, err := st.DB().Query(
		`SELECT target_handle, COUNT(*) FROM engagements
		 WHERE by_self = 1 AND recorded_at >= datetime('now', ?)
		 GROUP BY target_handle`, since); err == nil {
		defer rows.Close()
		for rows.Next() {
			var h string
			var c int
			if err := rows.Scan(&h, &c); err == nil && h != "" {
				outgoing[h] = c
			}
		}
	} else if !isMissingTableErr(err) {
		// fall through to empty result
	}

	if rows, err := st.DB().Query(
		`SELECT target_handle, COUNT(*) FROM engagements
		 WHERE by_self = 0 AND recorded_at >= datetime('now', ?)
		 GROUP BY target_handle`, since); err == nil {
		defer rows.Close()
		for rows.Next() {
			var h string
			var c int
			if err := rows.Scan(&h, &c); err == nil && h != "" {
				incoming[h] = c
			}
		}
	}

	all := map[string]bool{}
	for h := range outgoing {
		all[h] = true
	}
	for h := range incoming {
		all[h] = true
	}

	out := []reciprocityRow{}
	for h := range all {
		if handleFilter != "" && h != handleFilter {
			continue
		}
		o, i := outgoing[h], incoming[h]
		net := o - i
		denom := o
		if denom == 0 {
			denom = 1
		}
		drift := float64(net) / float64(denom)
		out = append(out, reciprocityRow{
			Handle:   h,
			Outgoing: o,
			Incoming: i,
			Net:      net,
			Drift:    drift,
		})
	}
	sort.Slice(out, func(i, j int) bool { return math.Abs(out[i].Drift) > math.Abs(out[j].Drift) })
	return out, nil
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
