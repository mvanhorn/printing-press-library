// Copyright 2026 Harvey The AI Guy and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored V2 session signon flow. Lives in its own file so regenerate
// preserves it (see hand-edit durability rules).

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/ticktick/internal/config"
)

// xDeviceHeader is required by the V2 signon endpoint; without it the API
// rejects the login as coming from an unknown device.
const xDeviceHeader = `{"platform":"web","os":"Windows 10","device":"Chrome 126.0.0.0","name":"","version":6070,"channel":"website"}`

// pp:data-source live
func newAuthSignonCmd(flags *rootFlags) *cobra.Command {
	var username string
	var password string

	cmd := &cobra.Command{
		Use:   "signon",
		Short: "Sign in with username/password and save the V2 session token",
		Long: "Signs on to TickTick's V2 API with your account credentials and saves the returned session token to the credentials file.\n" +
			"Credentials come from --username/--password or the TICKTICK_USERNAME / TICKTICK_PASSWORD environment variables.\n" +
			"Google-SSO accounts without a password should instead copy the 't' cookie from a logged-in browser and run 'auth set-token <token>'.",
		Example: "  ticktick-pp-cli auth signon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sign on to TickTick and save the session token")
				return nil
			}
			if username == "" {
				username = os.Getenv("TICKTICK_USERNAME")
			}
			if password == "" {
				password = os.Getenv("TICKTICK_PASSWORD")
			}
			if username == "" || password == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("username and password are required (flags or TICKTICK_USERNAME / TICKTICK_PASSWORD)"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]string{"username": username, "password": password}
			params := map[string]string{"wc": "true", "remember": "true"}
			headers := map[string]string{"X-Device": xDeviceHeader}
			data, status, err := c.PostWithParamsAndHeaders(ctx, "/user/signon", params, body, headers)
			if err != nil {
				return apiErr(fmt.Errorf("signon request failed: %w", err))
			}
			if status < 200 || status >= 300 {
				return authErr(fmt.Errorf("signon rejected (HTTP %d) — check credentials; Google-SSO accounts may need a password set at ticktick.com, or use 'auth set-token' with the browser 't' cookie", status))
			}
			var resp struct {
				Token    string `json:"token"`
				Username string `json:"username"`
			}
			if err := json.Unmarshal(data, &resp); err != nil || resp.Token == "" {
				return apiErr(fmt.Errorf("signon response did not include a session token"))
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.AuthHeaderVal = ""
			if err := cfg.SaveCredential(resp.Token); err != nil {
				return configErr(fmt.Errorf("saving session token: %w", err))
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"signed_on": true,
					"username":  resp.Username,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Signed on as %s; session token saved.\n", resp.Username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "TickTick account email (or set TICKTICK_USERNAME)")
	cmd.Flags().StringVar(&password, "password", "", "TickTick account password (or set TICKTICK_PASSWORD)")
	return cmd
}
