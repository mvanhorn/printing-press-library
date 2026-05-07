package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/criteria"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
)

func newResearchPlanCmd(flags *rootFlags) *cobra.Command {
	var (
		criteriaText string
		identity     string
		minutes      int
	)
	cmd := &cobra.Command{
		Use:   "research-plan <anchor>",
		Short: "Output a JSON query plan agents execute in a loop — typed, country-aware, ordered by trust, ready to fan out.",
		Long: `Emit a JSON-shaped query plan for an agent to execute. The plan resolves
the anchor through Nominatim, classifies criteria, and lists every typed
client call (with parameters, locale, expected trust) the agent should run.
The CLI does not execute the plan — that's the agent's job. Drop into an
agent loop to fan out across editorial + Wikipedia + Reddit + OSM + the
country's local-language sources without re-deriving the dispatch each call.`,
		Example: strings.Trim(`
  # Emit the plan and pipe to jq
  wanderlust-goat-pp-cli research-plan "Bukchon Hanok Village, Seoul" \
    --criteria "hand-pulled noodles, locals only" --identity "food traveler" --json

  # Narrow to client + method only
  wanderlust-goat-pp-cli research-plan "Marais, Paris" \
    --criteria "natural wine, no scene" \
    --json --select sources,calls.client,calls.method`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			anchor := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			res, err := resolveAnchor(ctx, anchor)
			if err != nil {
				return err
			}
			parsed := criteria.Parse(criteriaText)
			plan := dispatch.Build(dispatch.AnchorRef{
				Query: res.Query, Lat: res.Lat, Lng: res.Lng,
				Country: res.Country, Display: res.Display,
			}, parsed.Intent, parsed.RedditKW, walkingMinutesToMeters(minutes))
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		},
	}
	cmd.Flags().StringVar(&criteriaText, "criteria", "", "Free-text criteria for the plan.")
	cmd.Flags().StringVar(&identity, "identity", "", "Free-text identity (informational only — does not change the plan structure).")
	cmd.Flags().IntVar(&minutes, "minutes", 15, "Walking-time radius in minutes (drives radius_meters in each call).")
	return cmd
}
