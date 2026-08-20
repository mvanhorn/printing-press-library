// Hand-authored novel command: research gap detector. Not generated.
package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
	"github.com/spf13/cobra"
)

type gapFinding struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type gapsOutput struct {
	Query        string       `json:"query"`
	TotalMatches int          `json:"total_matches"`
	Analyzed     int          `json:"analyzed"`
	LatestYear   int          `json:"latest_year"`
	HasRCT       bool         `json:"has_rct_or_stronger"`
	Findings     []gapFinding `json:"findings"`
	Note         string       `json:"note,omitempty"`
}

var populationProbes = map[string]string{
	"pediatric/children":  `(?i)\b(child|children|p(a)?ediatric|infant|adolescent)\b`,
	"older adults":        `(?i)\b(elderly|older adults?|geriatric|aged)\b`,
	"pregnancy":           `(?i)\b(pregnan|maternal|prenatal|gestational)\b`,
	"low-income settings": `(?i)\b(low[- ]income|developing countr|sub-saharan|lmic)\b`,
}

// sortedProbeLabels returns the population probe labels in sorted order so
// findings are emitted deterministically (Go map iteration order is random).
func sortedProbeLabels() []string {
	labels := make([]string, 0, len(populationProbes))
	for label := range populationProbes {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return labels
}

func newNovelGapsCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "gaps <query>",
		Short: "Identify understudied populations, missing study types, and research opportunities",
		Long: "Analyze the corpus for a topic and surface gaps: missing strong study designs\n" +
			"(RCTs, meta-analyses), stale evidence, thin volume, and understudied populations.\n" +
			"Findings are heuristic signals, not a systematic gap analysis.",
		Example:     "  scientific-consensus-pp-cli gaps \"pediatric long covid\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would detect research gaps for the topic")
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
			works, total, err := fetchWorks(ctx, c, query, "", "relevance_score:desc", limit)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := gapsOutput{Query: query, TotalMatches: total, Analyzed: len(works)}
			if len(works) == 0 {
				out.Note = "no works found; this topic itself may be a gap, or the query is too narrow"
				return emit(cmd, flags, out, func(w io.Writer) { renderGaps(w, out) })
			}

			enrichPubTypes(ctx, works, 50)
			cs := classifyWorks(works)
			apexRank := scengine.TierRank(scengine.ApexDesign(cs))
			out.HasRCT = apexRank <= scengine.TierRank(scengine.DesignRCT)

			combined := strings.Builder{}
			prog := newProgress(flags, "scanning works", len(works))
			for i, wk := range works {
				if wk.Year > out.LatestYear {
					out.LatestYear = wk.Year
				}
				combined.WriteString(strings.ToLower(wk.Title))
				combined.WriteByte(' ')
				combined.WriteString(strings.ToLower(wk.Abstract))
				combined.WriteByte(' ')
				prog.update(i + 1)
			}
			prog.done()
			hay := combined.String()

			// Finding: no strong designs.
			if !out.HasRCT {
				out.Findings = append(out.Findings, gapFinding{Kind: "missing-strong-designs",
					Detail: "no randomized trials, systematic reviews, or meta-analyses among the analyzed works — evidence rests on weaker designs"})
			}
			// Finding: stale evidence.
			thisYear := time.Now().Year()
			if out.LatestYear > 0 && thisYear-out.LatestYear >= 4 {
				out.Findings = append(out.Findings, gapFinding{Kind: "stale-evidence",
					Detail: fmt.Sprintf("most recent analyzed study is from %d (%d+ years old) — replication / updates may be missing", out.LatestYear, thisYear-out.LatestYear)})
			}
			// Finding: thin volume.
			if total < 50 {
				out.Findings = append(out.Findings, gapFinding{Kind: "thin-literature",
					Detail: fmt.Sprintf("only %d total works match this topic — an understudied area", total)})
			}
			// Population gaps, in deterministic (sorted-label) order.
			for _, label := range sortedProbeLabels() {
				if !reMatch(populationProbes[label], hay) {
					out.Findings = append(out.Findings, gapFinding{Kind: "understudied-population",
						Detail: "little or no coverage of " + label})
				}
			}
			if len(out.Findings) == 0 {
				out.Findings = append(out.Findings, gapFinding{Kind: "well-covered",
					Detail: "no obvious gaps detected: strong designs present, recent, and reasonably broad"})
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderGaps(w, out) })
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 60, "number of works to analyze (max 200)")
	return cmd
}

func renderGaps(w io.Writer, o gapsOutput) {
	fmt.Fprintf(w, "Research gaps: %s\n", o.Query)
	fmt.Fprintf(w, "  %d total matches; analyzed %d; latest year %d; strong designs: %v\n\n", o.TotalMatches, o.Analyzed, o.LatestYear, o.HasRCT)
	for _, f := range o.Findings {
		fmt.Fprintf(w, "  • [%s] %s\n", f.Kind, f.Detail)
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}
