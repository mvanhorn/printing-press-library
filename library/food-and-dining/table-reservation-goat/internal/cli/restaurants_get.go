// Hand-rewritten in Phase 3 to delegate to the cross-network source clients.

package cli

// PATCH: scaffold-endpoint-redirects — see .printing-press-patches.json for the change-set rationale.

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
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
			"Use `opentable:<slug>` or `tock:<slug>` to disambiguate.",
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
			detail := restaurantDetail{Slug: slug, FetchedAt: time.Now().UTC().Format(time.RFC3339)}
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
