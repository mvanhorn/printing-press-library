// Hand-rewritten in Phase 3 to delegate to the cross-network source clients.
// The generated scaffold called `client.Get("/restaurants", params)` against
// opentable.com which 404s; restaurants list is the primary discovery surface
// so it must work cross-network. We delegate to the same code path `goat`
// uses to ensure consistency.

package cli

// PATCH: scaffold-endpoint-redirects — see .printing-press-patches.json for the change-set rationale.
// PATCH: location-native-redesign — U6 wires `restaurants list` to the typed
// ResolveLocation pipeline. The previous `_ = flagMetro` discard at the
// bottom of the func was issue #406 repro 1: --metro was parsed and then
// dropped on the floor before reaching the query. U6 replaces that with a
// proper resolve-and-filter loop plus a new --location/--batch-accept-ambiguous
// pair that supersedes --metro going forward.

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
)

// metroDeprecationOnce gates the once-per-process stderr warning emitted
// when a caller uses --metro instead of --location. Using sync.Once keeps
// the warning idempotent without a hand-rolled mutex+bool pair. Tests
// reset it via resetMetroDeprecationWarning() so each scenario can pin
// the first-call behavior.
var metroDeprecationOnce sync.Once

// resetMetroDeprecationWarning re-arms the metroDeprecationOnce gate so
// tests can pin the first-call stderr emission independently per case.
// Production code never calls this — it exists purely for test
// isolation, similar to the test-only resets in metro_hydration_test.go.
func resetMetroDeprecationWarning() {
	metroDeprecationOnce = sync.Once{}
}

func newRestaurantsListCmd(flags *rootFlags) *cobra.Command {
	var flagQuery string
	var flagLatitude float64
	var flagLongitude float64
	var flagLocation string
	var flagAcceptAmbiguous bool
	var flagMetro string
	var flagNeighborhood string
	var flagCuisine string
	var flagPriceBand int
	var flagAccolade string
	var flagPartySize int
	var flagNetwork string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List restaurants across OpenTable and Tock",
		Long: "Cross-network restaurant search backed by Surf-cleared OpenTable SSR " +
			"and Tock SSR. Identical underlying data path as `goat`; this command " +
			"is the resource-style entry point.",
		Example:     "  table-reservation-goat-pp-cli restaurants list --query 'omakase' --party 2 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve --location / --metro into a typed GeoContext (or a
			// disambiguation envelope) before any provider call so the
			// pre-filter (lat/lng injected into goatQueryOpenTable /
			// goatQueryTock) and the post-filter (applyGeoFilter) share a
			// single source of truth.
			gc, envelope, locationErr, acceptedAmbiguous := resolveLocationFlags(
				cmd.ErrOrStderr(),
				flagLocation,
				flagMetro,
				flagAcceptAmbiguous,
			)
			if locationErr != nil {
				return locationErr
			}
			if envelope != nil {
				// Disambiguation envelope replaces the result list entirely.
				// printJSONFiltered marshals through the same --select /
				// --compact / --quiet pipeline so agents get a uniform shape.
				return printJSONFiltered(cmd.OutOrStdout(), envelope, flags)
			}

			lat, lng := flagLatitude, flagLongitude
			if gc != nil {
				// Pre-filter: project the GeoContext into provider input.
				// The ForOpenTable() projection carries lat/lng only today;
				// Tock takes city+slug+lat/lng but goatQueryTock derives
				// city from the metro arg, so feed the gc centroid via
				// lat/lng for both legs.
				ot := gc.ForOpenTable()
				lat, lng = ot.Lat, ot.Lng
			}

			if dryRunOK(flags) {
				resp := goatResponse{
					Query:     flagQuery,
					Results:   []goatResult{{Network: "opentable", Name: "(dry-run sample)", MatchScore: 1.0}},
					Sources:   []string{"opentable", "tock"},
					QueriedAt: time.Now().UTC().Format(time.RFC3339),
				}
				resp.LocationResolved, resp.LocationWarning = decorateForList(gc, acceptedAmbiguous)
				return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			net := strings.ToLower(flagNetwork)
			results := []goatResult{}
			errors := []string{}
			sources := []string{}
			query := flagQuery
			if net == "" || net == "opentable" {
				sources = append(sources, "opentable")
				if r, err := goatQueryOpenTable(ctx, session, query, lat, lng); err != nil {
					errors = append(errors, "opentable: "+err.Error())
				} else {
					results = append(results, r...)
				}
			}
			if net == "" || net == "tock" {
				sources = append(sources, "tock")
				// restaurants_list doesn't carry a metro/date/time/party context,
				// so fall back to NYC defaults — goatQueryTock applies the same
				// fallbacks internally when these are zero/empty.
				date := time.Now().UTC().Format("2006-01-02")
				if r, err := goatQueryTock(ctx, session, query, "", date, "19:00", 2, lat, lng); err != nil {
					errors = append(errors, "tock: "+err.Error())
				} else {
					results = append(results, r...)
				}
			}

			// Post-filter: when a GeoContext is set, hard-reject results
			// outside the radius (the explicit-flag intent is authoritative).
			// nil gc -> applyGeoFilter is a no-op.
			if gc != nil {
				results = applyGeoFilter(results, gc, metroFilterHardReject)
				// Rank: match score desc, name asc for determinism — matches
				// goat's post-filter ordering so cross-command results sort
				// the same way.
				sort.SliceStable(results, func(i, j int) bool {
					if results[i].MatchScore != results[j].MatchScore {
						return results[i].MatchScore > results[j].MatchScore
					}
					return results[i].Name < results[j].Name
				})
			}

			if flagLimit > 0 && len(results) > flagLimit {
				results = results[:flagLimit]
			}
			resp := goatResponse{
				Query:     query,
				Results:   results,
				Errors:    errors,
				Sources:   sources,
				QueriedAt: time.Now().UTC().Format(time.RFC3339),
			}
			resp.LocationResolved, resp.LocationWarning = decorateForList(gc, acceptedAmbiguous)
			return printJSONFiltered(cmd.OutOrStdout(), resp, flags)
		},
	}
	cmd.Flags().StringVar(&flagQuery, "query", "", "Free-text query (matches name, cuisine, neighborhood)")
	cmd.Flags().Float64Var(&flagLatitude, "latitude", 0, "Latitude for geo search")
	cmd.Flags().Float64Var(&flagLongitude, "longitude", 0, "Longitude for geo search")
	cmd.Flags().StringVar(&flagLocation, "location", "",
		"Free-form location: 'bellevue, wa', 'seattle', '47.6,-122.3', or 'seattle metro'. "+
			"Resolves to a typed GeoContext and hard-rejects out-of-region results.")
	cmd.Flags().BoolVar(&flagAcceptAmbiguous, "batch-accept-ambiguous", false,
		"BATCH-ONLY escape hatch: when --location is ambiguous, force-pick the top "+
			"candidate instead of returning a disambiguation envelope. Interactive "+
			"agents must NOT set this — it defeats the disambiguation contract.")
	cmd.Flags().StringVar(&flagMetro, "metro", "",
		"Metro slug (e.g., chicago, seattle). DEPRECATED — use --location <city>. "+
			"Implicit --batch-accept-ambiguous is canonical-only: a single-hit registry lookup "+
			"preserves the legacy result shape; ambiguous or unknown values return the standard "+
			"disambiguation envelope just like --location would.")
	cmd.Flags().StringVar(&flagNeighborhood, "neighborhood", "", "Neighborhood slug")
	cmd.Flags().StringVar(&flagCuisine, "cuisine", "", "Cuisine filter")
	cmd.Flags().IntVar(&flagPriceBand, "max-price", 0, "Maximum price band 1-4")
	cmd.Flags().StringVar(&flagAccolade, "accolade", "", "Filter by accolade (michelin, worlds50best)")
	cmd.Flags().IntVar(&flagPartySize, "party", 2, "Party size for availability filter")
	cmd.Flags().StringVar(&flagNetwork, "network", "", "Restrict to one network (opentable, tock)")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Max restaurants to return")
	_ = flagNeighborhood
	_ = flagCuisine
	_ = flagPriceBand
	_ = flagAccolade
	_ = flagPartySize
	return cmd
}

// decorateForList wraps DecorateWithLocationContext with the tier
// inference needed for the wiring path. ResolveLocation doesn't return
// the tier directly (the tier is an internal step inside the pipeline),
// so the caller infers it from the shape of the returned GeoContext
// via inferTierFromGeoContext (in confidence.go).
func decorateForList(gc *GeoContext, acceptedAmbiguous bool) (*LocationResolvedField, *LocationWarningField) {
	if gc == nil {
		return nil, nil
	}
	tier := inferTierFromGeoContext(gc, acceptedAmbiguous)
	return DecorateWithLocationContext(gc, tier, acceptedAmbiguous)
}
