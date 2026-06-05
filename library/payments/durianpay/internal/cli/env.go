// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: sandbox/live environment switching. Durianpay keeps fully
// separate credentials per environment (dp_test_/dp_live_ keys, separate SNAP
// keypairs), so the switch rewrites base_url in the config file and reports
// which credential env vars must change with it.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/durianpay/internal/config"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

const (
	liveBaseURL    = "https://api.durianpay.id/v1"
	sandboxBaseURL = "https://api-sandbox.durianpay.id/v1"
)

// pp:data-source local
func newEnvCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env [sandbox|live|show]",
		Short: "Show or switch the active environment (sandbox vs live base URL)",
		Long: strings.Trim(`
Shows or switches the environment this CLI talks to. Switching rewrites
base_url in the config file. Credentials are NOT switched for you — sandbox
and live use completely separate API keys (dp_test_ vs dp_live_) and separate
SNAP keypairs, so the command reports which env vars you still need to flip.
A DURIANPAY_BASE_URL env var always overrides the config file.
`, "\n"),
		Example: strings.Trim(`
  durianpay-pp-cli env show
  durianpay-pp-cli env sandbox
  durianpay-pp-cli env live --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "show",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show or switch the configured environment")
				return nil
			}
			mode := "show"
			if len(args) > 0 {
				mode = strings.ToLower(args[0])
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			current := "live"
			if strings.Contains(cfg.BaseURL, "sandbox") {
				current = "sandbox"
			}
			switch mode {
			case "show":
				return flags.printJSON(cmd, map[string]any{
					"environment":     current,
					"base_url":        cfg.BaseURL,
					"base_url_source": map[bool]string{true: "env:DURIANPAY_BASE_URL", false: "config"}[os.Getenv("DURIANPAY_BASE_URL") != ""],
					"config_path":     cfg.Path,
				})
			case "sandbox", "live":
				target := liveBaseURL
				if mode == "sandbox" {
					target = sandboxBaseURL
				}
				if err := writeBaseURL(cfg.Path, target); err != nil {
					return configErr(err)
				}
				out := map[string]any{
					"environment": mode,
					"base_url":    target,
					"config_path": cfg.Path,
					"reminders": []string{
						"DURIANPAY_API_KEY must be a " + map[string]string{"sandbox": "dp_test_", "live": "dp_live_"}[mode] + " key",
						"SNAP credentials (DURIANPAY_SNAP_CLIENT_KEY/SECRET/PRIVATE_KEY) are per-environment too",
						"cached SNAP tokens are environment-scoped and will re-mint automatically",
					},
				}
				if os.Getenv("DURIANPAY_BASE_URL") != "" {
					out["warning"] = "DURIANPAY_BASE_URL is set and overrides the config file — unset it for this switch to take effect"
				}
				return flags.printJSON(cmd, out)
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown environment %q: use sandbox, live, or show", mode))
			}
		},
	}
	return cmd
}

// writeBaseURL updates (or creates) the config file with the new base_url,
// preserving every other key.
func writeBaseURL(path, baseURL string) error {
	raw := map[string]any{}
	// #nosec G304 -- path is the CLI's own config file path, not arbitrary user input.
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parsing config %s: %w", path, err)
		}
	}
	raw["base_url"] = baseURL
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := toml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
