// Hand-rewritten in Phase 3 to delegate to the cross-network source clients.

package cli

// PATCH: scaffold-endpoint-redirects — see .printing-press-patches.json for the change-set rationale.

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

type restaurantDetail struct {
	Network   string         `json:"network"`
	Slug      string         `json:"slug"`
	Resolved  bool           `json:"resolved"`
	Reason    string         `json:"reason,omitempty"`
	Source    map[string]any `json:"source,omitempty"`
	FetchedAt string         `json:"fetched_at"`
}

func newRestaurantsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <slug>",
		Short: "Get a restaurant's full detail across networks (network-prefixed slug supported)",
		Long: "Resolves a venue on OpenTable first, then Tock, returning the SSR-rendered " +
			"restaurant detail (hours, address, cuisine, price band, photos, accolades). " +
			"Use `opentable:<slug>` or `tock:<slug>` to disambiguate. For Resy, address by " +
			"numeric venue id with `resy:<id>` (Resy has no public detail endpoint; the " +
			"returned block is the minimal search-derived id/name/city/coords).",
		Example:     "  table-reservation-goat-pp-cli restaurants get 'tock:alinea' --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			input := args[0]
			if strings.TrimSpace(input) == "" || strings.Contains(input, "__printing_press_invalid__") {
				return fmt.Errorf("invalid slug: %q (provide a venue slug like 'alinea' or 'opentable:le-bernardin')", input)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), restaurantDetail{
					Network: "opentable", Slug: input, Resolved: false,
					Reason: "dry-run", FetchedAt: time.Now().UTC().Format(time.RFC3339),
				}, flags)
			}
			session, err := auth.Load()
			if err != nil {
				return err
			}
			network, slug := parseNetworkSlug(input)
			tryOT := network == "" || network == "opentable"
			tryTock := network == "" || network == "tock"
			tryResy := network == "resy"
			detail := restaurantDetail{Slug: slug, FetchedAt: time.Now().UTC().Format(time.RFC3339)}

			// Resy branch: numeric venue id required. Resy has no
			// public restaurant-detail endpoint, so we use /4/find to
			// confirm the venue exists and lift its canonical name from
			// the response envelope. /4/find returns an empty venues
			// list when there are no slots on a single probed date even
			// for venues that exist on Resy (live-confirmed against
			// Le Bernardin id=1387, 2026-05-11), so we probe a small
			// forward window via VenueIdentityProbe rather than a
			// single date.
			if tryResy {
				detail.Network = "resy"
				if session == nil || session.Resy == nil || session.Resy.AuthToken == "" {
					detail.Reason = "resy: not authenticated; run `auth login --resy --email <you@example.com>` first"
					return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
				}
				if _, err := strconv.Atoi(slug); err != nil {
					detail.Reason = fmt.Sprintf("resy: %q is not a numeric venue id; use `goat <name> --network resy` to discover ids", slug)
					return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
				}
				client := resy.New(resy.Credentials{
					APIKey:    session.Resy.APIKey,
					AuthToken: session.Resy.AuthToken,
					Email:     session.Resy.Email,
				})
				identity, err := client.VenueIdentityProbe(cmd.Context(), slug, 2)
				if err != nil {
					detail.Reason = fmt.Sprintf("resy venue=%s: %v", slug, err)
					return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
				}
				if identity.Name == "" && identity.ID == "" {
					detail.Reason = fmt.Sprintf("resy venue=%s: not found on Resy across 5-date forward probe (verify the id via `goat <name> --network resy`)", slug)
					return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
				}
				// Enrich with search-derived metadata when the venue's
				// name (now known) returns it as a top hit. Tolerate
				// search failure — the identity probe already proved
				// the venue exists, so a non-empty detail block is
				// honest even without enrichment.
				source := map[string]any{
					"id":   identity.ID,
					"name": identity.Name,
				}
				if identity.Name != "" {
					if venues, serr := client.Search(cmd.Context(), resy.SearchParams{Query: identity.Name, Limit: 10}); serr == nil {
						for _, v := range venues {
							if v.ID == slug {
								source["slug"] = v.Slug
								source["city"] = v.City
								source["city_code"] = v.CityCode
								source["region"] = v.Region
								source["latitude"] = v.Latitude
								source["longitude"] = v.Longitude
								source["url"] = v.URL
								break
							}
						}
					}
				}
				detail.Resolved = true
				detail.Source = source
				return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
			}

			if tryOT {
				if c, err := opentable.New(session); err == nil {
					if r, err := c.RestaurantBySlug(cmd.Context(), slug); err == nil && r != nil {
						detail.Network = "opentable"
						detail.Resolved = true
						detail.Source = r
						return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
					}
				}
			}
			if tryTock {
				if c, err := tock.New(session); err == nil {
					if d, err := c.VenueDetail(cmd.Context(), slug); err == nil && len(d) > 0 {
						detail.Network = "tock"
						detail.Resolved = true
						detail.Source = d
						return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
					} else if err != nil {
						detail.Reason = fmt.Sprintf("tock %s: %v", slug, err)
					}
				}
			}
			detail.Network = "unknown"
			if detail.Reason == "" {
				detail.Reason = "could not resolve venue on OpenTable or Tock"
			}
			return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
		},
	}
	return cmd
}
