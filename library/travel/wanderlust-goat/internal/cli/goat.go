package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/googleplaces"
)

// newGoatCmd is `near` with --llm forced off and the human-table output
// trimmed for token efficiency. Matches the brief: "Same as near but
// explicitly no-LLM (heuristic criteria match)."
func newGoatCmd(flags *rootFlags) *cobra.Command {
	var (
		criteria      string
		identity      string
		minutes       float64
		top           int
		seedLimit     int
		includedTypes []string
		anchorFlag    string
	)
	cmd := &cobra.Command{
		Use:   "goat [anchor]",
		Short: "Same as near, no-LLM, deterministic heuristic criteria match",
		Long: `goat is the no-LLM compound. Identical pipeline to near but the criteria-match
score is computed from the static keyword table in internal/criteria/, never
ANTHROPIC_API_KEY. Use this when you want determinism, when no API budget is
available, or in CI.`,
		Example: strings.Trim(`
  wanderlust-goat-pp-cli goat 35.6895,139.6917 --criteria "bouldering gym" --minutes 20
  wanderlust-goat-pp-cli goat "Hotel Okura" --criteria "kissaten" --minutes 12 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			anchor := anchorFlag
			if anchor == "" && len(args) > 0 {
				anchor = args[0]
			}
			if anchor == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			res, err := dispatch.ResolveAnchor(ctx, anchor)
			if err != nil {
				return apiErr(err)
			}
			d, err := dispatch.New()
			if err != nil {
				if errors.Is(err, googleplaces.ErrMissingAPIKey) {
					return authErr(fmt.Errorf("%w (set GOOGLE_PLACES_API_KEY; doctor will show the setup)", err))
				}
				return configErr(err)
			}
			effectiveTypes := includedTypes
			if len(effectiveTypes) == 0 {
				effectiveTypes = defaultIncludedTypesFromCriteria(criteria, identity)
			}
			plan := dispatch.Plan{
				Anchor:        anchor,
				Resolved:      res,
				Criteria:      criteria,
				Identity:      identity,
				RadiusMinutes: minutes,
				SeedLimit:     seedLimit,
				IncludedTypes: effectiveTypes,
				UseLLM:        false, // brief: explicitly no-LLM
			}
			results, trace, err := d.Run(ctx, plan)
			if err != nil {
				return apiErr(err)
			}
			if top > 0 && len(results) > top {
				results = results[:top]
			}
			out := struct {
				Anchor  dispatch.AnchorResolution `json:"anchor"`
				Results []dispatch.Result         `json:"results"`
				Trace   dispatch.Trace            `json:"trace"`
			}{Anchor: res, Results: results, Trace: trace}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			renderResultsTable(cmd, results, res)
			return nil
		},
	}
	cmd.Flags().StringVar(&anchorFlag, "anchor", "", "anchor as <lat>,<lng> or address (positional also accepted)")
	cmd.Flags().StringVar(&criteria, "criteria", "", "free-text criteria")
	cmd.Flags().StringVar(&identity, "identity", "", "free-text identity / persona")
	cmd.Flags().Float64Var(&minutes, "minutes", 15, "walking-time radius in minutes")
	cmd.Flags().IntVar(&top, "top", 5, "return top N results")
	cmd.Flags().IntVar(&seedLimit, "seed-limit", 20, "max Google Places candidates to seed (1-20)")
	cmd.Flags().StringSliceVar(&includedTypes, "type", nil, "Google Places type filter, repeatable")
	return cmd
}
