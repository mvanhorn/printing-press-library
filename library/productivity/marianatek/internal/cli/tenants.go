// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.
// PATCH(retro #marianatek-multi-tenant): manage per-tenant config files at
// ~/.config/marianatek-pp-cli/tenants/<slug>.toml.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/config"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

func newTenantsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tenants",
		Short: "List and manage per-tenant configurations",
		Long: `Mariana Tek uses per-brand subdomains, so one user typically configures
multiple tenants (e.g. kolmkontrast, barrysbootcamp, y7-studio). Each tenant
has its own bearer token and base URL stored at
~/.config/marianatek-pp-cli/tenants/<slug>.toml.

Use these subcommands to inspect, default, and remove tenant configs.
To CREATE a tenant config, run "auth from-browser --tenant <slug> '<cookie>'".`,
	}
	cmd.AddCommand(newTenantsListCmd(flags))
	cmd.AddCommand(newTenantsSetDefaultCmd(flags))
	cmd.AddCommand(newTenantsRemoveCmd(flags))
	return cmd
}

type tenantRow struct {
	Tenant     string `json:"tenant"`
	BaseURL    string `json:"base_url,omitempty"`
	Default    bool   `json:"is_default,omitempty"`
	HasAuth    bool   `json:"has_auth"`
	ConfigPath string `json:"config_path"`
}

func newTenantsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured tenants",
		Example: `  marianatek-pp-cli tenants list
  marianatek-pp-cli tenants list --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			tenants, err := config.ListTenants()
			if err != nil {
				return fmt.Errorf("listing tenants: %w", err)
			}
			root, _ := config.Load(flags.configPath)
			defaultTenant := ""
			if root != nil {
				defaultTenant = root.DefaultTenant
			}
			sort.Strings(tenants)
			rows := make([]tenantRow, 0, len(tenants))
			for _, t := range tenants {
				cfg, err := config.LoadTenant(flags.configPath, t)
				configPath, pathErr := config.TenantConfigPath(t)
				if pathErr != nil {
					return pathErr
				}
				row := tenantRow{
					Tenant:     t,
					Default:    t == defaultTenant,
					ConfigPath: configPath,
				}
				if err == nil && cfg != nil {
					row.BaseURL = cfg.BaseURL
					row.HasAuth = cfg.AuthHeader() != ""
				}
				rows = append(rows, row)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	return cmd
}

func newTenantsSetDefaultCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set-default <tenant>",
		Short:   "Set the default tenant in the root config",
		Example: `  marianatek-pp-cli tenants set-default kolmkontrast`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}
			slug := args[0]
			if err := config.ValidateTenantSlug(slug); err != nil {
				return usageErr(err)
			}
			tenants, _ := config.ListTenants()
			seen := false
			for _, t := range tenants {
				if t == slug {
					seen = true
					break
				}
			}
			if !seen {
				return fmt.Errorf("no tenant config for %q (run `marianatek-pp-cli auth from-browser --tenant %s '<cookie>'` first)", slug, slug)
			}
			if dryRunOK(flags) {
				return nil
			}
			return writeRootDefault(flags.configPath, slug)
		},
	}
	return cmd
}

func newTenantsRemoveCmd(flags *rootFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:     "remove <tenant>",
		Short:   "Delete a tenant's config file (does not revoke the token upstream)",
		Example: `  marianatek-pp-cli tenants remove old-studio --yes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return cmd.Help()
			}
			slug := args[0]
			path, err := config.TenantConfigPath(slug)
			if err != nil {
				return usageErr(err)
			}
			if _, err := os.Stat(path); os.IsNotExist(err) {
				if flags.ignoreMissing {
					return nil
				}
				return fmt.Errorf("no tenant config at %s", path)
			}
			if !yes && !flags.yes {
				return fmt.Errorf("pass --yes to confirm removal of %s", path)
			}
			if dryRunOK(flags) {
				return nil
			}
			return os.Remove(path)
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm removal")
	return cmd
}

// writeRootDefault updates only the `default_tenant` field in the root config,
// preserving any other settings the user may have placed there.
func writeRootDefault(configPath, slug string) error {
	if err := config.ValidateTenantSlug(slug); err != nil {
		return err
	}
	path := configPath
	if path == "" {
		path = os.Getenv("MARIANATEK_CONFIG")
	}
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".config", "marianatek-pp-cli", "config.toml")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	cfgMap := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = toml.Unmarshal(data, &cfgMap)
	}
	cfgMap["default_tenant"] = slug
	out, err := toml.Marshal(cfgMap)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}
