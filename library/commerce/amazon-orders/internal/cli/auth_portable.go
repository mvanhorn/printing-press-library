package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-orders/internal/config"
	"github.com/spf13/cobra"
)

// PortableSession is the JSON shape used by `auth export` and `auth import`.
// It is intentionally minimal: just the cookie material plus a marker so a
// future format bump can be detected.
type PortableSession struct {
	Schema     string    `json:"schema"`  // "amazon-orders-session/v1"
	Domain     string    `json:"domain"`  // .amazon.com, .amazon.in, etc.
	Cookies    string    `json:"cookies"` // semicolon-joined "k=v" pairs
	ExportedAt time.Time `json:"exported_at"`
	Source     string    `json:"source,omitempty"` // "chrome", "manual", "1password", etc.
	Note       string    `json:"note,omitempty"`   // optional human-friendly note
}

// newAuthExportCmd dumps the active cookie jar as portable JSON. Designed for
// piping into a secrets manager (1Password, Vault, etc.) so the cookie value
// is never seen by an LLM that is orchestrating the agent.
//
// Pattern:
//
//	amazon-orders-pp-cli auth export | op create item login \
//	  --title "amazon-orders" --notes-plain -
//
// Or for a roundtrip onto a headless host:
//
//	amazon-orders-pp-cli auth export --output ./session.json
//	ssh agent "cat - | amazon-orders-pp-cli auth import --stdin" < ./session.json
func newAuthExportCmd(flags *rootFlags) *cobra.Command {
	var output string
	var note string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the active cookie jar as portable JSON for stash-and-inject workflows",
		Long: `Dumps the current authenticated session as JSON suitable for:

  - piping into a secrets manager (1Password, Vault, Bitwarden) so the cookie
    value never enters an LLM's context window;
  - copying to a headless host that will run the CLI as an agent.

The output schema is "amazon-orders-session/v1". Read it back with:

  amazon-orders-pp-cli auth import --stdin < session.json

Or set AMAZON_COOKIES directly (the env var path declared in the spec) so
no on-disk file is ever needed.

See SKILL.md "Headless agent setup with 1Password" for the full LLM-free
roundtrip pattern (capture once, stash in op://Agent/amazon-orders-session,
inject into any other host via 'op read | auth import --stdin').`,
		Example: `  # Export to a 0600 temp file, upload to 1Password, then shred:
  TMP=$(mktemp -t amazon-orders-session.XXXXXX.json) && chmod 600 "$TMP"
  amazon-orders-pp-cli auth export --output "$TMP" --note "captured $(date -u +%FT%TZ)"
  op document create "$TMP" --title amazon-orders-session --vault Agent
  shred -u "$TMP" 2>/dev/null || rm -f "$TMP"

  # Inline pipe (cookie value never crosses a shell variable):
  amazon-orders-pp-cli auth export | op document create - --title amazon-orders-session --vault Agent

  # Quick local export to inspect or copy:
  amazon-orders-pp-cli auth export --output ./session.json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				// Don't expose real cookie material in verify mock runs.
				fmt.Fprintln(cmd.OutOrStdout(), `{"schema":"amazon-orders-session/v1","domain":".amazon.com","cookies":"<redacted>","exported_at":"1970-01-01T00:00:00Z","source":"verify-mock"}`)
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			// PATCH(greptile-auth-export-uses-authheader): delegate cookie selection to AuthHeader() so exported bytes match the live request when AMAZON_COOKIES is set.
			// Delegate the cookie-selection rule to cfg.AuthHeader() so the
			// exported bytes are always exactly what the live request would
			// use. Prior implementation read AccessToken first, which inverted
			// the env-var-wins-over-file rule that AuthHeader() enforces —
			// under AMAZON_COOKIES (the headless-agent path documented in
			// SKILL.md), export would have dumped a stale persisted token
			// instead of the active env-var session.
			cookies := strings.TrimSpace(cfg.AuthHeader())
			if cookies == "" {
				return fmt.Errorf("no active session — run 'auth login --chrome' first")
			}
			// PATCH(greptile-export-source-from-authsource): label exported
			// sessions with the actual source (env:AMAZON_COOKIES, config,
			// browser) classified by Load(), instead of hardcoding "chrome".
			// Roundtrips via env-var or stdin-import had been emitted as
			// "chrome", which misled 1Password/agent inspection.
			src := cfg.AuthSource
			if src == "" {
				src = "chrome"
			}
			domain, err := cookieDomainFromConfig(cfg)
			if err != nil {
				return configErr(fmt.Errorf("invalid base_url: %w", err))
			}

			session := PortableSession{
				Schema:     "amazon-orders-session/v1",
				Domain:     domain,
				Cookies:    cookies,
				ExportedAt: time.Now().UTC(),
				Source:     src,
				Note:       note,
			}
			b, err := json.MarshalIndent(session, "", "  ")
			if err != nil {
				return fmt.Errorf("marshalling session: %w", err)
			}
			b = append(b, '\n')
			if output == "" || output == "-" {
				_, err = cmd.OutOrStdout().Write(b)
				return err
			}
			// Write file with 0600 perms — cookie material is sensitive.
			if err := os.WriteFile(output, b, 0o600); err != nil {
				return fmt.Errorf("writing %s: %w", output, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %d bytes to %s (mode 0600)\n", len(b), output)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "File to write to (default: stdout). The file is created with 0600 permissions.")
	cmd.Flags().StringVar(&note, "note", "", "Optional human-friendly note saved alongside the cookies.")
	return cmd
}

// newAuthImportCmd reads a PortableSession back into the local config. Cookies
// can come from --input <file>, --stdin, or the AMAZON_COOKIES env var.
//
// Together with `auth export`, this is the canonical workflow for stashing a
// session in a secrets manager (1Password, Vault) and injecting it into a
// headless agent run without ever showing the cookie material to the LLM
// that drives the agent.
func newAuthImportCmd(flags *rootFlags) *cobra.Command {
	var input string
	var fromStdin bool
	var rawCookies bool

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a portable session JSON (or raw cookie string) into the local config",
		Long: `Reads cookies into the local config so subsequent commands can authenticate.

Three modes:

  --input <file>    — read JSON from a file
  --stdin           — read JSON from standard input (use with 'op read', curl, etc.)
  --raw-cookies     — treat input as a raw "k=v; k=v" cookie string instead of JSON

The accepted JSON schema is "amazon-orders-session/v1" produced by
'auth export'. Older / handwritten payloads with just a "cookies" key
also work. AMAZON_COOKIES env var is also auto-detected when neither
--input nor --stdin is given.

The LLM-free pattern: a wrapper script (or 'op read' on the same line)
pipes a stashed session into stdin. The cookie value never enters an
LLM's context window or shell variable. See SKILL.md "Headless agent
setup with 1Password" for the full roundtrip and refresh recipe.`,
		Example: `  # From a 1Password document (recommended, op:// URI form):
  op read "op://Agent/amazon-orders-session/file" | amazon-orders-pp-cli auth import --stdin

  # From a 1Password document (long form):
  op document get amazon-orders-session --vault Agent | amazon-orders-pp-cli auth import --stdin

  # From a file:
  amazon-orders-pp-cli auth import --input ./session.json

  # From AMAZON_COOKIES env var (one-shot containers):
  AMAZON_COOKIES="$(op read 'op://Agent/amazon-orders-session/file')" amazon-orders-pp-cli auth import

  # From a raw "k=v; k=v" cookie string (e.g. browser DevTools):
  amazon-orders-pp-cli auth import --stdin --raw-cookies <<< "session-id=...; ubid-main=..."`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would import session (verify mock)")
				return nil
			}
			var blob []byte
			var err error
			switch {
			case input != "":
				blob, err = os.ReadFile(input)
				if err != nil {
					return fmt.Errorf("reading %s: %w", input, err)
				}
			case fromStdin:
				blob, err = io.ReadAll(bufio.NewReader(os.Stdin))
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
			case os.Getenv("AMAZON_COOKIES") != "":
				blob = []byte(os.Getenv("AMAZON_COOKIES"))
				rawCookies = true
			default:
				return fmt.Errorf("provide --input <file>, --stdin, or set AMAZON_COOKIES")
			}
			blob = []byte(strings.TrimSpace(string(blob)))
			if len(blob) == 0 {
				return fmt.Errorf("empty input")
			}

			var cookies string
			var importedDomain string
			if rawCookies || (blob[0] != '{' && blob[0] != '[') {
				cookies = string(blob)
			} else {
				var session PortableSession
				if err := json.Unmarshal(blob, &session); err != nil {
					return fmt.Errorf("parsing session JSON: %w", err)
				}
				if session.Cookies == "" {
					return fmt.Errorf("session JSON has no cookies field")
				}
				cookies = session.Cookies
				importedDomain = session.Domain
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if importedDomain != "" {
				domain, err := normalizeAmazonCookieDomain(importedDomain)
				if err != nil {
					return authErr(err)
				}
				cfg.BaseURL = baseURLForCookieDomain(domain)
			}
			if err := cfg.SaveTokens("", "", cookies, "", time.Time{}); err != nil {
				return configErr(fmt.Errorf("saving cookies: %w", err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Imported session into %s\n", cfg.Path)
			fmt.Fprintln(cmd.OutOrStdout(), "Run 'auth status' to validate, or any other command to use the new session.")
			return nil
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "File to read JSON from (default: stdin or AMAZON_COOKIES env var).")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read JSON from standard input.")
	cmd.Flags().BoolVar(&rawCookies, "raw-cookies", false, "Treat input as a raw 'k=v; k=v' cookie string instead of session JSON.")
	return cmd
}
