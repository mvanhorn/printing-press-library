package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/term"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/gravitus/internal/config"
)

// newAuthLoginPasswordCmd adds password-based login to the auth command.
// This handles the Django CSRF dance: GET login page → extract CSRF → POST credentials.
func newAuthLoginPasswordCmd(flags *rootFlags) *cobra.Command {
	var email, password string

	cmd := &cobra.Command{
		Use:   "login-password",
		Short: "Log in with email and password (handles CSRF automatically)",
		Long: `Authenticate with your Gravitus email and password.

Handles the Django CSRF token exchange automatically:
  1. Fetches the login page to get the CSRF token
  2. POSTs your credentials with the token
  3. Stores the session cookie for future requests

Use this instead of --chrome on Windows or when Chrome cookie tools aren't available.`,
		Example: `  gravitus-pp-cli auth login-password --email me@example.com
  gravitus-pp-cli auth login-password --email me@example.com --password mypass`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			// Prompt for missing credentials
			if email == "" {
				email, err = prompt(cmd.InOrStdin(), w, "Gravitus email: ")
				if err != nil {
					return err
				}
			}
			if password == "" {
				password, err = promptSecret(w, "Password: ")
				if err != nil {
					return err
				}
			}

			fmt.Fprintln(w, "Logging in to Gravitus...")

			sessionCookie, userID, err := gravitusDjangoLogin(cfg.BaseURL, email, password)
			if err != nil {
				return authErr(fmt.Errorf("login failed: %w", err))
			}

			// Store as "sessionid=<value>" format so the Cookie header is correct
			cookieStr := "sessionid=" + sessionCookie

			if err := cfg.SaveTokens("", "", cookieStr, "", time.Time{}); err != nil {
				return configErr(fmt.Errorf("saving session: %w", err))
			}

			// Save user ID to config if discovered
			if userID != "" {
				if err := saveUserID(cfg, userID); err != nil {
					fmt.Fprintf(w, "warning: could not save user ID: %v\n", err)
				} else {
					fmt.Fprintf(w, "User ID: %s\n", userID)
				}
			}

			fmt.Fprintf(w, "%s Logged in successfully. Session saved to %s\n", green("OK"), cfg.Path)
			fmt.Fprintf(w, "Run 'gravitus-pp-cli sync' to pull your workout history.\n")
			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Gravitus account email")
	cmd.Flags().StringVar(&password, "password", "", "Gravitus account password (omit to be prompted; passing via flag may appear in shell history)")
	return cmd
}

// gravitusDjangoLogin performs the full Django login flow and returns the session cookie value.
func gravitusDjangoLogin(baseURL, email, password string) (sessionID string, userID string, err error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	loginURL := strings.TrimRight(baseURL, "/") + "/accounts/sign_in/"

	// Step 1: GET login page to obtain CSRF token
	resp, err := client.Get(loginURL)
	if err != nil {
		return "", "", fmt.Errorf("fetching login page: %w", err)
	}
	defer resp.Body.Close()

	csrfToken := extractCSRFToken(resp.Body)
	if csrfToken == "" {
		return "", "", fmt.Errorf("could not find CSRF token on login page")
	}

	// Step 2: POST credentials with CSRF token
	formData := url.Values{
		"email":              {email},
		"password":           {password},
		"csrfmiddlewaretoken": {csrfToken},
	}

	req, err := http.NewRequest("POST", loginURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("creating login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", loginURL)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp2, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("posting login form: %w", err)
	}
	defer resp2.Body.Close()

	// Step 3: Extract session cookie from jar
	u, _ := url.Parse(loginURL)
	for _, cookie := range jar.Cookies(u) {
		if cookie.Name == "sessionid" {
			sessionID = cookie.Value
		}
	}

	if sessionID == "" {
		return "", "", fmt.Errorf("login did not set a session cookie — check your email and password")
	}

	// Step 4: Try to discover user ID from the final redirect URL
	if resp2.Request != nil && resp2.Request.URL != nil {
		finalURL := resp2.Request.URL.String()
		// Profile redirect is typically /users/{id}/ or /profile/
		if idx := strings.Index(finalURL, "/users/"); idx >= 0 {
			rest := finalURL[idx+7:]
			if slash := strings.Index(rest, "/"); slash > 0 {
				userID = rest[:slash]
			}
		}
	}

	return sessionID, userID, nil
}

// extractCSRFToken finds the csrfmiddlewaretoken value in a Django login page.
func extractCSRFToken(r io.Reader) string {
	doc, err := html.Parse(r)
	if err != nil {
		return ""
	}
	var token string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "input" {
			name := ""
			value := ""
			for _, a := range n.Attr {
				if a.Key == "name" {
					name = a.Val
				}
				if a.Key == "value" {
					value = a.Val
				}
			}
			if name == "csrfmiddlewaretoken" && value != "" {
				token = value
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if token != "" {
				return
			}
			walk(c)
		}
	}
	walk(doc)
	return token
}

// saveUserID persists the discovered user ID into the config file.
func saveUserID(cfg *config.Config, userID string) error {
	cfg.GravitusUserID = userID
	return cfg.SaveUserID(userID)
}

func prompt(r io.Reader, w io.Writer, label string) (string, error) {
	fmt.Fprint(w, label)
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text()), nil
	}
	return "", fmt.Errorf("no input")
}

func promptSecret(w io.Writer, label string) (string, error) {
	fmt.Fprint(w, label)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(w) // newline after hidden input
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	return strings.TrimSpace(string(pw)), nil
}
