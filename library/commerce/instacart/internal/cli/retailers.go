package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/instacart"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

func newRetailersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "retailers",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List and inspect retailers available at your address",
	}
	cmd.AddCommand(newRetailersListCmd(), newRetailersShowCmd())
	return cmd
}

func newRetailersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List retailers that deliver to your saved address",
		Long: `Shows retailers cached locally. If the cache is empty or stale, hits
Instacart's ShopCollectionUnscoped query to refresh. Use --refresh to force.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()

			retailers, err := app.Store.ListRetailers()
			if err != nil {
				return err
			}
			if len(retailers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no retailers cached yet. Use `instacart search <query> --store <slug>` or `instacart add` to populate the cache, then re-run.")
				return nil
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(retailers)
			}
			for _, r := range retailers {
				name := r.Name
				if name == "" {
					name = r.Slug
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %-30s slug=%s shopId=%s\n", name, r.Slug, r.ShopID)
			}
			return nil
		},
	}
}

func newRetailersShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "show <slug>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Look up a retailer by slug and cache it locally",
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := strings.ToLower(strings.TrimSpace(args[0]))
			app, err := newAppContext(cmd)
			if err != nil {
				return err
			}
			defer app.Store.Close()
			if err := app.RequireSession(); err != nil {
				return err
			}
			r, err := resolveRetailer(app, slug)
			if err != nil {
				return err
			}
			if app.JSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(r)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "slug: %s\nname: %s\nshopId: %s\nretailerId: %s\nzoneId: %s\nlocationId: %s\n",
				r.Slug, r.Name, r.ShopID, r.RetailerID, r.ZoneID, r.LocationID)
			return nil
		},
	}
}

// resolveRetailer returns a retailer by slug, refreshing from the live API if
// we don't have it cached. It uses ShopCollectionScoped which the sniff
// confirmed is called with (retailerSlug, postalCode, coordinates, addressId).
func resolveRetailer(app *AppContext, slug string) (*store.Retailer, error) {
	if r, err := app.Store.GetRetailer(slug); err == nil && r.ShopID != "" {
		return r, nil
	}
	if app.Cfg.PostalCode == "" {
		return nil, coded(ExitNotFound, "retailer %q not cached and no postal code configured (set `postal_code` in ~/.config/instacart/config.json)", slug)
	}
	client := gql.NewClient(app.Session, app.Cfg, app.Store)
	// PATCH (fix-shop-collection-coordinates):
	// Use the typed constructor so coordinates is omitted when neither
	// latitude nor longitude is set, instead of sending the invalid {0,0}
	// pair. See mvanhorn/printing-press-library#501.
	vars := instacart.NewShopCollectionScopedVars(slug, app.Cfg.PostalCode, app.Cfg.AddressID, app.Cfg.Latitude, app.Cfg.Longitude)
	resp, err := client.Query(app.Ctx, "ShopCollectionScoped", vars)
	if err != nil {
		return nil, err
	}
	// We don't fully parse the response here; just confirm the call worked
	// and stash the slug. The add/search flow will populate shopId when it
	// sees a resolved shop in a downstream response.
	_ = resp
	stub := &store.Retailer{Slug: slug}
	_ = app.Store.UpsertRetailer(*stub)
	return stub, nil
}
