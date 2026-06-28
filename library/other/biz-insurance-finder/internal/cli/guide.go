package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

// guideProvider bundles everything a user needs to drive one provider's quote.
type guideProvider struct {
	Rank        int                         `json:"rank"`
	Provider    insurance.Provider          `json:"provider"`
	Tier        string                      `json:"tier"`
	Score       int                         `json:"score"`
	Reasons     []string                    `json:"reasons,omitempty"`
	Cautions    []string                    `json:"cautions,omitempty"`
	AnswerSheet insurance.AnswerSheet       `json:"answer_sheet"`
	Checklist   insurance.ProviderChecklist `json:"checklist"`
}

// guideOutput is the full guided plan.
type guideOutput struct {
	ImporterClass bool                `json:"importer_class"`
	Warnings      []insurance.Warning `json:"warnings"`
	Providers     []guideProvider     `json:"providers"`
}

func newGuideCmd(f *rootFlags) *cobra.Command {
	var top int
	var all bool
	cmd := &cobra.Command{
		Use:   "guide",
		Short: "Full guided plan: warnings + per-provider URL, answer sheet, and checklist",
		Long: `guide is the end-to-end walkthrough. It prints the underwriting warnings for
your profile, then for each shortlisted provider: the quote-start URL, a
paste-ready answer sheet, and the manual-actions checklist. It guides you
through your own browser - it never fills, submits, or pays on your behalf.

Two-gate submit rule: review the filled values, then YOU click submit.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := f.loadProfile()
			if err != nil {
				return err
			}
			reg, err := f.loadRegistry()
			if err != nil {
				return err
			}
			recs := insurance.Match(p, reg.Providers)
			if !all {
				recs = insurance.Shortlist(recs)
			}
			if top > 0 && len(recs) > top {
				recs = recs[:top]
			}

			out := guideOutput{
				ImporterClass: p.IsImporterClass(),
				Warnings:      insurance.Warnings(p),
			}
			for _, r := range recs {
				out.Providers = append(out.Providers, guideProvider{
					Rank:        r.Rank,
					Provider:    r.Provider,
					Tier:        r.Tier,
					Score:       r.Score,
					Reasons:     r.Reasons,
					Cautions:    r.Cautions,
					AnswerSheet: insurance.GenerateAnswerSheet(p, r.Provider),
					Checklist:   insurance.GenerateChecklist(r.Provider),
				})
			}
			return f.emit(cmd, func(w io.Writer) { renderGuide(w, out) }, out)
		},
	}
	cmd.Flags().IntVar(&top, "top", 3, "Limit to the top N providers (0 = no limit)")
	cmd.Flags().BoolVar(&all, "all", false, "Include the avoid tier too")
	return cmd
}

func renderGuide(w io.Writer, out guideOutput) {
	fmt.Fprintf(w, "%s\n", bold("=== Insurance quote guide ==="))
	if out.ImporterClass {
		fmt.Fprintf(w, "%s\n", yellow("You are a deemed manufacturer (importer/private-label). Routed to specialty markets."))
	}
	fmt.Fprintln(w)

	renderWarnings(w, out.Warnings)
	fmt.Fprintln(w)

	for i, gp := range out.Providers {
		fmt.Fprintf(w, "%s\n", bold(fmt.Sprintf("--- %d. %s [%s] ---", gp.Rank, gp.Provider.Name, gp.Tier)))
		fmt.Fprintf(w, "%s %s\n", dim("Start here:"), gp.Provider.StartURL())
		for _, reason := range gp.Reasons {
			fmt.Fprintf(w, "  - %s\n", reason)
		}
		for _, c := range gp.Cautions {
			fmt.Fprintf(w, "  %s %s\n", yellow("!"), c)
		}
		fmt.Fprintln(w)
		renderAnswerSheet(w, gp.AnswerSheet)
		fmt.Fprintln(w)
		renderChecklist(w, gp.Checklist)
		fmt.Fprintf(w, "\n  %s review every value above, then YOU click submit. The tool does not.\n",
			yellow("TWO-GATE SUBMIT:"))
		if i < len(out.Providers)-1 {
			fmt.Fprintln(w, dim("\n----------------------------------------\n"))
		}
	}
}
