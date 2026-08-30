// Hand-authored novel command: side-by-side claim comparison. Not generated.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
	"github.com/spf13/cobra"
)

type compareOutput struct {
	ClaimA   consensusOutput `json:"claim_a"`
	ClaimB   consensusOutput `json:"claim_b"`
	Stronger string          `json:"stronger_support"`
	Note     string          `json:"note,omitempty"`
}

// computeConsensus runs the full fetch -> classify -> score -> consensus pass
// for one claim. Shared by `compare` and `batch`.
func computeConsensus(ctx context.Context, c apiGetter, claim string, limit, yearFrom int, enrich bool) (consensusOutput, error) {
	filter := ""
	if yearFrom > 0 {
		filter = fmt.Sprintf("from_publication_date:%d-01-01", yearFrom)
	}
	works, _, err := fetchWorks(ctx, c, claim, filter, "relevance_score:desc", limit)
	if err != nil {
		return consensusOutput{}, err
	}
	// Relevance gate before enrichment, matching consensus: an excluded work
	// must cost no PubMed lookup and must not reach the score. Compare scores
	// each claim independently, so an off-topic work on one side skews that
	// side's verdict and the comparison built from it.
	works = filterRelevant(claim, works)
	// Same retraction gate the consensus command applies. computeConsensus is
	// the scoring path for BOTH compare and batch, so leaving it out here
	// would let a retracted work set the evidence tier of one side of a
	// comparison while the standalone command excluded it.
	works, retracted := filterRetracted(works)
	if enrich {
		enrichPubTypes(ctx, works, 50)
	}
	scored, stances := scoreWorks(ctx, works, claim)
	r := scengine.Consensus(scored)
	out := consensusOutput{
		Claim: claim, Verdict: r.Verdict, ConsensusScore: r.ConsensusScore, Confidence: r.Confidence,
		EvidenceStrength: r.EvidenceStrength, ApexDesign: r.ApexDesign, StudyCount: r.StudyCount,
		Supporting: r.Supporting, Refuting: r.Refuting, Mixed: r.Mixed, Inconclusive: r.Inconclusive,
		TotalCitations: r.TotalCitations, NearUnanimous: r.NearUnanimous,
		RetractedExcluded: len(retracted),
		Method:            stanceMethodLabel(stances),
		TopSupporting:     topByStance(stances, scengine.StanceSupporting, 2),
		TopRefuting:       topByStance(stances, scengine.StanceRefuting, 2),
		AllStudies:        append(allStudyBriefs(stances), retractedBriefs(retracted)...),
	}
	return out, nil
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var enrich bool

	cmd := &cobra.Command{
		Use:   "compare <claim-a> <claim-b>",
		Short: "Run two consensus analyses side-by-side to compare competing claims",
		Long: "Run the consensus engine for two claims and present them side by side, with the\n" +
			"more strongly supported claim flagged. Use this to weigh competing interventions\n" +
			"or contradictory claims.",
		Example:     "  scientific-consensus-pp-cli compare \"statins reduce mortality\" \"statins increase diabetes risk\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare two claims")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("two claim arguments are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			prog := newProgress(flags, "analyzing claims", 2)
			prog.update(1)
			a, err := computeConsensus(ctx, c, args[0], limit, 0, enrich)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			prog.update(2)
			b, err := computeConsensus(ctx, c, args[1], limit, 0, enrich)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			prog.done()
			out := compareOutput{ClaimA: a, ClaimB: b}
			switch {
			case a.ConsensusScore > b.ConsensusScore+0.05:
				out.Stronger = a.Claim
			case b.ConsensusScore > a.ConsensusScore+0.05:
				out.Stronger = b.Claim
			default:
				out.Stronger = "comparable"
			}
			out.Note = compareNote(a, b)
			return emit(cmd, flags, out, func(w io.Writer) { renderCompare(w, out) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 40, "number of works to analyze per claim (max 200)")
	cmd.Flags().BoolVar(&enrich, "enrich", true, "enrich classification with PubMed publication types")
	return cmd
}

// compareNote is the advisory note for a two-claim comparison. An empty
// StudyCount is not the same as "returned no works": a retracted-empty side
// still has RetractedExcluded and AllStudies populated, and those works
// already passed the relevance gate. The wording matches consensusNote —
// retracted works are "relevant work(s) ... excluded as retracted" — so the
// note cannot contradict the retracted studies in the same result.
func compareNote(a, b consensusOutput) string {
	if a.StudyCount > 0 && b.StudyCount > 0 {
		return ""
	}
	aRetracted := emptyDueToRetraction(a)
	bRetracted := emptyDueToRetraction(b)
	aMissing := a.StudyCount == 0 && !aRetracted
	bMissing := b.StudyCount == 0 && !bRetracted
	switch {
	case (aRetracted || bRetracted) && (aMissing || bMissing):
		return "one claim returned no works and the other had all fetched work(s) excluded as retracted; comparison may be unreliable"
	case aRetracted || bRetracted:
		return "one or both claims had all fetched work(s) excluded as retracted; comparison may be unreliable"
	default:
		return "one or both claims returned no works; comparison may be unreliable"
	}
}

// emptyDueToRetraction reports that a side has no scorable studies because
// every relevant work was excluded as retracted. RetractedExcluded is the
// primary signal; AllStudies is the fallback when the count was not set but
// the retracted works are still listed.
func emptyDueToRetraction(o consensusOutput) bool {
	if o.StudyCount != 0 {
		return false
	}
	return o.RetractedExcluded > 0 || len(o.AllStudies) > 0
}

func renderCompare(w io.Writer, o compareOutput) {
	row := func(label, av, bv string) { fmt.Fprintf(w, "  %-18s %-32s %-32s\n", label, av, bv) }
	fmt.Fprintln(w, "Claim comparison:")
	row("", truncate(o.ClaimA.Claim, 30), truncate(o.ClaimB.Claim, 30))
	row("verdict", string(o.ClaimA.Verdict), string(o.ClaimB.Verdict))
	row("consensus score", fmt.Sprintf("%+.2f", o.ClaimA.ConsensusScore), fmt.Sprintf("%+.2f", o.ClaimB.ConsensusScore))
	row("confidence", fmt.Sprintf("%.0f%%", o.ClaimA.Confidence*100), fmt.Sprintf("%.0f%%", o.ClaimB.Confidence*100))
	row("evidence strength", string(o.ClaimA.EvidenceStrength), string(o.ClaimB.EvidenceStrength))
	row("studies", fmt.Sprintf("%d", o.ClaimA.StudyCount), fmt.Sprintf("%d", o.ClaimB.StudyCount))
	fmt.Fprintf(w, "\n  Stronger support: %s\n", o.Stronger)
	if o.Note != "" {
		fmt.Fprintf(w, "  Note: %s\n", o.Note)
	}
}
