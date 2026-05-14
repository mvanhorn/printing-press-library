package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yt-studio/internal/ytstudio"
)

// newLoginCmd is a thin wrapper around `auth login` that also walks the user
// through capturing a Studio session cookie set when --studio is passed.
// Per the design spec, this is the ONE interactive command in the CLI; all
// other commands work non-interactively against the credentials it harvests.
func newLoginCmd(flags *rootFlags) *cobra.Command {
	var (
		studioOnly bool
		oauthOnly  bool
		session    string
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "One-time interactive setup: OAuth (Data + Analytics APIs) + optional Studio cookie capture",
		Long: strings.TrimSpace(`
Convenience wrapper:
  - ` + "`yt-studio-pp-cli login`" + `               → OAuth flow + Studio capture walkthrough
  - ` + "`yt-studio-pp-cli login --oauth-only`" + `  → OAuth only (use ` + "`auth login`" + ` directly for scripted setups)
  - ` + "`yt-studio-pp-cli login --studio-only`" + ` → skip OAuth, capture Studio cookies only

Studio capture is interactive: it walks you through opening Studio in your
browser, exporting cookies via the browser DevTools, and pasting them into
this CLI. The captured session is stored at
~/.openclaw/state/yt-studio/studio-session.json (mode 600).

This command short-circuits under the printing-press verifier (no real
state mutation in verify mode).`),
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// In verify mode we don't actually do anything interactive.
			if os.Getenv("PRINTING_PRESS_VERIFY") == "1" {
				fmt.Fprintln(cmd.OutOrStdout(), "would walk user through OAuth + Studio cookie capture")
				return nil
			}
			if flags.noInput {
				return usageErr(errors.New("login is interactive; --no-input is incompatible. Use `yt-studio-pp-cli auth login --client-id ... --client-secret ...` for headless OAuth setup."))
			}

			if !studioOnly {
				fmt.Fprintln(cmd.OutOrStdout(), "Step 1/2: OAuth (Data API + Analytics API)")
				fmt.Fprintln(cmd.OutOrStdout(), "  Run `yt-studio-pp-cli auth login --client-id <id> --client-secret <secret>`")
				fmt.Fprintln(cmd.OutOrStdout(), "  Pre-requisites: create an OAuth 2.0 Desktop Client in Google Cloud Console")
				fmt.Fprintln(cmd.OutOrStdout(), "  and enable both YouTube Data API v3 and YouTube Analytics API on the project.")
				fmt.Fprintln(cmd.OutOrStdout(), "  Get credentials at: https://console.cloud.google.com/apis/credentials")
				fmt.Fprintln(cmd.OutOrStdout(), "")
				if oauthOnly {
					return nil
				}
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Step 2/2: Studio session capture")
			fmt.Fprintln(cmd.OutOrStdout(), "  1. Open https://studio.youtube.com in your browser (logged in to YOUR channel).")
			fmt.Fprintln(cmd.OutOrStdout(), "  2. Open DevTools → Application → Cookies → studio.youtube.com")
			fmt.Fprintln(cmd.OutOrStdout(), "  3. Copy these cookies (one per line in `name=value` format):")
			fmt.Fprintln(cmd.OutOrStdout(), "       LOGIN_INFO, SAPISID, HSID, SSID, APISID, SID, __Secure-1PSID, __Secure-3PSID, __Secure-3PAPISID")
			fmt.Fprintln(cmd.OutOrStdout(), "  4. Paste all cookies below, then send EOF (Ctrl-D on Unix, Ctrl-Z then Enter on Windows).")
			fmt.Fprintln(cmd.OutOrStdout(), "")
			fmt.Fprintln(cmd.OutOrStdout(), ">>> paste cookies (one per line, name=value), then Ctrl-D:")

			cookies, err := readCookiePairs(os.Stdin)
			if err != nil {
				return err
			}
			if len(cookies) == 0 {
				return usageErr(errors.New("no cookies pasted; aborting"))
			}
			sf := &ytstudio.SessionFile{
				CapturedAt: time.Now().UTC(),
				Cookies:    cookies,
				ClientName: "62",
			}
			if err := ytstudio.Save(session, sf); err != nil {
				return fmt.Errorf("saving Studio session: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nSaved Studio session with %d cookies → %s\n", len(cookies), pathOrDefault(session))
			fmt.Fprintln(cmd.OutOrStdout(), "Run `yt-studio-pp-cli sniff-doctor` to verify.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&studioOnly, "studio-only", false, "Skip OAuth instructions and jump to Studio cookie capture")
	cmd.Flags().BoolVar(&oauthOnly, "oauth-only", false, "Only print OAuth instructions; skip Studio capture")
	cmd.Flags().StringVar(&session, "session", "", "Path to Studio session JSON (default: ~/.openclaw/state/yt-studio/studio-session.json)")
	return cmd
}

func readCookiePairs(r *os.File) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 256*1024), 256*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Support `name=value` and `name<TAB>value`
		var name, value string
		switch {
		case strings.Contains(line, "="):
			parts := strings.SplitN(line, "=", 2)
			name = strings.TrimSpace(parts[0])
			value = strings.TrimSpace(parts[1])
		case strings.Contains(line, "\t"):
			parts := strings.SplitN(line, "\t", 2)
			name = strings.TrimSpace(parts[0])
			value = strings.TrimSpace(parts[1])
		default:
			continue
		}
		if name == "" || value == "" {
			continue
		}
		out[name] = value
	}
	return out, sc.Err()
}

func pathOrDefault(p string) string {
	if p == "" {
		return ytstudio.DefaultPath()
	}
	return p
}
