package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/algolia"
)

// `hiring` and `freelance` find the latest "Ask HN: Who is hiring"
// (or "Freelancer? Seeking freelancer?") thread by querying Algolia
// with author=whoishiring and the appropriate title prefix. We then
// fetch the thread via /items/{id} and filter top-level comments by
// the user-supplied regex.

type hiringPost struct {
	ID        int    `json:"id"`
	Author    string `json:"author"`
	CreatedAt int64  `json:"created_at_i"`
	Text      string `json:"text"`
	URL       string `json:"hn_url"`
}

func findLatestThread(ac *algolia.Client, titleSubstr string) (*algolia.SearchHit, error) {
	resp, err := ac.Search("", algolia.SearchOpts{
		Tags:        "story,author_whoishiring",
		ByDate:      true,
		HitsPerPage: 20,
	})
	if err != nil {
		return nil, err
	}
	for i := range resp.Hits {
		h := resp.Hits[i]
		if strings.Contains(strings.ToLower(h.Title), strings.ToLower(titleSubstr)) {
			return &h, nil
		}
	}
	return nil, fmt.Errorf("no whoishiring thread containing %q in the last 20 results", titleSubstr)
}

func filterHiringPosts(node *algolia.ItemNode, re *regexp.Regexp) []hiringPost {
	out := []hiringPost{}
	for _, c := range node.Children {
		text := stripHTML(c.Text)
		if re == nil || re.MatchString(text) {
			out = append(out, hiringPost{
				ID:        c.ID,
				Author:    c.Author,
				CreatedAt: c.CreatedAtI,
				Text:      text,
				URL:       fmt.Sprintf("https://news.ycombinator.com/item?id=%d", c.ID),
			})
		}
	}
	return out
}

func runHiringCommand(cmd *cobra.Command, flags *rootFlags, args []string, titleSubstr string) error {
	if dryRunOK(flags) {
		return nil
	}
	var re *regexp.Regexp
	if len(args) > 0 && args[0] != "" {
		compiled, err := regexp.Compile("(?i)" + args[0])
		if err != nil {
			return usageErr(fmt.Errorf("invalid regex: %w", err))
		}
		re = compiled
	}

	ac := algolia.New(flags.timeout)
	thread, err := findLatestThread(ac, titleSubstr)
	if err != nil {
		return apiErr(err)
	}
	full, err := ac.Item(fmt.Sprintf("%s", thread.ObjectID))
	if err != nil {
		return apiErr(err)
	}
	posts := filterHiringPosts(full, re)

	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		envelope := map[string]any{
			"thread_id":    thread.ObjectID,
			"thread_title": thread.Title,
			"matched":      len(posts),
			"posts":        posts,
		}
		j, _ := json.MarshalIndent(envelope, "", "  ")
		return printOutput(cmd.OutOrStdout(), j, true)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s — %d posts matched\n\n", thread.Title, len(posts))
	for _, p := range posts {
		body := strings.ReplaceAll(p.Text, "\n", " ")
		fmt.Fprintf(cmd.OutOrStdout(), "  %s — %s\n  %s\n  %s\n\n", p.Author, displayStampUnix(p.CreatedAt), truncateAtRune(body, 200), p.URL)
	}
	return nil
}

func displayStampUnix(ts int64) string {
	return formatHumanTime(ts)
}

func newHiringCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hiring [regex]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Filter the latest 'Ask HN: Who is hiring' thread by regex",
		Long: `Find the latest 'Ask HN: Who is hiring?' thread on HN and filter the
top-level posts with a case-insensitive regex.

Each post is emitted with author, created date, body, and HN URL.`,
		Example: strings.Trim(`
  # Show every post (no filter)
  hackernews-pp-cli hiring

  # Remote Go positions
  hackernews-pp-cli hiring '(remote|REMOTE).*\bGo\b'

  # JSON
  hackernews-pp-cli hiring 'rust' --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHiringCommand(cmd, flags, args, "Who is hiring")
		},
	}
	return cmd
}

func newFreelanceCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "freelance [regex]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Filter the latest 'Freelancer? Seeking freelancer?' thread by regex",
		Example: strings.Trim(`
  hackernews-pp-cli freelance
  hackernews-pp-cli freelance 'react native'
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHiringCommand(cmd, flags, args, "Freelancer")
		},
	}
	return cmd
}
