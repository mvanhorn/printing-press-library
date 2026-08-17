// Hand-authored novel command: consensus engine. Not generated.
package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
	"github.com/spf13/cobra"
)

type workBrief struct {
	Title      string          `json:"title"`
	Year       int             `json:"year,omitempty"`
	DOI        string          `json:"doi,omitempty"`
	CitedBy    int             `json:"cited_by_count"`
	Design     scengine.Design `json:"design"`
	Stance     scengine.Stance `json:"stance"`
	StanceConf float64         `json:"stance_confidence"`
}

type consensusOutput struct {
	Claim            string                    `json:"claim"`
	Verdict          scengine.Verdict          `json:"verdict"`
	ConsensusScore   float64                   `json:"consensus_score"`
	Confidence       float64                   `json:"confidence"`
	EvidenceStrength scengine.EvidenceStrength `json:"evidence_strength"`
	ApexDesign       scengine.Design           `json:"apex_design"`
	StudyCount       int                       `json:"study_count"`
	Supporting       int                       `json:"supporting"`
	Refuting         int                       `json:"refuting"`
	Mixed            int                       `json:"mixed"`
	Inconclusive     int                       `json:"inconclusive"`
	TotalCitations   int                       `json:"total_citations"`
	Method           string                    `json:"stance_method"`
	TopSupporting    []workBrief               `json:"top_supporting"`
	TopRefuting      []workBrief               `json:"top_refuting"`
	Note             string                    `json:"note,omitempty"`
}

func newNovelConsensusCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var yearFrom int
	var enrich bool

	cmd := &cobra.Command{
		Use:   "consensus <claim>",
		Short: "Answer 'what does the evidence say about X' with a consensus score across sources",
		Long: "Fetch the most relevant works for a claim, classify each study's design and\n" +
			"stance, and compute a tier- and citation-weighted Consensus Score, Confidence,\n" +
			"and Evidence Strength. Stance is heuristic without an AI key. Do NOT treat the\n" +
			"score as a peer-reviewed conclusion; use `evidence` to inspect study designs.",
		Example:     "  scientific-consensus-pp-cli consensus \"vitamin D reduces respiratory infections\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would analyze consensus for the claim")
				return nil
			}
			claim, err := requireQuery(args)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			filter := ""
			if yearFrom > 0 {
				filter = fmt.Sprintf("from_publication_date:%d-01-01", yearFrom)
			}
			works, _, err := fetchWorks(ctx, c, claim, filter, "relevance_score:desc", limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if enrich {
				enrichPubTypes(ctx, works, 50)
			}

			scored, stances := scoreWorks(works, claim)
			result := scengine.Consensus(scored)

			out := consensusOutput{
				Claim: claim, Verdict: result.Verdict, ConsensusScore: result.ConsensusScore,
				Confidence: result.Confidence, EvidenceStrength: result.EvidenceStrength,
				ApexDesign: result.ApexDesign, StudyCount: result.StudyCount,
				Supporting: result.Supporting, Refuting: result.Refuting, Mixed: result.Mixed,
				Inconclusive: result.Inconclusive, TotalCitations: result.TotalCitations,
				Method: "heuristic",
			}
			out.TopSupporting = topByStance(stances, scengine.StanceSupporting, 3)
			out.TopRefuting = topByStance(stances, scengine.StanceRefuting, 3)
			if result.StudyCount == 0 {
				out.Note = "no works found; try a broader claim or --data-source live"
			} else if result.Verdict == scengine.VerdictInsufficient {
				out.Note = "fewer than 3 directional studies; treat as preliminary"
			}

			return emit(cmd, flags, out, func(w io.Writer) { renderConsensus(w, out) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 40, "number of works to analyze (max 200)")
	cmd.Flags().IntVar(&yearFrom, "year-from", 0, "only include works published from this year onward")
	cmd.Flags().BoolVar(&enrich, "enrich", true, "enrich study-design classification with PubMed publication types")
	return cmd
}

func topByStance(stances []workStance, stance scengine.Stance, n int) []workBrief {
	matches := make([]workStance, 0)
	for _, s := range stances {
		if s.Stance == stance {
			matches = append(matches, s)
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].Work.CitedBy > matches[j].Work.CitedBy })
	if len(matches) > n {
		matches = matches[:n]
	}
	out := make([]workBrief, 0, len(matches))
	for _, m := range matches {
		out = append(out, workBrief{
			Title: m.Work.Title, Year: m.Work.Year, DOI: m.Work.DOI, CitedBy: m.Work.CitedBy,
			Design: m.Design, Stance: m.Stance, StanceConf: m.Confidence,
		})
	}
	return out
}

func renderConsensus(w io.Writer, o consensusOutput) {
	fmt.Fprintf(w, "Claim: %s\n\n", o.Claim)
	fmt.Fprintf(w, "  Verdict:           %s\n", o.Verdict)
	fmt.Fprintf(w, "  Consensus score:   %+.2f  (-1 refute … +1 support)\n", o.ConsensusScore)
	fmt.Fprintf(w, "  Confidence:        %.0f%%\n", o.Confidence*100)
	fmt.Fprintf(w, "  Evidence strength: %s (apex: %s)\n", o.EvidenceStrength, o.ApexDesign)
	fmt.Fprintf(w, "  Studies analyzed:  %d  (support %d / refute %d / mixed %d / inconclusive %d)\n",
		o.StudyCount, o.Supporting, o.Refuting, o.Mixed, o.Inconclusive)
	fmt.Fprintf(w, "  Total citations:   %d\n", o.TotalCitations)
	fmt.Fprintf(w, "  Stance method:     %s\n", o.Method)
	if len(o.TopSupporting) > 0 {
		fmt.Fprintln(w, "\n  Top supporting:")
		for _, b := range o.TopSupporting {
			fmt.Fprintf(w, "    • [%d, cites=%d, %s] %s\n", b.Year, b.CitedBy, b.Design, truncate(b.Title, 80))
		}
	}
	if len(o.TopRefuting) > 0 {
		fmt.Fprintln(w, "\n  Top refuting:")
		for _, b := range o.TopRefuting {
			fmt.Fprintf(w, "    • [%d, cites=%d, %s] %s\n", b.Year, b.CitedBy, b.Design, truncate(b.Title, 80))
		}
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}
