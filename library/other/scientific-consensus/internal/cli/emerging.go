// Hand-authored novel command: emerging-topics detector. Not generated.
package cli

import (
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type emergingTopic struct {
	Topic       string  `json:"topic"`
	RecentWorks int     `json:"recent_works"`
	PriorWorks  int     `json:"prior_works"`
	GrowthRatio float64 `json:"growth_ratio"`
}

type emergingOutput struct {
	Field       string          `json:"field,omitempty"`
	WindowYears int             `json:"window_years"`
	Topics      []emergingTopic `json:"topics"`
	Note        string          `json:"note,omitempty"`
}

func newNovelEmergingCmd(flags *rootFlags) *cobra.Command {
	var field string
	var window int
	var limit int
	var minRecent int

	cmd := &cobra.Command{
		Use:   "emerging",
		Short: "Detect the fastest-growing research topics and exploding publication trends",
		Long: "Compare topic publication counts in a recent window against the preceding window\n" +
			"of equal length and rank topics by growth ratio. Use --field to scope to a\n" +
			"discipline or query.",
		Example:     "  scientific-consensus-pp-cli emerging --field neuroscience --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would detect emerging topics")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if window <= 0 {
				window = 2
			}
			now := time.Now().Year()
			recentFrom := now - window
			priorFrom := now - 2*window

			recentFilter := fmt.Sprintf("from_publication_date:%d-01-01", recentFrom)
			priorFilter := fmt.Sprintf("from_publication_date:%d-01-01,to_publication_date:%d-12-31", priorFrom, recentFrom-1)

			recent, err := fetchGroupBy(ctx, c, "/works", field, recentFilter, "primary_topic.id")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			prior, err := fetchGroupBy(ctx, c, "/works", field, priorFilter, "primary_topic.id")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			priorByName := map[string]int{}
			for _, g := range prior {
				priorByName[g.Name] = g.Count
			}
			topics := make([]emergingTopic, 0, len(recent))
			for _, g := range recent {
				if g.Count < minRecent || g.Name == "" || g.Name == "unknown" {
					continue
				}
				p := priorByName[g.Name]
				topics = append(topics, emergingTopic{
					Topic: g.Name, RecentWorks: g.Count, PriorWorks: p,
					GrowthRatio: round2f(float64(g.Count) / float64(p+1)),
				})
			}
			sort.SliceStable(topics, func(i, j int) bool {
				if topics[i].GrowthRatio != topics[j].GrowthRatio {
					return topics[i].GrowthRatio > topics[j].GrowthRatio
				}
				return topics[i].RecentWorks > topics[j].RecentWorks
			})
			if len(topics) > limit {
				topics = topics[:limit]
			}
			out := emergingOutput{Field: field, WindowYears: window, Topics: topics}
			if len(topics) == 0 {
				out.Note = "no topics met the threshold; lower --min-recent or widen --field"
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderEmerging(w, out) })
		},
	}
	cmd.Flags().StringVar(&field, "field", "", "scope to a discipline or query (e.g. neuroscience)")
	cmd.Flags().IntVar(&window, "window", 2, "length in years of each comparison window")
	cmd.Flags().IntVar(&limit, "limit", 15, "number of topics to return")
	cmd.Flags().IntVar(&minRecent, "min-recent", 30, "minimum recent-window works for a topic to qualify")
	return cmd
}

func renderEmerging(w io.Writer, o emergingOutput) {
	scope := o.Field
	if scope == "" {
		scope = "all fields"
	}
	fmt.Fprintf(w, "Emerging topics (%s, %d-year windows):\n\n", scope, o.WindowYears)
	for i, t := range o.Topics {
		fmt.Fprintf(w, "  %2d. %-50s  %dx  (%d→%d)\n", i+1, truncate(t.Topic, 50), int(t.GrowthRatio), t.PriorWorks, t.RecentWorks)
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}
