package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/producthunt/internal/phgql"

	"github.com/spf13/cobra"
)

// newAuthOnboardCmd builds the interactive `auth onboard` wizard. Default
// path teaches the user to mint a developer token (the simplest flow);
// --oauth switches to the client_credentials path for CI/automation.
func newAuthOnboardCmd(flags *rootFlags) *cobra.Command {
	var (
		oauth          bool
		token          string
		clientID       string
		clientSecret   string
		nonInteractive bool
	)
	cmd := &cobra.Command{
		Use:   "onboard",
		Short: "Interactive walkthrough that creates a free Product Hunt OAuth app and stores your developer token",
		Long: strings.Trim(`
Walks you through getting Product Hunt API access:

  1. Opens https://www.producthunt.com/v2/oauth/applications in your browser
  2. Tells you to create an OAuth application — the redirect URL field is
     required by the form but unused for the developer-token flow, so set it
     to `+"`https://localhost/callback`"+` (PH never redirects to it).
  3. Tells you to scroll to the bottom of your app page and click `+"`Create Token`"+`
     to generate a developer token (never expires).
  4. Accepts the pasted token, validates it via a `+"`viewer`"+` GraphQL ping, and
     saves it to your config (~/.config/producthunt-pp-cli/config.toml).

`+"`--oauth`"+` switches to the alternate flow: paste the api_key and api_secret
from the same OAuth app page; the CLI exchanges them for an access token via
the `+"`client_credentials`"+` grant and refreshes on 401. Use this for CI where
you'd rather not share a personal developer token.

Use `+"`--token <value>`"+` to skip the prompt (handy for scripting, but the
value will land in shell history — set `+"`PRODUCT_HUNT_TOKEN`"+` directly when
possible).
`, "\n"),
		Example: strings.Trim(`
  producthunt-pp-cli auth onboard
  producthunt-pp-cli auth onboard --token <your-developer-token>
  producthunt-pp-cli auth onboard --oauth --client-id <id> --client-secret <secret>
`, "\n"),
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would launch onboarding wizard for Product Hunt")
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			w := cmd.OutOrStdout()

			if oauth {
				return runOauthOnboard(cmd.Context(), cfg, w, clientID, clientSecret, nonInteractive)
			}

			fmt.Fprintln(w, "Setting up Product Hunt API access (developer-token flow).")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "1. Open https://www.producthunt.com/v2/oauth/applications in your browser.")
			fmt.Fprintln(w, "   - Sign in with your Product Hunt account.")
			fmt.Fprintln(w, "   - Click `Add an Application`.")
			fmt.Fprintln(w, "   - Pick any name. The Redirect URL field is required by the form but is")
			fmt.Fprintln(w, "     NOT used for the developer-token flow — set it to:")
			fmt.Fprintln(w, "        https://localhost/callback")
			fmt.Fprintln(w, "")
			fmt.Fprintln(w, "2. Open the application page, scroll to the bottom, and click `Create Token`.")
			fmt.Fprintln(w, "   The developer token never expires.")
			fmt.Fprintln(w, "")

			if !nonInteractive {
				_ = openInBrowser("https://www.producthunt.com/v2/oauth/applications")
			}

			if token == "" {
				if nonInteractive {
					return fmt.Errorf("auth onboard: --token <value> is required when --no-input is set")
				}
				fmt.Fprintln(w, "3. Paste your developer token here and press Enter:")
				fmt.Fprint(w, "   token> ")
				reader := bufio.NewReader(os.Stdin)
				line, _ := reader.ReadString('\n')
				token = strings.TrimSpace(line)
			}
			if token == "" {
				return fmt.Errorf("auth onboard: no token provided")
			}

			cfg.AuthHeaderVal = ""
			cfg.AccessToken = token
			cfg.ClientID = ""
			cfg.ClientSecret = ""
			if err := cfg.SaveTokens("", "", token, "", cfg.TokenExpiry); err != nil {
				return configErr(fmt.Errorf("saving token: %w", err))
			}

			// Validate
			c := phgql.New(cfg)
			var resp phgql.ViewerResponse
			if _, err := c.Query(cmd.Context(), phgql.ViewerQuery, nil, &resp); err != nil {
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "Token saved, but the validation `viewer` query failed:")
				fmt.Fprintln(w, "  ", err.Error())
				fmt.Fprintln(w, "Re-check the token at https://www.producthunt.com/v2/oauth/applications and run `auth set-token <new>`.")
				return err
			}
			if resp.Viewer.User != nil {
				fmt.Fprintln(w, "")
				fmt.Fprintf(w, "Authenticated as @%s (%s).\n", resp.Viewer.User.Username, resp.Viewer.User.Name)
				fmt.Fprintln(w, "Run `producthunt-pp-cli whoami` to see your full profile + remaining complexity-points budget.")
			} else {
				fmt.Fprintln(w, "Token saved. `viewer` returned null — unusual for the developer-token flow.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&oauth, "oauth", false, "Use OAuth client_credentials flow instead of a developer token")
	cmd.Flags().StringVar(&token, "token", "", "Skip the interactive prompt and save this token directly")
	cmd.Flags().StringVar(&clientID, "client-id", "", "OAuth client_id (with --oauth)")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "OAuth client_secret (with --oauth)")
	cmd.Flags().BoolVar(&nonInteractive, "no-input", false, "Do not prompt; require values via flags")
	return cmd
}

func runOauthOnboard(ctx context.Context, cfg *config.Config, w interface{ Write(p []byte) (int, error) }, clientID, clientSecret string, nonInteractive bool) error {
	wfln := func(s string) { fmt.Fprintln(w, s) }
	wfln("Setting up Product Hunt API access (OAuth client_credentials flow).")
	wfln("")
	wfln("1. Open https://www.producthunt.com/v2/oauth/applications and create an")
	wfln("   application if you have not already (Redirect URL: https://localhost/callback).")
	wfln("2. Copy the API Key and API Secret from your application page.")
	wfln("")
	if !nonInteractive {
		_ = openInBrowser("https://www.producthunt.com/v2/oauth/applications")
	}
	if clientID == "" || clientSecret == "" {
		if nonInteractive {
			return fmt.Errorf("auth onboard --oauth: --client-id and --client-secret are required when --no-input is set")
		}
		fmt.Fprint(w, "   client_id> ")
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		clientID = strings.TrimSpace(line)
		fmt.Fprint(w, "   client_secret> ")
		line, _ = reader.ReadString('\n')
		clientSecret = strings.TrimSpace(line)
	}
	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("auth onboard --oauth: client_id and client_secret are required")
	}
	cfg.AuthHeaderVal = ""
	if err := cfg.SaveTokens(clientID, clientSecret, "", "", cfg.TokenExpiry); err != nil {
		return configErr(fmt.Errorf("saving credentials: %w", err))
	}
	c := phgql.New(cfg)
	var p phgql.PostsResponse
	if _, err := c.Query(ctx, phgql.PostsQuery, map[string]any{"first": 1}, &p); err != nil {
		wfln("")
		wfln("Credentials saved, but a sample posts query failed:")
		wfln("  " + err.Error())
		return err
	}
	wfln("")
	wfln("OAuth credentials validated. The CLI exchanges them for an access token on each call (cached + refreshed on 401).")
	wfln("Note: under client_credentials, `whoami` returns null — public scope has no user context.")
	return nil
}

func openInBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}
