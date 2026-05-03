// Copyright 2026 cathryn-lavery. Licensed under Apache-2.0. See LICENSE.
// Doctor reports readiness without exposing secrets or probing PII-bearing endpoints.

package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/tiktok-shop/internal/config"
	"github.com/spf13/cobra"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI health and TikTok Shop auth readiness",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := flags.loadConfig()
			if err != nil {
				return err
			}

			report := map[string]any{
				"config":                "ok",
				"config_path":           cfg.Path,
				"version":               version,
				"official_docs_checked": config.OfficialDocs,
			}

			report["app_credentials"] = status(cfg.HasAppCredentials())
			report["token_bundle"] = status(cfg.HasTokenBundle())
			report["shop_selector"] = status(cfg.HasShopSelector())
			report["open_api_base_url"] = valueOrPending(cfg.BaseURL, client.DefaultOpenAPIBaseURL)
			report["auth_base_url"] = valueOrPending(cfg.AuthBaseURL, client.DefaultAuthBaseURL)
			report["auth_source"] = valueOrPending(cfg.AuthSource, "not configured")
			report["env_vars"] = envReport()
			report["token_validation"] = "configured-material check only; use 'shops info --dry-run' to inspect signed request shape before live calls"
			report["token_validation_doc_url"] = client.AuthorizationOverviewURL
			report["commerce_endpoints"] = "read-only confirmed endpoints implemented; inventory mutation and returns/refunds deferred"
			report["mutation_policy"] = "no mutation retries; inventory update remains placeholder pending idempotency design"

			if flags.asJSON {
				return printJSON(cmd, report)
			}

			for _, row := range []struct{ key, label string }{
				{"config", "Config"},
				{"app_credentials", "App credentials"},
				{"token_bundle", "Token bundle"},
				{"shop_selector", "Shop selector"},
				{"token_validation", "Token validation"},
				{"commerce_endpoints", "Commerce endpoints"},
			} {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %v\n", row.label, report[row.key])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Config path: %s\n", cfg.Path)
			return nil
		},
	}
}

func status(ok bool) string {
	if ok {
		return "configured"
	}
	return "missing"
}

func valueOrPending(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func envReport() map[string]bool {
	vars := []string{
		config.EnvAppKey,
		config.EnvAppSecret,
		config.EnvAccessToken,
		config.EnvRefreshToken,
		config.EnvShopID,
		config.EnvShopCipher,
		config.EnvBaseURL,
		config.EnvAuthBaseURL,
	}
	out := make(map[string]bool, len(vars))
	for _, name := range vars {
		out[name] = os.Getenv(name) != ""
	}
	return out
}
