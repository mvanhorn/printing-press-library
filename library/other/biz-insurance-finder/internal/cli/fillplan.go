package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/biz-insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

func newFillPlanCmd(f *rootFlags) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "fill-plan [provider-id]",
		Short: "Emit a machine-fillable plan (auto-fill values + the human-gated steps)",
		Long: `fill-plan turns your saved profile into a structured plan a browser agent can
drive: the quote URL, the fields to auto-fill (the data you already provided),
and the human-gated steps it must STOP for - CAPTCHA, account/password,
EIN/SSN, payment, and the final submit.

This is the data layer for the agent-driven filler: the tool fills what you
already gave it and hands off CAPTCHA / IDs / payment / submit to you. With no
id it uses your top match; with --all it emits one plan per shortlisted
provider. Pair with '--json' to feed a browser automation agent.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := f.loadProfile()
			if err != nil {
				return err
			}
			reg, err := f.loadRegistry()
			if err != nil {
				return err
			}
			if all {
				recs := insurance.Shortlist(insurance.Match(p, reg.Providers))
				plans := make([]insurance.FillPlan, 0, len(recs))
				for _, r := range recs {
					plans = append(plans, insurance.GenerateFillPlan(p, r.Provider))
				}
				return f.emit(cmd, func(w io.Writer) {
					for i, pl := range plans {
						if i > 0 {
							fmt.Fprintln(w)
						}
						renderFillPlan(w, pl)
					}
				}, plans)
			}
			prov, err := resolveProvider(f, reg, p, args)
			if err != nil {
				return err
			}
			plan := insurance.GenerateFillPlan(p, prov)
			return f.emit(cmd, func(w io.Writer) { renderFillPlan(w, plan) }, plan)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Emit a fill plan for every shortlisted provider")
	return cmd
}

func renderFillPlan(w io.Writer, fp insurance.FillPlan) {
	fmt.Fprintf(w, "%s fill plan\n", bold(fp.ProviderName))
	fmt.Fprintf(w, "%s %s\n\n", dim("Open:"), fp.QuoteURL)
	fmt.Fprintf(w, "%s (the filler types these - data you already provided)\n", green("AUTO-FILL"))
	for _, fld := range fp.AutoFill {
		fmt.Fprintf(w, "  %-38s %s\n", fld.Label+":", fld.Value)
	}
	fmt.Fprintf(w, "\n%s (the filler STOPS - you do these)\n", yellow("HUMAN GATES"))
	for _, g := range fp.HumanGates {
		fmt.Fprintf(w, "  %s %s\n", yellow("[!]"), bold(g.Label))
		fmt.Fprintf(w, "      %s\n", dim(g.Note))
	}
}
