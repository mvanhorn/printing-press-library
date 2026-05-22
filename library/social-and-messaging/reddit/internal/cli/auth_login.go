// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/reddit/internal/config"
)

// newAuthLoginCmd performs the Reddit OAuth2 script-app password grant flow,
// exchanging client credentials + Reddit username/password for a bearer token,
// and persisting the token to the local config. The token is good for 1 hour;
// refresh with `auth refresh` (uses stored refresh token when available).
//
// Reddit OAuth2 reference:
//
//	POST https://www.reddit.com/api/v1/access_token
//	Authorization: Basic base64(client_id:client_secret)
//	Content-Type: application/x-www-form-urlencoded
//	Body: grant_type=password&username=X&password=Y[&otp=Z]
//
// Required env vars (REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET, REDDIT_USERNAME,
// REDDIT_PASSWORD, REDDIT_USER_AGENT) can be passed via flags instead.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		clientID     string
		clientSecret string
		username     string
		password     string
		twoFA        string
		userAgent    string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Acquire a bearer token via Reddit OAuth2 script-app password grant",
		Long: `Acquire a Reddit OAuth2 bearer token using the script-app password grant flow.

Reddit free tier allows 100 QPM per client_id. The token expires in 1 hour;
re-run 'auth login' (or 'auth refresh' if you have a refresh token) to renew.

Required app type: 'script' from https://www.reddit.com/prefs/apps
Required env vars (or pass via flags):
  REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET (from the app you created)
  REDDIT_USERNAME, REDDIT_PASSWORD       (your Reddit account)
  REDDIT_USER_AGENT                      (e.g. "myapp/0.1 by /u/<username>")`,
		Example: `  reddit-pp-cli auth login
  reddit-pp-cli auth login --client-id ABC --client-secret XYZ --username me --password pw
  REDDIT_USER_AGENT="my-app/0.1 by /u/me" reddit-pp-cli auth login`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if clientID == "" {
				clientID = os.Getenv("REDDIT_CLIENT_ID")
			}
			if clientSecret == "" {
				clientSecret = os.Getenv("REDDIT_CLIENT_SECRET")
			}
			if username == "" {
				username = os.Getenv("REDDIT_USERNAME")
			}
			if password == "" {
				password = os.Getenv("REDDIT_PASSWORD")
			}
			if userAgent == "" {
				userAgent = os.Getenv("REDDIT_USER_AGENT")
			}

			missing := []string{}
			if clientID == "" {
				missing = append(missing, "REDDIT_CLIENT_ID (or --client-id)")
			}
			if clientSecret == "" {
				missing = append(missing, "REDDIT_CLIENT_SECRET (or --client-secret)")
			}
			if username == "" {
				missing = append(missing, "REDDIT_USERNAME (or --username)")
			}
			if password == "" {
				missing = append(missing, "REDDIT_PASSWORD (or --password)")
			}
			if userAgent == "" {
				missing = append(missing, "REDDIT_USER_AGENT (or --user-agent)")
			}
			if len(missing) > 0 {
				return usageErr(fmt.Errorf("missing required credentials: %s", strings.Join(missing, ", ")))
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would acquire OAuth2 token from https://www.reddit.com/api/v1/access_token")
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would POST grant_type=password to /api/v1/access_token")
				return nil
			}

			pw := password
			if twoFA != "" {
				pw = password + ":" + twoFA
			}

			form := url.Values{}
			form.Set("grant_type", "password")
			form.Set("username", username)
			form.Set("password", pw)

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			req, err := http.NewRequestWithContext(ctx, "POST",
				"https://www.reddit.com/api/v1/access_token",
				strings.NewReader(form.Encode()))
			if err != nil {
				return apiErr(fmt.Errorf("building request: %w", err))
			}
			req.SetBasicAuth(clientID, clientSecret)
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("User-Agent", userAgent)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return apiErr(fmt.Errorf("posting to access_token endpoint: %w", err))
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != 200 {
				return authErr(fmt.Errorf("Reddit returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
			}

			var tok struct {
				AccessToken  string `json:"access_token"`
				TokenType    string `json:"token_type"`
				ExpiresIn    int    `json:"expires_in"`
				Scope        string `json:"scope"`
				RefreshToken string `json:"refresh_token"`
				Error        string `json:"error"`
			}
			if err := json.Unmarshal(body, &tok); err != nil {
				return apiErr(fmt.Errorf("parsing token response: %w", err))
			}
			if tok.Error != "" {
				return authErr(fmt.Errorf("Reddit error: %s", tok.Error))
			}
			if tok.AccessToken == "" {
				return authErr(fmt.Errorf("Reddit returned empty access_token"))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.AuthHeaderVal = ""
			expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
			if err := cfg.SaveTokens("", "", tok.AccessToken, tok.RefreshToken, expiry); err != nil {
				return configErr(fmt.Errorf("saving token: %w", err))
			}

			out := map[string]any{
				"authenticated": true,
				"token_type":    tok.TokenType,
				"expires_in":    tok.ExpiresIn,
				"scope":         tok.Scope,
				"config_path":   cfg.Path,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s acquired (expires in %ds, scope: %s)\n",
				green("Token"), tok.ExpiresIn, tok.Scope)
			fmt.Fprintf(cmd.OutOrStdout(), "Saved to: %s\n", cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&clientID, "client-id", "", "Reddit app client ID (default $REDDIT_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Reddit app client secret (default $REDDIT_CLIENT_SECRET)")
	cmd.Flags().StringVar(&username, "username", "", "Reddit username (default $REDDIT_USERNAME)")
	cmd.Flags().StringVar(&password, "password", "", "Reddit password (default $REDDIT_PASSWORD)")
	cmd.Flags().StringVar(&twoFA, "two-fa", "", "Optional 2FA code (appended as password:CODE)")
	cmd.Flags().StringVar(&userAgent, "user-agent", "", "Reddit-compliant User-Agent (default $REDDIT_USER_AGENT)")
	return cmd
}
