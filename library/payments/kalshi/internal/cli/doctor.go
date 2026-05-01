// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/kalshi/internal/config"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Check CLI health",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{}

			// Check config
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				report["config"] = fmt.Sprintf("error: %s", err)
			} else {
				report["config"] = "ok"
				report["config_path"] = cfg.Path
				report["base_url"] = cfg.BaseURL
			}

			// Check auth
			if cfg != nil {
				if cfg.HasAuth() {
					report["auth"] = "configured (RSA-PSS)"
					report["auth_source"] = cfg.AuthSource
					report["api_key"] = maskLast4(cfg.APIKey)
				} else if cfg.APIKey != "" {
					report["auth"] = "partial (API key set but no private key)"
					report["auth_hint"] = "export KALSHI_PRIVATE_KEY_PATH=~/.kalshi/private_key.pem"
				} else {
					report["auth"] = "not configured"
					report["auth_hint"] = "export KALSHI_API_KEY=<your-key> KALSHI_PRIVATE_KEY_PATH=<path>"
				}
			}

			// Check auth environment variables
			envChecks := map[string]string{
				"KALSHI_API_KEY":          os.Getenv("KALSHI_API_KEY"),
				"KALSHI_PRIVATE_KEY_PATH": os.Getenv("KALSHI_PRIVATE_KEY_PATH"),
				"KALSHI_PRIVATE_KEY":      os.Getenv("KALSHI_PRIVATE_KEY"),
			}
			setCount := 0
			for _, v := range envChecks {
				if v != "" {
					setCount++
				}
			}
			report["env_vars"] = fmt.Sprintf("%d/%d set", setCount, len(envChecks))

			// Check API connectivity
			if cfg != nil && cfg.BaseURL != "" {
				httpClient := &http.Client{Timeout: 5 * time.Second}
				baseURL := strings.TrimRight(cfg.BaseURL, "/")

				// Try the exchange status endpoint (unauthenticated)
				statusResp, statusErr := httpClient.Get(baseURL + "/exchange/status")
				if statusErr != nil {
					report["api"] = fmt.Sprintf("unreachable: %s", statusErr)
				} else {
					statusResp.Body.Close()
					if statusResp.StatusCode < 400 {
						report["api"] = "reachable"
					} else {
						report["api"] = fmt.Sprintf("error: HTTP %d", statusResp.StatusCode)
					}
				}

				// Validate credentials with authenticated request
				if cfg.HasAuth() {
					c, clientErr := flags.newClient()
					if clientErr != nil {
						report["credentials"] = fmt.Sprintf("error: %s", clientErr)
					} else {
						_, getErr := c.Get("/portfolio/balance", nil)
						if getErr != nil {
							report["credentials"] = fmt.Sprintf("invalid: %s", getErr)
						} else {
							report["credentials"] = "valid"
						}
					}
				}
			}

			report["version"] = version

			if flags.asJSON {
				return flags.printJSON(cmd, report)
			}

			// Human-readable output
			w := cmd.OutOrStdout()
			checkKeys := []struct{ key, label string }{
				{"config", "Config"},
				{"auth", "Auth"},
				{"api", "API"},
				{"credentials", "Credentials"},
			}
			for _, ck := range checkKeys {
				v, ok := report[ck.key]
				if !ok {
					continue
				}
				s := fmt.Sprintf("%v", v)
				indicator := green("OK")
				if strings.Contains(s, "error") || strings.Contains(s, "not configured") || strings.Contains(s, "unreachable") || strings.Contains(s, "invalid") {
					indicator = red("FAIL")
				} else if strings.Contains(s, "not ") || strings.Contains(s, "skipped") || strings.Contains(s, "partial") {
					indicator = yellow("WARN")
				}
				fmt.Fprintf(w, "  %s %s: %s\n", indicator, ck.label, s)
			}
			for _, key := range []string{"config_path", "base_url", "auth_source", "api_key", "version"} {
				if v, ok := report[key]; ok {
					fmt.Fprintf(w, "  %s: %v\n", key, v)
				}
			}
			if hint, ok := report["auth_hint"]; ok {
				fmt.Fprintf(w, "  hint: %v\n", hint)
			}
			return nil
		},
	}
}
