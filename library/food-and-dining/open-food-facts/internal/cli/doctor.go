// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Report Open Food Facts request posture and local configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := currentConfig()
			report := map[string]any{
				"cli":         "open-food-facts-pp-cli",
				"go":          runtime.Version(),
				"os":          runtime.GOOS + "/" + runtime.GOARCH,
				"base_url":    cfg.BaseURL,
				"read_only":   true,
				"auth":        "none for read operations",
				"user_agent":  cfg.UserAgent,
				"configured":  configuredEnv(),
				"rate_limits": rateLimitFacts(),
				"status":      "ok",
			}
			if flags.json || flags.agent || flags.compact {
				return writeJSON(cmd, flags, report)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "open-food-facts-pp-cli doctor\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  go: %s\n", runtime.Version())
			fmt.Fprintf(cmd.OutOrStdout(), "  os: %s/%s\n", runtime.GOOS, runtime.GOARCH)
			fmt.Fprintf(cmd.OutOrStdout(), "  base_url: %s\n", cfg.BaseURL)
			fmt.Fprintf(cmd.OutOrStdout(), "  user_agent: %s\n", cfg.UserAgent)
			fmt.Fprintf(cmd.OutOrStdout(), "  read_only: true\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  status: ok\n")
			return nil
		},
	}
}

func configuredEnv() map[string]bool {
	return map[string]bool{
		baseURLEnv:       os.Getenv(baseURLEnv) != "",
		userAgentEnv:     os.Getenv(userAgentEnv) != "",
		contactEmailEnv:  os.Getenv(contactEmailEnv) != "",
		"read_api_key":   false,
		"write_session":  false,
		"write_password": false,
	}
}
