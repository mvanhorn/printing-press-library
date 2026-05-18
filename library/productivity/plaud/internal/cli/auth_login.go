// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

// Plaud-specific auth login command. Implements the email+password →
// JWT exchange documented in sergivalverde/plaud-toolkit's source:
//
//   POST /auth/access-token
//   Content-Type: application/x-www-form-urlencoded
//   Body: username=<email>&password=<password>
//
// Response: { status, msg?, access_token, token_type }. status:0 success.
// Tokens last ~300 days (decoded from JWT exp). The generator's auth.go
// emitted setup/set-token/logout/status. This file adds the login subcommand
// without colliding with that generated file.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mvanhorn/printing-press-library/library/productivity/plaud/internal/config"
)

// init wires the login subcommand into the auth command tree. Cobra runs
// init() for every package on import; root.go's newAuthCmd is called once
// per Execute, so we attach to the AuthCmd construction by re-opening it
// here via an init hook. Cleaner approach: just register a top-level hook
// that root.go calls.
//
// Simpler: expose a function the root.go init can call. Done via a
// package-level slice of "extra auth subcommands" registered in init().
// This avoids editing the generated auth.go (which would conflict on regen).

func init() {
	registerAuthExtra("login", newAuthLoginCmd)
}

// authExtraRegistry holds factories for additional auth subcommands. The
// auth.go's newAuthCmd is generated and we don't want to modify it, so the
// `auth` command pulls in extras from this registry at construction time.
//
// Implementation hook: see auth_extras_wiring.go for the modification to
// newAuthCmd's AddCommand list. (For v1 we patch root.go's AddCommand
// instead and expose loginCmd at the top level — see registerAuthExtra
// below.)
var authExtraRegistry = map[string]func(*rootFlags) *cobra.Command{}

func registerAuthExtra(name string, factory func(*rootFlags) *cobra.Command) {
	authExtraRegistry[name] = factory
}

// AuthLoginCmd is exported so root.go can attach it directly to the auth
// subcommand. Used by the wiring helper below.
func AuthLoginCmd(flags *rootFlags) *cobra.Command {
	return newAuthLoginCmd(flags)
}

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var flagEmail, flagPassword, flagRegion string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with Plaud email + password and persist the JWT",
		Long: "Performs the email/password exchange against Plaud's /auth/access-token\n" +
			"endpoint and persists the resulting JWT to the config file. Tokens are\n" +
			"long-lived (~300 days); the CLI re-runs the login on its own when the\n" +
			"token is within 30 days of expiry.\n\n" +
			"Credentials can also come from environment variables: PLAUD_EMAIL,\n" +
			"PLAUD_PASSWORD. The CLI prompts interactively for missing values when\n" +
			"a TTY is available.",
		Example: `  plaud-pp-cli auth login --email me@example.com
  PLAUD_EMAIL=me@example.com PLAUD_PASSWORD=... plaud-pp-cli auth login
  plaud-pp-cli auth login --region eu`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			email := strings.TrimSpace(flagEmail)
			if email == "" {
				email = strings.TrimSpace(os.Getenv("PLAUD_EMAIL"))
			}
			if email == "" {
				email = strings.TrimSpace(cfg.Email)
			}
			password := flagPassword
			if password == "" {
				password = os.Getenv("PLAUD_PASSWORD")
			}
			region := strings.TrimSpace(flagRegion)
			if region == "" {
				region = strings.TrimSpace(cfg.Region)
			}

			if email == "" {
				if flags.noInput {
					return usageErr(fmt.Errorf("email required (--email, PLAUD_EMAIL, or interactive prompt)"))
				}
				fmt.Fprint(cmd.OutOrStdout(), "Plaud email: ")
				reader := os.Stdin
				var line string
				_, err := fmt.Fscanln(reader, &line)
				if err != nil {
					return usageErr(fmt.Errorf("reading email: %w", err))
				}
				email = strings.TrimSpace(line)
			}
			if password == "" {
				if flags.noInput {
					return usageErr(fmt.Errorf("password required (--password, PLAUD_PASSWORD, or interactive prompt)"))
				}
				fmt.Fprint(cmd.OutOrStdout(), "Plaud password: ")
				pwBytes, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Fprintln(cmd.OutOrStdout())
				if err != nil {
					return usageErr(fmt.Errorf("reading password: %w", err))
				}
				password = string(pwBytes)
			}

			baseURL := config.BaseURLForRegion(region)
			ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
			defer cancel()

			token, expiresAt, finalRegion, err := plaudLogin(ctx, baseURL, region, email, password)
			if err != nil {
				return authErr(err)
			}

			cfg.PlaudToken = token
			cfg.AccessToken = token
			cfg.TokenExpiry = expiresAt
			cfg.Email = email
			cfg.Region = finalRegion
			cfg.BaseURL = config.BaseURLForRegion(finalRegion)
			cfg.AuthSource = "plaud-login"

			if err := writeConfig(cfg); err != nil {
				return configErr(fmt.Errorf("persisting config: %w", err))
			}

			daysLeft := int(time.Until(expiresAt).Hours() / 24)
			fmt.Fprintln(cmd.OutOrStdout(), green("Login successful"))
			fmt.Fprintf(cmd.OutOrStdout(), "  Region: %s\n  Token valid for ~%d days\n  Config: %s\n", finalRegion, daysLeft, cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagEmail, "email", "", "Plaud account email (or set PLAUD_EMAIL)")
	cmd.Flags().StringVar(&flagPassword, "password", "", "Plaud password (or set PLAUD_PASSWORD; interactive prompt otherwise)")
	cmd.Flags().StringVar(&flagRegion, "region", "", "Plaud region: us, eu, or ap (default us; auto-routes on -302)")
	return cmd
}

// plaudLogin POSTs the form-urlencoded credentials and returns the JWT.
// Handles the -302 region-redirect-and-retry pattern.
func plaudLogin(ctx context.Context, baseURL, region, email, password string) (token string, expiresAt time.Time, finalRegion string, err error) {
	if region == "" {
		region = "us"
	}
	finalRegion = region

	form := url.Values{}
	form.Set("username", email)
	form.Set("password", password)

	for attempt := 0; attempt < 2; attempt++ {
		req, rerr := http.NewRequestWithContext(ctx, "POST", baseURL+"/auth/access-token", strings.NewReader(form.Encode()))
		if rerr != nil {
			return "", time.Time{}, finalRegion, fmt.Errorf("building login request: %w", rerr)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "https://app.plaud.ai")
		req.Header.Set("Referer", "https://app.plaud.ai/")
		req.Header.Set("app-platform", "web")
		req.Header.Set("app-language", "en")
		req.Header.Set("edit-from", "web")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

		resp, rerr := http.DefaultClient.Do(req)
		if rerr != nil {
			return "", time.Time{}, finalRegion, fmt.Errorf("login request: %w", rerr)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var envelope struct {
			Status      int    `json:"status"`
			Msg         string `json:"msg"`
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			Data        struct {
				Domains struct {
					API string `json:"api"`
				} `json:"domains"`
			} `json:"data"`
		}
		if jerr := json.Unmarshal(body, &envelope); jerr != nil {
			return "", time.Time{}, finalRegion, fmt.Errorf("parsing login response (HTTP %d): %w", resp.StatusCode, jerr)
		}

		// Region redirect: switch and retry once.
		if envelope.Status == -302 && envelope.Data.Domains.API != "" && attempt == 0 {
			baseURL = envelope.Data.Domains.API
			if strings.Contains(baseURL, "euc1") {
				finalRegion = "eu"
			} else if strings.Contains(baseURL, "apse1") {
				finalRegion = "ap"
			} else {
				finalRegion = "us"
			}
			continue
		}

		if envelope.Status != 0 {
			msg := envelope.Msg
			if msg == "" {
				msg = fmt.Sprintf("status %d", envelope.Status)
			}
			return "", time.Time{}, finalRegion, fmt.Errorf("login failed: %s", msg)
		}
		if envelope.AccessToken == "" {
			return "", time.Time{}, finalRegion, fmt.Errorf("login response missing access_token (HTTP %d)", resp.StatusCode)
		}

		expiresAt = decodeJWTExpiry(envelope.AccessToken)
		return envelope.AccessToken, expiresAt, finalRegion, nil
	}
	return "", time.Time{}, finalRegion, fmt.Errorf("login retry budget exhausted")
}

// decodeJWTExpiry pulls the `exp` claim out of a JWT without verifying the
// signature. Used only to estimate token lifetime for re-login scheduling.
func decodeJWTExpiry(jwt string) time.Time {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return time.Time{}
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Try with padding stripped/added — JWT libs differ.
		payload, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return time.Time{}
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}
	}
	if claims.Exp == 0 {
		return time.Time{}
	}
	return time.Unix(claims.Exp, 0)
}

// writeConfig persists the config to disk at mode 0600. Mirrors the shape
// used by the generator's auth set-token command. Config is written as JSON
// (the file extension may be .yaml, but the generator's Load() uses
// json.Unmarshal — keep symmetric).
func writeConfig(cfg *config.Config) error {
	if cfg.Path == "" {
		return fmt.Errorf("config path empty")
	}
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(cfg.Path, out, 0o600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}
