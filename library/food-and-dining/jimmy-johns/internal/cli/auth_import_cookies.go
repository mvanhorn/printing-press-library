// PATCH: hand-authored cookie import adapter — extends the generated auth flow with a `--from-file`
// reader for `browser-use cookies export` JSON. Not produced by the generator because press v4.2.2
// doesn't fully drive cookie auth (see .printing-press-patches.json patch id "cookie-auth-adapter").

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/jimmy-johns/internal/config"
)

// PATCH: split cookie-character validation into name vs value. Cookie names
// follow RFC 6265 §4.1.1 token rules (no `=`, `;`, `,`, space, control chars,
// or separator-special chars). Cookie values follow the cookie-octet range
// `%x21, %x23-2B, %x2D-3A, %x3C-5B, %x5D-7E` — which DOES include `=` (%x3D).
// The previous shared validator forbade `=` everywhere; that silently dropped
// PerimeterX clearance cookies like `_px3`/`_pxvid` whose base64 payloads
// carry `=` padding, defeating the entire reason the import flow exists.

// validCookieName returns true iff s is a valid cookie name (RFC 6265 token).
// Empty names are rejected (a name is required).
func validCookieName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
		switch r {
		case ' ', '\t', '"', '(', ')', ',', '/', ':', ';', '<', '=', '>', '?', '@', '[', '\\', ']', '{', '}':
			return false
		}
	}
	return true
}

// validCookieValue returns true iff s contains only RFC 6265 cookie-octet
// characters. Permits `=` (used as base64 padding in many session cookies).
// Forbids the header-structure delimiters that would smuggle additional
// cookies or forge CRLF header injection: `;`, `,`, space, tab, `"`, `\`,
// plus all CTLs.
func validCookieValue(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
		switch r {
		case ' ', '\t', '"', ',', ';', '\\':
			return false
		}
	}
	return true
}

// browserUseCookie is the shape `browser-use cookies export <file>` writes.
// Many fields are passthrough from Playwright; we only need name/value/domain.
type browserUseCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"httpOnly"`
}

func newAuthImportCookiesCmd(flags *rootFlags) *cobra.Command {
	var fromFile string
	cmd := &cobra.Command{
		Use:   "import-cookies",
		Short: "Import session cookies from a browser-use export",
		Long: `Import session cookies for jimmyjohns.com from a JSON file produced by
'browser-use cookies export'. Filters to jimmyjohns.com cookies, builds a
Cookie header, and saves it to config.Headers so every API request sends it.

Workflow:

  1. Open jimmyjohns.com in real Chrome, solve any PerimeterX challenge,
     browse naturally (search a store, browse menu, view rewards).
  2. Run: browser-use -b real --profile "Default" cookies export ~/jj-cookies.json
  3. Run: jimmy-johns-pp-cli auth import-cookies --from-file ~/jj-cookies.json
  4. Test: jimmy-johns-pp-cli stores list --address 98112 --json

Note: PerimeterX may invalidate the session if it detects automation between
steps 1 and 2. Run the export immediately after natural browsing.`,
		Example: `  jimmy-johns-pp-cli auth import-cookies --from-file ~/jj-cookies.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if fromFile == "" {
				return cmd.Help()
			}
			raw, err := os.ReadFile(fromFile)
			if err != nil {
				return fmt.Errorf("reading %s: %w", fromFile, err)
			}
			var cookies []browserUseCookie
			if err := json.Unmarshal(raw, &cookies); err != nil {
				return fmt.Errorf("parsing %s: %w", fromFile, err)
			}
			// PATCH: validate name/value characters per RFC 6265 §4.1.1 before assembly.
			// A malformed export with `;` in a value would smuggle multiple cookies into the header
			// and could even forge a header injection if a value contained CR/LF. Skip + warn instead.
			w := cmd.OutOrStdout()
			var pairs []string
			skipped := 0
			for _, c := range cookies {
				dom := strings.TrimPrefix(c.Domain, ".")
				if dom != "jimmyjohns.com" && !strings.HasSuffix(dom, ".jimmyjohns.com") {
					continue
				}
				if c.Name == "" {
					continue
				}
				if !validCookieName(c.Name) || !validCookieValue(c.Value) {
					fmt.Fprintf(w, "warning: skipping cookie %q with disallowed characters in name or value\n", c.Name)
					skipped++
					continue
				}
				pairs = append(pairs, c.Name+"="+c.Value)
			}
			if len(pairs) == 0 {
				return fmt.Errorf("no jimmyjohns.com cookies found in %s (skipped %d invalid)", fromFile, skipped)
			}
			cookieHeader := strings.Join(pairs, "; ")

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cfg.Headers == nil {
				cfg.Headers = map[string]string{}
			}
			cfg.Headers["Cookie"] = cookieHeader
			// Clear any stale AccessToken — cookie auth doesn't use Bearer.
			cfg.AccessToken = ""
			cfg.RefreshToken = ""
			if err := cfg.SaveHeaders(); err != nil {
				return configErr(fmt.Errorf("saving cookies: %w", err))
			}
			fmt.Fprintf(w, "%s Imported %d jimmyjohns.com cookies (%d bytes)\n", green("OK"), len(pairs), len(cookieHeader))
			fmt.Fprintf(w, "Session saved to %s\n", cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Path to browser-use cookies export JSON (required at runtime)")
	return cmd
}
