// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: self-hosted login flow. POSTs /api/auth/login with
// username/password and stores the returned token in the CLI config, so
// commands authenticate without the user hand-managing UMAMI_TOKEN.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/umami/internal/config"
)

// pp:data-source live
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var username string
	var password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to a self-hosted Umami instance and store the API token",
		Long: `Log in to a self-hosted Umami instance with the dashboard username and
password, then store the returned token in the CLI config. Umami Cloud users
should skip this and set UMAMI_API_KEY instead.

Credentials come from --username/--password or the UMAMI_USERNAME and
UMAMI_PASSWORD environment variables. The token issued by self-hosted Umami
is long-lived; re-run this command if the API starts returning 401.`,
		Example: "  umami-pp-cli auth login --username admin --password 's3cret'\n  UMAMI_USERNAME=admin UMAMI_PASSWORD=s3cret umami-pp-cli auth login",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would log in and store the token in config")
				return nil
			}
			if username == "" {
				username = os.Getenv("UMAMI_USERNAME")
			}
			if password == "" {
				password = os.Getenv("UMAMI_PASSWORD")
			}
			if username == "" || password == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--username and --password (or UMAMI_USERNAME / UMAMI_PASSWORD) are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]string{"username": username, "password": password}
			data, status, err := c.Post(ctx, "/api/auth/login", body)
			if status == 401 {
				return authErr(fmt.Errorf("incorrect username or password"))
			}
			if err != nil {
				return apiErr(fmt.Errorf("login request failed: %w", err))
			}
			var resp struct {
				Token string `json:"token"`
				User  struct {
					ID       string `json:"id"`
					Username string `json:"username"`
					Role     string `json:"role"`
					IsAdmin  bool   `json:"isAdmin"`
				} `json:"user"`
			}
			if err := json.Unmarshal(data, &resp); err != nil || resp.Token == "" {
				return apiErr(fmt.Errorf("unexpected login response (status %d)", status))
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(fmt.Errorf("loading config: %w", err))
			}
			if err := cfg.SaveTokens("", "", resp.Token, "", time.Time{}); err != nil {
				return configErr(fmt.Errorf("saving token: %w", err))
			}
			out := map[string]any{
				"ok":       true,
				"username": resp.User.Username,
				"role":     resp.User.Role,
				"is_admin": resp.User.IsAdmin,
				"stored":   cfg.Path,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Umami dashboard username")
	cmd.Flags().StringVar(&password, "password", "", "Umami dashboard password")
	return cmd
}
