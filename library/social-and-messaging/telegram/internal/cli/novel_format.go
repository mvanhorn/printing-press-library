// `format` — novel #5 (lint half). Local Telegram-HTML lint that pinpoints
// the byte offset of malformed tags before the API rejects them with 400.

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newFormatCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "format",
		Short: "Local message formatters and linters (no API call)",
	}
	cmd.AddCommand(newFormatHTMLLintCmd(flags))
	return cmd
}

func newFormatHTMLLintCmd(flags *rootFlags) *cobra.Command {
	var (
		text      string
		readStdin bool
	)
	cmd := &cobra.Command{
		Use:   "html-lint",
		Short: "Validate Telegram HTML before sending; reports offsets for malformed entities",
		Long:  "Telegram's HTML parse mode rejects unknown tags and unbalanced entities with HTTP 400 ('can't parse entities'). This command performs the same check locally and returns the byte offset of the first problem, with a suggested fix.",
		Example: strings.Trim(`
  telegram-pp-cli format html-lint --text "<b>release</b> <unknown>X</unknown>"
  cat post.html | telegram-pp-cli format html-lint --stdin --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("text") && !readStdin && len(args) == 0 {
				return cmd.Help()
			}
			body, err := resolveSendText(text, readStdin, args)
			if err != nil {
				return usageErr(err)
			}
			report := lintTelegramHTML(body)
			if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
				return err
			}
			if !report.OK {
				return apiErr(fmt.Errorf("html-lint: %s at offset %d", report.Issue, report.Offset))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "Text to lint")
	cmd.Flags().BoolVar(&readStdin, "stdin", false, "Read text from stdin")
	return cmd
}

type htmlLintReport struct {
	OK      bool   `json:"ok"`
	Issue   string `json:"issue,omitempty"`
	Offset  int    `json:"offset,omitempty"`
	Snippet string `json:"snippet,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

// lintTelegramHTML scans `s` for tags Telegram does not accept and for
// unbalanced angle brackets. Returns the first issue. Allowed tags per the
// Bot API HTML-style docs (https://core.telegram.org/bots/api#html-style):
// b, strong, i, em, u, ins, s, strike, del, span (tg-spoiler class only),
// tg-spoiler, a, code, pre, br.
func lintTelegramHTML(s string) htmlLintReport {
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch != '<' {
			continue
		}
		end := strings.IndexByte(s[i:], '>')
		if end == -1 {
			return htmlLintReport{
				Issue:   "unterminated '<' (missing closing '>')",
				Offset:  i,
				Snippet: snippet(s, i, 24),
				Hint:    "Escape stray '<' as &lt; or close the tag",
			}
		}
		tag := s[i : i+end+1]
		if !isAllowedTelegramTag(tag) {
			return htmlLintReport{
				Issue:   "tag not in Telegram's HTML whitelist: " + tag,
				Offset:  i,
				Snippet: snippet(s, i, 32),
				Hint:    "Use --html-escape on send, or pick from b/strong/i/em/u/s/code/pre/a/tg-spoiler",
			}
		}
		i += end
	}
	return htmlLintReport{OK: true}
}

func isAllowedTelegramTag(tag string) bool {
	return allowedHTMLTagRE.MatchString(tag)
}

func snippet(s string, offset, span int) string {
	end := offset + span
	if end > len(s) {
		end = len(s)
	}
	return s[offset:end]
}

// Verify io.Discard is reachable to silence Go's tooling complaint if no
// other file uses it. (No-op in production paths.)
var _ = io.Discard
var _ = os.Stdin
