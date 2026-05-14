package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytanalytics"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstore"
)

func newRetentionCmd(flags *rootFlags) *cobra.Command {
	var (
		ascii     bool
		startDate string
		endDate   string
		drops     int
		dbPath    string
		fresh     bool
	)

	cmd := &cobra.Command{
		Use:   "retention [video_id]",
		Short: "Retention curve for a video (100 buckets) with sharpest drops auto-annotated",
		Long: strings.TrimSpace(`
Returns the audience retention curve for a video as 100 ratio buckets from
the YouTube Analytics API. By default the curve is cached locally and read
from the store; pass --fresh to force a live API call.

The 3 sharpest drops are auto-annotated — each annotation includes the
elapsed-video-time ratio, the before/after ratios, and the drop magnitude.`),
		Example: strings.Trim(`
  # ASCII sparkline + JSON of drops
  yt-studio-pp-cli retention dQw4w9WgXcQ --ascii

  # Force a fresh fetch and show only the drops
  yt-studio-pp-cli retention dQw4w9WgXcQ --fresh --json --select drops
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			videoID := args[0]
			ctx := cmd.Context()

			db, closer, err := ensureDB(ctx, flags, dbPath)
			if err != nil {
				return err
			}
			defer closer()

			var points []float64
			source := "store"
			if !fresh {
				if cur, err := ytstore.LatestRetentionCurve(ctx, db, videoID); err == nil {
					points = cur.Points
				}
			}
			if len(points) == 0 || fresh {
				token, err := loadOAuthToken(flags)
				if err != nil {
					return err
				}
				ac := ytanalytics.New(token)
				if startDate == "" {
					startDate = time.Now().UTC().AddDate(0, 0, -28).Format("2006-01-02")
				}
				if endDate == "" {
					endDate = time.Now().UTC().Format("2006-01-02")
				}
				points, err = ac.RetentionCurve(ctx, videoID, startDate, endDate)
				if err != nil {
					var apiE *ytanalytics.Error
					if errors.As(err, &apiE) {
						switch apiE.Kind {
						case ytanalytics.KindAuth:
							return authErr(err)
						case ytanalytics.KindRateLimit:
							return apiErr(fmt.Errorf("youtube analytics rate-limited (exit 7); back off: %w", err))
						}
					}
					return err
				}
				if len(points) == 0 {
					return notFoundErr(fmt.Errorf("no retention data for video %s; not yet processed by YouTube or no Analytics access on this channel", videoID))
				}
				if err := ytstore.SaveRetentionCurve(ctx, db, videoID, points); err != nil {
					_ = err // best effort
				}
				source = "live"
			}

			ann := findSharpestDrops(points, drops)

			result := map[string]any{
				"video_id": videoID,
				"buckets":  len(points),
				"points":   points,
				"drops":    ann,
				"source":   source,
				"recorded": time.Now().UTC().Format(time.RFC3339),
			}
			if ascii {
				result["sparkline"] = asciiSparkline(points, 80)
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, result)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Retention curve for %s (%d buckets, source=%s)\n", videoID, len(points), source)
			if ascii {
				fmt.Fprintln(w, asciiSparkline(points, 80))
			}
			if len(ann) > 0 {
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Sharpest drops:")
				for i, a := range ann {
					fmt.Fprintf(w, "  %d. at %.0f%% of video: %.3f → %.3f (drop %.3f)\n",
						i+1, a.VideoTimeRatio*100, a.BeforeRatio, a.AfterRatio, a.DropMagnitude)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&ascii, "ascii", false, "Render an ASCII sparkline of the curve")
	cmd.Flags().BoolVar(&fresh, "fresh", false, "Force a live API call rather than reading from the store")
	cmd.Flags().IntVar(&drops, "drops", 3, "Number of sharpest drops to auto-annotate")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date (YYYY-MM-DD); defaults to 28 days ago")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date (YYYY-MM-DD); defaults to today")
	cmd.Flags().StringVar(&dbPath, "db", "", "Custom database path")
	return cmd
}
