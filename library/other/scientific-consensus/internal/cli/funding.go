// Hand-authored novel command: funding-pattern analysis (Crossref). Not generated.
package cli

import (
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/source"
	"github.com/spf13/cobra"
)

type fundingOutput struct {
	Query          string              `json:"query"`
	Sampled        int                 `json:"sampled_works"`
	WithFunder     int                 `json:"works_with_funder"`
	FunderCoverage float64             `json:"funder_coverage_pct"`
	TopFunders     []source.FunderStat `json:"top_funders"`
	Note           string              `json:"note,omitempty"`
}

func newNovelFundingCmd(flags *rootFlags) *cobra.Command {
	var sample, limit int

	cmd := &cobra.Command{
		Use:   "funding <query>",
		Short: "Analyze funding patterns and funder concentration for a topic (via Crossref)",
		Long: "Sample Crossref works for a topic and aggregate their declared funders to show\n" +
			"who funds research on the topic and how concentrated that funding is. Funder\n" +
			"metadata is sparse in Crossref, so coverage is reported alongside the ranking.",
		Example:     "  scientific-consensus-pp-cli funding \"e-cigarette safety\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would analyze funding patterns")
				return nil
			}
			query, err := requireQuery(args)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if cliutil.IsDogfoodEnv() && sample > 50 {
				sample = 50
			}
			stats, withFunder, err := source.CrossrefFunders(ctx, query, sample)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			out := fundingOutput{Query: query, Sampled: sample, WithFunder: withFunder}
			if sample > 0 {
				out.FunderCoverage = round1(float64(withFunder) * 100 / float64(sample))
			}
			if len(stats) > limit {
				stats = stats[:limit]
			}
			out.TopFunders = stats
			if withFunder == 0 {
				out.Note = "no funder metadata found in the Crossref sample; coverage is often sparse for this topic"
			}
			return emit(cmd, flags, out, func(w io.Writer) { renderFunding(w, out) })
		},
	}
	cmd.Flags().IntVar(&sample, "sample", 100, "number of Crossref works to sample (max 100 in dogfood)")
	cmd.Flags().IntVar(&limit, "limit", 15, "number of top funders to show")
	return cmd
}

func renderFunding(w io.Writer, o fundingOutput) {
	fmt.Fprintf(w, "Funding analysis: %s\n", o.Query)
	fmt.Fprintf(w, "  sampled %d works; %d had funder data (%.1f%% coverage)\n\n", o.Sampled, o.WithFunder, o.FunderCoverage)
	for i, f := range o.TopFunders {
		fmt.Fprintf(w, "  %2d. %-56s %d\n", i+1, truncate(f.Funder, 56), f.Works)
	}
	if o.Note != "" {
		fmt.Fprintf(w, "\n  Note: %s\n", o.Note)
	}
}
