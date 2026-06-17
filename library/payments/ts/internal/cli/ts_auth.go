// Copyright 2026 Dickie and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored OAuth2 client-credentials login. Not generated. TreasurySpring
// mints bearer tokens at /oauth/token using HTTP Basic auth (client_id:secret)
// with a client_credentials grant. The generated bearer client cannot bootstrap
// itself, so this command performs the exchange directly and caches the token
// via the standard config path, exactly as `auth set-token` would.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/ts/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/ts/internal/config"
)

// tokenURLFromBase derives the token endpoint from the configured API base URL.
// The token endpoint lives at the host root (/oauth/token), not under the
// /api/v1 path prefix the resource endpoints use.
func tokenURLFromBase(baseURL string) string {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "https://api.treasuryspring.com/oauth/token"
	}
	return u.Scheme + "://" + u.Host + "/oauth/token"
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var clientID string
	var clientSecret string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Exchange client credentials for a bearer token and cache it",
		Long: strings.Trim(`
Performs the OAuth2 client-credentials exchange against /oauth/token and saves
the resulting bearer token to the config file. Reads TS_CLIENT_ID and
TS_CLIENT_SECRET from the environment unless --client-id / --client-secret are
passed. The token is never printed.`, "\n"),
		Example: strings.Trim(`
  ts-pp-cli auth login
  ts-pp-cli auth login --client-id <id> --client-secret <secret>`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			id := strings.TrimSpace(clientID)
			if id == "" {
				id = strings.TrimSpace(cfg.ClientID)
			}
			secret := strings.TrimSpace(clientSecret)
			if secret == "" {
				secret = strings.TrimSpace(cfg.ClientSecret)
			}
			tokenURL := tokenURLFromBase(cfg.BaseURL)

			// Verify-friendly: never hit the network in the verifier sandbox or
			// under --dry-run. Print what would happen and exit 0.
			if cliutil.IsVerifyEnv() || flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would exchange client credentials at %s\n", tokenURL)
				return nil
			}

			if id == "" || secret == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("set TS_CLIENT_ID and TS_CLIENT_SECRET (or pass --client-id/--client-secret)"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			token, expiry, err := exchangeClientCredentials(ctx, tokenURL, id, secret)
			if err != nil {
				return err
			}

			cfg.AuthHeaderVal = ""
			if err := cfg.SaveTokens(id, secret, token, "", expiry); err != nil {
				return configErr(fmt.Errorf("saving token: %w", err))
			}

			if flags.asJSON || flags.agent {
				out := map[string]any{"logged_in": true, "config_path": cfg.Path}
				if !expiry.IsZero() {
					out["expires_at"] = expiry.UTC().Format(time.RFC3339)
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if expiry.IsZero() {
				fmt.Fprintf(cmd.OutOrStdout(), "Logged in. Token saved to %s\n", cfg.Path)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Logged in. Token saved to %s (expires %s)\n", cfg.Path, expiry.UTC().Format(time.RFC3339))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth2 client id (default: $TS_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "OAuth2 client secret (default: $TS_CLIENT_SECRET)")
	return cmd
}

// exchangeClientCredentials POSTs the client_credentials grant to the token
// endpoint using HTTP Basic auth and returns the access token plus its expiry.
func exchangeClientCredentials(ctx context.Context, tokenURL, clientID, clientSecret string) (string, time.Time, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("building token request: %w", err)
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Carry an explicit client timeout (derived from the bound context's
	// deadline) so the token exchange matches the rest of the client, which
	// never relies on http.DefaultClient's unbounded transport.
	client := &http.Client{}
	if deadline, ok := ctx.Deadline(); ok {
		client.Timeout = time.Until(deadline)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("requesting token: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", time.Time{}, fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", time.Time{}, fmt.Errorf("parsing token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("token endpoint response did not contain access_token")
	}
	var expiry time.Time
	if tr.ExpiresIn > 0 {
		expiry = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return tr.AccessToken, expiry, nil
}
