// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/config"

	"github.com/chromedp/chromedp"
	"github.com/spf13/cobra"
)

// signInURL is the dominos.com homepage. Deep-linking to a hash-route like
// /pages/customer/#!/customer/signIn/ does NOT reliably open the sign-in
// modal in the SPA — the homepage with its "SIGN IN & EARN REWARDS" button
// is the path that always works.
const signInURL = "https://www.dominos.com/"

// harvestExpr reads sessionStorage.accessTokens['customer-oauth'] AND
// sessionStorage.Customer.{CustomerID,Email,FirstName,LastName} from the
// dominos.com page context. Returns a JSON envelope so the caller never has
// to know their opaque CustomerID — it's harvested alongside the bearer.
const harvestExpr = `(function(){var out={token:'',customer_id:'',email:'',first_name:'',last_name:''};try{var at=JSON.parse(sessionStorage.getItem('accessTokens')||'{}');out.token=at['customer-oauth']||''}catch(e){}try{var c=JSON.parse(sessionStorage.getItem('Customer')||'{}');out.customer_id=c.CustomerID||'';out.email=c.Email||'';out.first_name=c.FirstName||'';out.last_name=c.LastName||''}catch(e){}return JSON.stringify(out)})()`

// harvestEnvelope is the shape harvestExpr returns. Use it to decode the JS
// result into Go fields rather than parsing strings.
type harvestEnvelope struct {
	Token      string `json:"token"`
	CustomerID string `json:"customer_id"`
	Email      string `json:"email"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
}

// pollInterval is how often we recheck sessionStorage while the user is
// signing in. Short enough to feel responsive, long enough to not hammer
// the page.
const pollInterval = 1500 * time.Millisecond

func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var paste bool
	var stdinToken bool
	var headless bool
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Open Chrome, sign in, and harvest a bearer token automatically",
		Long: strings.Trim(`
Spawns a dedicated Chrome window pointed at dominos.com sign-in, waits for
you to complete the login (handles captcha, 2FA, anything Domino's gates
with), then reads the bearer JWT from sessionStorage and saves it to
~/.config/dominos-pp-cli/config.toml. No copy-paste required.

The window uses an isolated profile directory under
~/.config/dominos-pp-cli/chrome/, so it does NOT touch your default Chrome.
First time you run it, you'll need to sign in fresh. The Chrome profile
persists between runs (so subsequent logins are faster), but the token has
a short Domino's-side expiry (~1 hour) so you'll re-run when it expires.

Alternative flows:
  --paste     Skip the browser, prompt for a token you harvested elsewhere
  --stdin     Read the token from stdin (e.g. pbpaste | auth login --stdin)
`, "\n"),
		Example:     "  dominos-pp-cli auth login\n  dominos-pp-cli auth login --paste\n  pbpaste | dominos-pp-cli auth login --stdin",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			w := cmd.OutOrStdout()

			if stdinToken {
				return saveTokenFromReader(w, flags, os.Stdin, "stdin")
			}
			if paste {
				return saveTokenFromReader(w, flags, bufio.NewReader(os.Stdin), "paste prompt")
			}

			// chromedp path: spawn Chrome, wait for sign-in, harvest token + customer.
			env, err := harvestTokenViaChrome(cmd.Context(), w, headless, timeout)
			if err != nil {
				fmt.Fprintf(w, "\nChrome harvest failed: %v\n", err)
				fmt.Fprintln(w, "Falling back to manual paste. Open dominos.com, sign in, then in DevTools Console run:")
				fmt.Fprintln(w, "  copy(JSON.parse(sessionStorage.getItem('accessTokens'))['customer-oauth'])")
				fmt.Fprint(w, "Token: ")
				return saveTokenFromReader(w, flags, bufio.NewReader(os.Stdin), "manual fallback")
			}
			return saveHarvestedAuth(w, flags, env, "chrome harvest")
		},
	}
	cmd.Flags().BoolVar(&paste, "paste", false, "Skip the browser-spawn step; prompt for the token directly")
	cmd.Flags().BoolVar(&stdinToken, "stdin", false, "Read the token from stdin (e.g. pbpaste | auth login --stdin)")
	cmd.Flags().BoolVar(&headless, "headless", false, "Run Chrome headless (rare; usually you want to see the sign-in window)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Maximum time to wait for the user to complete sign-in")
	return cmd
}

// harvestTokenViaChrome spawns a chromedp-controlled Chrome window pointed
// at the dominos sign-in page, waits for the user to authenticate, and
// returns the harvested bearer JWT + Customer profile (CustomerID, email,
// name) so callers default to the saved CustomerID instead of asking the
// user to paste an opaque identifier. Profile persists between runs at
// ~/.config/dominos-pp-cli/chrome/ so re-logins are faster.
func harvestTokenViaChrome(parent context.Context, w io.Writer, headless bool, timeout time.Duration) (harvestEnvelope, error) {
	var empty harvestEnvelope
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	// Persistent user-data-dir keeps cookies between runs (so re-logins are faster).
	home, _ := os.UserHomeDir()
	userDataDir := filepath.Join(home, ".config", "dominos-pp-cli", "chrome")
	if err := os.MkdirAll(userDataDir, 0o700); err != nil {
		return empty, fmt.Errorf("creating chrome profile dir: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.UserDataDir(userDataDir),
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	fmt.Fprintln(w, "Opening Chrome — sign in to Domino's normally; the token will be picked up automatically.")
	fmt.Fprintln(w, "(Profile persists at ~/.config/dominos-pp-cli/chrome — subsequent logins are faster.)")
	if err := chromedp.Run(browserCtx, chromedp.Navigate(signInURL)); err != nil {
		return empty, fmt.Errorf("navigating to sign-in: %w", err)
	}

	deadline := time.Now().Add(timeout)
	tickCount := 0
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return empty, ctx.Err()
		default:
		}
		var raw string
		if err := chromedp.Run(browserCtx, chromedp.Evaluate(harvestExpr, &raw)); err == nil && raw != "" {
			var env harvestEnvelope
			if jerr := json.Unmarshal([]byte(raw), &env); jerr == nil && env.Token != "" {
				fmt.Fprintln(w, "Sign-in detected — harvesting token + customer profile...")
				return env, nil
			}
		}
		tickCount++
		if tickCount%20 == 0 {
			elapsed := time.Since(deadline.Add(-timeout)).Round(time.Second)
			fmt.Fprintf(w, "Still waiting for sign-in (%s elapsed, timeout %s)...\n", elapsed, timeout)
		}
		time.Sleep(pollInterval)
	}
	return empty, fmt.Errorf("timed out after %s waiting for sign-in", timeout)
}

// saveHarvestedAuth saves the bearer token AND the customer profile fields
// from a chromedp harvest so subsequent commands (customer orders, customer
// loyalty) can default to the saved CustomerID instead of asking the user.
func saveHarvestedAuth(w io.Writer, flags *rootFlags, env harvestEnvelope, source string) error {
	if env.Token == "" {
		return fmt.Errorf("harvest from %s missing bearer token; aborting", source)
	}
	if len(env.Token) < 20 {
		return fmt.Errorf("token from %s is suspiciously short (%d chars); aborting", source, len(env.Token))
	}
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	cfg.AuthHeaderVal = ""
	cfg.DominosCustomerID = env.CustomerID
	if env.Email != "" {
		cfg.DominosUsername = env.Email
	}
	if err := cfg.SaveTokens("", "", env.Token, "", time.Time{}); err != nil {
		return configErr(fmt.Errorf("saving harvested token: %w", err))
	}
	if flags.asJSON {
		return printJSONFiltered(w, map[string]any{
			"saved":       true,
			"config_path": cfg.Path,
			"source":      source,
			"customer_id": env.CustomerID,
			"email":       env.Email,
			"token_chars": len(env.Token),
		}, flags)
	}
	if env.FirstName != "" {
		fmt.Fprintf(w, "Signed in as %s %s (%s).\n", env.FirstName, env.LastName, env.Email)
	}
	fmt.Fprintf(w, "Saved bearer token + customer ID to %s.\n", cfg.Path)
	fmt.Fprintln(w, "Customer-scoped commands (customer orders, customer loyalty) now default to your saved CustomerID.")
	return nil
}

// saveTokenFromReader reads a bearer token from r, validates it, and saves
// it to the local config so cfg.AuthHeader() returns "Bearer <token>" on
// subsequent commands.
func saveTokenFromReader(w io.Writer, flags *rootFlags, r io.Reader, source string) error {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("reading token from %s: %w", source, err)
	}
	token := strings.TrimSpace(line)
	token = strings.Trim(token, `"`)
	if token == "" {
		return fmt.Errorf("no token received from %s; aborting save", source)
	}
	if len(token) < 20 {
		return fmt.Errorf("token from %s is suspiciously short (%d chars); aborting", source, len(token))
	}

	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return configErr(err)
	}
	cfg.AuthHeaderVal = ""
	emptyClient := ""
	emptyRefresh := ""
	if err := cfg.SaveTokens(emptyClient, emptyClient, token, emptyRefresh, time.Time{}); err != nil {
		return configErr(fmt.Errorf("saving harvested token: %w", err))
	}
	if flags.asJSON {
		return printJSONFiltered(w, map[string]any{
			"saved":       true,
			"config_path": cfg.Path,
			"source":      source,
			"token_chars": len(token),
		}, flags)
	}
	fmt.Fprintf(w, "Saved bearer token to %s (length: %d chars).\n", cfg.Path, len(token))
	fmt.Fprintln(w, "Run `dominos-pp-cli auth status` to confirm. The token expires after Domino's session timeout (~1 hour).")
	return nil
}
