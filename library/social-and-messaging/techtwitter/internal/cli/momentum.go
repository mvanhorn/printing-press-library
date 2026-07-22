// Hand-authored novel command: momentum. Captures the current heatmap into the
// local snapshot history, then reports which topics are rising, falling, or new
// versus a prior snapshot — movement no single live call can show.
//
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

type ttMomentumRow struct {
	Keyword         string `json:"keyword"`
	Slug            string `json:"slug,omitempty"`
	Status          string `json:"status"`
	Count           int    `json:"count"`
	CountDelta      int    `json:"count_delta"`
	Engagement      int    `json:"engagement"`
	EngagementDelta int    `json:"engagement_delta"`
}

type ttMomentumResult struct {
	CapturedAt string          `json:"captured_at"`
	ComparedTo string          `json:"compared_to,omitempty"`
	Source     string          `json:"source"`
	Baseline   bool            `json:"baseline"`
	Note       string          `json:"note,omitempty"`
	Topics     []ttMomentumRow `json:"topics"`
}

func newNovelMomentumCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string
	var limit int

	cmd := &cobra.Command{
		Use:   "momentum",
		Short: "Show which topics are rising, falling, or newly appearing across the heatmap snapshots stored on each sync.",
		Long: "Capture the current topic heatmap into the local snapshot history, then compare it against " +
			"a prior snapshot within the window (default 7d) to classify each topic as rising, falling, " +
			"steady, or new. The first run records a baseline; run again later to see movement.",
		Example:     "  techtwitter-pp-cli momentum --window 7d --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would capture a heatmap snapshot and report topic momentum")
				return nil
			}
			dur, err := ttParseWindow(window)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			dbPath = ttResolveDB(dbPath)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			topics, source := ttCurrentTopics(ctx, flags, db)
			captured, err := ttCaptureSnapshot(db.DB(), topics)
			if err != nil {
				return fmt.Errorf("capturing snapshot: %w", err)
			}
			if captured == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "no heatmap topics available (run `sync` or check connectivity)")
				if ttWantsJSON(cmd, flags) {
					return ttEmitJSON(cmd, flags, ttMomentumResult{Source: source, Baseline: true, Topics: []ttMomentumRow{}})
				}
				return nil
			}

			times, err := ttDistinctSnapshotTimes(db.DB())
			if err != nil {
				return err
			}

			result := ttMomentumResult{CapturedAt: captured, Source: source, Topics: []ttMomentumRow{}}
			current, err := ttSnapshotTopics(db.DB(), captured)
			if err != nil {
				return fmt.Errorf("loading current snapshot: %w", err)
			}

			prior := ttPriorSnapshotWithin(times, captured, ttCutoff(dur))
			if prior == "" {
				result.Baseline = true
				result.Note = "Baseline snapshot recorded. Run momentum again later to see topic movement."
				for _, t := range topics {
					result.Topics = append(result.Topics, ttMomentumRow{
						Keyword: t.Keyword, Slug: t.Slug, Status: "baseline",
						Count: t.Count, Engagement: t.Engagement,
					})
				}
			} else {
				result.ComparedTo = prior
				priorTopics, err := ttSnapshotTopics(db.DB(), prior)
				if err != nil {
					return fmt.Errorf("loading prior snapshot: %w", err)
				}
				for kw, cur := range current {
					row := ttMomentumRow{Keyword: kw, Slug: cur.Slug, Count: cur.Count, Engagement: cur.Engagement}
					if p, ok := priorTopics[kw]; ok {
						row.CountDelta = cur.Count - p.Count
						row.EngagementDelta = cur.Engagement - p.Engagement
						switch {
						case row.CountDelta > 0:
							row.Status = "rising"
						case row.CountDelta < 0:
							row.Status = "falling"
						default:
							row.Status = "steady"
						}
					} else {
						row.Status = "new"
						row.CountDelta = cur.Count
						row.EngagementDelta = cur.Engagement
					}
					result.Topics = append(result.Topics, row)
				}
				sort.Slice(result.Topics, func(i, j int) bool {
					if result.Topics[i].CountDelta != result.Topics[j].CountDelta {
						return result.Topics[i].CountDelta > result.Topics[j].CountDelta
					}
					return result.Topics[i].Count > result.Topics[j].Count
				})
			}
			if limit > 0 && len(result.Topics) > limit {
				result.Topics = result.Topics[:limit]
			}

			if ttWantsJSON(cmd, flags) {
				return ttEmitJSON(cmd, flags, result)
			}
			w := cmd.OutOrStdout()
			if result.Baseline {
				fmt.Fprintf(w, "%s\n%s\n\n", bold("TOPIC MOMENTUM — baseline"), result.Note)
			} else {
				fmt.Fprintf(w, "%s\n\n", bold(fmt.Sprintf("TOPIC MOMENTUM — last %s", dur)))
			}
			for _, t := range result.Topics {
				arrow := "•"
				switch t.Status {
				case "rising", "new":
					arrow = green("▲")
				case "falling":
					arrow = red("▼")
				}
				fmt.Fprintf(w, "  %s %-28s %-8s Δcount %+d  Δeng %+d\n", arrow, truncate(t.Keyword, 28), t.Status, t.CountDelta, t.EngagementDelta)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techtwitter-pp-cli/data.db)")
	cmd.Flags().StringVar(&window, "window", "7d", "Comparison window (24h, 48h, 7d, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum topics to return")
	return cmd
}
