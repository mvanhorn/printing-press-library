// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// PATCH: v0.1 `auth login --chrome --service <name>` + `auth services`.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
)

// serviceCookieDomain maps user-facing service slugs to the cookie host the
// chromecookies helper must read.
var serviceCookieDomain = map[string]string{
	"huberman":   "hubermanlab.com",
	"acquired":   "acquired.fm",
	"founders":   "founderspodcast.com",
	"peterattia": "peterattiamd.com",
	"spotify":    "spotify.com",
}

func knownServices() []string {
	out := []string{}
	for k := range serviceCookieDomain {
		out = append(out, k)
	}
	return out
}

func newAuthLoginChromeServiceCmd(flags *rootFlags) *cobra.Command {
	var (
		flagService string
		flagProfile string
	)
	cmd := &cobra.Command{
		Use:   "login-service",
		Short: "Capture Chrome cookies for one publisher service (huberman|acquired|founders|peterattia|spotify)",
		Long: `Reads the cookies for the named service's domain out of your local Chrome
profile and writes them to ~/.config/podcast-goat/cookies/cookies-<service>.json.

This is the cookie source for the cookie-tier dispatcher.
Equivalent shorthand: ` + "`auth login --chrome --service <name>`" + ` (use this command).`,
		Example: `  podcast-goat-pp-cli auth login-service --service huberman
  podcast-goat-pp-cli auth login-service --service acquired --profile "Work"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagService == "" {
				return fmt.Errorf("--service is required (one of: %s)", strings.Join(knownServices(), ", "))
			}
			domain, ok := serviceCookieDomain[flagService]
			if !ok {
				return fmt.Errorf("unknown service %q (known: %s)", flagService, strings.Join(knownServices(), ", "))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(),
					"would read Chrome cookies for %s -> %s (verify mode short-circuit)\n",
					flagService, source.CookieFile(flagService),
				)
				return nil
			}
			tool, err := detectCookieTool()
			if err != nil {
				return authErr(fmt.Errorf("no cookie extraction tool found: %w", err))
			}
			profileDir, err := resolveChromeProfile(cmd.OutOrStdout(), os.Stdin, domain, flagProfile)
			if err != nil {
				return authErr(err)
			}
			cookieHeader, err := extractCookies(tool, domain, profileDir)
			if err != nil {
				return authErr(err)
			}
			cookies := parseCookieString(cookieHeader)
			if len(cookies) == 0 {
				return authErr(fmt.Errorf("no cookies extracted for %s (try logging in at %s and re-running)", domain, "https://"+domain))
			}

			recs := []source.CookieRecord{}
			for name, value := range cookies {
				recs = append(recs, source.CookieRecord{
					Name:   name,
					Value:  value,
					Domain: "." + domain,
					Path:   "/",
				})
			}
			raw, err := json.MarshalIndent(map[string]any{
				"service":     flagService,
				"domain":      domain,
				"captured_at": time.Now().UTC().Format(time.RFC3339),
				"cookies":     recs,
			}, "", "  ")
			if err != nil {
				return err
			}
			outPath := source.CookieFile(flagService)
			if err := os.MkdirAll(filepath.Dir(outPath), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(outPath, raw, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "captured %d cookies for service=%s -> %s\n", len(recs), flagService, outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagService, "service", "", "Service slug (huberman|acquired|founders|peterattia|spotify)")
	cmd.Flags().StringVar(&flagProfile, "profile", "", "Chrome profile name (auto-detected when not set)")
	return cmd
}

// newAuthServicesCmd shows one row per service with cookie age + last-fetch.
func newAuthServicesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "services",
		Short:       "Per-service cookie health (one row per publisher)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			type row struct {
				Service     string `json:"service"`
				Domain      string `json:"domain"`
				HasCookie   bool   `json:"has_cookie"`
				CapturedAt  string `json:"captured_at,omitempty"`
				CookieCount int    `json:"cookie_count"`
				LastFetch   string `json:"last_fetch,omitempty"`
				Remediation string `json:"remediation,omitempty"`
			}
			var rows []row
			for svc, domain := range serviceCookieDomain {
				r := row{Service: svc, Domain: domain, HasCookie: source.HasCookie(svc)}
				if !r.HasCookie {
					r.Remediation = fmt.Sprintf("podcast-goat-pp-cli auth login-service --service %s", svc)
					rows = append(rows, r)
					continue
				}
				raw, err := os.ReadFile(source.CookieFile(svc))
				if err == nil {
					var env struct {
						CapturedAt string                 `json:"captured_at"`
						Cookies    []source.CookieRecord  `json:"cookies"`
					}
					_ = json.Unmarshal(raw, &env)
					r.CapturedAt = env.CapturedAt
					r.CookieCount = len(env.Cookies)
				}
				// HEAD against the public host as a liveness ping.
				if !cliutil.IsVerifyEnv() {
					client := &http.Client{Timeout: 5 * time.Second}
					req, _ := http.NewRequestWithContext(cmd.Context(), "HEAD", "https://"+domain, nil)
					if resp, err := client.Do(req); err == nil {
						r.LastFetch = fmt.Sprintf("HEAD %s -> %d", domain, resp.StatusCode)
						_ = resp.Body.Close()
					} else {
						r.LastFetch = "unreachable"
					}
				}
				rows = append(rows, r)
			}
			if flags.asJSON {
				out, _ := json.MarshalIndent(rows, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}
			headers := []string{"service", "domain", "cookies", "captured", "ping", "remediation"}
			var data [][]string
			for _, r := range rows {
				cookies := "0"
				if r.HasCookie {
					cookies = fmt.Sprintf("%d", r.CookieCount)
				}
				data = append(data, []string{
					r.Service, r.Domain, cookies, r.CapturedAt, r.LastFetch, r.Remediation,
				})
			}
			return flags.printTable(cmd, headers, data)
		},
	}
	return cmd
}
