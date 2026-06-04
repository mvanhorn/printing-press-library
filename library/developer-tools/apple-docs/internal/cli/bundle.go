// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/applejson"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/client"

	"github.com/spf13/cobra"
)

func newNovelBundleCmd(flags *rootFlags) *cobra.Command {
	var flagDepth int
	var flagMaxTokens int

	cmd := &cobra.Command{
		Use:   "bundle <symbol>",
		Short: "Bundle a symbol's Markdown plus its depth-N See-Also pages into one token-budgeted blob",
		Long: strings.TrimSpace(`
Bundle a symbol's Markdown plus the Markdown of its depth-N See-Also references
into a single token-budgeted blob, ready to paste into an agent prompt.

Use this command to get a self-contained Markdown context blob about a symbol
plus its closest relatives, without doing N round-trips and N JSON parses.

Do NOT use it for a single doc page; use 'doc get --markdown' instead. Do NOT
use it for sample-code projects; use 'sample-code list' instead.

Token budget is approximated as chars/4. Pages whose render would push the
total over --max-tokens are truncated, not silently dropped — the bundle stays
deterministic at any budget.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli bundle swiftui/view/onappear --depth 1 --max-tokens 4000
  apple-docs-pp-cli bundle foundation/url --depth 0 --max-tokens 2000
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would bundle symbol with depth=%d max-tokens=%d\n", flagDepth, flagMaxTokens)
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<symbol> is required"))
			}
			if flagDepth < 0 || flagDepth > 3 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--depth must be 0..3"))
			}
			if flagMaxTokens < 200 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--max-tokens must be >= 200"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out, err := bundleSymbol(cmd, c, args[0], flagDepth, flagMaxTokens)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDepth, "depth", 1, "How many hops of See-Also references to include (0..3)")
	cmd.Flags().IntVar(&flagMaxTokens, "max-tokens", 4000, "Approximate token budget (chars / 4); blob is truncated to fit")
	return cmd
}

func bundleSymbol(cmd *cobra.Command, c *client.Client, root string, depth, maxTokens int) (string, error) {
	maxChars := maxTokens * 4
	var sb strings.Builder
	visited := map[string]bool{}

	type item struct {
		path string
		hop  int
	}
	queue := []item{{path: root, hop: 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		key := strings.ToLower(strings.Trim(cur.path, "/ "))
		if visited[key] {
			continue
		}
		visited[key] = true

		page, err := applejson.FetchDoc(cmd.Context(), c, cur.path)
		if err != nil {
			if cur.hop == 0 {
				return "", err
			}
			continue
		}
		md := renderMarkdown(page)
		header := fmt.Sprintf("\n\n<!-- depth=%d source=%s -->\n", cur.hop, cur.path)
		piece := header + md
		remaining := maxChars - sb.Len()
		if remaining <= 0 {
			break
		}
		if len(piece) > remaining {
			// Walk back to a UTF-8 rune boundary so we never emit a
			// half-cut multi-byte rune. Apple docs include non-ASCII
			// characters (curly quotes, em-dashes, occasional Chinese
			// terminology); cutting one mid-rune would produce
			// invalid UTF-8 and corrupt downstream JSON marshaling.
			cut := remaining
			for cut > 0 && piece[cut]&0xC0 == 0x80 {
				cut--
			}
			piece = piece[:cut] + "\n... [truncated]"
			sb.WriteString(piece)
			break
		}
		sb.WriteString(piece)

		if cur.hop < depth {
			for _, ref := range pickSeeAlso(page) {
				if visited[strings.ToLower(strings.Trim(ref, "/ "))] {
					continue
				}
				queue = append(queue, item{path: ref, hop: cur.hop + 1})
			}
		}
	}
	return strings.TrimLeft(sb.String(), "\n"), nil
}

// pickSeeAlso returns the relative paths of references on a page whose
// kind is a symbol, sample-code, or article — entries that make sense
// to bundle alongside. Capped to 8 per page so a single hop doesn't blow
// the budget.
//
// The output is sorted by path so bundle and port-to produce
// deterministic, reproducible walks across runs. Without the sort, Go's
// randomized map iteration over p.References would make the
// cap-at-8 slice arbitrary per run.
func pickSeeAlso(p *applejson.DocPage) []string {
	var out []string
	for _, ref := range p.References {
		if ref.URL == "" {
			continue
		}
		if !strings.HasPrefix(ref.URL, "/documentation/") {
			continue
		}
		rel := strings.TrimPrefix(ref.URL, "/documentation/")
		if strings.EqualFold(strings.Trim(rel, "/"), strings.Trim(strings.TrimPrefix(p.Identifier, "doc://com.apple."), "/")) {
			continue
		}
		switch ref.Kind {
		case "symbol", "article", "sampleCode":
			out = append(out, rel)
		}
	}
	sort.Strings(out)
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}
