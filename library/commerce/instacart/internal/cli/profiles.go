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
				sess, err := auth.LoadSession()
				if err != nil {
					return coded(ExitAuth, "no session — run `instacart auth login` first")
				}
				addr, err := fetchAddressByID(cmd.Context(), sess, cfg, addrID)
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
				// UseProfile only fails when the named profile is missing.
				// Since we just SetProfile(name) one line above, this is
				// effectively unreachable — but if UseProfile's contract
				// ever changes, ExitNotFound is the honest signal (it's
				// not a transient retryable condition).
				if err := cfg.UseProfile(name); err != nil {
					return coded(ExitNotFound, "%v", err)
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
to get ` + "`omar-1528-37th-ave-e`" + ` etc.). Use ` + "`--overwrite`" + ` to refresh profiles
for addresses you already have saved (the same address_id) -- without it,
re-imports of already-saved addresses are reported as skipped.

` + "`--overwrite`" + ` only updates same-address re-imports. If two different
Instacart addresses would slugify to the same name (e.g., two
distinct addresses both producing ` + "`1528-37th-ave-e`" + `), the second one
is suffixed (` + "`1528-37th-ave-e-2`" + `) regardless of ` + "`--overwrite`" + ` so the
existing profile pointing at the first address isn't silently
redirected.

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
			addrs, err := userAddressFetcherFrom(cmd.Context())(cmd.Context(), sess, cfg)
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
				// requestedBase is the slug we'd have used if there were no
				// collision (`prefix + slugified(street)`). Tracked so
				// `--use <base>` can resolve to the actual imported profile
				// when collisions force a `-2` suffix; otherwise `UseProfile`
				// would silently activate the colliding pre-existing profile.
				requestedBase string `json:"-"`
			}
			results := make([]result, 0, len(addrs))
			// `seen` tracks names taken either by existing config or by an
			// earlier address in *this* import. We don't pre-seed it from
			// cfg.Profiles because the idempotency rule below already
			// special-cases that case: an existing profile pointing at the
			// same address_id is a re-import (skip/update), and only when
			// the same name is held by a *different* address do we suffix.
			seen := map[string]bool{}
			for _, a := range addrs {
				base := prefix + slugifyName(a.StreetAddress)
				if base == prefix || base == "" {
					base = prefix + "address-" + a.ID
				}
				// Truncate to ValidProfileName's cap with prefix included
				// so a long street + prefix combination doesn't abort the
				// whole import at SetProfile. profileNameMaxLen is mirrored
				// from internal/config.
				if len(base) > profileNameMaxLen {
					base = strings.TrimRight(base[:profileNameMaxLen], "-")
				}

				// Resolve the final name with idempotency: if a profile by
				// this name already exists for the same address, treat it
				// as a re-import; if for a different address, suffix until
				// we find an unused slot (or hit an existing same-address
				// match at the suffixed name).
				name, kind := resolveImportName(base, a, cfg.Profiles, seen, overwrite)
				switch kind {
				case "skipped":
					results = append(results, result{Name: name, Status: "skipped", Street: a.StreetAddress, Default: a.IsDefault, requestedBase: base})
					seen[name] = true
					continue
				}
				status := kind // "created" or "updated"
				p := config.Profile{
					Name:       name,
					Label:      a.StreetAddress,
					AddressID:  a.ID,
					PostalCode: a.PostalCode,
					Latitude:   a.Latitude,
					Longitude:  a.Longitude,
				}
				if err := cfg.SetProfile(p); err != nil {
					// SetProfile only fails on input-shape (invalid name) —
					// typically because --prefix introduced an illegal
					// character. Return ExitUsage so agent retry loops
					// don't spin on the same bad prefix. ExitTransient
					// here would tell agents "retry, the network is
					// flaky" which is the opposite of what happened.
					return coded(ExitUsage, "%v", err)
				}
				seen[name] = true
				results = append(results, result{Name: name, Status: status, Street: a.StreetAddress, Default: a.IsDefault, requestedBase: base})
			}
			// Persist the imported profiles BEFORE attempting --use so a
			// bad --use value (typo, mismatched name) doesn't silently
			// discard everything we just imported. Greptile P1 on #643:
			// the user would see a "not found" error and re-running would
			// re-fetch + re-save 12 addresses they already had on disk.
			if err := cfg.Save(); err != nil {
				return err
			}
			if setActive != "" {
				refs := make([]importedRef, 0, len(results))
				for _, r := range results {
					refs = append(refs, importedRef{Name: r.Name, Base: r.requestedBase})
				}
				resolved, warn, err := resolveSetActive(setActive, refs, cfg.Profiles)
				if err != nil {
					return coded(ExitNotFound, "%v (after import — pick one of %s)", err, strings.Join(cfg.ProfileNames(), ", "))
				}
				if err := cfg.UseProfile(resolved); err != nil {
					return coded(ExitNotFound, "%v (after import — pick one of %s)", err, strings.Join(cfg.ProfileNames(), ", "))
				}
				if warn != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), warn)
				}
				if err := cfg.Save(); err != nil {
					return err
				}
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
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Re-import profiles for addresses already saved (update in place instead of skipping); name collisions with a different address still get a -N suffix")
	cmd.Flags().StringVar(&setActive, "use", "", "After importing, set this profile name as active")
	return cmd
}

// userAddressFetcher is the shape of fetchUserAddresses. It's a named type
// so tests can stand in an in-memory fake without standing up the GraphQL
// stack, and so the override can be propagated via context rather than a
// mutable package-level variable.
type userAddressFetcher func(ctx context.Context, sess *auth.Session, cfg *config.Config) ([]fetchedAddress, error)

// userAddressFetcherKey is the context-value key tests use to inject a
// fake fetcher. Using an unexported zero-sized type as the key (the
// standard library idiom) prevents collisions with any other package
// using context values, and using context — which is immutable from the
// consumer side — means parallel tests that each set their own ctx value
// don't race the way a mutable package-level variable would.
type userAddressFetcherKey struct{}

// userAddressFetcherFrom returns the test-injected fetcher when one is on
// the context, otherwise the real implementation. Production code never
// sets the key, so this is a noop overhead in the live path.
func userAddressFetcherFrom(ctx context.Context) userAddressFetcher {
	if f, ok := ctx.Value(userAddressFetcherKey{}).(userAddressFetcher); ok && f != nil {
		return f
	}
	return fetchUserAddresses
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

// fetchAddressByID takes a logged-in session + loaded config so it stays
// symmetric with fetchUserAddresses below: the caller in
// newConfigProfilesAddCmd has already loaded both, and re-reading them
// here would duplicate disk I/O and silently mask a partially-edited
// config the caller saw at command start.
func fetchAddressByID(ctx context.Context, sess *auth.Session, cfg *config.Config, addrID string) (*fetchedAddress, error) {
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

// profileNameMaxLen mirrors the cap baked into config.ValidProfileName's
// regex. Keep these two in sync — diverging gives "regenerated a name that
// then fails validation" bugs.
const profileNameMaxLen = 40

func slugifyName(s string) string {
	s = strings.ToLower(s)
	s = slugifyRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > profileNameMaxLen {
		s = strings.TrimRight(s[:profileNameMaxLen], "-")
	}
	return s
}

// resolveImportName picks the final profile name for one imported address.
//
// Cases:
//   - base unused in cfg + unused this run → ("base", "created")
//   - base in cfg pointing at the same address → ("base", "skipped" or "updated")
//   - base in cfg pointing at a DIFFERENT address, or already taken this run
//     → suffix "-2", "-3", ... until either an unused slot is found, or a
//     same-address match is found at the suffixed name.
//
// This is what makes `profiles import` idempotent: re-running without
// `--overwrite` reports each existing address as "skipped" rather than
// inventing a duplicate at "<name>-2".
func resolveImportName(base string, a fetchedAddress, existing map[string]config.Profile, seenThisRun map[string]bool, overwrite bool) (string, string) {
	name := base
	for i := 2; ; i++ {
		p, inCfg := existing[name]
		if inCfg && p.AddressID == a.ID {
			if overwrite {
				return name, "updated"
			}
			return name, "skipped"
		}
		if !inCfg && !seenThisRun[name] {
			return name, "created"
		}
		// Collision with a different address, or with a name already
		// allocated earlier in this same import run. Suffix and retry.
		next := fmt.Sprintf("%s-%d", base, i)
		// Keep the suffixed name within the validator's length cap.
		if len(next) > profileNameMaxLen {
			next = strings.TrimRight(base[:profileNameMaxLen-len(fmt.Sprintf("-%d", i))], "-") + fmt.Sprintf("-%d", i)
		}
		name = next
	}
}

// importedRef is a tuple of (final resolved name, the base slug we asked for)
// for one row of the import result table. resolveSetActive operates on this
// view so the two struct fields it cares about don't leak the rest of the
// import result schema.
type importedRef struct {
	Name string
	Base string
}

// resolveSetActive picks which profile `profiles import --use <name>` should
// activate, given the user-supplied `setActive` value and the import result
// rows. It returns the resolved profile name, an optional warning string
// (empty when the resolution is unambiguous), and an error.
//
// Resolution order:
//  1. An imported row whose final resolved Name == setActive.
//  2. An imported row whose requestedBase == setActive and whose resolved
//     Name differs (the collision case the Greptile P1 caught). When there
//     are multiple such rows, ambiguity is an error.
//  3. A pre-existing profile already in `existing` with that exact name —
//     useful when the user wanted to keep an older profile active and
//     `import` was just a top-up.
//  4. Otherwise, a "no such profile" error.
//
// The function is package-private and exists only to keep newConfigProfilesImportCmd
// readable; the import command otherwise can't see the `result` slice type defined
// inside its own RunE closure.
func resolveSetActive(setActive string, imported []importedRef, existing map[string]config.Profile) (string, string, error) {
	// 1. Exact match on resolved name.
	for _, r := range imported {
		if r.Name == setActive {
			return r.Name, "", nil
		}
	}
	// 2. Match on requested base (collision case).
	var baseMatches []importedRef
	for _, r := range imported {
		if r.Base == setActive && r.Name != setActive {
			baseMatches = append(baseMatches, r)
		}
	}
	switch len(baseMatches) {
	case 1:
		warn := fmt.Sprintf(
			"note: --use %q matched a name already taken; activating the imported profile %q instead",
			setActive, baseMatches[0].Name,
		)
		return baseMatches[0].Name, warn, nil
	case 0:
		// fall through to pre-existing match
	default:
		names := make([]string, 0, len(baseMatches))
		for _, r := range baseMatches {
			names = append(names, r.Name)
		}
		return "", "", fmt.Errorf("ambiguous --use %q: multiple imported profiles match that base (%s)",
			setActive, strings.Join(names, ", "))
	}
	// 3. Pre-existing profile by exact name.
	if _, ok := existing[setActive]; ok {
		return setActive, "", nil
	}
	// 4. Nothing matched.
	return "", "", fmt.Errorf("profile %q not found", setActive)
}
