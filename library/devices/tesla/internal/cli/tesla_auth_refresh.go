// tesla auth login / refresh — OAuth refresh-token capture + bearer mint flow.
// Hand-coded; out-of-tree from generator. The generated auth.go already
// provides setup / status / set-token / logout; this file adds login + refresh.
//
// Token storage routes through tesla_auth_storage.go's facade so the bearer
// lives in the same config.toml the rest of the CLI reads from. The legacy
// authRecord type below stays alive solely for the one-time migration from
// the v0.1 ~/.config/tesla-pp-cli/auth.json file.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

const (
	teslaTokenURL    = "https://auth.tesla.com/oauth2/v3/token"
	teslaClientID    = "ownerapi"
	teslaTokenScopes = "openid email offline_access"
)

func newTeslaAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		paste        bool
		refreshToken string
		via          string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Tesla; default is an in-process PKCE flow via your browser",
		Long: `Default flow: opens Tesla's real login page in your browser, you log in
(MFA happens at Tesla's page; we never see your password or codes), you get
redirected to a 404 on auth.tesla.com, and you paste that URL back here. We
extract the auth code, exchange it via PKCE, and store both tokens in your
config.toml so the rest of the CLI picks them up.

Fallback modes for headless / CI / scripted use:
  --paste                 Read a pre-captured refresh token from stdin
  --refresh-token <tok>   Supply the refresh token as a flag (skips stdin)
  --via tesla-auth        Subprocess into the adriankumpf/tesla_auth binary
                          (must be on $PATH; shows you the same OAuth page in
                          a native window). Useful if you'd rather click than
                          paste.`,
		Example: "  tesla-pp-cli auth login\n  tesla-pp-cli auth login --paste\n  tesla-pp-cli auth login --refresh-token eyJ...\n  tesla-pp-cli auth login --via tesla-auth",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Surface the tesla_auth detection hint when applicable, except in
			// verify/dry-run/JSON modes where we don't want extra stderr noise.
			if via == "" && !flags.asJSON && !cliutil.IsVerifyEnv() && !dryRunOK(flags) {
				maybeHintTeslaAuthAvailable(cmd.OutOrStderr())
			}

			switch {
			case via != "":
				if via != "tesla-auth" {
					return fmt.Errorf("--via %q not supported; the only recognised value is 'tesla-auth'", via)
				}
				return runTeslaAuthSubprocessFlow(cmd, flags)
			case refreshToken != "":
				return runTeslaRefreshTokenFlow(cmd, flags, refreshToken)
			case paste:
				return runTeslaPasteFlow(cmd, flags)
			default:
				return runTeslaPKCEFlow(cmd, flags)
			}
		},
	}
	cmd.Flags().BoolVar(&paste, "paste", false, "Read a pre-captured refresh token from stdin (skips browser)")
	cmd.Flags().StringVar(&refreshToken, "refresh-token", "", "Tesla refresh token (skips stdin and browser)")
	cmd.Flags().StringVar(&via, "via", "", "Use an external helper to perform login (currently: tesla-auth)")
	cmd.MarkFlagsMutuallyExclusive("paste", "refresh-token", "via")
	return cmd
}

// runTeslaPasteFlow reads a refresh token from stdin and exchanges it for a
// bearer. Preserved as a headless / CI fallback for users who can't use the
// browser flow (no browser, no $DISPLAY, scripted runners).
func runTeslaPasteFlow(cmd *cobra.Command, flags *rootFlags) error {
	if cliutil.IsVerifyEnv() {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true, "status": "logged_in", "method": "paste"}, flags)
	}
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "method": "paste"}, flags)
	}
	fmt.Fprintln(cmd.OutOrStderr(), "Paste your Tesla refresh token, then press Ctrl-D (or Enter then Ctrl-D):")
	b, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return err
	}
	tok := strings.TrimSpace(string(b))
	if tok == "" {
		fmt.Fprintln(cmd.OutOrStderr(), "No refresh token provided. Either capture one via 'tesla-pp-cli auth login' (default browser flow) or via tesla_auth.")
		return fmt.Errorf("no refresh token provided")
	}
	return loginWithRefreshToken(cmd, flags, tok, "paste")
}

// runTeslaRefreshTokenFlow exchanges a flag-supplied refresh token. The
// quietest path; no prompts, no browser, fits cleanly into automation.
func runTeslaRefreshTokenFlow(cmd *cobra.Command, flags *rootFlags, refreshToken string) error {
	if cliutil.IsVerifyEnv() {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true, "status": "logged_in", "method": "refresh_token"}, flags)
	}
	if dryRunOK(flags) {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "method": "refresh_token"}, flags)
	}
	tok := strings.TrimSpace(refreshToken)
	if tok == "" {
		return fmt.Errorf("--refresh-token cannot be empty")
	}
	return loginWithRefreshToken(cmd, flags, tok, "refresh_token")
}

// loginWithRefreshToken is the shared tail of paste / refresh-token flows: it
// exchanges the refresh token for an access token, stores both via the U1
// facade, and emits the result envelope. The pkce flow uses runTeslaPKCEFlow
// directly (different exchange path).
func loginWithRefreshToken(cmd *cobra.Command, flags *rootFlags, tok, method string) error {
	cfg, err := config.Load(flagsConfigPath(flags))
	if err != nil {
		return configErr(err)
	}
	access, expiresIn, newRefresh, err := exchangeRefreshToken(tok)
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second).UTC()
	finalRefresh := pickRefresh(tok, newRefresh)
	if err := saveTeslaTokens(cfg, finalRefresh, access, expiresAt); err != nil {
		return err
	}
	return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
		"status":       "logged_in",
		"method":       method,
		"expires_at":   expiresAt.Format(time.RFC3339),
		"expires_in":   expiresIn,
		"storage_path": cfg.Path,
		"hint":         "Run 'tesla auth refresh' to re-mint the bearer at any point.",
	}, flags)
}

func newTeslaAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "refresh",
		Short: "Re-mint the access token using the stored refresh token",
		Long: `Reads the refresh token from config.toml, exchanges it for a fresh 8-hour
access token, and writes both tokens back. Run after a 401 error or whenever
you want to confirm refresh works end-to-end.`,
		Example: "  tesla-pp-cli auth refresh --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"verify_noop": true, "status": "refreshed"}, flags)
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true}, flags)
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			tokens, err := loadTeslaTokens(cfg)
			if err != nil {
				return err
			}
			if tokens.RefreshToken == "" {
				return fmt.Errorf("no refresh token stored; run 'tesla auth login' first")
			}
			access, expiresIn, newRefresh, err := exchangeRefreshToken(tokens.RefreshToken)
			if err != nil {
				return err
			}
			expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second).UTC()
			finalRefresh := pickRefresh(tokens.RefreshToken, newRefresh)
			if err := saveTeslaTokens(cfg, finalRefresh, access, expiresAt); err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":       "refreshed",
				"expires_at":   expiresAt.Format(time.RFC3339),
				"expires_in":   expiresIn,
				"storage_path": cfg.Path,
			}, flags)
		},
	}
	return cmd
}

// authRecord is the v0.1 on-disk schema for ~/.config/tesla-pp-cli/auth.json.
// Kept solely for migrateLegacyAuthJSON's one-time JSON decode; new code writes
// tokens via saveTeslaTokens (cfg.SaveTokens), not this struct.
type authRecord struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
	IssuedAt     string `json:"issued_at"`
}

func exchangeRefreshToken(refresh string) (string, int, string, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("client_id", teslaClientID)
	form.Set("refresh_token", refresh)
	form.Set("scope", teslaTokenScopes)

	req, err := http.NewRequest("POST", teslaTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "tesla-pp-cli/1.0")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, "", fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", 0, "", fmt.Errorf("token exchange http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", 0, "", fmt.Errorf("parse token response: %w", err)
	}
	if out.AccessToken == "" {
		return "", 0, "", fmt.Errorf("token response missing access_token")
	}
	return out.AccessToken, out.ExpiresIn, out.RefreshToken, nil
}

func pickRefresh(old, fresh string) string {
	if strings.TrimSpace(fresh) == "" {
		return old
	}
	return fresh
}
