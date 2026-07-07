// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Withings OAuth2 authorization-code login + refresh. Hand-authored because
// Withings' flow is non-standard (action=requesttoken token endpoint,
// single-use rotating refresh tokens) — see internal/client/withings_token.go.
// Wired into the `auth` command tree from auth.go.

package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/withings/internal/config"
)

// randomState returns a cryptographically random OAuth state token for CSRF
// protection (RFC 6749 §10.12). crypto/rand.Read does not fail in practice; the
// fixed fallback exists only so a catastrophic RNG failure cannot panic the
// login flow.
func randomState() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "withings-pp-cli-fallback-state"
	}
	return hex.EncodeToString(b)
}

const (
	withingsAuthorizeURL = "https://account.withings.com/oauth2_user/authorize2"
	withingsScopes       = "user.info,user.metrics,user.activity,user.sleepevents"
	defaultRedirectURI   = "http://localhost:8765/callback"
)

// resolveClientCreds pulls client id/secret from flags, then env, then config.
func resolveClientCreds(clientID, clientSecret string, cfg *config.Config) (string, string) {
	if clientID == "" {
		clientID = os.Getenv("WITHINGS_CLIENT_ID")
	}
	if clientID == "" && cfg != nil {
		clientID = cfg.ClientID
	}
	if clientSecret == "" {
		clientSecret = os.Getenv("WITHINGS_CLIENT_SECRET")
	}
	if clientSecret == "" && cfg != nil {
		clientSecret = cfg.ClientSecret
	}
	return clientID, clientSecret
}

func buildAuthorizeURL(clientID, redirectURI, state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("scope", withingsScopes)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	return withingsAuthorizeURL + "?" + q.Encode()
}

func openBrowser(rawURL string) error {
	// rawURL is the program-constructed Withings authorize URL (built by
	// buildAuthorizeURL from url.Values), never user-supplied free text. These
	// are the standard per-OS open-browser invocations.
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", rawURL).Start() // #nosec G204 -- program-built authorize URL, standard macOS open
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start() // #nosec G204 -- program-built authorize URL, standard Windows open
	default:
		return exec.Command("xdg-open", rawURL).Start() // #nosec G204 -- program-built authorize URL, standard Linux open
	}
}

func newWithingsLoginCmd(flags *rootFlags) *cobra.Command {
	var clientID, clientSecret, redirectURI, code string
	var launch, printURL bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authorize with Withings via OAuth2 (browser) and store rotating tokens",
		Long: "Authorize with Withings via OAuth2.\n\n" +
			"First register a free app at https://developer.withings.com (Public API integration)\n" +
			"to get a client id and secret. Provide them via --client-id/--client-secret or the\n" +
			"WITHINGS_CLIENT_ID / WITHINGS_CLIENT_SECRET env vars.\n\n" +
			"Default flow: a local callback server captures the code automatically — register\n" +
			"  " + defaultRedirectURI + "\n" +
			"as your app's callback URL. For a custom callback, pass --redirect-uri and either\n" +
			"--print-url (then authorize manually) followed by --code <code> to finish.",
		Example: "  withings-pp-cli auth login --client-id ID --client-secret SECRET\n" +
			"  withings-pp-cli auth login --print-url --redirect-uri https://my.app/cb\n" +
			"  withings-pp-cli auth login --code AUTH_CODE --redirect-uri https://my.app/cb",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			clientID, clientSecret = resolveClientCreds(clientID, clientSecret, cfg)
			if redirectURI == "" {
				redirectURI = defaultRedirectURI
			}

			if clientID == "" || clientSecret == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("client id and secret required: pass --client-id/--client-secret or set WITHINGS_CLIENT_ID / WITHINGS_CLIENT_SECRET (register an app at https://developer.withings.com)"))
			}

			state := randomState()
			authURL := buildAuthorizeURL(clientID, redirectURI, state)

			// Verify-friendly: never open a browser or bind a socket under the
			// verifier or a dry run. Print what would happen and exit 0.
			if cliutil.IsVerifyEnv() || dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would open authorize URL:", authURL)
				return nil
			}

			// --print-url: emit the URL for a manual flow and stop.
			if printURL {
				fmt.Fprintln(cmd.OutOrStdout(), authURL)
				fmt.Fprintln(cmd.ErrOrStderr(), "\nAuthorize in your browser, then finish with:")
				fmt.Fprintf(cmd.ErrOrStderr(), "  withings-pp-cli auth login --client-id %s --client-secret <secret> --redirect-uri %s --code <code>\n", clientID, redirectURI)
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// --code: user already authorized; exchange directly (non-interactive).
			if code != "" {
				if _, err := c.ExchangeAuthCode(cmd.Context(), clientID, clientSecret, code, redirectURI); err != nil {
					return classifyAPIError(err, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Authorized. Tokens saved to %s\n", cfg.Path)
				return nil
			}

			// Interactive: only the default localhost callback can be auto-captured.
			if redirectURI != defaultRedirectURI {
				return usageErr(fmt.Errorf("custom --redirect-uri requires manual flow: run with --print-url, authorize, then re-run with --code <code>"))
			}
			gotCode, err := runCallbackServer(cmd.Context(), redirectURI, authURL, state, launch, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if _, err := c.ExchangeAuthCode(cmd.Context(), clientID, clientSecret, gotCode, redirectURI); err != nil {
				return classifyAPIError(err, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Authorized. Tokens saved to %s\n", cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&clientID, "client-id", "", "Withings app client id (or WITHINGS_CLIENT_ID)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Withings app client secret (or WITHINGS_CLIENT_SECRET)")
	cmd.Flags().StringVar(&redirectURI, "redirect-uri", "", "OAuth redirect URI (default "+defaultRedirectURI+")")
	cmd.Flags().StringVar(&code, "code", "", "Authorization code (for manual flow after --print-url)")
	cmd.Flags().BoolVar(&launch, "launch", false, "Open the authorize URL in your browser")
	cmd.Flags().BoolVar(&printURL, "print-url", false, "Print the authorize URL and exit (manual flow)")
	return cmd
}

// runCallbackServer binds the redirect port, opens/prints the authorize URL,
// and blocks until Withings redirects back with ?code=. Times out after 5 min.
func runCallbackServer(ctx context.Context, redirectURI, authURL, expectedState string, launch bool, errw io.Writer) (string, error) {
	u, err := url.Parse(redirectURI)
	if err != nil {
		return "", usageErr(fmt.Errorf("invalid redirect uri: %w", err))
	}
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		return "", fmt.Errorf("cannot bind %s for OAuth callback: %w (free the port or use --print-url + --code)", u.Host, err)
	}
	defer ln.Close()

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{
		ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != u.Path {
				http.NotFound(w, r)
				return
			}
			if e := r.URL.Query().Get("error"); e != "" {
				// Don't reflect the provider-supplied error string into the HTML
				// response (avoids XSS in the browser tab); the detail goes to the
				// terminal via errCh instead.
				fmt.Fprint(w, "Authorization failed. You can close this tab and return to the terminal.")
				errCh <- fmt.Errorf("authorization denied: %s", e)
				return
			}
			// Verify the OAuth state to prevent CSRF (RFC 6749 §10.12): a forged
			// callback from another page during the listen window would carry a
			// missing or wrong state.
			if st := r.URL.Query().Get("state"); st != expectedState {
				http.Error(w, "state mismatch", http.StatusBadRequest)
				errCh <- fmt.Errorf("OAuth state mismatch (possible CSRF); aborting login")
				return
			}
			gotCode := r.URL.Query().Get("code")
			if gotCode == "" {
				http.Error(w, "missing code", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, "Authorized. You can close this tab and return to the terminal.")
			codeCh <- gotCode
		}),
	}
	go srv.Serve(ln)
	defer srv.Close()

	fmt.Fprintln(errw, "Open this URL to authorize Withings (waiting for callback):")
	fmt.Fprintln(errw, authURL)
	if launch {
		_ = openBrowser(authURL)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	select {
	case gotCode := <-codeCh:
		return gotCode, nil
	case e := <-errCh:
		return "", authErr(e)
	case <-waitCtx.Done():
		return "", fmt.Errorf("timed out waiting for OAuth callback (5 min); try --print-url + --code")
	}
}

func newWithingsRefreshCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "refresh",
		Short:       "Refresh the access token using the stored refresh token",
		Example:     "  withings-pp-cli auth refresh",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() || dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would refresh the Withings access token")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := c.RefreshAccessToken(cmd.Context()); err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"refreshed": true})
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Access token refreshed.")
			return nil
		},
	}
}
