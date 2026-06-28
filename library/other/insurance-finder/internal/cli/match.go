package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/insurance-finder/internal/insurance"
	"github.com/spf13/cobra"
)

func newMatchCmd(f *rootFlags) *cobra.Command {
	var shortlistOnly bool
	cmd := &cobra.Command{
		Use:   "match",
		Short: "Rank providers for your profile (importer -> specialty markets)",
		Long:  "match ranks the registry for your saved profile and explains why. An importer / private-label / manufacturer is routed to specialty markets; the mainstream instant-quote carriers that decline that class are pushed to an 'avoid' tier.",
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
			out := recs
			if shortlistOnly {
				out = insurance.Shortlist(recs)
			}
			return f.emit(cmd, func(w io.Writer) { renderMatch(w, p, out) }, out)
		},
	}
	cmd.Flags().BoolVar(&shortlistOnly, "shortlist", false, "Only show recommended/consider tiers (drop the avoid tier)")
	return cmd
}

func renderMatch(w io.Writer, p insurance.Profile, recs []insurance.Recommendation) {
	if p.IsImporterClass() {
		fmt.Fprintf(w, "%s %s\n\n", bold("Profile:"), yellow("importer / private-label / manufacturer (deemed manufacturer) -> route to specialty markets"))
	} else {
		fmt.Fprintf(w, "%s low-hazard reseller/service -> mainstream instant-quote carriers are a fine first stop\n\n", bold("Profile:"))
	}
	groups := []struct {
		tier  string
		label string
	}{
		{insurance.TierRecommended, green("RECOMMENDED")},
		{insurance.TierConsider, "CONSIDER"},
		{insurance.TierAvoid, red("AVOID")},
	}
	for _, g := range groups {
		first := true
		for _, r := range recs {
			if r.Tier != g.tier {
				continue
			}
			if first {
				fmt.Fprintf(w, "%s\n", bold(g.label))
				first = false
			}
			fmt.Fprintf(w, "  %2d. %-22s %s\n", r.Rank, r.Provider.ID, dim(fmt.Sprintf("(score %d, %s)", r.Score, r.Provider.Type)))
			for _, reason := range r.Reasons {
				fmt.Fprintf(w, "      - %s\n", reason)
			}
			for _, c := range r.Cautions {
				fmt.Fprintf(w, "      %s %s\n", yellow("!"), c)
			}
		}
		if !first {
			fmt.Fprintln(w)
		}
	}
	fmt.Fprintf(w, "Next: %s for URLs + paste-ready answer sheets.\n", bold("insurance-finder guide"))
}
