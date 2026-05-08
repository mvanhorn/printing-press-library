package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/closedsignal"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/dispatch"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/googleplaces"
	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/regions"
)

// newWhyCmd prints every source that mentioned a place, the trust weight,
// country boost, walking time, criteria match, and the final goat-score
// breakdown — auditable per the brief.
func newWhyCmd(flags *rootFlags) *cobra.Command {
	var (
		anchor   string
		country  string
		criteria string
		minutes  float64
	)
	cmd := &cobra.Command{
		Use:   "why [place-name]",
		Short: "Score breakdown: every source, trust weight, country boost, criteria match",
		Long: `why prints the audit trail for one place. Stage-1 (Google Places SearchText)
finds the canonical entry; the regions table drives Stage-2 fanout; the
score is the same trust-weighted formula near and goat use.

Use this when a ranking surprises you, when you need cited evidence, or when
debugging dispatcher behavior.`,
		Example: strings.Trim(`
  wanderlust-goat-pp-cli why "Bear Pond Espresso" --json
  wanderlust-goat-pp-cli why "Le Coucou" --country FR --criteria "natural wine"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.TrimSpace(strings.Join(args, " "))
			if name == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			cc := strings.ToUpper(strings.TrimSpace(country))
			if cc == "" && anchor != "" {
				if r, err := dispatch.ResolveAnchor(ctx, anchor); err == nil {
					cc = r.Country
				}
			}
			if cc == "" {
				cc = "*"
			}
			region := regions.Lookup(cc)

			// Use Google Places SearchText to resolve the canonical
			// candidate. Without an API key, we skip Stage-1 and surface
			// only the Stage-2 evidence the dispatcher would have walked.
			g, gerr := googleplaces.NewClient()
			if errors.Is(gerr, googleplaces.ErrMissingAPIKey) {
				return authErr(fmt.Errorf("%w (set GOOGLE_PLACES_API_KEY)", gerr))
			} else if gerr != nil {
				return configErr(gerr)
			}
			places, err := g.SearchText(ctx, name, 0, 0, 0, 1, region.PrimaryLanguage)
			if err != nil {
				return apiErr(err)
			}
			if len(places) == 0 {
				return notFoundErr(fmt.Errorf("no Google Places match for %q", name))
			}
			top := places[0]

			anchorRes := dispatch.AnchorResolution{
				Lat: top.Lat, Lng: top.Lng, Country: cc, Display: top.Address,
			}
			plan := dispatch.Plan{
				Anchor:        name,
				Resolved:      anchorRes,
				Criteria:      criteria,
				RadiusMinutes: minutes,
				SeedLimit:     5,
			}
			d := dispatch.NewWithSeed(&singlePlaceSeed{place: top}, dispatch.DefaultRegistry())
			results, trace, err := d.Run(ctx, plan)
			if err != nil {
				return apiErr(err)
			}
			out := struct {
				Name      string                 `json:"name"`
				Region    regions.Region         `json:"region"`
				Top       *dispatch.Result       `json:"top,omitempty"`
				Trace     dispatch.Trace         `json:"trace"`
				ClosedAll []closedsignal.Verdict `json:"closed_signals,omitempty"`
			}{
				Name:   name,
				Region: region,
				Trace:  trace,
			}
			if len(results) > 0 {
				r := results[0]
				out.Top = &r
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&anchor, "anchor", "", "anchor (used to derive country)")
	cmd.Flags().StringVar(&country, "country", "", "ISO 3166-1 alpha-2 country code")
	cmd.Flags().StringVar(&criteria, "criteria", "", "free-text criteria for criteria-match score component")
	cmd.Flags().Float64Var(&minutes, "minutes", 15, "walking-time radius in minutes (informational)")
	return cmd
}

// singlePlaceSeed wraps a single googleplaces.Place as a SeedClient. Lets
// `why` reuse the dispatcher's Stage-2/Stage-3 paths for one resolved place.
type singlePlaceSeed struct{ place googleplaces.Place }

func (s *singlePlaceSeed) NearbySearch(ctx context.Context, lat, lng, radius float64, types []string, max int, lang string) ([]googleplaces.Place, error) {
	return []googleplaces.Place{s.place}, nil
}
