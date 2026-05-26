// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// `config` command tree — named GHL location profiles. Persists to
// ~/.config/gohighlevel-pp-cli/locations.toml. Activated profile is
// stored in active-location.toml.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gohighlevel/internal/cliutil"
)

// locationOverride is set by the global --location flag (registered in
// root.go's AddCommand block by addLocationGlobalFlag). It is consulted
// by any command that resolves a location at runtime.
var locationOverride string

type locationProfile struct {
	Name       string `toml:"name" json:"name"`
	LocationID string `toml:"location_id" json:"location_id"`
	TokenEnv   string `toml:"token_env,omitempty" json:"token_env,omitempty"`
}

type locationsFile struct {
	Profiles []locationProfile `toml:"profiles"`
}

func locationsConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "gohighlevel-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

func loadLocations() (*locationsFile, error) {
	dir, err := locationsConfigDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "locations.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &locationsFile{}, nil
		}
		return nil, err
	}
	var lf locationsFile
	if err := toml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing locations: %w", err)
	}
	return &lf, nil
}

func saveLocations(lf *locationsFile) error {
	dir, err := locationsConfigDir()
	if err != nil {
		return err
	}
	data, err := toml.Marshal(lf)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "locations.toml"), data, 0o600)
}

func saveActiveLocation(name string, p locationProfile) error {
	dir, err := locationsConfigDir()
	if err != nil {
		return err
	}
	wrapper := struct {
		Active locationProfile `toml:"active"`
	}{Active: p}
	data, err := toml.Marshal(wrapper)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "active-location.toml"), data, 0o600)
}

func readActiveLocation() *locationProfile {
	dir, err := locationsConfigDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "active-location.toml"))
	if err != nil {
		return nil
	}
	var wrapper struct {
		Active locationProfile `toml:"active"`
	}
	if err := toml.Unmarshal(data, &wrapper); err != nil {
		return nil
	}
	if wrapper.Active.LocationID == "" {
		return nil
	}
	return &wrapper.Active
}

// resolveLocationID returns the effective location ID for the current
// invocation. Priority: --location flag > active-location.toml.
func resolveLocationID() string {
	if locationOverride != "" {
		lf, err := loadLocations()
		if err == nil {
			for _, p := range lf.Profiles {
				if p.Name == locationOverride {
					return p.LocationID
				}
			}
		}
		// Treat raw input as the location ID itself.
		return locationOverride
	}
	if a := readActiveLocation(); a != nil {
		return a.LocationID
	}
	return ""
}

func newConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage named GHL location profiles",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newConfigUseCmd(flags))
	cmd.AddCommand(newConfigListCmd(flags))
	cmd.AddCommand(newConfigAddCmd(flags))
	return cmd
}

func newConfigUseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active GHL location profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			name := args[0]
			lf, err := loadLocations()
			if err != nil {
				return configErr(err)
			}
			for _, p := range lf.Profiles {
				if p.Name == name {
					if cliutil.IsVerifyEnv() {
						fmt.Fprintf(cmd.OutOrStdout(), "would activate location %q (id=%s)\n", name, p.LocationID)
						return nil
					}
					if err := saveActiveLocation(name, p); err != nil {
						return configErr(err)
					}
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"active": p,
					}, flags)
				}
			}
			return notFoundErr(fmt.Errorf("no location profile named %q (run 'config list')", name))
		},
	}
	return cmd
}

func newConfigListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List configured GHL location profiles",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			lf, err := loadLocations()
			if err != nil {
				return configErr(err)
			}
			active := readActiveLocation()
			out := struct {
				Active   *locationProfile  `json:"active,omitempty"`
				Profiles []locationProfile `json:"profiles"`
			}{Active: active, Profiles: lf.Profiles}
			sort.Slice(out.Profiles, func(i, j int) bool {
				return out.Profiles[i].Name < out.Profiles[j].Name
			})
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			if active != nil {
				fmt.Fprintf(w, "active: %s (%s)\n", active.Name, active.LocationID)
			} else {
				fmt.Fprintln(w, "active: (none)")
			}
			fmt.Fprintln(w, "name\tlocation_id\ttoken_env")
			for _, p := range out.Profiles {
				fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.LocationID, p.TokenEnv)
			}
			return nil
		},
	}
	return cmd
}

func newConfigAddCmd(flags *rootFlags) *cobra.Command {
	var name, locID, tokenEnv string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add or update a GHL location profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if name == "" || locID == "" {
				if flags.asJSON {
					_ = json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"error": "--name and --location-id are required",
					})
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), "error: --name and --location-id are required")
				}
				return usageErr(fmt.Errorf("missing required flags"))
			}
			if tokenEnv == "" {
				tokenEnv = "GHL_PIT_TOKEN"
			}
			lf, err := loadLocations()
			if err != nil {
				return configErr(err)
			}
			updated := false
			for i, p := range lf.Profiles {
				if strings.EqualFold(p.Name, name) {
					lf.Profiles[i] = locationProfile{Name: name, LocationID: locID, TokenEnv: tokenEnv}
					updated = true
					break
				}
			}
			if !updated {
				lf.Profiles = append(lf.Profiles, locationProfile{Name: name, LocationID: locID, TokenEnv: tokenEnv})
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "would persist location %q (id=%s, env=%s)\n", name, locID, tokenEnv)
				return nil
			}
			if err := saveLocations(lf); err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), lf.Profiles[len(lf.Profiles)-1], flags)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Profile name (e.g. kwcp, think)")
	cmd.Flags().StringVar(&locID, "location-id", "", "GHL location (sub-account) ID")
	cmd.Flags().StringVar(&tokenEnv, "token-env", "GHL_PIT_TOKEN", "Env var that holds the PIT token")
	return cmd
}

// addLocationGlobalFlag wires the global --location persistent flag on the
// root command. Called from root.go's AddCommand block.
func addLocationGlobalFlag(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVar(&locationOverride, "location", "", "Named GHL location profile (overrides the active one)")
}
