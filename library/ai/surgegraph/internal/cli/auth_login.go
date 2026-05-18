package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/surgegraph/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/surgegraph/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/surgegraph/internal/oauth"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// newAuthLoginCmd wires the OAuth 2.1 (Authorization Code + PKCE +
// Dynamic Client Registration) browser flow against the configured base
// URL. The flow is the only supported credential acquisition path for
// real SurgeGraph users — auth set-token remains for paste-based
// scripted setups.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var noBrowser bool
	var timeout time.Duration
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Browser-based OAuth login (PKCE + Dynamic Client Registration)",
		Long: strings.TrimSpace(`
Browser-based OAuth login against the configured SurgeGraph base URL.

Performs RFC 7591 Dynamic Client Registration, then runs an Authorization
Code + PKCE flow with a one-shot local callback receiver. The access
token and refresh token are persisted alongside the registered client
credentials in the config file.

By default, opens the OS browser to the consent page. Use --no-browser
to print the URL and copy/paste yourself.
`),
		Example: strings.Trim(`
  surgegraph-pp-cli auth login
  surgegraph-pp-cli auth login --no-browser
  surgegraph-pp-cli auth login --timeout 10m
`, "\n"),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would open browser for OAuth login (skipped under PRINTING_PRESS_VERIFY)")
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			opts := oauth.LoginOptions{
				BaseURL:    cfg.BaseURL,
				ClientName: "surgegraph-pp-cli",
				Stdout:     cmd.OutOrStdout(),
				Timeout:    timeout,
			}
			if !noBrowser {
				opts.Browser = oauth.OpenBrowser
			}
			result, err := oauth.Login(ctx, opts)
			if err != nil {
				return fmt.Errorf("oauth login: %w", err)
			}
			// Persist tokens into the same TOML file the rest of the CLI reads.
			cfg.ClientID = result.Client.ClientID
			cfg.ClientSecret = result.Client.ClientSecret
			cfg.AccessToken = result.Tokens.AccessToken
			cfg.RefreshToken = result.Tokens.RefreshToken
			if !result.Tokens.ExpiresAt.IsZero() {
				cfg.TokenExpiry = result.Tokens.ExpiresAt
			}
			// PATCH(greptile-3): do NOT also write the OAuth access token into
			// SurgegraphToken — that field is the static-token (env / set-token)
			// slot. Duplicating here meant a stale copy would win over AccessToken
			// after any future refresh, since AuthHeader() prefers SurgegraphToken.
			// PATCH(greptile-6): a prior `auth set-token` may have left SurgegraphToken
			// populated in the config file. config.Load() preserves it, and AuthHeader()
			// prefers it over AccessToken — so without clearing here, the static token
			// would win and the OAuth flow would be a no-op from the CLI's perspective.
			cfg.SurgegraphToken = ""
			cfg.AuthSource = "config:access_token"
			if err := saveConfigTOML(cfg); err != nil {
				return fmt.Errorf("saving credentials: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Authenticated. Token saved to %s\n", cfg.Path)
			if !result.Tokens.ExpiresAt.IsZero() {
				fmt.Fprintf(cmd.OutOrStdout(), "Access token expires at %s (in %s)\n",
					result.Tokens.ExpiresAt.Format(time.RFC3339),
					time.Until(result.Tokens.ExpiresAt).Round(time.Second))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the authorization URL instead of launching a browser")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Maximum time to wait for the browser callback")
	return cmd
}

// saveConfigTOML serializes Config back to its TOML file, creating the
// parent directory as needed. The generated config package only exposes
// Load(); writing it back is novel-feature work.
func saveConfigTOML(cfg *config.Config) error {
	if cfg.Path == "" {
		home, _ := os.UserHomeDir()
		cfg.Path = filepath.Join(home, ".config", "surgegraph-pp-cli", "config.toml")
	}
	// PATCH(greptile-2): use 0o700 so a freshly-created credentials directory does not leak the existence of config.toml to other local users.
	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o700); err != nil {
		return err
	}
	buf, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfg.Path, buf, 0o600); err != nil {
		return err
	}
	return nil
}
