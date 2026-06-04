package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoho-expense/internal/config"
)

func newAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Force-refresh the Zoho access token using the stored refresh_token",
		Example: strings.Trim(`
  zoho-expense-pp-cli auth refresh
  zoho-expense-pp-cli auth refresh --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			refreshToken := cfg.RefreshToken
			if refreshToken == "" {
				refreshToken = cfg.ZohoExpenseRefreshToken
			}
			clientID := cfg.ClientID
			if clientID == "" {
				clientID = cfg.ZohoExpenseClientId
			}
			clientSecret := cfg.ClientSecret
			if clientSecret == "" {
				clientSecret = cfg.ZohoExpenseClientSecret
			}
			if refreshToken == "" {
				return authErr(fmt.Errorf("no refresh_token in config; run 'auth login' first or set ZOHO_EXPENSE_REFRESH_TOKEN"))
			}
			if clientID == "" {
				return authErr(fmt.Errorf("no client_id in config; run 'auth login' first or set ZOHO_EXPENSE_CLIENT_ID"))
			}

			tokenURL := cfg.TokenURL
			if tokenURL == "" {
				tokenURL = "https://accounts.zoho.in/oauth/v2/token"
			}

			form := url.Values{
				"grant_type":    {"refresh_token"},
				"refresh_token": {refreshToken},
				"client_id":     {clientID},
			}
			if clientSecret != "" {
				form.Set("client_secret", clientSecret)
			}

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
			if err != nil {
				return fmt.Errorf("building refresh request: %w", err)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			httpClient := &http.Client{Timeout: 30 * time.Second}
			resp, err := httpClient.Do(req)
			if err != nil {
				return apiErr(fmt.Errorf("refreshing access token: %w", err))
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 400 {
				return apiErr(fmt.Errorf("refreshing access token: HTTP %d: %s", resp.StatusCode, string(body)))
			}

			// Zoho commonly returns "expires_in_sec" (seconds, e.g. 3600).
			// Some deployments return "expires_in" — which may be seconds
			// or milliseconds depending on grant/server. Be tolerant.
			var parsed struct {
				AccessToken  string          `json:"access_token"`
				RefreshToken string          `json:"refresh_token"`
				ExpiresIn    json.Number     `json:"expires_in"`
				ExpiresInSec json.Number     `json:"expires_in_sec"`
				TokenType    string          `json:"token_type"`
				APIDomain    string          `json:"api_domain"`
				ErrorField   string          `json:"error"`
				ErrorMessage string          `json:"error_description"`
				Raw          json.RawMessage `json:"-"`
			}
			if err := json.Unmarshal(body, &parsed); err != nil {
				return fmt.Errorf("parsing refresh response: %w", err)
			}
			if parsed.ErrorField != "" {
				return authErr(fmt.Errorf("refresh failed: %s %s", parsed.ErrorField, parsed.ErrorMessage))
			}
			if parsed.AccessToken == "" {
				return apiErr(fmt.Errorf("refresh response missing access_token: %s", string(body)))
			}

			seconds := int64(0)
			if s, err := parsed.ExpiresInSec.Int64(); err == nil && s > 0 {
				seconds = s
			} else if s, err := parsed.ExpiresIn.Int64(); err == nil && s > 0 {
				seconds = s
				// Heuristic: Zoho refresh tokens last ~1h; values > 7 days
				// of seconds are almost certainly milliseconds.
				if seconds > 7*24*3600 {
					seconds = seconds / 1000
				}
			}
			expiry := time.Time{}
			if seconds > 0 {
				expiry = time.Now().Add(time.Duration(seconds) * time.Second)
			}

			refreshOut := refreshToken
			if parsed.RefreshToken != "" {
				refreshOut = parsed.RefreshToken
			}
			if err := cfg.SaveTokens(clientID, clientSecret, parsed.AccessToken, refreshOut, expiry); err != nil {
				return fmt.Errorf("saving tokens: %w", err)
			}

			if flags.asJSON {
				out := map[string]any{
					"refreshed":      true,
					"expires_at":     "",
					"expires_in_sec": seconds,
				}
				if !expiry.IsZero() {
					out["expires_at"] = expiry.UTC().Format(time.RFC3339)
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			if seconds > 0 {
				fmt.Fprintf(os.Stderr, "Access token refreshed (expires in %dm)\n", seconds/60)
			} else {
				fmt.Fprintf(os.Stderr, "Access token refreshed\n")
			}
			return nil
		},
	}
	return cmd
}
