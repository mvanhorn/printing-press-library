// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.
// PATCH(retro #marianatek-browser-auth): adds `auth from-browser` to extract the
// bearer token from the Mariana Tek iframe cookie value the user pastes from
// DevTools. The Customer API's only consumer-facing OAuth flow is via the
// hosted iframe widget, so users without a registered OAuth app can only
// obtain a token by signing in to their tenant's site and lifting it out of
// the browser session.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/marianatek/internal/config"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

// mtCookieBlob is the shape of the JSON stored in
// `mt.token.https://{tenant}.marianaiframes.com` cookies set on the studio's
// parent domain (kolmkontrast.com, barrysbootcamp.com, etc.).
type mtCookieBlob struct {
	Expires   any           `json:"expires"`
	TokenData mtCookieToken `json:"tokenData"`
}

type mtCookieToken struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expiresIn"`
}

func newAuthFromBrowserCmd(flags *rootFlags) *cobra.Command {
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "from-browser [cookie-json]",
		Short: "Extract the bearer token from a Mariana Tek iframe cookie value",
		Long: `Extract the OAuth bearer token from the Mariana Tek iframe cookie blob you
copy out of your browser's DevTools and save it to the config file.

How to obtain the cookie value:
  1. Open your studio's site (e.g. https://kolmkontrast.com) in Chrome and
     log in to your account if you aren't already.
  2. Open DevTools (Cmd-Option-I) -> Application -> Storage -> Cookies ->
     pick the studio domain.
  3. Find the cookie named ` + "`" + `mt.token.https://<tenant>.marianaiframes.com` + "`" + `
     and copy its Value (a JSON blob like
     ` + "`" + `{"expires":..., "tokenData":{"accessToken":"...","refreshToken":"...","tokenType":"Bearer","scope":"...","expiresIn":3600}}` + "`" + `).
  4. Pass that JSON to this command as a single argument, or pipe it on stdin
     with --stdin.

This command writes the access token to your config file under
oauth_authorization, then the auto-Bearer-prefix path in config.AuthHeader()
sends it on every request.`,
		Example: `  marianatek-pp-cli auth from-browser '{"expires":..., "tokenData":{"accessToken":"...","refreshToken":"...","tokenType":"Bearer","scope":"...","expiresIn":3600}}'
  pbpaste | marianatek-pp-cli auth from-browser --stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var raw string
			switch {
			case fromStdin:
				b, err := readAllStdin()
				if err != nil {
					return err
				}
				raw = strings.TrimSpace(string(b))
			case len(args) == 1:
				raw = strings.TrimSpace(args[0])
			default:
				return usageErr(fmt.Errorf("provide the cookie JSON as a single argument, or use --stdin"))
			}
			if raw == "" {
				return usageErr(fmt.Errorf("empty input"))
			}
			// Strip surrounding quotes if the user wrapped the value
			raw = strings.Trim(raw, "'\"")

			var blob mtCookieBlob
			if err := json.Unmarshal([]byte(raw), &blob); err != nil {
				return fmt.Errorf("cookie value is not valid JSON: %w (paste only the cookie Value, not the entire cURL)", err)
			}
			token := blob.TokenData.AccessToken
			if token == "" {
				return fmt.Errorf("cookie JSON parsed but `tokenData.accessToken` is empty; are you sure you copied the right cookie? Expected `mt.token.https://<tenant>.marianaiframes.com`")
			}

			if err := persistOAuthToken(flags, token); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Saved access token (length %d, type %s). Run `marianatek-pp-cli doctor` to verify.\n", len(token), blob.TokenData.TokenType)
			return nil
		},
	}
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read the cookie JSON from stdin instead of an argument")
	return cmd
}

func readAllStdin() ([]byte, error) {
	st, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if (st.Mode() & os.ModeCharDevice) != 0 {
		return nil, fmt.Errorf("stdin is a TTY; pipe the cookie JSON in or drop --stdin")
	}
	return readAll(os.Stdin)
}

func readAll(f *os.File) ([]byte, error) {
	const chunk = 4096
	var out []byte
	buf := make([]byte, chunk)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			out = append(out, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return out, nil
			}
			return out, err
		}
	}
}

// persistOAuthToken writes the access token into the CLI's config file under
// oauth_authorization. AuthHeader() auto-prefixes "Bearer " at read time.
//
// PATCH(retro #marianatek-multi-tenant): when flags.tenant or
// MARIANATEK_TENANT names a tenant, the token lands in the per-tenant config
// at ~/.config/marianatek-pp-cli/tenants/<slug>.toml instead of the root file.
// This lets one user manage multiple studios without overwriting credentials.
func persistOAuthToken(flags *rootFlags, token string) error {
	tenant := flags.tenant
	if tenant == "" {
		tenant = os.Getenv("MARIANATEK_TENANT")
	}

	var configPath string
	if tenant != "" {
		var err error
		configPath, err = config.TenantConfigPath(tenant)
		if err != nil {
			return err
		}
	} else {
		configPath = flags.configPath
		if configPath == "" {
			configPath = os.Getenv("MARIANATEK_CONFIG")
		}
		if configPath == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			configPath = filepath.Join(home, ".config", "marianatek-pp-cli", "config.toml")
		}
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return err
	}

	cfg := map[string]any{}
	if data, err := os.ReadFile(configPath); err == nil {
		_ = toml.Unmarshal(data, &cfg)
	}
	if _, ok := cfg["base_url"]; !ok {
		if tenant != "" {
			cfg["base_url"] = fmt.Sprintf("https://%s.marianatek.com", tenant)
		} else {
			cfg["base_url"] = inferBaseURL()
		}
	}
	if _, ok := cfg["base_path"]; !ok {
		cfg["base_path"] = "/api/customer/v1"
	}
	cfg["oauth_authorization"] = token
	// Clear any legacy auth_header so the auto-Bearer path is the one in effect.
	delete(cfg, "auth_header")

	out, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}
	if err := os.WriteFile(configPath, out, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", configPath, err)
	}
	return nil
}

// inferBaseURL returns a useful default for the CLI's base URL. Users still
// edit config.toml or pass --config to point at a specific tenant; this is
// just the seed value when the file is being created from scratch.
func inferBaseURL() string {
	if v := os.Getenv("MARIANATEK_BASE_URL"); v != "" {
		return v
	}
	return "https://<tenant>.marianatek.com"
}
