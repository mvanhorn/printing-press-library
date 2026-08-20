// Hand-authored novel command: topic-drift detector. Not generated.
package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"
)

type driftTopic struct {
	Topic      string  `json:"topic"`
	EarlyShare float64 `json:"early_share"`
	LateShare  float64 `json:"late_share"`
	DeltaPct   float64 `json:"delta_pct"`
}

type driftOutput struct {
	Query      string       `json:"query"`
	FromYear   int          `json:"from_year"`
	ToYear     int          `json:"to_year"`
	MidYear    int          `json:"mid_year"`
	Emerging   []driftTopic `json:"emerging"`
	Fading     []driftTopic `json:"fading"`
	Note       string       `json:"note,omitempty"`
}

// validateDriftSpan enforces that both bounds are set and the analysis span is
// at least 2 years — the midpoint split needs a non-empty year on each side.
func validateDriftSpan(fromYear, toYear int) error {
	if fromYear == 0 || toYear == 0 || toYear < fromYear+2 {
		return usageErr(fmt.Errorf("drift requires --from and --to with a span of at least 2 years (got --from %d --to %d): a shorter span leaves the early window empty", fromYear, toYear))
	}
	return nil
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var fromYear, toYear, limit int

	cmd := &cobra.Command{
		Use:   "drift <query>",
		Short: "Compare a topic's subtopic distribution between two year windows",
		Long: "Split the span [--from, --to] at its midpoint and compare the share of each\n" +
			"subtopic in the early vs late half. Surfaces emerging (rising share) and fading\n" +
			"(falling share) subtopics within a field.",
		Example:     "  scientific-consensus-pp-cli drift \"machine learning genomics\" --from 2015 --to 2025 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare topic drift between two windows")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			if err := validateDriftSpan(fromYear, toYear); err != nil {
				_ = cmd.Usage()
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Integer division is safe here: validateDriftSpan guarantees
			// toYear >= fromYear+2, so mid >= fromYear+1 and the early window
			// [fromYear, mid-1] always spans at least one full year.
			mid := (fromYear + toYear) / 2
			earlyFilter := fmt.Sprintf("from_publication_date:%d-01-01,to_publication_date:%d-12-31", fromYear, mid-1)
			lateFilter := fmt.Sprintf("from_publication_date:%d-01-01,to_publication_date:%d-12-31", mid, toYear)

			early, err := fetchGroupBy(ctx, c, "/works", query, earlyFilter, "primary_topic.id")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			late, err := fetchGroupBy(ctx, c, "/works", query, lateFilter, "primary_topic.id")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			earlyShare := shareMap(early)
			lateShare := shareMap(late)
			seen := map[string]bool{}
			var topics []driftTopic
			for name := range earlyShare {
				seen[name] = true
			}
			for name := range lateShare {
				seen[name] = true
			}
			for name := range seen {
				if name == "" || name == "unknown" {
					continue
				}
				e, l := earlyShare[name], lateShare[name]
				topics = append(topics, driftTopic{
					Topic: name, EarlyShare: round1(e * 100), LateShare: round1(l * 100),
					DeltaPct: round1((l - e) * 100),
				})
			}
			sort.SliceStable(topics, func(i, j int) bool { return topics[i].DeltaPct > topics[j].DeltaPct })
			out := driftOutput{Query: query, FromYear: fromYear, ToYear: toYear, MidYear: mid}
			for _, t := range topics {
				if t.DeltaPct > 0 && len(out.Emerging) < limit {
					out.Emerging = append(out.Emerging, t)
				}
			}
			for i := len(topics) - 1; i >= 0; i-- {
				if topics[i].DeltaPct < 0 && len(out.Fading) < limit {
					out.Fading = append(out.Fading, topics[i])
				}
			}
			if len(topics) == 0 {
				out.Note = "no topic data in these windows; widen the year range or query"
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderDrift(w, out) })
		},
	}
	cmd.Flags().IntVar(&fromYear, "from", 0, "start year of the span (required; the span must cover at least 2 years)")
	cmd.Flags().IntVar(&toYear, "to", 0, "end year of the span (required; must be at least --from + 2)")
	cmd.Flags().IntVar(&limit, "limit", 10, "number of emerging/fading subtopics to show")
	return cmd
}

func shareMap(rows []groupRow) map[string]float64 {
	total := 0
	for _, r := range rows {
		total += r.Count
	}
	out := map[string]float64{}
	if total == 0 {
		return out
	}
	for _, r := range rows {
		out[r.Name] = float64(r.Count) / float64(total)
	}
	return out
}

func renderDrift(w io.Writer, o driftOutput) {
	fmt.Fprintf(w, "Topic drift: %s  (%d–%d vs %d–%d)\n\n", o.Query, o.FromYear, o.MidYear-1, o.MidYear, o.ToYear)
	fmt.Fprintln(w, "  Emerging (rising share):")
	for _, t := range o.Emerging {
		fmt.Fprintf(w, "    ↑ %-46s %+.1f pts (%.1f%%→%.1f%%)\n", truncate(t.Topic, 46), t.DeltaPct, t.EarlyShare, t.LateShare)
	}
	fmt.Fprintln(w, "\n  Fading (falling share):")
	for _, t := range o.Fading {
		fmt.Fprintf(w, "    ↓ %-46s %+.1f pts (%.1f%%→%.1f%%)\n", truncate(t.Topic, 46), t.DeltaPct, t.EarlyShare, t.LateShare)
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}
