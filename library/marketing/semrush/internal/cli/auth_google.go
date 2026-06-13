// Google OAuth flow for Sheets push. First-run pops a browser to Google's
// consent screen, captures the auth code via a loopback listener on localhost,
// exchanges for tokens, and persists a refresh token at
// ~/.config/semrush-pp-cli/google-token.json. Subsequent runs use the refresh
// token silently.
package cli

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	sheetsScope = "https://www.googleapis.com/auth/spreadsheets"
	// driveScope is required for cloning sheets (drive.files.copy) so the
	// `client onboard` workflow can duplicate the KRAM template. The narrower
	// drive.file scope only sees files the app created, which won't reach a
	// user-owned template.
	driveScope = "https://www.googleapis.com/auth/drive"
)

func newAuthGoogleCmd(_ *rootFlags) *cobra.Command {
	var clientFile string
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Set up Google Sheets OAuth — runs a browser-based consent flow once",
		Long: "Authorize semrush-pp-cli to push to your Google Sheets. The first " +
			"run opens a browser to Google's consent page; subsequent runs use the " +
			"saved refresh token. Scope: spreadsheets (read/write any sheet you own " +
			"or have edit access to).",
		Example: strings.Trim(`
  semrush-pp-cli auth google --client ~/Downloads/semrush-google-oauth-client.json
  semrush-pp-cli auth google                # if you've placed the client at the default path
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if clientFile == "" {
				// Default lookup paths
				home, _ := os.UserHomeDir()
				candidates := []string{
					filepath.Join(home, ".config", "semrush-pp-cli", "google-oauth-client.json"),
					filepath.Join(home, "Downloads", "semrush-google-oauth-client.json"),
				}
				for _, p := range candidates {
					if _, err := os.Stat(p); err == nil {
						clientFile = p
						break
					}
				}
				if clientFile == "" {
					return fmt.Errorf("no OAuth client JSON found — pass --client <path>")
				}
			}

			tokenPath, err := googleTokenPath()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
				return err
			}

			// Persist the client JSON into our config dir so subsequent uses don't
			// depend on the user's Downloads.
			cfgClientPath := filepath.Join(filepath.Dir(tokenPath), "google-oauth-client.json")
			if clientFile != cfgClientPath {
				data, err := os.ReadFile(clientFile)
				if err != nil {
					return err
				}
				if err := os.WriteFile(cfgClientPath, data, 0o600); err != nil {
					return err
				}
			}

			config, err := loadGoogleOAuthConfig(cfgClientPath)
			if err != nil {
				return err
			}
			token, err := runGoogleOAuthLoopback(cmd.Context(), config, cmd)
			if err != nil {
				return err
			}
			data, _ := json.MarshalIndent(token, "", "  ")
			if err := os.WriteFile(tokenPath, data, 0o600); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Google Sheets auth saved → %s\n", tokenPath)
			fmt.Fprintln(cmd.OutOrStdout(), "You can now run 'semrush-pp-cli sheets push <sheet-id>'.")
			return nil
		},
	}
	cmd.Flags().StringVar(&clientFile, "client", "", "Path to the OAuth client JSON downloaded from Google Cloud Console")
	return cmd
}

func loadGoogleOAuthConfig(path string) (*oauth2.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading client JSON: %w", err)
	}
	cfg, err := google.ConfigFromJSON(data, sheetsScope, driveScope)
	if err != nil {
		return nil, fmt.Errorf("parsing client JSON: %w", err)
	}
	return cfg, nil
}

// loadGoogleDriveService returns an authenticated Drive service. Uses the
// same OAuth token as Sheets; the drive scope is requested at auth time.
func loadGoogleDriveService(ctx context.Context) (*drive.Service, error) {
	tokenPath, err := googleTokenPath()
	if err != nil {
		return nil, err
	}
	clientPath := filepath.Join(filepath.Dir(tokenPath), "google-oauth-client.json")
	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Google auth not configured — run 'semrush-pp-cli auth google --client <path-to-oauth-client.json>' first")
		}
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(tokenData, &token); err != nil {
		return nil, err
	}
	config, err := loadGoogleOAuthConfig(clientPath)
	if err != nil {
		return nil, err
	}
	tokenSource := config.TokenSource(ctx, &token)
	fresh, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing Google token: %w (re-run 'semrush-pp-cli auth google' — drive scope may be missing)", err)
	}
	if fresh.AccessToken != token.AccessToken {
		fdata, _ := json.MarshalIndent(fresh, "", "  ")
		_ = os.WriteFile(tokenPath, fdata, 0o600)
	}
	return drive.NewService(ctx, option.WithTokenSource(tokenSource))
}

// runGoogleOAuthLoopback runs the loopback OAuth flow: spins a localhost
// listener, opens the user's default browser to Google's consent screen, and
// captures the redirect.
func runGoogleOAuthLoopback(ctx context.Context, config *oauth2.Config, cmd *cobra.Command) (*oauth2.Token, error) {
	// Bind to an ephemeral port on loopback. Use `localhost` in the redirect
	// URI (not 127.0.0.1) so it matches the typical Desktop OAuth client's
	// registered `http://localhost` URI — Google treats them as distinct hosts
	// for redirect-URI matching even though they resolve to the same address.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	config.RedirectURL = fmt.Sprintf("http://localhost:%d/callback", port)

	state := randomState()
	authURL := config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.ApprovalForce, // ensure refresh_token is returned
	)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("state"); got != state {
			http.Error(w, "state mismatch", http.StatusBadRequest)
			errCh <- fmt.Errorf("OAuth state mismatch (possible CSRF) — got %q", got)
			return
		}
		if errVal := q.Get("error"); errVal != "" {
			http.Error(w, "denied: "+errVal, http.StatusForbidden)
			errCh <- fmt.Errorf("OAuth denied: %s", errVal)
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "no code in callback", http.StatusBadRequest)
			errCh <- fmt.Errorf("OAuth callback had no code")
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html><body style="font-family: -apple-system, sans-serif; padding: 40px; text-align: center;">
<h2>Authorization complete</h2>
<p>You can close this window and return to your terminal.</p>
</body></html>`))
		codeCh <- code
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	fmt.Fprintln(cmd.OutOrStdout(), "Opening your browser for Google sign-in…")
	fmt.Fprintln(cmd.OutOrStdout(), "If the browser does not open, visit this URL manually:")
	fmt.Fprintln(cmd.OutOrStdout(), "  "+authURL)
	_ = openInBrowser(authURL)

	select {
	case code := <-codeCh:
		token, err := config.Exchange(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("exchanging code for token: %w", err)
		}
		return token, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for browser callback (5 minutes)")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// loadGoogleClient returns an authenticated Sheets service. Refreshes the token
// silently if expired.
func loadGoogleSheetsService(ctx context.Context) (*sheets.Service, error) {
	tokenPath, err := googleTokenPath()
	if err != nil {
		return nil, err
	}
	clientPath := filepath.Join(filepath.Dir(tokenPath), "google-oauth-client.json")
	tokenData, err := os.ReadFile(tokenPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Google auth not configured — run 'semrush-pp-cli auth google --client <path-to-oauth-client.json>' first")
		}
		return nil, err
	}
	var token oauth2.Token
	if err := json.Unmarshal(tokenData, &token); err != nil {
		return nil, fmt.Errorf("parsing google-token.json: %w", err)
	}
	config, err := loadGoogleOAuthConfig(clientPath)
	if err != nil {
		return nil, err
	}
	tokenSource := config.TokenSource(ctx, &token)
	// Force a refresh if expired; persist refreshed token back.
	fresh, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing Google token: %w (re-run 'semrush-pp-cli auth google')", err)
	}
	if fresh.AccessToken != token.AccessToken {
		fdata, _ := json.MarshalIndent(fresh, "", "  ")
		_ = os.WriteFile(tokenPath, fdata, 0o600)
	}
	return sheets.NewService(ctx, option.WithTokenSource(tokenSource))
}

func googleTokenPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "semrush-pp-cli", "google-token.json"), nil
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("don't know how to open browser on %s", runtime.GOOS)
	}
	return cmd.Start()
}
