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

// newNearCmd is the headline two-stage funnel command. Stage 1 seeds from
// Google Places; Stage 2 deep-researches via the regions table; Stage 3
// ranks with trust weights and returns the top N.
func newNearCmd(flags *rootFlags) *cobra.Command {
	var (
		criteria      string
		identity      string
		minutes       float64
		top           int
		seedLimit     int
		includedTypes []string
		useLLM        bool
		anchorFlag    string
	)
	cmd := &cobra.Command{
		Use:   "near [anchor]",
		Short: "Find 3-5 amazing things within walking distance — two-stage funnel",
		Long: `near runs the two-stage funnel:
  Stage 1: seed candidates via Google Places (within walking minutes)
  Stage 2: deep-research each candidate against locale-aware sources
  Stage 3: trust-weighted rank with closed-signal kill-gate

Anchor accepts a free-text address ("Park Hyatt Tokyo"), or "<lat>,<lng>".
Walking radius is computed as minutes × 4.5 km/h ÷ 1.3 tortuosity, not crow-flies meters.`,
		Example: strings.Trim(`
  wanderlust-goat-pp-cli near "Park Hyatt Tokyo" --criteria "vintage jazz kissaten with no tourists" --identity "coffee snob into 70s kissaten culture" --minutes 15
  wanderlust-goat-pp-cli near 35.6895,139.6917 --criteria "high-end seafood with counter seating" --minutes 12 --json
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
				UseLLM:        useLLM,
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
				Region  string                    `json:"region"`
				Results []dispatch.Result         `json:"results"`
				Trace   dispatch.Trace            `json:"trace"`
			}{
				Anchor: res, Region: trace.Region.PrimaryLanguage, Results: results, Trace: trace,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			renderResultsTable(cmd, out.Results, out.Anchor)
			return nil
		},
	}
	cmd.Flags().StringVar(&anchorFlag, "anchor", "", "anchor as <lat>,<lng> or address (positional also accepted)")
	cmd.Flags().StringVar(&criteria, "criteria", "", "free-text criteria (e.g. \"vintage jazz kissaten with no tourists\")")
	cmd.Flags().StringVar(&identity, "identity", "", "free-text identity / persona (e.g. \"coffee snob into 70s kissaten culture\")")
	cmd.Flags().Float64Var(&minutes, "minutes", 15, "walking-time radius in minutes (4.5 km/h × 1.3 tortuosity)")
	cmd.Flags().IntVar(&top, "top", 5, "return top N results (default 5)")
	cmd.Flags().IntVar(&seedLimit, "seed-limit", 20, "max Google Places candidates to seed (1-20)")
	cmd.Flags().StringSliceVar(&includedTypes, "type", nil, "Google Places type filter, repeatable (cafe, restaurant, etc.)")
	cmd.Flags().BoolVar(&useLLM, "llm", false, "use ANTHROPIC_API_KEY for sharper criteria judgment (default heuristic)")
	return cmd
}

// renderResultsTable is the compact human view used when stdout is a TTY
// and --json is not set.
func renderResultsTable(cmd *cobra.Command, results []dispatch.Result, anchor dispatch.AnchorResolution) {
	w := newTabWriter(cmd.OutOrStdout())
	defer w.Flush()
	fmt.Fprintf(w, "%s\nresults near %s (%.4f, %.4f, country %s):\n", bold("wanderlust-goat"), anchor.Display, anchor.Lat, anchor.Lng, anchor.Country)
	fmt.Fprintln(w, "RANK\tNAME\tWALK (min)\tSCORE\tWHY")
	for i, r := range results {
		fmt.Fprintf(w, "%d\t%s\t%.1f\t%.2f\t%s\n", i+1, truncate(r.Name, 40), r.WalkingMinutes, r.Score.Total, truncate(r.Why, 60))
	}
}
