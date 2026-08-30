// Hand-authored novel command: study-quality estimator. Not generated.
package cli

import (
	"fmt"
	"io"
	"math"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
	"github.com/spf13/cobra"
)

type qualityWork struct {
	Title   string          `json:"title"`
	Year    int             `json:"year,omitempty"`
	DOI     string          `json:"doi,omitempty"`
	Design  scengine.Design `json:"design"`
	CitedBy int             `json:"cited_by_count"`
	Venue   string          `json:"venue,omitempty"`
	Score   int             `json:"quality_score"` // 0..100
	Grade   string          `json:"grade"`
}

type qualityOutput struct {
	Query       string         `json:"query"`
	Analyzed    int            `json:"analyzed"`
	MeanScore   float64        `json:"mean_score"`
	GradeCounts map[string]int `json:"grade_counts"`
	TopWorks    []qualityWork  `json:"top_works"`
	Note        string         `json:"note,omitempty"`
}

func newNovelQualityCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var enrich bool

	cmd := &cobra.Command{
		Use:   "quality <query>",
		Short: "Estimate study quality from design, venue, citations, and sample-size cues",
		Long: "Score each retrieved work 0–100 from a weighted blend of study design (strongest\n" +
			"signal), citation mass, venue presence, and sample-size cues, then summarize the\n" +
			"grade distribution. A coarse triage signal, not a formal risk-of-bias assessment.",
		Example:     "  scientific-consensus-pp-cli quality \"omega-3 depression\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would estimate study quality")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			works, _, err := fetchWorks(ctx, c, query, "", "relevance_score:desc", limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if enrich {
				enrichPubTypes(ctx, works, 50)
			}
			out := qualityOutput{Query: query, Analyzed: len(works), GradeCounts: map[string]int{}}
			scored := make([]qualityWork, 0, len(works))
			var sum float64
			prog := newProgress(flags, "grading works", len(works))
			for i, wk := range works {
				cls := scengine.ClassifyDesign(wk.Title, wk.Abstract, wk.Type, wk.PubTypes)
				score := qualityScore(cls.Design, wk.CitedBy, wk.Venue, wk.Abstract)
				g := grade(score)
				out.GradeCounts[g]++
				sum += float64(score)
				scored = append(scored, qualityWork{
					Title: wk.Title, Year: wk.Year, DOI: wk.DOI, Design: cls.Design,
					CitedBy: wk.CitedBy, Venue: wk.Venue, Score: score, Grade: g,
				})
				prog.update(i + 1)
			}
			prog.done()
			if len(scored) > 0 {
				out.MeanScore = round1(sum / float64(len(scored)))
			}
			sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
			if len(scored) > 5 {
				out.TopWorks = scored[:5]
			} else {
				out.TopWorks = scored
			}
			if len(works) == 0 {
				out.Note = "no works found; try a broader query"
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderQuality(w, out) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "number of works to score (max 200)")
	cmd.Flags().BoolVar(&enrich, "enrich", true, "enrich classification with PubMed publication types")
	return cmd
}

// qualityScore blends design tier (0..40), citation mass (0..30), venue presence
// (0..15), and sample-size cue (0..15).
func qualityScore(d scengine.Design, citedBy int, venue, abstract string) int {
	rank := scengine.TierRank(d)
	maxRank := scengine.TierRank(scengine.DesignUnknown)
	designPts := 40 * (1 - float64(rank)/float64(maxRank))

	if citedBy < 0 {
		citedBy = 0
	}
	citePts := math.Min(30, 6*math.Log1p(float64(citedBy)))

	venuePts := 0.0
	if venue != "" {
		venuePts = 15
	}

	samplePts := 0.0
	if reMatch(`(?i)\b(n\s?=\s?\d{3,}|\d{4,}\s+(patients|participants|subjects)|nationwide|population-based|multicenter|multicentre)\b`, abstract) {
		samplePts = 15
	}

	score := int(math.Round(designPts + citePts + venuePts + samplePts))
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score
}

func grade(score int) string {
	switch {
	case score >= 80:
		return "A"
	case score >= 65:
		return "B"
	case score >= 45:
		return "C"
	case score >= 25:
		return "D"
	default:
		return "F"
	}
}

func renderQuality(w io.Writer, o qualityOutput) {
	fmt.Fprintf(w, "Study quality: %s\n", o.Query)
	fmt.Fprintf(w, "  %d works scored; mean %.1f/100\n", o.Analyzed, o.MeanScore)
	fmt.Fprint(w, "  grades:")
	for _, g := range []string{"A", "B", "C", "D", "F"} {
		fmt.Fprintf(w, " %s=%d", g, o.GradeCounts[g])
	}
	fmt.Fprint(w, "\n\n")
	for _, wk := range o.TopWorks {
		fmt.Fprintf(w, "  %3d (%s) [%s] %s\n", wk.Score, wk.Grade, wk.Design, truncate(wk.Title, 70))
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}
