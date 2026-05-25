package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/jinko/internal/output"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	auth := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication credentials.",
	}
	auth.AddCommand(newAuthLoginCmd(), newAuthLogoutCmd(), newAuthStatusCmd())
	return auth
}

func newAuthLoginCmd() *cobra.Command {
	var key string
	c := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Jinko (paste your API key).",
		Long: `Save a Jinko API key to ~/.jinko/config.yaml so future commands pick it up
without re-authentication. Get a key at https://app.gojinko.com/devplatform.

OAuth device-flow login is supported by @gojinko/cli (the TypeScript CLI) and
will land in a future printing-press release. For now, use --key with a jnk_... token.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := output.Parse(globals.format)
			if err != nil {
				return &InputError{Message: err.Error()}
			}
			token := strings.TrimSpace(key)
			if token == "" {
				return &InputError{Message: "--key is required (run with --key jnk_...)"}
			}
			if !strings.HasPrefix(token, "jnk_") {
				return &InputError{Message: "API key must start with \"jnk_\" — get one at https://app.gojinko.com/devplatform"}
			}
			if err := client.WriteConfigFile(client.ConfigFile{APIKey: token}); err != nil {
				return err
			}
			path, _ := client.ConfigPath()
			return output.Write(map[string]any{
				"status":  "ok",
				"method":  string(client.MethodAPIKey),
				"message": fmt.Sprintf("API key saved to %s", path),
			}, f)
		},
	}
	c.Flags().StringVar(&key, "key", "", "API key (jnk_...)")
	return c
}

func newAuthLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Clear stored credentials.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := output.Parse(globals.format)
			if err != nil {
				return &InputError{Message: err.Error()}
			}
			if err := client.ClearAuthConfig(); err != nil {
				return err
			}
			_ = client.ClearCLISession() // rotate correlation id
			return output.Write(map[string]any{
				"status":  "ok",
				"message": "Credentials cleared.",
			}, f)
		},
	}
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current authentication status.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			f, err := output.Parse(globals.format)
			if err != nil {
				return &InputError{Message: err.Error()}
			}
			auth, err := client.ResolveAuth(globals.apiKey)
			if err != nil {
				var authErr *client.AuthError
				if asErr(err, &authErr) {
					return output.Write(map[string]any{
						"authenticated": false,
						"message":       "No credentials configured. Run `jinko auth login --key jnk_...` or set JINKO_API_KEY.",
					}, f)
				}
				return err
			}
			source := "config"
			if globals.apiKey != "" {
				source = "flag"
			} else if os.Getenv(client.EnvAPIKey) != "" {
				source = "env"
			}
			return output.Write(map[string]any{
				"authenticated": true,
				"method":        string(auth.Method),
				"token":         maskToken(auth.Token),
				"source":        source,
			}, f)
		},
	}
}

func maskToken(s string) string {
	if len(s) <= 12 {
		return "***"
	}
	return s[:8] + "..." + s[len(s)-4:]
}

// asErr is a tiny errors.As wrapper to keep the auth command terse.
func asErr(err error, target any) bool {
	if err == nil {
		return false
	}
	type unwrapper interface{ Unwrap() error }
	for cur := err; cur != nil; {
		switch t := target.(type) {
		case **client.AuthError:
			if e, ok := cur.(*client.AuthError); ok {
				*t = e
				return true
			}
		}
		u, ok := cur.(unwrapper)
		if !ok {
			break
		}
		cur = u.Unwrap()
	}
	return false
}
