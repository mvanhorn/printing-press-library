// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored auth flow for the SRAM AXS CLI (Auth0 password-realm). Not generated.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/axs/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/axs/internal/config"

	"github.com/spf13/cobra"
)

// Public SPA OAuth parameters, lifted from the official axs.sram.com web client
// bundle. The client id is a public SPA identifier, not a secret.
const (
	axsAuthTokenURL = "https://sramid-auth.sram.com/oauth/token"
	axsAuthClientID = "zIvfleoh46jy4behzZdkFoUIiW70KX23"
	axsAuthRealm    = "sramid-db"
	axsAuthAudience = "https://api.quarqnet.com"
	axsAuthScope    = "openid profile email offline_access"
	axsAuthGrant    = "http://auth0.com/oauth/grant-type/password-realm"
)

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var email, password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in with your SRAM account (email + password)",
		Long: "Authenticate against SRAM's identity service using your SRAM account email and password, " +
			"and store the resulting access token for subsequent commands. " +
			"Credentials may be passed with --email/--password or the SRAM_AXS_EMAIL/SRAM_AXS_PASSWORD env vars. " +
			"The password is never written to disk or logs — only the returned token is stored.",
		Example: "  SRAM_AXS_EMAIL=you@example.com SRAM_AXS_PASSWORD=... axs-pp-cli auth login",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if email == "" {
				email = os.Getenv("SRAM_AXS_EMAIL")
			}
			if password == "" {
				password = os.Getenv("SRAM_AXS_PASSWORD")
			}
			// Side-effect guard: never dial out during a verify pass.
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would authenticate against SRAM identity service")
				return nil
			}
			if email == "" || password == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("email and password are required (use --email/--password or SRAM_AXS_EMAIL/SRAM_AXS_PASSWORD)"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			tok, err := axsPasswordRealmLogin(ctx, email, password)
			if err != nil {
				return err
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
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"authenticated": true,
					"expires_at":    expiry.Format(time.RFC3339),
					"config_path":   cfg.Path,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Signed in. Token saved to %s (expires %s)\n", cfg.Path, expiry.Format(time.RFC3339))
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "SRAM account email (or set SRAM_AXS_EMAIL)")
	cmd.Flags().StringVar(&password, "password", "", "SRAM account password (or set SRAM_AXS_PASSWORD)")
	return cmd
}

type axsTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

func axsPasswordRealmLogin(ctx context.Context, email, password string) (*axsTokenResponse, error) {
	payload := map[string]string{
		"grant_type": axsAuthGrant,
		"client_id":  axsAuthClientID,
		"realm":      axsAuthRealm,
		"audience":   axsAuthAudience,
		"scope":      axsAuthScope,
		"username":   email,
		"password":   password,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, axsAuthTokenURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contacting SRAM identity service: %w", err)
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = dec.Decode(&e)
		if e.ErrorDescription != "" {
			return nil, fmt.Errorf("login failed (%d): %s", resp.StatusCode, e.ErrorDescription)
		}
		return nil, fmt.Errorf("login failed: HTTP %d", resp.StatusCode)
	}
	var tok axsTokenResponse
	if err := dec.Decode(&tok); err != nil {
		return nil, fmt.Errorf("decoding token response: %w", err)
	}
	if tok.AccessToken == "" {
		return nil, fmt.Errorf("login succeeded but no access token was returned")
	}
	return &tok, nil
}
