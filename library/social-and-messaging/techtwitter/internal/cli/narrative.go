// Hand-authored novel command: narrative. Diffs heatmap snapshots to surface
// keywords that newly emerged or accelerated, each grounded in supporting
// stored tweets — the emergence view the live narrative-alert snapshot lacks.
//
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

type ttNarrativeRow struct {
	Keyword         string          `json:"keyword"`
	Slug            string          `json:"slug,omitempty"`
	Status          string          `json:"status"`
	Count           int             `json:"count"`
	CountDelta      int             `json:"count_delta"`
	EngagementDelta int             `json:"engagement_delta"`
	Support         []ttEvidenceRow `json:"support,omitempty"`
}

type ttNarrativeResult struct {
	CapturedAt string           `json:"captured_at"`
	ComparedTo string           `json:"compared_to,omitempty"`
	Source     string           `json:"source"`
	Baseline   bool             `json:"baseline"`
	Note       string           `json:"note,omitempty"`
	Narratives []ttNarrativeRow `json:"narratives"`
}

func newNovelNarrativeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var window string
	var limit int

	cmd := &cobra.Command{
		Use:   "narrative",
		Short: "Surface keywords that newly emerged or accelerated versus prior snapshots, grounded in supporting stored tweets.",
		Long: "Capture the current heatmap into the local snapshot history, then diff it against a prior " +
			"snapshot within the window (default 7d) to surface keywords that just emerged or accelerated. " +
			"Each is grounded in supporting stored tweets. The first run records a baseline.",
		Example:     "  techtwitter-pp-cli narrative --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would capture a heatmap snapshot and surface emerging narratives")
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
					return ttEmitJSON(cmd, flags, ttNarrativeResult{Source: source, Baseline: true, Narratives: []ttNarrativeRow{}})
				}
				return nil
			}

			times, err := ttDistinctSnapshotTimes(db.DB())
			if err != nil {
				return err
			}
			current, err := ttSnapshotTopics(db.DB(), captured)
			if err != nil {
				return fmt.Errorf("loading current snapshot: %w", err)
			}
			result := ttNarrativeResult{CapturedAt: captured, Source: source, Narratives: []ttNarrativeRow{}}

			prior := ttPriorSnapshotWithin(times, captured, ttCutoff(dur))
			if prior == "" {
				result.Baseline = true
				result.Note = "Baseline snapshot recorded. Narrative emergence needs at least two snapshots; showing the currently leading topics."
				leading := append([]ttTopic(nil), topics...)
				sort.Slice(leading, func(i, j int) bool { return leading[i].Engagement > leading[j].Engagement })
				for _, t := range leading {
					result.Narratives = append(result.Narratives, ttNarrativeRow{
						Keyword: t.Keyword, Slug: t.Slug, Status: "leading",
						Count: t.Count, Support: narrativeSupport(db, t.Keyword),
					})
					if limit > 0 && len(result.Narratives) >= limit {
						break
					}
				}
			} else {
				result.ComparedTo = prior
				priorTopics, err := ttSnapshotTopics(db.DB(), prior)
				if err != nil {
					return fmt.Errorf("loading prior snapshot: %w", err)
				}
				for kw, cur := range current {
					p, existed := priorTopics[kw]
					row := ttNarrativeRow{Keyword: kw, Slug: cur.Slug, Count: cur.Count}
					if !existed {
						row.Status = "emerging"
						row.CountDelta = cur.Count
						row.EngagementDelta = cur.Engagement
					} else {
						row.CountDelta = cur.Count - p.Count
						row.EngagementDelta = cur.Engagement - p.Engagement
						if row.EngagementDelta <= 0 && row.CountDelta <= 0 {
							continue // not emerging or accelerating
						}
						row.Status = "accelerating"
					}
					row.Support = narrativeSupport(db, kw)
					result.Narratives = append(result.Narratives, row)
				}
				sort.Slice(result.Narratives, func(i, j int) bool {
					// Emerging before accelerating, then by engagement delta.
					if (result.Narratives[i].Status == "emerging") != (result.Narratives[j].Status == "emerging") {
						return result.Narratives[i].Status == "emerging"
					}
					return result.Narratives[i].EngagementDelta > result.Narratives[j].EngagementDelta
				})
				if limit > 0 && len(result.Narratives) > limit {
					result.Narratives = result.Narratives[:limit]
				}
				if len(result.Narratives) == 0 {
					result.Note = "No newly emerging or accelerating keywords since the prior snapshot."
				}
			}

			if ttWantsJSON(cmd, flags) {
				return ttEmitJSON(cmd, flags, result)
			}
			w := cmd.OutOrStdout()
			title := "EMERGING NARRATIVES"
			if result.Baseline {
				title = "EMERGING NARRATIVES — baseline"
			}
			fmt.Fprintf(w, "%s\n", bold(title))
			if result.Note != "" {
				fmt.Fprintf(w, "%s\n", result.Note)
			}
			fmt.Fprintln(w)
			for _, n := range result.Narratives {
				fmt.Fprintf(w, "  %s %-28s Δeng %+d\n", green("⬆"), truncate(n.Keyword, 28), n.EngagementDelta)
				for _, s := range n.Support {
					fmt.Fprintf(w, "      ↳ %s\n", truncate(s.Title, 90))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techtwitter-pp-cli/data.db)")
	cmd.Flags().StringVar(&window, "window", "7d", "Comparison window (24h, 48h, 7d, 30d)")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum narratives to return")
	return cmd
}

// narrativeSupport finds up to two stored tweets mentioning the keyword. Topic
// keywords land in each tweet's keywords array (inside the data JSON) rather
// than the visible text, so the match runs against the data column.
func narrativeSupport(db *store.Store, keyword string) []ttEvidenceRow {
	like := "%" + keyword + "%"
	tweets, err := ttScanTweets(db.DB(),
		`WHERE content_type != 'article' AND data LIKE ?
		 ORDER BY (COALESCE(bookmark_count,0)*4 + COALESCE(comment_count,0)*3 + COALESCE(retweet_count,0)*2 + COALESCE(like_count,0)) DESC
		 LIMIT 2`, like)
	if err != nil {
		return nil
	}
	out := make([]ttEvidenceRow, 0, len(tweets))
	for _, t := range tweets {
		out = append(out, tweetEvidence(t, "Supporting tweet mentioning the keyword."))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
