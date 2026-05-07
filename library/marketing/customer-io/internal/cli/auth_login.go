// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

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

	"github.com/mvanhorn/printing-press-library/library/marketing/customer-io/internal/config"

	"github.com/spf13/cobra"
)

// newAuthLoginCmd exchanges a Service Account token (sa_live_*) for a
// short-lived JWT and saves it via SaveTokens. The cached JWT becomes the
// Bearer credential for both the Journeys UI API (/v1/...) and the CDP
// control plane (/cdp/api/...). Re-run before the JWT expires; SA tokens
// themselves are long-lived.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var saToken string
	var region string
	var readOnly bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Exchange a Service Account token (sa_live_*) for a JWT and cache it",
		Long: `Exchange a Service Account token for a short-lived JWT.

The Service Account token (sa_live_* prefix) is the long-lived credential.
Customer.io's API uses a short-lived JWT minted from that token via the
OAuth 2.0 client-credentials grant at
  POST <base>/v1/service_accounts/oauth/token

Login resolves the SA token from --sa-token, then $CIO_TOKEN, and
caches the resulting JWT at ~/.config/customer-io-pp-cli/config.toml. The
JWT is used as the Bearer credential for every subsequent request.

JWTs expire (typically minutes-to-hours); re-run login when 'auth status'
or any command reports an authentication failure.`,
		Example: strings.Trim(`
  customer-io-pp-cli auth login --sa-token sa_live_xxx --region us
  customer-io-pp-cli auth login --region eu
  CIO_TOKEN=sa_live_xxx customer-io-pp-cli auth login
  customer-io-pp-cli auth login --read-only
`, "\n"),
		Annotations: map[string]string{
			"mcp:hidden": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			tok := strings.TrimSpace(saToken)
			if tok == "" {
				tok = strings.TrimSpace(os.Getenv("CIO_TOKEN"))
			}
			if tok == "" {
				return usageErr(fmt.Errorf("no Service Account token; pass --sa-token or set CIO_TOKEN"))
			}
			if !strings.HasPrefix(tok, "sa_live_") && !strings.HasPrefix(tok, "sa_test_") {
				return usageErr(fmt.Errorf("token does not look like a Service Account token (expected sa_live_* or sa_test_* prefix)"))
			}

			region = strings.ToLower(strings.TrimSpace(region))
			if region == "" {
				if v := strings.ToLower(strings.TrimSpace(os.Getenv("CUSTOMERIO_REGION"))); v != "" {
					region = v
				} else {
					region = "us"
				}
			}
			if region != "us" && region != "eu" {
				return usageErr(fmt.Errorf("region must be us or eu (got %q)", region))
			}
			baseURL := baseURLForRegion(region)

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			tr, err := exchangeServiceAccountToken(ctx, baseURL, tok, readOnly)
			if err != nil {
				return apiErr(fmt.Errorf("token exchange against %s: %w", baseURL, err))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			// Clear any legacy auth_header so AuthHeader() falls through to
			// "Bearer " + AccessToken with the freshly minted JWT.
			cfg.AuthHeaderVal = ""
			// Region drives BaseURL persistence.
			cfg.BaseURL = baseURL
			expiresAt := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
			if tr.ExpiresIn <= 0 {
				expiresAt = time.Now().Add(55 * time.Minute)
			}
			if err := cfg.SaveTokens("", "", tr.AccessToken, "", expiresAt); err != nil {
				return configErr(fmt.Errorf("saving JWT: %w", err))
			}

			expiresIn := int(time.Until(expiresAt).Seconds())

			// Discover account_id + environment_ids visible to the SA token.
			account, accErr := fetchCurrentAccount(ctx, baseURL, tr.AccessToken)

			if flags.asJSON {
				out := map[string]any{
					"saved":      true,
					"region":     region,
					"base_url":   baseURL,
					"expires_in": expiresIn,
					"scope":      tr.Scope,
					"config":     cfg.Path,
				}
				if account != nil {
					out["account_id"] = account.ID
					out["account_name"] = account.Name
					out["environment_ids"] = account.EnvironmentIDs
				}
				if accErr != nil {
					out["account_lookup_warning"] = accErr.Error()
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "JWT cached (region=%s, expires_in=%ds)\n", region, expiresIn)
			fmt.Fprintf(cmd.OutOrStdout(), "Config: %s\n", cfg.Path)
			if account != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Account: %s (%s)\n", account.Name, account.ID)
				if len(account.EnvironmentIDs) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Environment IDs (workspaces): %s\n", strings.Join(account.EnvironmentIDs, ", "))
					fmt.Fprintln(cmd.OutOrStdout(), "Pass one as the first positional argument to env-scoped commands, e.g. 'customer-io-pp-cli campaigns list "+account.EnvironmentIDs[0]+"'.")
				}
			} else if accErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: account lookup failed (%v) — auth still cached\n", accErr)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Run 'customer-io-pp-cli doctor' to verify connectivity.")
			return nil
		},
	}
	cmd.Flags().StringVar(&saToken, "sa-token", "", "Service Account token (sa_live_*); falls back to $CIO_TOKEN")
	cmd.Flags().StringVar(&region, "region", "", "Region: us or eu (defaults to $CUSTOMERIO_REGION or us)")
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Request a read-only JWT (only GET requests succeed)")
	return cmd
}

// baseURLForRegion mirrors the canonical mapping used by the official cio CLI.
func baseURLForRegion(region string) string {
	if strings.ToLower(strings.TrimSpace(region)) == "eu" {
		return "https://eu.fly.customer.io"
	}
	return "https://us.fly.customer.io"
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope,omitempty"`
}

type accountInfo struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	EnvironmentIDs []string `json:"environment_ids"`
}

// fetchCurrentAccount calls /v1/accounts/current with the freshly-minted JWT
// to discover the account_id and the environment_ids visible to the SA token.
// Failure here is non-fatal: auth is still cached, the user just doesn't get
// the env-id hints.
func fetchCurrentAccount(ctx context.Context, baseURL, jwt string) (*accountInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/v1/accounts/current", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Accept", "application/json")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from /v1/accounts/current", resp.StatusCode)
	}
	var raw struct {
		Account accountInfo `json:"account"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding account: %w", err)
	}
	if raw.Account.ID == "" {
		return nil, fmt.Errorf("no account in response")
	}
	return &raw.Account, nil
}

// exchangeServiceAccountToken performs the RFC-6749 client-credentials grant
// against POST <base>/v1/service_accounts/oauth/token with the SA token in
// the client_secret field. Customer.io accepts no client_id.
func exchangeServiceAccountToken(ctx context.Context, baseURL, saToken string, readOnly bool) (*oauthTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_secret", saToken)
	if readOnly {
		form.Set("scope", "read_only")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/v1/service_accounts/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from token endpoint: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tr oauthTokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned empty access_token")
	}
	return &tr, nil
}
