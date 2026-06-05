// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/powerbi/internal/config"

	"github.com/spf13/cobra"
)

// powerBIScope is the resource scope for the Power BI REST API. AAD requires
// the `.default` suffix on v2.0 endpoints.
const powerBIScope = "https://analysis.windows.net/powerbi/api/.default"

// azCLIClientID is the well-known public-client AAD app ID used by the Azure
// CLI. Microsoft authorized it for device-code login against Power BI so users
// don't need to register their own AAD app for personal use. See:
// https://github.com/Azure/azure-cli/blob/main/src/azure-cli-core/azure/cli/core/_profile.py
const azCLIClientID = "04b07795-8ddb-461a-bbee-02f9e1bf7b46"

type deviceCodeResp struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var tenant, clientID, clientSecret string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in to Power BI (device-code flow by default; service-principal with --client-secret)",
		Long: `Sign in to Power BI and persist a bearer token.

Two modes:

  Device-code (default, interactive): opens a browser to verify the device, no
  AAD app registration required. Use this on personal machines.

  Service-principal (non-interactive): pass --tenant, --client-id, and
  --client-secret. Use this in CI, Docker, or headless agents.

The token is cached at ~/.config/powerbi-pp-cli/config.toml and refreshed
automatically when the access token expires (~1 hour TTL).`,
		Example: `  # Device-code (personal use, no Azure CLI needed)
  powerbi-pp-cli auth login

  # Specific tenant
  powerbi-pp-cli auth login --tenant contoso.onmicrosoft.com

  # Service principal (headless / CI)
  powerbi-pp-cli auth login --tenant TENANT_ID --client-id CLIENT_ID --client-secret CLIENT_SECRET`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			t := tenant
			if t == "" {
				t = "organizations" // multi-tenant default
			}

			if clientSecret != "" {
				// Service principal client_credentials flow
				if clientID == "" || tenant == "" {
					return usageErr(fmt.Errorf("--client-id and --tenant are required when --client-secret is provided"))
				}
				return runServicePrincipalLogin(cmd, cfg, flags, tenant, clientID, clientSecret)
			}
			// Device-code flow
			id := clientID
			if id == "" {
				id = azCLIClientID
			}
			return runDeviceCodeLogin(cmd, cfg, flags, t, id)
		},
	}
	cmd.Flags().StringVar(&tenant, "tenant", "", "AAD tenant ID or domain (default: organizations / common)")
	cmd.Flags().StringVar(&clientID, "client-id", "", "AAD application (client) ID. Defaults to the Azure CLI's public client ID for device-code mode.")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "AAD application secret. Triggers service-principal client_credentials flow.")
	return cmd
}

func runDeviceCodeLogin(cmd *cobra.Command, cfg *config.Config, flags *rootFlags, tenant, clientID string) error {
	w := cmd.OutOrStdout()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	// Step 1: request a device code.
	dcURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/devicecode", tenant)
	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("scope", powerBIScope+" offline_access")
	resp, err := httpClient.Post(dcURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return apiErr(fmt.Errorf("requesting device code: %w", err))
	}
	defer resp.Body.Close()
	var dc deviceCodeResp
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return apiErr(fmt.Errorf("decoding device code response: %w", err))
	}
	if dc.DeviceCode == "" {
		return apiErr(fmt.Errorf("AAD did not return a device code (status %s)", resp.Status))
	}

	if !flags.asJSON {
		if dc.Message != "" {
			fmt.Fprintln(w, dc.Message)
		} else {
			fmt.Fprintf(w, "Open %s and enter code %s\n", dc.VerificationURI, dc.UserCode)
		}
		fmt.Fprintln(w, "Waiting for sign-in...")
	}

	// Step 2: poll the token endpoint.
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenant)
	interval := time.Duration(dc.Interval) * time.Second
	if interval < 3*time.Second {
		interval = 5 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	if deadline.IsZero() {
		deadline = time.Now().Add(15 * time.Minute)
	}
	for time.Now().Before(deadline) {
		time.Sleep(interval)
		pollForm := url.Values{}
		pollForm.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		pollForm.Set("client_id", clientID)
		pollForm.Set("device_code", dc.DeviceCode)
		pResp, err := httpClient.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(pollForm.Encode()))
		if err != nil {
			return apiErr(fmt.Errorf("polling token endpoint: %w", err))
		}
		var tr tokenResp
		dec := json.NewDecoder(pResp.Body)
		_ = dec.Decode(&tr)
		pResp.Body.Close()
		if tr.AccessToken != "" {
			expiry := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
			if err := cfg.SaveTokens(clientID, "", tr.AccessToken, tr.RefreshToken, expiry); err != nil {
				return configErr(err)
			}
			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{
					"signed_in":   true,
					"mode":        "device-code",
					"expires_at":  expiry.Format(time.RFC3339),
					"config_path": cfg.Path,
				}, flags)
			}
			fmt.Fprintf(w, "Signed in. Token saved to %s (expires %s)\n", cfg.Path, expiry.Format(time.RFC3339))
			return nil
		}
		// Continue polling on authorization_pending; bail on anything else.
		if tr.Error != "" && tr.Error != "authorization_pending" && tr.Error != "slow_down" {
			return authErr(fmt.Errorf("AAD: %s: %s", tr.Error, tr.ErrorDesc))
		}
		if tr.Error == "slow_down" {
			interval += 2 * time.Second
		}
	}
	return authErr(fmt.Errorf("device-code flow timed out before user completed sign-in"))
}

func runServicePrincipalLogin(cmd *cobra.Command, cfg *config.Config, flags *rootFlags, tenant, clientID, clientSecret string) error {
	w := cmd.OutOrStdout()
	httpClient := &http.Client{Timeout: 30 * time.Second}
	tokenURL := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenant)
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", powerBIScope)
	resp, err := httpClient.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return apiErr(fmt.Errorf("requesting service-principal token: %w", err))
	}
	defer resp.Body.Close()
	var tr tokenResp
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return apiErr(fmt.Errorf("decoding token response: %w", err))
	}
	if tr.AccessToken == "" {
		msg := tr.Error
		if tr.ErrorDesc != "" {
			msg = tr.ErrorDesc
		}
		if msg == "" {
			msg = resp.Status
		}
		return authErr(fmt.Errorf("AAD returned no access token: %s", msg))
	}
	expiry := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	if err := cfg.SaveTokens(clientID, clientSecret, tr.AccessToken, "", expiry); err != nil {
		return configErr(err)
	}
	if flags.asJSON {
		return printJSONFiltered(w, map[string]any{
			"signed_in":   true,
			"mode":        "service-principal",
			"expires_at":  expiry.Format(time.RFC3339),
			"config_path": cfg.Path,
		}, flags)
	}
	fmt.Fprintf(w, "Signed in (service principal). Token saved to %s (expires %s)\n", cfg.Path, expiry.Format(time.RFC3339))
	return nil
}
