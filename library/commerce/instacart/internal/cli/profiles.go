// PATCH (instacart-address-profiles): adds an `instacart config profiles`
// subtree so users with multiple delivery addresses (home, work, second
// residence) can save each as a named profile and switch between them with
// `instacart config profiles use <name>` instead of re-running
// `config set-address --id` every time. Also adds a per-call `--profile`
// persistent flag (wired in root.go) and an `import` command that pulls
// every saved Instacart address via the existing CurrentUserAddresses
// GraphQL query and persists each as a profile.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
)

func newConfigProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage named address profiles (home, work, second residence, ...)",
		Long: `Profiles are named snapshots of location data. The CLI's GraphQL calls
only ever read one location at a time, so switching between addresses
normally means re-running ` + "`config set-address --id`" + ` each time.
A profile is a saved copy of those four fields (postal_code, address_id,
latitude, longitude) under a short name like "home" or "work".

Switch the active profile with ` + "`instacart config profiles use <name>`" + `,
or override for a single call with the persistent ` + "`--profile <name>`" + `
flag (e.g., ` + "`instacart --profile work add safeway 'cold brew'`" + `).

When no profiles are defined the CLI behaves exactly as before — the
top-level config keys remain authoritative.`,
	}
	cmd.AddCommand(
		newConfigProfilesListCmd(),
		newConfigProfilesShowCmd(),
		newConfigProfilesAddCmd(),
		newConfigProfilesUseCmd(),
		newConfigProfilesRmCmd(),
		newConfigProfilesImportCmd(),
	)
	return cmd
}

func newConfigProfilesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List saved profiles (active one is marked with *)",
		Example:     "  instacart config profiles list\n  instacart config profiles list --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			names := cfg.ProfileNames()
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				rows := make([]map[string]any, 0, len(names))
				for _, n := range names {
					p, _ := cfg.GetProfile(n)
					rows = append(rows, map[string]any{
						"name":        p.Name,
						"label":       p.Label,
						"postal_code": p.PostalCode,
						"address_id":  p.AddressID,
						"latitude":    p.Latitude,
						"longitude":   p.Longitude,
						"active":      cfg.ActiveProfile == p.Name,
					})
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"active":   cfg.ActiveProfile,
					"profiles": rows,
				})
			}
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no profiles saved. Add one with `instacart config profiles add <name> --id <address_id>` or `--lat ... --lon ...`.")
				return nil
			}
			for _, n := range names {
				p, _ := cfg.GetProfile(n)
				marker := "  "
				if cfg.ActiveProfile == n {
					marker = "* "
				}
				label := p.Label
				if label == "" {
					label = "(no label)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%-20s %s\n", marker, p.Name, label)
			}
			return nil
		},
	}
}

func newConfigProfilesShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "show <name>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Print the full contents of one profile",
		Args:        cobra.ExactArgs(1),
		Example:     "  instacart config profiles show home",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			p, ok := cfg.GetProfile(args[0])
			if !ok {
				return coded(ExitNotFound, "profile %q not found", args[0])
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(p)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"name:        %s\nlabel:       %s\nactive:      %t\npostal_code: %q\naddress_id:  %q\nlatitude:    %v\nlongitude:   %v\nzone_id:     %q\n",
				p.Name, p.Label, cfg.ActiveProfile == p.Name, p.PostalCode, p.AddressID, p.Latitude, p.Longitude, p.ZoneID)
			return nil
		},
	}
}

func newConfigProfilesAddCmd() *cobra.Command {
	var (
		addrID string
		label  string
		lat    float64
		lon    float64
		postal string
		zoneID string
		use    bool
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a profile, either by Instacart address_id or by raw coordinates",
		Long: `Add a profile under <name>. Two modes:

  By Instacart address_id (recommended — looks up the real address via
  the cached GetAddressById GraphQL op):
      instacart config profiles add home --id 73256642

  By raw coordinates (no network call required):
      instacart config profiles add wildwood --lat 48.6768 --lon -122.3165 \
        --postal 98284 --label "Sedro-Woolley vacation house"

` + "`--use`" + ` also activates the profile in the same call.`,
		Args: cobra.ExactArgs(1),
		Example: "  instacart config profiles add home --id 73256642\n" +
			"  instacart config profiles add work --lat 47.6740 --lon -122.1215 --postal 98052 --label \"Microsoft Building 33\"\n" +
			"  instacart config profiles add home --id 73256642 --use",
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !config.ValidProfileName(name) {
				return coded(ExitUsage, "invalid profile name %q (use lowercase letters, digits, '.', '-', '_')", name)
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			hasID := addrID != ""
			hasCoords := cmd.Flags().Changed("lat") && cmd.Flags().Changed("lon")
			if !hasID && !hasCoords {
				return coded(ExitUsage, "provide either --id <address_id> or --lat <N> --lon <N>")
			}
			if hasID && hasCoords {
				return coded(ExitUsage, "--id and --lat/--lon are mutually exclusive (use --id and the lookup fills coords for you)")
			}

			p := config.Profile{Name: name, Label: label, ZoneID: zoneID}

			if hasID {
				addr, err := fetchAddressByID(cmd.Context(), addrID)
				if err != nil {
					return err
				}
				p.AddressID = addr.ID
				p.PostalCode = addr.PostalCode
				p.Latitude = addr.Latitude
				p.Longitude = addr.Longitude
				if p.Label == "" {
					p.Label = addr.StreetAddress
				}
			} else {
				p.Latitude = lat
				p.Longitude = lon
				p.PostalCode = postal
			}

			if err := cfg.SetProfile(p); err != nil {
				return coded(ExitUsage, "%v", err)
			}
			if use {
				if err := cfg.UseProfile(name); err != nil {
					return coded(ExitTransient, "%v", err)
				}
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"saved":   true,
					"profile": p,
					"active":  cfg.ActiveProfile,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved profile %q (%s) postal=%q lat=%v lon=%v\n",
				p.Name, p.Label, p.PostalCode, p.Latitude, p.Longitude)
			if use {
				fmt.Fprintf(cmd.OutOrStdout(), "now active: %s\n", cfg.ActiveProfile)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&addrID, "id", "", "Instacart address_id (looked up via GetAddressById)")
	cmd.Flags().StringVar(&label, "label", "", "Human-readable hint shown in `profiles list` (optional)")
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude (use with --lon, alternative to --id)")
	cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude (use with --lat, alternative to --id)")
	cmd.Flags().StringVar(&postal, "postal", "", "Postal code (used with --lat/--lon)")
	cmd.Flags().StringVar(&zoneID, "zone", "", "Override Instacart zone_id for this profile (rarely needed)")
	cmd.Flags().BoolVar(&use, "use", false, "Also activate this profile immediately")
	return cmd
}

func newConfigProfilesUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "use <name>",
		Short:   "Switch the active profile (copies its location to top-level config)",
		Args:    cobra.ExactArgs(1),
		Example: "  instacart config profiles use work",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := cfg.UseProfile(args[0]); err != nil {
				return coded(ExitNotFound, "%v", err)
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"active":      cfg.ActiveProfile,
					"postal_code": cfg.PostalCode,
					"address_id":  cfg.AddressID,
					"latitude":    cfg.Latitude,
					"longitude":   cfg.Longitude,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "active profile: %s (postal=%q lat=%v lon=%v)\n",
				cfg.ActiveProfile, cfg.PostalCode, cfg.Latitude, cfg.Longitude)
			return nil
		},
	}
}

func newConfigProfilesRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name>",
		Short:   "Delete a profile (clears active_profile if it was the one removed)",
		Args:    cobra.ExactArgs(1),
		Example: "  instacart config profiles rm wildwood",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			wasActive := cfg.ActiveProfile == args[0]
			if err := cfg.DeleteProfile(args[0]); err != nil {
				return coded(ExitNotFound, "%v", err)
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"removed":          args[0],
					"cleared_active":   wasActive,
					"remaining_active": cfg.ActiveProfile,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed profile %q\n", args[0])
			if wasActive {
				fmt.Fprintln(cmd.OutOrStdout(), "(was active — no profile is currently active; top-level config still applies)")
			}
			return nil
		},
	}
}

func newConfigProfilesImportCmd() *cobra.Command {
	var (
		prefix    string
		overwrite bool
		setActive string
	)
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Create one profile per saved Instacart address (via CurrentUserAddresses GraphQL)",
		Long: `Fetches the addresses on your Instacart account and saves each as a
profile. Profile names are slugified from the street address (e.g.,
"1528 37th Ave E" -> "1528-37th-ave-e"). Collisions get a numeric
suffix.

Use ` + "`--prefix`" + ` to namespace the imported profiles (e.g., ` + "`--prefix omar-`" + `
to get ` + "`omar-1528-37th-ave-e`" + ` etc.). Use ` + "`--overwrite`" + ` to replace
existing profiles with the same name; otherwise existing profiles are
left untouched and reported as skipped.

After importing, rename anything ugly with ` + "`profiles rm`" + ` + ` + "`profiles add`" + `
or just leave them; the CLI treats every profile equally.`,
		Example: "  instacart config profiles import\n  instacart config profiles import --prefix omar- --overwrite",
		RunE: func(cmd *cobra.Command, args []string) error {
			sess, err := auth.LoadSession()
			if err != nil {
				return coded(ExitAuth, "no session — run `instacart auth login` first")
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			addrs, err := fetchUserAddresses(cmd.Context(), sess, cfg)
			if err != nil {
				return err
			}
			if len(addrs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no saved Instacart addresses found on this account")
				return nil
			}

			type result struct {
				Name    string `json:"name"`
				Status  string `json:"status"` // "created", "updated", "skipped"
				Street  string `json:"street"`
				Default bool   `json:"default,omitempty"`
			}
			results := make([]result, 0, len(addrs))
			seen := map[string]bool{}
			for n := range cfg.Profiles {
				seen[n] = true
			}
			for _, a := range addrs {
				base := prefix + slugifyName(a.StreetAddress)
				if base == prefix || base == "" {
					base = prefix + "address-" + a.ID
				}
				name := base
				for i := 2; seen[name] && !(overwrite && profileMatchesAddr(cfg.Profiles[name], a)); i++ {
					name = fmt.Sprintf("%s-%d", base, i)
				}
				status := "created"
				if _, existed := cfg.Profiles[name]; existed {
					if !overwrite {
						results = append(results, result{Name: name, Status: "skipped", Street: a.StreetAddress, Default: a.IsDefault})
						continue
					}
					status = "updated"
				}
				p := config.Profile{
					Name:       name,
					Label:      a.StreetAddress,
					AddressID:  a.ID,
					PostalCode: a.PostalCode,
					Latitude:   a.Latitude,
					Longitude:  a.Longitude,
				}
				if err := cfg.SetProfile(p); err != nil {
					return coded(ExitTransient, "%v", err)
				}
				seen[name] = true
				results = append(results, result{Name: name, Status: status, Street: a.StreetAddress, Default: a.IsDefault})
			}
			if setActive != "" {
				if err := cfg.UseProfile(setActive); err != nil {
					return coded(ExitNotFound, "%v (after import — pick one of %s)", err, strings.Join(cfg.ProfileNames(), ", "))
				}
			}
			if err := cfg.Save(); err != nil {
				return err
			}
			asJSON, _ := cmd.Flags().GetBool("json")
			if asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"imported": results,
					"active":   cfg.ActiveProfile,
				})
			}
			for _, r := range results {
				marker := " "
				if r.Default {
					marker = "*"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s %-8s %-32s %s\n", marker, r.Status, r.Name, r.Street)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d address(es) processed. Switch with `instacart config profiles use <name>`.\n", len(results))
			return nil
		},
	}
	cmd.Flags().StringVar(&prefix, "prefix", "", "Prefix to prepend to every imported profile name")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Replace existing profiles whose names collide")
	cmd.Flags().StringVar(&setActive, "use", "", "After importing, set this profile name as active")
	return cmd
}

// fetchedAddress is the slim shape we need from GetAddressById /
// CurrentUserAddresses. Keeping it scoped to this file avoids reaching
// across the cli/internal layers for a private type.
type fetchedAddress struct {
	ID            string
	StreetAddress string
	PostalCode    string
	Latitude      float64
	Longitude     float64
	IsDefault     bool
}

func fetchAddressByID(ctx context.Context, addrID string) (*fetchedAddress, error) {
	sess, err := auth.LoadSession()
	if err != nil {
		return nil, coded(ExitAuth, "no session — run `instacart auth login` first")
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	st, err := store.Open()
	if err != nil {
		return nil, err
	}
	defer st.Close()

	client := gql.NewClient(sess, cfg, st)
	resp, err := client.Query(ctx, "GetAddressById", map[string]any{"id": addrID})
	if err != nil {
		return nil, coded(ExitTransient, "fetching address: %v", err)
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
		return nil, coded(ExitTransient, "parsing address response: %v", err)
	}
	addr := envelope.Data.Address
	if addr == nil || addr.ID == "" {
		return nil, coded(ExitNotFound, "address %s not found (check the id and that you are logged in to the same account)", addrID)
	}
	return &fetchedAddress{
		ID:            addr.ID,
		StreetAddress: addr.StreetAddress,
		PostalCode:    addr.PostalCode,
		Latitude:      addr.Latitude,
		Longitude:     addr.Longitude,
	}, nil
}

func fetchUserAddresses(ctx context.Context, sess *auth.Session, cfg *config.Config) ([]fetchedAddress, error) {
	st, err := store.Open()
	if err != nil {
		return nil, err
	}
	defer st.Close()
	client := gql.NewClient(sess, cfg, st)

	// Reuses the same payload tryAutoPopulateLocation does (see config.go).
	resp, err := client.Mutation(ctx, "CurrentUserAddresses", map[string]any{}, currentUserAddressesQuery)
	if err != nil {
		return nil, coded(ExitTransient, "fetching addresses: %v", err)
	}
	if len(resp.Errors) > 0 {
		return nil, coded(ExitTransient, "GraphQL error: %s", resp.Errors[0].Message)
	}
	var envelope struct {
		Data struct {
			CurrentUser *struct {
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
		return nil, coded(ExitTransient, "parsing addresses response: %v", err)
	}
	if envelope.Data.CurrentUser == nil {
		return nil, nil
	}
	out := make([]fetchedAddress, 0, len(envelope.Data.CurrentUser.Addresses))
	for _, a := range envelope.Data.CurrentUser.Addresses {
		out = append(out, fetchedAddress{
			ID:            a.ID,
			StreetAddress: a.StreetAddress,
			PostalCode:    a.PostalCode,
			Latitude:      a.Latitude,
			Longitude:     a.Longitude,
			IsDefault:     a.IsDefault,
		})
	}
	// Stable order: default first (so `--use <name-of-default-after-slug>` is easy
	// to reason about), then by ID.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].IsDefault != out[j].IsDefault {
			return out[i].IsDefault
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// slugifyName turns a free-text address into a profile-name-safe slug.
// Lowercases, replaces runs of non-[a-z0-9] with "-", trims dashes,
// caps at 40 chars (the same length the config validator enforces).
var slugifyRE = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyName(s string) string {
	s = strings.ToLower(s)
	s = slugifyRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	return s
}

func profileMatchesAddr(p config.Profile, a fetchedAddress) bool {
	return p.AddressID == a.ID
}
