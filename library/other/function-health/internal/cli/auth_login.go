// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/other/function-health/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/function-health/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/function-health/internal/firebase"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// _ os reference kept for promptSecret's Stdin use
var _ = os.Stdin

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var email string
	var password string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Exchange your Function Health email and password for a Firebase id-token (and store it locally)",
		Long: `Log in to Function Health via Firebase Authentication. The CLI POSTs your
email + password to Google's Identity Toolkit, receives an idToken (1-hour TTL)
and a refreshToken, and stores both in your config file with mode 0600.

This is the supported way to authenticate. Set FUNCTION_HEALTH_TOKEN in your
environment to override the stored credential for one-shot use (CI, scripts).`,
		Example: "  function-health-pp-cli auth login --email you@example.com\n" +
			"  FH_EMAIL=you@example.com FH_PASSWORD=*** function-health-pp-cli auth login\n",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Resolve from flag/env/keychain/.env first.
			resolved, resolvedPwd, source, _ := resolveCredentials(email, password)
			if resolved != "" {
				email = resolved
			}
			if resolvedPwd != "" {
				password = resolvedPwd
			}
			if source != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "auth: credentials loaded from %s\n", source)
			}
			if email == "" {
				if flags != nil && flags.noInput {
					return usageErr(errors.New("--email required in --no-input mode (or set FH_EMAIL / macOS Keychain / .env)"))
				}
				v, err := promptLine(cmd, "Function Health email: ")
				if err != nil {
					return err
				}
				email = strings.TrimSpace(v)
			}
			if password == "" {
				if flags != nil && flags.noInput {
					return usageErr(errors.New("--password required in --no-input mode (or set FH_PASSWORD / macOS Keychain / .env)"))
				}
				v, err := promptSecret(cmd, "Function Health password: ")
				if err != nil {
					return err
				}
				password = v
			}
			if email == "" || password == "" {
				return usageErr(errors.New("email and password are required"))
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 25*time.Second)
			defer cancel()
			client := firebase.NewClient()
			tokens, err := client.SignInWithPassword(ctx, email, password)
			if err != nil {
				return authErr(fmt.Errorf("firebase signInWithPassword: %w", err))
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.AuthSource = "firebase"
			if err := cfg.SaveTokens("", "", tokens.IDToken, tokens.RefreshToken, tokens.Expiry); err != nil {
				return configErr(fmt.Errorf("save tokens: %w", err))
			}

			expiresIn := time.Until(tokens.Expiry).Round(time.Second)
			out := map[string]any{
				"status":     "ok",
				"email":      tokens.Email,
				"local_id":   tokens.LocalID,
				"expires_at": tokens.Expiry.Format(time.RFC3339),
				"expires_in": expiresIn.String(),
				"config":     cfg.Path,
			}
			if flags != nil && flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Signed in as %s\n  id-token expires in %s (%s)\n  credentials stored: %s\n",
				tokens.Email, expiresIn, tokens.Expiry.Format(time.RFC3339), cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Function Health email (or set FH_EMAIL)")
	cmd.Flags().StringVar(&password, "password", "", "Function Health password. WARNING: a value passed here is visible in `ps`/process listings and your shell history — prefer FH_PASSWORD or the interactive prompt (omit the flag)")
	return cmd
}

func newAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "refresh",
		Short:       "Refresh the stored Firebase id-token using the stored refresh-token",
		Long:        "Refresh exchanges the stored refresh-token for a new id-token without prompting for a password. If the refresh-token is invalid (API_KEY_INVALID, USER_DISABLED, etc.), re-run `auth login`.",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cfg.RefreshToken == "" {
				return authErr(errors.New("no refresh-token stored; run `function-health-pp-cli auth login` first"))
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 20*time.Second)
			defer cancel()
			client := firebase.NewClient()
			tokens, err := client.Refresh(ctx, cfg.RefreshToken)
			if err != nil {
				var apiErr *firebase.APIError
				if errors.As(err, &apiErr) && apiErr.IsRefreshable() {
					return authErr(fmt.Errorf("refresh failed (%s); re-run `function-health-pp-cli auth login`", apiErr.Message))
				}
				return authErr(fmt.Errorf("firebase refresh: %w", err))
			}
			cfg.AuthSource = "firebase"
			if err := cfg.SaveTokens("", "", tokens.IDToken, tokens.RefreshToken, tokens.Expiry); err != nil {
				return configErr(fmt.Errorf("save tokens: %w", err))
			}
			expiresIn := time.Until(tokens.Expiry).Round(time.Second)
			out := map[string]any{
				"status":     "ok",
				"expires_at": tokens.Expiry.Format(time.RFC3339),
				"expires_in": expiresIn.String(),
			}
			if flags != nil && flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Refreshed id-token; expires in %s (%s)\n", expiresIn, tokens.Expiry.Format(time.RFC3339))
			return nil
		},
	}
	return cmd
}

// promptLine reads a single line from stdin. Used for the email prompt.
func promptLine(cmd *cobra.Command, prompt string) (string, error) {
	if cliutil.IsVerifyEnv() {
		return "verify-mode@example.com", nil
	}
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

// promptSecret reads a password from stdin without echoing. Used for the
// password prompt.
func promptSecret(cmd *cobra.Command, prompt string) (string, error) {
	if cliutil.IsVerifyEnv() {
		return "verify-mode-password", nil
	}
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	fd := int(syscall.Stdin)
	if !term.IsTerminal(fd) {
		r := bufio.NewReader(os.Stdin)
		line, err := r.ReadString('\n')
		fmt.Fprintln(cmd.OutOrStdout())
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(line), nil
	}
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(cmd.OutOrStdout())
	if err != nil {
		return "", err
	}
	return string(b), nil
}
