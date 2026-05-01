// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/kalshi/internal/config"
)

func newAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication credentials",
	}

	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthSetupCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))

	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Show authentication status",
		Example: "  kalshi-pp-cli auth status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			w := cmd.OutOrStdout()
			if !cfg.HasAuth() {
				fmt.Fprintln(w, red("Not authenticated"))
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Set your credentials:")
				fmt.Fprintln(w, "  export KALSHI_API_KEY=\"your-api-key-uuid\"")
				fmt.Fprintln(w, "  export KALSHI_PRIVATE_KEY_PATH=\"~/.kalshi/private_key.pem\"")
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Or run: kalshi-pp-cli auth setup")
				if cfg.APIKey != "" && cfg.PrivateKey() == nil {
					fmt.Fprintln(w, "")
					fmt.Fprintln(w, yellow("API key found but no private key configured."))
				}
				return authErr(fmt.Errorf("no credentials configured"))
			}

			fmt.Fprintln(w, green("Authenticated"))
			fmt.Fprintf(w, "  API Key: %s\n", maskLast4(cfg.APIKey))
			if cfg.AuthSource != "" {
				fmt.Fprintf(w, "  Source:  %s\n", cfg.AuthSource)
			}
			fmt.Fprintf(w, "  Config:  %s\n", cfg.Path)
			fmt.Fprintf(w, "  Base URL: %s\n", cfg.BaseURL)
			return nil
		},
	}
}

func newAuthSetupCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "setup",
		Short:   "Show how to configure Kalshi API credentials",
		Example: "  kalshi-pp-cli auth setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Kalshi API Authentication Setup")
			fmt.Fprintln(w, "================================")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "1. Go to https://kalshi.com/api and generate an API key pair")
			fmt.Fprintln(w, "2. Save your private key to a file (e.g., ~/.kalshi/private_key.pem)")
			fmt.Fprintln(w, "3. Set environment variables:")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "   export KALSHI_API_KEY=\"your-api-key-uuid\"")
			fmt.Fprintln(w, "   export KALSHI_PRIVATE_KEY_PATH=\"~/.kalshi/private_key.pem\"")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "Or add to your config file:")
			cfg, _ := config.Load(flags.configPath)
			if cfg != nil {
				fmt.Fprintf(w, "   %s\n", cfg.Path)
			}
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "   api_key = \"your-api-key-uuid\"")
			fmt.Fprintln(w, "   private_key_path = \"~/.kalshi/private_key.pem\"")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "For demo/sandbox environment:")
			fmt.Fprintln(w, "   export KALSHI_ENV=demo")
			return nil
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "logout",
		Short:   "Show how to clear stored credentials",
		Example: "  kalshi-pp-cli auth logout",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "To clear Kalshi credentials:")
			fmt.Fprintln(w, "  unset KALSHI_API_KEY")
			fmt.Fprintln(w, "  unset KALSHI_PRIVATE_KEY_PATH")

			if os.Getenv("KALSHI_API_KEY") != "" {
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, yellow("Note: KALSHI_API_KEY is currently set in your environment."))
			}
			return nil
		},
	}
}

func maskLast4(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}
