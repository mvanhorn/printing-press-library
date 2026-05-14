package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newRetentionCohortCmd(flags *rootFlags) *cobra.Command {
	var (
		pattern string
		days    int
		ascii   bool
		dbPath  string
		kind    string
	)
	cmd := &cobra.Command{
		Use:   "retention-cohort",
		Short: "Average retention across videos whose title matches a regex (read-only)",
		Long: strings.TrimSpace(`
Aggregates retention curves across multiple videos whose titles match a
regex pattern. Useful for comparing format cohorts ("Rework", "Build Guide",
"Tier List") against each other.

Each matching video must have at least one retention curve in the local
store; run 'yt-studio-pp-cli sync' first to populate the store.`),
		Example: strings.Trim(`
  yt-studio-pp-cli retention-cohort --pattern "Rework" --days 90 --ascii
  yt-studio-pp-cli retention-cohort --pattern "Build Guide|Tier" --kind own --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if pattern == "" {
				return usageErr(errors.New("--pattern is required"))
			}
			re, err := regexp.Compile("(?i)" + pattern)
			if err != nil {
				return usageErr(fmt.Errorf("invalid regex: %w", err))
			}
			ctx := cmd.Context()
			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			// We use a broad LIKE filter to cut down videos, then regex in Go for precision.
			vids, err := ytstore.VideosMatchingTitleLike(ctx, db, "%", kind)
			if err != nil {
				return err
			}
			matched := []ytstore.Video{}
			for _, v := range vids {
				if re.MatchString(v.Title) {
					matched = append(matched, v)
				}
			}
			if len(matched) == 0 {
				return notFoundErr(fmt.Errorf("no videos matched pattern %q (kind=%s)", pattern, kind))
			}

			// Aggregate retention curves
			var sum [100]float64
			var counts [100]int
			used := 0
			for _, v := range matched {
				cur, err := ytstore.LatestRetentionCurve(ctx, db, v.ID)
				if err != nil {
					continue
				}
				if len(cur.Points) != 100 {
					continue
				}
				for i, p := range cur.Points {
					sum[i] += p
					counts[i]++
				}
				used++
			}
			if used == 0 {
				return notFoundErr(fmt.Errorf("none of the %d matched videos have a retention curve in the store; run `yt-studio-pp-cli sync` first", len(matched)))
			}
			avg := make([]float64, 100)
			for i := 0; i < 100; i++ {
				if counts[i] > 0 {
					avg[i] = sum[i] / float64(counts[i])
				}
			}
			drops := findSharpestDrops(avg, 3)

			worst := 0
			worstVal := 1.0
			for i, v := range avg {
				if v < worstVal {
					worstVal = v
					worst = i
				}
			}

			res := map[string]any{
				"pattern":           pattern,
				"days":              days,
				"matched_videos":    len(matched),
				"cohort_size":       used,
				"avg_retention_pct": avgPct(avg),
				"worst_bucket":      map[string]any{"index": worst, "value": worstVal},
				"drops":             drops,
				"points":            avg,
			}
			if ascii {
				res["sparkline"] = asciiSparkline(avg, 80)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Cohort retention for pattern %q (%d videos)\n", pattern, used)
			fmt.Fprintf(w, "  average retention: %.1f%%\n", avgPct(avg)*100)
			fmt.Fprintf(w, "  worst bucket: %d (%.3f)\n", worst, worstVal)
			if ascii {
				fmt.Fprintln(w, asciiSparkline(avg, 80))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&pattern, "pattern", "", "Title regex (case-insensitive)")
	cmd.Flags().IntVar(&days, "days", 90, "Time window in days (informational; cohort uses store contents)")
	cmd.Flags().BoolVar(&ascii, "ascii", false, "Render an ASCII sparkline")
	cmd.Flags().StringVar(&kind, "kind", "own", "Channel kind filter: own | competitor | (empty for any)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}

func avgPct(p []float64) float64 {
	if len(p) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range p {
		sum += v
	}
	return sum / float64(len(p))
}
