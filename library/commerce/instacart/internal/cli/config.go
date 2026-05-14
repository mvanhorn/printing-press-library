// PATCH (fix-instacart-location-config-546): adds an `instacart config`
// subtree so the location keys (postal_code, address_id, latitude,
// longitude) can be set via CLI instead of hand-editing the JSON file.
// Closes the cold-install gap reported in mvanhorn/printing-press-library#546.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and modify location-related config (postal code, address ID, coordinates)",
		Long: `Instacart's GraphQL API requires location data (latitude/longitude or a
known address_id) on every retailer lookup. The CLI reads these from
` + "`~/.config/instacart/config.json`" + ` (or the OS equivalent). Use the
subcommands below to populate them without hand-editing the file.

If you don't know your coordinates, the easiest path is:
  1. ` + "`instacart auth login`" + ` (the post-login step will try to fetch
     your default Instacart address and populate everything automatically)
  2. If that doesn't work, find your address ID in the URL on
     https://www.instacart.com/store/account/your-account and run
     ` + "`instacart config set-address --id <id>`" + ` — the CLI uses the
     cached GetAddressById op to derive coordinates from the ID.
  3. As a last resort, look up your lat/lon (e.g., Google Maps,
     right-click → "What's here?") and run
     ` + "`instacart config set-coords --lat X --lon Y`" + `.`,
	}
	cmd.AddCommand(
		newConfigShowCmd(),
		newConfigSetCoordsCmd(),
		newConfigSetAddressCmd(),
	)
	return cmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "show",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Print the current location config",
		Example:     "  instacart config show\n  instacart config show --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			out := map[string]any{
				"postal_code": cfg.PostalCode,
				"address_id":  cfg.AddressID,
				"latitude":    cfg.Latitude,
				"longitude":   cfg.Longitude,
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "postal_code: %q\naddress_id:  %q\nlatitude:    %v\nlongitude:   %v\n",
				cfg.PostalCode, cfg.AddressID, cfg.Latitude, cfg.Longitude)
			if !locationReady(cfg) {
				fmt.Fprintln(cmd.OutOrStdout(), "\nWARNING: location is incomplete — see `instacart config --help`.")
			}
			return nil
		},
	}
}

func newConfigSetCoordsCmd() *cobra.Command {
	var lat, lon float64
	var postal string
	cmd := &cobra.Command{
		Use:   "set-coords",
		Short: "Save latitude/longitude (and optionally postal_code) to config",
		Example: "  instacart config set-coords --lat 47.6740 --lon -122.1215\n" +
			"  instacart config set-coords --lat 47.6740 --lon -122.1215 --postal 98052",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Use Changed() so a deliberate `--lat 0` (equator) or `--lon 0`
			// (prime meridian) isn't rejected as "not provided".
			if !cmd.Flags().Changed("lat") || !cmd.Flags().Changed("lon") {
				return coded(ExitUsage, "--lat and --lon are both required")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.Latitude = lat
			cfg.Longitude = lon
			if postal != "" {
				cfg.PostalCode = postal
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"saved":       true,
					"postal_code": cfg.PostalCode,
					"latitude":    cfg.Latitude,
					"longitude":   cfg.Longitude,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved: latitude=%v longitude=%v", cfg.Latitude, cfg.Longitude)
			if cfg.PostalCode != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " postal_code=%q", cfg.PostalCode)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude (e.g., 47.6740)")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude (e.g., -122.1215)")
	cmd.Flags().StringVar(&postal, "postal", "", "Postal code (optional)")
	return cmd
}

func newConfigSetAddressCmd() *cobra.Command {
	var addrID string
	cmd := &cobra.Command{
		Use:   "set-address",
		Short: "Save an Instacart address_id and auto-derive coordinates via GetAddressById",
		Long: `Persist an Instacart address_id and fetch its postal_code / latitude /
longitude from Instacart's GraphQL API using the already-cached
GetAddressById op. Requires a logged-in session.

Find your address_id by opening https://www.instacart.com/store/account/your-account
in Chrome with DevTools open; the URL fragment or a graphql variable
exposed in the Network tab will contain it. It looks like a UUID.`,
		Example: "  instacart config set-address --id 12345678-aaaa-bbbb-cccc-deadbeef0000",
		RunE: func(cmd *cobra.Command, args []string) error {
			if addrID == "" {
				return coded(ExitUsage, "--id is required")
			}
			sess, err := auth.LoadSession()
			if err != nil {
				return coded(ExitAuth, "no session — run `instacart auth login` first")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			st, err := store.Open()
			if err != nil {
				return err
			}
			defer st.Close()

			client := gql.NewClient(sess, cfg, st)
			vars := map[string]any{"id": addrID}
			resp, err := client.Query(context.Background(), "GetAddressById", vars)
			if err != nil {
				return coded(ExitTransient, "fetching address: %v", err)
			}

			var envelope struct {
				Data struct {
					Address *struct {
						ID            string  `json:"id"`
						PostalCode    string  `json:"postalCode"`
						Latitude      float64 `json:"latitude"`
						Longitude     float64 `json:"longitude"`
						StreetAddress string  `json:"streetAddress"`
					} `json:"address"`
				} `json:"data"`
			}
			if err := json.Unmarshal(resp.RawBody, &envelope); err != nil {
				return coded(ExitTransient, "parsing address response: %v", err)
			}
			addr := envelope.Data.Address
			if addr == nil || addr.ID == "" {
				return coded(ExitNotFound, "address %s not found (check the id and that you are logged in to the same account)", addrID)
			}

			cfg.AddressID = addr.ID
			cfg.PostalCode = addr.PostalCode
			cfg.Latitude = addr.Latitude
			cfg.Longitude = addr.Longitude
			if err := cfg.Save(); err != nil {
				return err
			}

			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"saved":          true,
					"address_id":     addr.ID,
					"postal_code":    addr.PostalCode,
					"latitude":       addr.Latitude,
					"longitude":      addr.Longitude,
					"street_address": addr.StreetAddress,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved address %q (%s)\n  postal_code=%q latitude=%v longitude=%v\n",
				addr.ID, addr.StreetAddress, addr.PostalCode, addr.Latitude, addr.Longitude)
			return nil
		},
	}
	cmd.Flags().StringVar(&addrID, "id", "", "Instacart address ID (UUID)")
	return cmd
}

// locationReady returns true when the config has enough location data for
// the ShopCollectionScoped bootstrap to succeed. Either coordinates OR an
// address ID is sufficient — both is preferred.
func locationReady(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.AddressID != "" {
		return true
	}
	if cfg.Latitude != 0 || cfg.Longitude != 0 {
		return true
	}
	return false
}

// currentUserAddressesQuery is the GraphQL text we POST after a successful
// auth login to look up the user's saved addresses. It uses POST + query
// text (no persisted-query hash) because the Instacart wrapper proves this
// path works for unhashed queries (see internal/gql/client.go and the
// UpdateCartItemsMutation precedent). If Instacart's schema doesn't expose
// `currentUser.addresses` (or renames it), this returns a GraphQL error
// envelope and tryAutoPopulateLocation degrades to a printed hint pointing
// at the manual setters.
const currentUserAddressesQuery = `query CurrentUserAddresses {
  currentUser {
    id
    addresses {
      id
      streetAddress
      postalCode
      latitude
      longitude
      isDefault
      __typename
    }
    __typename
  }
}`

// tryAutoPopulateLocation attempts to fetch the user's default Instacart
// address after a successful auth login and persist its postal_code,
// latitude, longitude, and address_id to config. On any failure it returns
// nil after writing a one-line hint to cmd's stdout — the auth flow itself
// already succeeded and we don't want a best-effort enrichment to mask that.
//
// Skips silently when location is already configured to avoid clobbering a
// user's manual values on every relogin.
func tryAutoPopulateLocation(cmd *cobra.Command, sess *auth.Session) {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	if locationReady(cfg) {
		return
	}

	st, err := store.Open()
	if err != nil {
		return
	}
	defer st.Close()

	client := gql.NewClient(sess, cfg, st)
	// Note on `client.Mutation`: this codebase uses Mutation as the
	// POST-with-raw-query-text path regardless of GraphQL operation type
	// — it does not set any operation-type metadata on the wire (see
	// `internal/gql/client.go:call`, which just serializes `operationName +
	// variables + query` into the body). The CurrentUserAddresses payload
	// is a `query`, not a `mutation`; reusing this method here mirrors the
	// existing UpdateCartItemsMutation convention. If the client ever
	// gains an explicit type marker, the right move is to add a thin
	// `client.PostRaw` and route this call through it.
	resp, err := client.Mutation(context.Background(), "CurrentUserAddresses", map[string]any{}, currentUserAddressesQuery)
	if err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNote: could not auto-populate location (%v).\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "      Run `instacart config set-address --id <id>` or `instacart config set-coords --lat <N> --lon <N>` to set it manually.")
		return
	}
	if len(resp.Errors) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNote: could not auto-populate location (GraphQL: %s).\n", resp.Errors[0].Message)
		fmt.Fprintln(cmd.OutOrStdout(), "      Run `instacart config set-address --id <id>` or `instacart config set-coords --lat <N> --lon <N>` to set it manually.")
		return
	}

	var envelope struct {
		Data struct {
			CurrentUser *struct {
				ID        string `json:"id"`
				Addresses []struct {
					ID            string  `json:"id"`
					StreetAddress string  `json:"streetAddress"`
					PostalCode    string  `json:"postalCode"`
					Latitude      float64 `json:"latitude"`
					Longitude     float64 `json:"longitude"`
					IsDefault     bool    `json:"isDefault"`
				} `json:"addresses"`
			} `json:"currentUser"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.RawBody, &envelope); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNote: auto-populate response could not be parsed (%v).\n", err)
		return
	}
	if envelope.Data.CurrentUser == nil || len(envelope.Data.CurrentUser.Addresses) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNote: no saved Instacart addresses found on this account. Add one at https://www.instacart.com/store/account/your-account, then re-run auth login (or use `instacart config set-coords`).")
		return
	}

	// Prefer the user-flagged default address. Fall back to the first.
	picked := envelope.Data.CurrentUser.Addresses[0]
	for _, a := range envelope.Data.CurrentUser.Addresses {
		if a.IsDefault {
			picked = a
			break
		}
	}

	cfg.AddressID = picked.ID
	cfg.PostalCode = picked.PostalCode
	cfg.Latitude = picked.Latitude
	cfg.Longitude = picked.Longitude
	if err := cfg.Save(); err != nil {
		fmt.Fprintf(cmd.OutOrStdout(), "\nNote: fetched address but failed to save config: %v\n", err)
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nauto-populated location from your default Instacart address:\n  %s — postal_code=%q lat=%v lon=%v\n",
		picked.StreetAddress, picked.PostalCode, picked.Latitude, picked.Longitude)
}
