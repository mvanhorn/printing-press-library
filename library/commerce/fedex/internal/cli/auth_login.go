// FedEx OAuth2 client_credentials login. Replaces the generator's generic
// `set-token` for users who have a Client ID + Client Secret pair and want
// the CLI to mint and refresh bearer tokens automatically.
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

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
	"github.com/spf13/cobra"
)

const (
	fedexSandboxBase = "https://apis-sandbox.fedex.com"
	fedexProdBase    = "https://apis.fedex.com"
	fedexTokenPath   = "/oauth/token"
	tokenSafetyWin   = 60 * time.Second
)

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

type fedexAuthError struct {
	TransactionID string `json:"transactionId"`
	Errors        []struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		clientID     string
		clientSecret string
		env          string
		service      string
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Mint an OAuth2 bearer token via FedEx client_credentials grant",
		Long: strings.TrimSpace(`
Mint an OAuth2 bearer token via the FedEx OAuth2 client_credentials grant
(POST /oauth/token with grant_type=client_credentials) and cache the token
on disk. Subsequent commands auto-refresh the token before expiry.

FedEx requires a SEPARATE developer-portal project for the Track API as of
Oct 2023 — Ship/Rate/Address and Track are mutually exclusive in a single
project. Use --service track to log in to your Track-only project; that
pair is stored separately and only used when the request path is /track/*.

Credentials default to env vars based on --service:
  --service default (default): FEDEX_API_KEY + FEDEX_SECRET_KEY
  --service track            : FEDEX_TRACK_API_KEY + FEDEX_TRACK_SECRET_KEY
`),
		Example: strings.Trim(`
  # Sandbox Ship/Rate/Address project (uses env vars)
  fedex-pp-cli auth login

  # Production
  fedex-pp-cli auth login --env prod

  # Track-only project (the second pair you need for tracking calls)
  fedex-pp-cli auth login --service track --env prod

  # Explicit credentials
  fedex-pp-cli auth login --client-id ABCDEF --client-secret SHHHH --env sandbox
`, "\n"),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would mint token via OAuth2 client_credentials")
				return nil
			}

			isTrack := false
			switch strings.ToLower(service) {
			case "track":
				isTrack = true
			case "", "default", "ship", "rate", "main":
				isTrack = false
			default:
				return fmt.Errorf("--service must be 'default' or 'track' (got %q)", service)
			}

			if clientID == "" {
				if isTrack {
					clientID = os.Getenv("FEDEX_TRACK_API_KEY")
				} else {
					clientID = os.Getenv("FEDEX_API_KEY")
				}
			}
			if clientSecret == "" {
				if isTrack {
					clientSecret = os.Getenv("FEDEX_TRACK_SECRET_KEY")
				} else {
					clientSecret = os.Getenv("FEDEX_SECRET_KEY")
				}
			}
			if clientID == "" || clientSecret == "" {
				envHint := "FEDEX_API_KEY/FEDEX_SECRET_KEY"
				if isTrack {
					envHint = "FEDEX_TRACK_API_KEY/FEDEX_TRACK_SECRET_KEY"
				}
				return authErr(fmt.Errorf("client ID and secret required (set --client-id/--client-secret or %s)", envHint))
			}

			base := fedexSandboxBase
			switch strings.ToLower(env) {
			case "prod", "production":
				base = fedexProdBase
			case "", "sandbox":
				base = fedexSandboxBase
			default:
				return fmt.Errorf("--env must be sandbox or prod (got %q)", env)
			}

			tok, err := mintFedExToken(cmd.Context(), http.DefaultClient, base, clientID, clientSecret)
			if err != nil {
				return authErr(err)
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
			label := "default (Ship/Rate/Address)"
			if isTrack {
				if err := cfg.SaveTrackTokens(clientID, clientSecret, tok.AccessToken, expiry); err != nil {
					return configErr(fmt.Errorf("saving track token: %w", err))
				}
				label = "Track-only project"
			} else {
				cfg.AuthHeaderVal = ""
				if err := cfg.SaveTokens(clientID, clientSecret, tok.AccessToken, "", expiry); err != nil {
					return configErr(fmt.Errorf("saving token: %w", err))
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Logged in to FedEx %s [%s]. Token expires %s (in %ds).\n",
				strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "apis-"),
				label,
				expiry.Format(time.RFC3339),
				tok.ExpiresIn,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&clientID, "client-id", "", "FedEx API key (Client ID); defaults to env var (varies by --service)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "FedEx Secret key (Client Secret); defaults to env var (varies by --service)")
	cmd.Flags().StringVar(&env, "env", "sandbox", "FedEx environment: sandbox (default) or prod")
	cmd.Flags().StringVar(&service, "service", "default", "Which FedEx project: default (Ship/Rate/Address) or track (Track-only project)")

	return cmd
}

// mintFedExToken POSTs the client_credentials grant to FedEx OAuth and
// returns the token response. Used by both auth login and the token
// refresh path in the client.
func mintFedExToken(ctx interface{}, httpClient *http.Client, baseURL, clientID, clientSecret string) (*tokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequest(http.MethodPost, baseURL+fedexTokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var ae fedexAuthError
		if json.Unmarshal(body, &ae) == nil && len(ae.Errors) > 0 {
			return nil, fmt.Errorf("FedEx auth error %s: %s (transaction %s)",
				ae.Errors[0].Code, ae.Errors[0].Message, ae.TransactionID)
		}
		return nil, fmt.Errorf("FedEx token endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tok tokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("FedEx token response missing access_token")
	}
	return &tok, nil
}
