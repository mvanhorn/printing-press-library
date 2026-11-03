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

	"github.com/mvanhorn/printing-press-library/library/other/gravitus/internal/config"
	"github.com/spf13/cobra"
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
	loginURL := strings.TrimRight(baseURL, "/") + "/accounts/sign_in/"
	loginPageURL, err := url.Parse(loginURL)
	if err != nil {
		return "", "", fmt.Errorf("invalid login page URL: %w", err)
	}
	if loginPageURL.Scheme != "https" || loginPageURL.Host == "" || loginPageURL.User != nil {
		return "", "", fmt.Errorf("login page must use HTTPS")
	}
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: 30 * time.Second,
		Jar:     jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if !sameHTTPSOrigin(loginPageURL, req.URL) {
				return fmt.Errorf("login redirect must use the same HTTPS origin")
			}
			return nil
		},
	}

	// Step 1: GET login page to obtain CSRF token
	resp, err := client.Get(loginURL)
	if err != nil {
		return "", "", fmt.Errorf("fetching login page: %w", err)
	}
	defer resp.Body.Close()

	formAction, csrfToken := extractLoginForm(resp.Body)
	if csrfToken == "" {
		return "", "", fmt.Errorf("could not find CSRF token on login page")
	}
	formPageURL := loginPageURL
	if resp.Request != nil && resp.Request.URL != nil {
		formPageURL = resp.Request.URL
	}
	postURL := formPageURL
	if formAction != "" {
		actionURL, parseErr := url.Parse(formAction)
		if parseErr != nil {
			return "", "", fmt.Errorf("invalid login form action: %w", parseErr)
		}
		postURL = formPageURL.ResolveReference(actionURL)
	}
	if !sameHTTPSOrigin(loginPageURL, postURL) || postURL.User != nil {
		return "", "", fmt.Errorf("login form action must use the same HTTPS origin as the login page")
	}

	// Step 2: POST credentials with CSRF token
	formData := url.Values{
		"username":            {email},
		"password":            {password},
		"csrfmiddlewaretoken": {csrfToken},
	}

	req, err := http.NewRequest("POST", postURL.String(), strings.NewReader(formData.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("creating login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", formPageURL.String())
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp2, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("posting login form: %w", err)
	}
	defer resp2.Body.Close()

	// Step 3: Extract session cookie from jar
	for _, cookie := range jar.Cookies(postURL) {
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

func sameHTTPSOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if u.Port() == "" {
			return "443"
		}
		return u.Port()
	}
	return a.Scheme == "https" && b.Scheme == "https" && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}

// extractLoginForm keeps the CSRF token paired with the action of its form.
func extractLoginForm(r io.Reader) (action, token string) {
	doc, err := html.Parse(r)
	if err != nil {
		return "", ""
	}
	var findForm func(*html.Node)
	findForm = func(n *html.Node) {
		if token != "" {
			return
		}
		if n.Type == html.ElementNode && n.Data == "form" {
			var formAction string
			for _, a := range n.Attr {
				if a.Key == "action" {
					formAction = a.Val
				}
			}
			var findToken func(*html.Node)
			findToken = func(child *html.Node) {
				if child.Type == html.ElementNode && child.Data == "input" {
					name, value := "", ""
					for _, a := range child.Attr {
						if a.Key == "name" {
							name = a.Val
						} else if a.Key == "value" {
							value = a.Val
						}
					}
					if name == "csrfmiddlewaretoken" {
						token = value
					}
				}
				for c := child.FirstChild; c != nil && token == ""; c = c.NextSibling {
					findToken(c)
				}
			}
			findToken(n)
			if token != "" {
				action = formAction
				return
			}
		}
		for c := n.FirstChild; c != nil && token == ""; c = c.NextSibling {
			findForm(c)
		}
	}
	findForm(doc)
	return action, token
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
