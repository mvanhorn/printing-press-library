// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/auth"
	"github.com/spf13/cobra"
)

// newAuthLoginCmd manages the optional Tier-1 Medium session cookie that unlocks
// member full bodies on the read path. v2 is $0/no-key: this cookie is the
// user's OWN session, never an API key, and is always optional — with none set,
// every command runs anonymously (Tier 0).
//
// Three import paths, first hit wins (see internal/auth):
//  1. MEDIUM_SESSION env ("sid=..; uid=.." or just the sid)
//  2. --cookie-file / MEDIUM_COOKIE_FILE (flat JSON {"sid":"..","uid":".."})
//  3. --chrome auto-extract — NOT built in (would need a macOS Keychain prompt
//     and an extra dependency); it prints the guidance for paths 1/2 instead.
//
// With no flag, `auth login` reports the current cookie status (which path, if
// any, is providing a session) without performing any network or Keychain call.
func newAuthLoginCmd(flags *rootFlags) *cobra.Command {
	var chrome bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Import an optional Medium session cookie (Tier 1, unlocks member full bodies)",
		Long: `Import the optional, user-supplied Medium session cookie that unlocks member
full article bodies on the read path. This is your OWN browser session, never an
API key — every command works without it (anonymously, Tier 0).

Supported import paths (first hit wins):
  • export MEDIUM_SESSION="sid=<sid>; uid=<uid>"
  • save {"sid":"<sid>","uid":"<uid>"} to a file and pass --cookie-file <path>
    (or set MEDIUM_COOKIE_FILE)

Copy sid/uid from your browser's medium.com cookies
(DevTools -> Application -> Cookies -> https://medium.com).

With no flag, this reports which path (if any) is currently providing a session.`,
		Example: `  medium-reader-pp-cli auth login
  medium-reader-pp-cli auth login --cookie-file ~/.medium-cookies.json
  medium-reader-pp-cli auth login --chrome`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()

			// --chrome: extract the session directly from the browser. In the
			// default (pure-Go) build this is a clearly-messaged stub; in a
			// `-tags kooky` build it really reads Chrome. Either outcome exits
			// cleanly (code 0): a stub message or a "sign in first" hint is an
			// instruction, not a failure of the requested operation.
			if chrome {
				cookies, err := auth.ExtractFromChrome()
				if err != nil {
					if flags.asJSON {
						return printJSONFiltered(w, map[string]any{"imported": false, "error": err.Error()}, flags)
					}
					fmt.Fprintln(w, err.Error())
					return nil
				}
				// Success (kooky build): persist to the cookie file if one was
				// given so the session is reusable. The raw token is never
				// printed — only a masked confirmation.
				saved := ""
				if flags.cookieFile != "" {
					if werr := auth.WriteCookieFile(flags.cookieFile, cookies); werr != nil {
						return configErr(werr)
					}
					saved = flags.cookieFile
				}
				if flags.asJSON {
					return printJSONFiltered(w, map[string]any{
						"imported": true,
						"has_sid":  cookies.Sid != "",
						"has_uid":  cookies.Uid != "",
						"saved_to": saved,
					}, flags)
				}
				fmt.Fprintln(w, "Imported your Medium session from Chrome (Tier 1).")
				fmt.Fprintf(w, "  sid: %s\n", masked(cookies.Sid))
				if cookies.Uid != "" {
					fmt.Fprintf(w, "  uid: %s\n", masked(cookies.Uid))
				}
				if saved != "" {
					fmt.Fprintf(w, "Saved to %s — use it with --cookie-file %s (or export MEDIUM_COOKIE_FILE=%s).\n", saved, saved, saved)
				} else {
					fmt.Fprintln(w, "Re-run with --cookie-file <path> to save it for reuse.")
				}
				return nil
			}

			cookies, err := auth.Load(auth.Options{CookieFile: flags.cookieFile})
			if err != nil {
				return configErr(err)
			}

			if flags.asJSON {
				return printJSONFiltered(w, map[string]any{
					"authenticated": !cookies.IsZero(),
					"has_sid":       cookies.Sid != "",
					"has_uid":       cookies.Uid != "",
				}, flags)
			}

			if cookies.IsZero() {
				fmt.Fprintln(w, "No Medium session cookie configured (running anonymously, Tier 0).")
				fmt.Fprintln(w, "")
				fmt.Fprintln(w, "To unlock member full bodies, import a cookie:")
				fmt.Fprintln(w, `  export MEDIUM_SESSION="sid=<sid>; uid=<uid>"`)
				fmt.Fprintln(w, `  or: medium-reader-pp-cli auth login --cookie-file <path>  (flat JSON {"sid":"..","uid":".."})`)
				return nil
			}

			fmt.Fprintln(w, "Medium session cookie present (Tier 1).")
			fmt.Fprintf(w, "  sid: %s\n", masked(cookies.Sid))
			if cookies.Uid != "" {
				fmt.Fprintf(w, "  uid: %s\n", masked(cookies.Uid))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&chrome, "chrome", false, "Attempt Chrome auto-extract (not built in; prints how to import via env or --cookie-file)")
	return cmd
}

// masked renders a cookie token as a short, non-leaking confirmation: the first
// few characters plus an ellipsis. It never prints the full token so scripted
// runs that capture stdout don't expose the session bytes.
//
// The first-4 slice is taken on runes, not bytes, so a token that starts with a
// multibyte character is never split mid-rune (which would emit invalid UTF-8).
func masked(s string) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 4 {
		return "****"
	}
	return string(r[:4]) + "…"
}
