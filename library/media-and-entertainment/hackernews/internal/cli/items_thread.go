package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/algolia"
)

func newItemsThreadCmd(flags *rootFlags) *cobra.Command {
	var depth int
	var flat bool
	var author string
	var match string
	var since string

	cmd := &cobra.Command{
		Use:   "thread <id>",
		Short: "Print a thread's comment tree using Algolia's one-shot fetch",
		Long: `Print a Hacker News thread's full comment tree in a single Algolia call.

The comment tree is fetched from Algolia's /items endpoint, which returns
the entire thread without recursive Firebase walks. By default the tree
is rendered as nested replies; --flat prints one comment per line.`,
		Example: strings.Trim(`
  # Tree view (default)
  hackernews-pp-cli items thread 12345678

  # Flat list, easier to grep
  hackernews-pp-cli items thread 12345678 --flat

  # Cap to depth 2 for huge threads
  hackernews-pp-cli items thread 12345678 --depth 2

  # Filter by author and time
  hackernews-pp-cli items thread 12345678 --author dang --since 24h

  # JSON for piping into jq
  hackernews-pp-cli items thread 12345678 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id := args[0]
			ac := algolia.New(flags.timeout)
			node, err := ac.Item(id)
			if err != nil {
				return apiErr(err)
			}

			var sinceCutoff int64
			if since != "" {
				ts, parseErr := parseSinceDuration(since)
				if parseErr != nil {
					return usageErr(fmt.Errorf("invalid --since: %w", parseErr))
				}
				sinceCutoff = ts.Unix()
			}
			var matchRE *regexp.Regexp
			if match != "" {
				re, reErr := regexp.Compile(match)
				if reErr != nil {
					return usageErr(fmt.Errorf("invalid --match regex: %w", reErr))
				}
				matchRE = re
			}

			// Filter the tree according to flags.
			filterNode(node, depth, 0, author, matchRE, sinceCutoff)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(node, "", "  ")
				return printOutput(cmd.OutOrStdout(), out, true)
			}

			if flat {
				renderFlat(cmd.OutOrStdout(), node, 0)
			} else {
				renderTree(cmd.OutOrStdout(), node, 0, "")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&depth, "depth", 0, "Maximum depth of replies to include (0 = no limit)")
	cmd.Flags().BoolVar(&flat, "flat", false, "Render comments as a flat list instead of a tree")
	cmd.Flags().StringVar(&author, "author", "", "Only include comments by this username")
	cmd.Flags().StringVar(&match, "match", "", "Only include comments whose text matches this regex")
	cmd.Flags().StringVar(&since, "since", "", "Only include comments newer than this duration (e.g., 24h, 7d)")
	return cmd
}

// filterNode prunes the tree under node. Children whose subtree has no
// matching comment are dropped; matching nodes survive even if their
// children don't, and intermediate non-matching nodes survive as frames
// when their subtree contains matches. The root (story) is never pruned.
// Depth is the cap (0 = no cap); cur is the depth of node itself.
//
// Filters only apply when at least one is set. With no filters, the whole
// tree under the depth cap is returned.
func filterNode(node *algolia.ItemNode, depth, cur int, author string, matchRE *regexp.Regexp, sinceCutoff int64) {
	if node == nil {
		return
	}
	if depth > 0 && cur >= depth {
		node.Children = nil
		return
	}
	hasFilters := author != "" || matchRE != nil || sinceCutoff > 0
	if !hasFilters {
		for i := range node.Children {
			filterNode(&node.Children[i], depth, cur+1, author, matchRE, sinceCutoff)
		}
		return
	}
	kept := node.Children[:0]
	for i := range node.Children {
		filterNode(&node.Children[i], depth, cur+1, author, matchRE, sinceCutoff)
		if commentMatches(&node.Children[i], author, matchRE, sinceCutoff) || len(node.Children[i].Children) > 0 {
			kept = append(kept, node.Children[i])
		}
	}
	node.Children = kept
}

func commentMatches(n *algolia.ItemNode, author string, matchRE *regexp.Regexp, sinceCutoff int64) bool {
	if author != "" && !strings.EqualFold(n.Author, author) {
		return false
	}
	if matchRE != nil && !matchRE.MatchString(n.Text) {
		return false
	}
	if sinceCutoff > 0 && n.CreatedAtI < sinceCutoff {
		return false
	}
	return true
}

func renderTree(w io.Writer, node *algolia.ItemNode, depth int, prefix string) {
	if node == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	header := node.Author
	if header == "" {
		header = "(unknown)"
	}
	when := relativeTimeFromUnix(node.CreatedAtI)
	if node.Title != "" {
		fmt.Fprintf(w, "%s%s [%d points] by %s — %s\n", indent, node.Title, node.Points, header, when)
	} else {
		fmt.Fprintf(w, "%s%s — %s\n", indent, header, when)
	}
	body := stripHTML(node.Text)
	if body != "" {
		for _, line := range strings.Split(body, "\n") {
			fmt.Fprintf(w, "%s    %s\n", indent, line)
		}
	}
	for i := range node.Children {
		renderTree(w, &node.Children[i], depth+1, prefix)
	}
}

func renderFlat(w io.Writer, node *algolia.ItemNode, depth int) {
	if node == nil {
		return
	}
	when := relativeTimeFromUnix(node.CreatedAtI)
	if node.Title != "" {
		fmt.Fprintf(w, "[story] %s by %s — %s\n", node.Title, node.Author, when)
	} else {
		body := stripHTML(node.Text)
		body = strings.ReplaceAll(body, "\n", " ")
		if len(body) > 240 {
			body = body[:240] + "..."
		}
		fmt.Fprintf(w, "%s — %s: %s\n", node.Author, when, body)
	}
	for i := range node.Children {
		renderFlat(w, &node.Children[i], depth+1)
	}
}

func relativeTimeFromUnix(ts int64) string {
	if ts <= 0 {
		return "?"
	}
	d := time.Since(time.Unix(ts, 0))
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	}
}

var htmlTagRE = regexp.MustCompile(`<[^>]+>`)
var htmlEntityRE = map[string]string{
	"&amp;":  "&",
	"&lt;":   "<",
	"&gt;":   ">",
	"&quot;": "\"",
	"&#39;":  "'",
	"&#x27;": "'",
	"&#x2F;": "/",
	"&nbsp;": " ",
}

// stripHTML removes Algolia-encoded HTML tags and decodes the common
// entities Algolia emits. It does not pull in golang.org/x/net/html;
// the Algolia text fields are simple — <p>, <a>, <i>, occasional <pre>.
func stripHTML(s string) string {
	if s == "" {
		return ""
	}
	out := htmlTagRE.ReplaceAllString(s, "")
	for k, v := range htmlEntityRE {
		out = strings.ReplaceAll(out, k, v)
	}
	return strings.TrimSpace(out)
}
