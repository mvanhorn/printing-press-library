// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/operon/internal/store"
)

// sparkChars is the Unicode block ramp used for the inline sparkline.
var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

type trustPoint struct {
	ObservedAtMS int64    `json:"observed_at_ms"`
	TrustScore   *float64 `json:"trust_score"`
}

func newCampaignTrustHistoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "trust-history <campaign-id>",
		Short: "Plot the locally observed trust score history for a campaign.",
		Long: `Render the trust score time series stored in scoutscore_history for the
given campaign id. Each call to 'operon-pp-cli sync' appends an observation
(when the campaign's trust score is non-null), so this view becomes a
publisher-visible audit trail over time.

Reads from the local store only.`,
		Example: strings.Trim(`
  operon-pp-cli campaign trust-history adv_changenow
  operon-pp-cli campaign trust-history adv_changenow --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			id := strings.TrimSpace(args[0])
			if id == "" {
				return cmd.Help()
			}

			path := dbPath
			if path == "" {
				path = store.DefaultPath("operon-pp-cli")
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query store: %s\n", path)
				fmt.Fprintf(cmd.OutOrStdout(), "would render trust history for advertiser/campaign: %s\n", id)
				return nil
			}

			ctx := context.Background()
			st, err := store.Open(ctx, path)
			if err != nil {
				return apiErr(fmt.Errorf("opening store: %w", err))
			}
			defer st.Close()

			history, err := st.GetTrustHistory(ctx, id)
			if err != nil {
				return apiErr(err)
			}

			points := make([]trustPoint, 0, len(history))
			for _, p := range history {
				points = append(points, trustPoint{ObservedAtMS: p.ObservedAt, TrustScore: p.TrustScore})
			}

			if flags.asJSON || flags.csv || flags.compact || flags.selectFields != "" {
				return printJSONFiltered(cmd.OutOrStdout(), points, flags)
			}

			if len(points) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No history. Run `operon-pp-cli sync` periodically to build a history.")
				return nil
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "campaign: %s\n", id)
			fmt.Fprintf(w, "points  : %d\n", len(points))
			fmt.Fprintf(w, "sparkline: %s\n", sparkline(points))
			fmt.Fprintln(w)
			fmt.Fprintln(w, "observed_at          trust_score")
			for _, p := range points {
				ts := time.UnixMilli(p.ObservedAtMS).Format(time.RFC3339)
				score := "-"
				if p.TrustScore != nil {
					score = fmt.Sprintf("%.2f", *p.TrustScore)
				}
				fmt.Fprintf(w, "%s  %s\n", ts, score)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Override the default store path")
	return cmd
}

// sparkline maps a series of trust scores onto the Unicode block ramp.
// Nil values render as a space so gaps stay visible.
func sparkline(points []trustPoint) string {
	if len(points) == 0 {
		return ""
	}
	min, max := 100.0, 0.0
	any := false
	for _, p := range points {
		if p.TrustScore == nil {
			continue
		}
		any = true
		if *p.TrustScore < min {
			min = *p.TrustScore
		}
		if *p.TrustScore > max {
			max = *p.TrustScore
		}
	}
	if !any {
		return strings.Repeat(" ", len(points))
	}
	rng := max - min
	if rng == 0 {
		rng = 1
	}
	var b strings.Builder
	for _, p := range points {
		if p.TrustScore == nil {
			b.WriteRune(' ')
			continue
		}
		bucket := int(((*p.TrustScore - min) / rng) * float64(len(sparkChars)-1))
		if bucket < 0 {
			bucket = 0
		}
		if bucket >= len(sparkChars) {
			bucket = len(sparkChars) - 1
		}
		b.WriteRune(sparkChars[bucket])
	}
	return b.String()
}
