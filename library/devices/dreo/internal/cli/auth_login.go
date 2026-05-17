// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/dreoauth"

	"github.com/spf13/cobra"
)

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	// auth login intentionally has NO --password flag.
	//
	// A `--password <secret>` invocation leaks the plaintext value into
	// /proc/<pid>/cmdline, `ps aux`, audit logs, and the user's shell
	// history. Greptile P1 finding (auth_login.go) called this out;
	// every reasonable IoT/cloud CLI rejects flag-supplied passwords
	// for the same reason. Credentials come from env vars only:
	//   DREO_USERNAME, DREO_PASSWORD (or `read -s` piped via the env).
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Exchange DREO_USERNAME/DREO_PASSWORD env vars for an access token (cached for subsequent calls)",
		Example: `  # Credentials come from env, never from flags (process tables / shell history leak).
  export DREO_USERNAME=me@example.com
  export DREO_PASSWORD='your-password'
  dreo-pp-cli auth login`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			username := os.Getenv("DREO_USERNAME")
			if username == "" {
				username = cfg.DreoUsername
			}
			password := os.Getenv("DREO_PASSWORD")
			if password == "" {
				password = cfg.DreoPassword
			}
			if username == "" || password == "" {
				return usageErr(fmt.Errorf("auth login requires DREO_USERNAME and DREO_PASSWORD env vars. Flag-supplied passwords are rejected because they leak into ps/proc/shell history"))
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would log in: skipped under PRINTING_PRESS_VERIFY")
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would log in as %s against %s\n", username, cfg.BaseURL)
				return nil
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resp, err := dreoauth.Login(ctx, cfg.BaseURL, username, password)
			if err != nil {
				return authErr(fmt.Errorf("auth login failed: %w", err))
			}
			if resp.AccessToken == "" {
				return authErr(fmt.Errorf("auth login: empty access token in response"))
			}

			// Persist token and region; switch base URL if region disagrees.
			expiry := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
			if resp.ExpiresIn <= 0 {
				expiry = time.Now().Add(30 * 24 * time.Hour) // sensible default
			}
			if err := cfg.SaveTokens("", "", resp.AccessToken, "", expiry); err != nil {
				return configErr(fmt.Errorf("saving token: %w", err))
			}
			if resp.Region != "" {
				_ = cfg.SaveRegion(resp.Region)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"authenticated": true,
					"email":         username,
					"region":        resp.Region,
					"expires_in":    resp.ExpiresIn,
					"config":        cfg.Path,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in as %s, region %s\n", username, resp.Region)
			fmt.Fprintf(cmd.OutOrStdout(), "Token saved to %s\n", cfg.Path)
			return nil
		},
	}
	return cmd
}
