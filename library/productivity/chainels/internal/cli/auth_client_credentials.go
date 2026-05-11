package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/chainels/internal/config"
	"github.com/spf13/cobra"
)

// newAuthClientCredentialsCmd issues a userless access_token via OAuth2
// client_credentials. Chainels' default `auth login` is authorization_code
// (browser-interactive); this command lets headless agents and CI runners
// authenticate without opening a browser.
func newAuthClientCredentialsCmd(flags *rootFlags) *cobra.Command {
	var clientID string
	var clientSecret string
	var scopes string
	cmd := &cobra.Command{
		Use:     "client-credentials",
		Short:   "Exchange client_id + client_secret for an access_token (no browser)",
		Long:    "OAuth2 client_credentials flow. POSTs to the token endpoint and saves the resulting access_token to the local config. Use this in CI or headless agent contexts where the interactive `auth login` browser handshake is impossible.",
		Example: "  chainels-pp-cli auth client-credentials --client-id $CHAINELS_CLIENT_ID --client-secret $CHAINELS_CLIENT_SECRET",
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if clientID == "" {
				return fmt.Errorf("--client-id is required (or set CHAINELS_CLIENT_ID)")
			}
			if clientSecret == "" {
				return fmt.Errorf("--client-secret is required (or set CHAINELS_CLIENT_SECRET)")
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			tokenURL := cfg.TokenURL
			if tokenURL == "" {
				tokenURL = "https://www.chainels.com/oauth/access_token"
			}
			params := url.Values{
				"grant_type":    {"client_credentials"},
				"client_id":     {clientID},
				"client_secret": {clientSecret},
			}
			if scopes != "" {
				params.Set("scope", strings.Join(strings.Fields(strings.ReplaceAll(scopes, ",", " ")), " "))
			}
			resp, err := http.PostForm(tokenURL, params)
			if err != nil {
				return fmt.Errorf("requesting token: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 400 {
				var body map[string]any
				_ = json.NewDecoder(resp.Body).Decode(&body)
				return fmt.Errorf("token request failed: HTTP %d: %v", resp.StatusCode, body)
			}
			var tokenResp struct {
				AccessToken string `json:"access_token"`
				ExpiresIn   int    `json:"expires_in"`
				TokenType   string `json:"token_type"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
				return fmt.Errorf("parsing token response: %w", err)
			}
			if tokenResp.AccessToken == "" {
				return fmt.Errorf("no access_token in token response")
			}
			expiry := time.Time{}
			if tokenResp.ExpiresIn > 0 {
				expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
			}
			if err := cfg.SaveTokens(clientID, clientSecret, tokenResp.AccessToken, "", expiry); err != nil {
				return fmt.Errorf("saving tokens: %w", err)
			}
			fmt.Fprintf(os.Stderr, "OK Token saved (grant=client_credentials")
			if !expiry.IsZero() {
				fmt.Fprintf(os.Stderr, ", expires %s", expiry.Format(time.RFC3339))
			}
			fmt.Fprintln(os.Stderr, ").")
			return nil
		},
	}
	cmd.Flags().StringVar(&clientID, "client-id", os.Getenv("CHAINELS_CLIENT_ID"), "OAuth2 client ID (defaults to $CHAINELS_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", os.Getenv("CHAINELS_CLIENT_SECRET"), "OAuth2 client secret (defaults to $CHAINELS_CLIENT_SECRET)")
	cmd.Flags().StringVar(&scopes, "scopes", "basic", "Space- or comma-separated OAuth scopes")
	return cmd
}
