package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/config"
)

// newZoomAuthCmd wires up `auth set-token`, `auth status`, `auth refresh`.
// These sit alongside the framework-emitted commands and own the Server-to-
// Server OAuth flow that the spec couldn't describe (Swagger 2.0 apiKey
// scheme can't express the exchange).
func newZoomAuthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Zoom Server-to-Server OAuth credentials",
		Long: "Manage the cached access token used for cloud commands. " +
			"S2S OAuth exchanges ZOOM_S2S_ACCOUNT_ID + ZOOM_S2S_CLIENT_ID + ZOOM_S2S_CLIENT_SECRET " +
			"against https://zoom.us/oauth/token (account_credentials grant) and caches the bearer " +
			"token at ~/.config/zoom-pp-cli/token.json.",
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newAuthSetTokenCmd(flags))
	cmd.AddCommand(newAuthStatusCmd(flags))
	cmd.AddCommand(newAuthRefreshCmd(flags))
	cmd.AddCommand(newAuthLogoutCmd(flags))
	return cmd
}

func newAuthSetTokenCmd(flags *rootFlags) *cobra.Command {
	var (
		accountID    string
		clientID     string
		clientSecret string
	)
	cmd := &cobra.Command{
		Use:   "set-token",
		Short: "Exchange S2S OAuth credentials for a cached bearer token",
		Long: "Reads --account-id / --client-id / --client-secret (or ZOOM_S2S_ACCOUNT_ID / ZOOM_S2S_CLIENT_ID / ZOOM_S2S_CLIENT_SECRET), " +
			"POSTs to https://zoom.us/oauth/token with grant_type=account_credentials, and caches the response token " +
			"at ~/.config/zoom-pp-cli/token.json. Subsequent invocations reuse the cached token until expiry.",
		Example: "  zoom-pp-cli auth set-token --json",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would exchange S2S OAuth credentials and cache token")
				return nil
			}
			if accountID == "" {
				accountID = os.Getenv("ZOOM_S2S_ACCOUNT_ID")
			}
			if clientID == "" {
				clientID = os.Getenv("ZOOM_S2S_CLIENT_ID")
			}
			if clientSecret == "" {
				clientSecret = os.Getenv("ZOOM_S2S_CLIENT_SECRET")
			}
			if accountID == "" || clientID == "" || clientSecret == "" {
				return errors.New("auth: missing credentials — supply --account-id, --client-id, --client-secret " +
					"or export ZOOM_S2S_ACCOUNT_ID / ZOOM_S2S_CLIENT_ID / ZOOM_S2S_CLIENT_SECRET")
			}
			if dryRunOK(flags) {
				_ = flags.printJSON(cmd, map[string]any{
					"would_exchange": true,
					"endpoint":       "https://zoom.us/oauth/token",
					"grant_type":     "account_credentials",
					"account_id":     redact(accountID),
				})
				return nil
			}

			tc, err := exchangeS2SToken(cmd.Context(), accountID, clientID, clientSecret)
			if err != nil {
				return err
			}
			if err := writeTokenCache(tc); err != nil {
				return fmt.Errorf("caching token: %w", err)
			}
			return flags.printJSON(cmd, map[string]any{
				"status":     "cached",
				"path":       config.TokenCachePath(),
				"expires_at": tc.ExpiresAt,
				"scope":      tc.Scope,
				"account_id": redact(tc.AccountID),
			})
		},
	}
	cmd.Flags().StringVar(&accountID, "account-id", "", "Zoom S2S OAuth account ID (or ZOOM_S2S_ACCOUNT_ID)")
	cmd.Flags().StringVar(&clientID, "client-id", "", "Zoom S2S OAuth client ID (or ZOOM_S2S_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Zoom S2S OAuth client secret (or ZOOM_S2S_CLIENT_SECRET)")
	return cmd
}

func newAuthStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether a Zoom S2S access token is currently available",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := map[string]any{
				"env_account_id":    os.Getenv("ZOOM_S2S_ACCOUNT_ID") != "",
				"env_client_id":     os.Getenv("ZOOM_S2S_CLIENT_ID") != "",
				"env_client_secret": os.Getenv("ZOOM_S2S_CLIENT_SECRET") != "",
				"env_access_token":  os.Getenv("ZOOM_S2S_ACCESS_TOKEN") != "",
				"cache_path":        config.TokenCachePath(),
			}
			if tc, err := readTokenCache(); err == nil {
				out["cache_present"] = true
				out["cache_expires_at"] = tc.ExpiresAt
				out["cache_expired"] = !tc.ExpiresAt.IsZero() && time.Until(tc.ExpiresAt) < 60*time.Second
				if tc.Scope != "" {
					out["cache_scope"] = tc.Scope
				}
			} else {
				out["cache_present"] = false
			}
			return flags.printJSON(cmd, out)
		},
	}
}

func newAuthRefreshCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Force a fresh S2S OAuth exchange (ignoring cache)",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would refresh S2S OAuth token")
				return nil
			}
			accountID := os.Getenv("ZOOM_S2S_ACCOUNT_ID")
			clientID := os.Getenv("ZOOM_S2S_CLIENT_ID")
			clientSecret := os.Getenv("ZOOM_S2S_CLIENT_SECRET")
			if accountID == "" || clientID == "" || clientSecret == "" {
				return errors.New("auth: refresh requires ZOOM_S2S_ACCOUNT_ID / ZOOM_S2S_CLIENT_ID / ZOOM_S2S_CLIENT_SECRET")
			}
			if dryRunOK(flags) {
				_ = flags.printJSON(cmd, map[string]any{"would_refresh": true})
				return nil
			}
			tc, err := exchangeS2SToken(cmd.Context(), accountID, clientID, clientSecret)
			if err != nil {
				return err
			}
			if err := writeTokenCache(tc); err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{
				"status":     "refreshed",
				"expires_at": tc.ExpiresAt,
			})
		},
	}
}

func newAuthLogoutCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Delete the cached S2S access token",
		Annotations: map[string]string{
			"mcp:read-only": "false",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.TokenCachePath()
			if dryRunOK(flags) {
				_ = flags.printJSON(cmd, map[string]any{"would_delete": path})
				return nil
			}
			err := os.Remove(path)
			if os.IsNotExist(err) {
				return flags.printJSON(cmd, map[string]any{"status": "no_cache", "path": path})
			}
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, map[string]any{"status": "deleted", "path": path})
		},
	}
}

// exchangeS2SToken POSTs to https://zoom.us/oauth/token with HTTP Basic auth.
// Returns a populated TokenCache.
func exchangeS2SToken(ctx context.Context, accountID, clientID, clientSecret string) (*config.TokenCache, error) {
	form := url.Values{}
	form.Set("grant_type", "account_credentials")
	form.Set("account_id", accountID)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://zoom.us/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(clientID, clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("zoom OAuth token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("zoom OAuth token: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var raw struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token: %s", string(body))
	}
	expiresIn := raw.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	exp := time.Now().Add(time.Duration(expiresIn) * time.Second)
	return &config.TokenCache{
		AccessToken: raw.AccessToken,
		TokenType:   raw.TokenType,
		ExpiresAt:   exp,
		Scope:       raw.Scope,
		AccountID:   accountID,
	}, nil
}

func writeTokenCache(tc *config.TokenCache) error {
	path := config.TokenCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func readTokenCache() (*config.TokenCache, error) {
	path := config.TokenCachePath()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tc config.TokenCache
	if err := json.Unmarshal(b, &tc); err != nil {
		return nil, err
	}
	return &tc, nil
}

func redact(s string) string {
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}
