package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newRedditQuotesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reddit-quotes <place-name-or-id>",
		Short: "Surface the highest-scored Reddit comment snippets that mention a place — verbatim quotes, no LLM summarization.",
		Long: `Cross-table join: searches goat_reddit_threads body and title for any string
that appears as a name or name_local on a place in goat_places. Returns the
verbatim title + body chunks with subreddit, score, and permalink. No LLM
summarization — agents that need 'real talk' from locals get the raw text
with provenance.`,
		Example: strings.Trim(`
  # Quotes mentioning a known place
  wanderlust-goat-pp-cli reddit-quotes "Kohi Bibi" --json

  # By a Wikipedia-derived name
  wanderlust-goat-pp-cli reddit-quotes "Tsukiji Fish Market"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := strings.Join(args, " ")
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			store, err := openGoatStore(cmd, flags)
			if err != nil {
				return err
			}
			defer store.Close()

			// Look up name_local variants for the input.
			var nameLocal string
			row := store.DB().QueryRowContext(ctx, `SELECT name_local FROM goat_places WHERE name = ? OR name_local = ? LIMIT 1`, name, name)
			_ = row.Scan(&nameLocal)

			needles := []string{name}
			if nameLocal != "" && nameLocal != name {
				needles = append(needles, nameLocal)
			}
			threads, err := store.QuotesForPlace(ctx, needles)
			if err != nil {
				return err
			}
			out := redditQuotesReport{Query: name, NameLocal: nameLocal}
			for _, th := range threads {
				out.Quotes = append(out.Quotes, redditQuote{
					Subreddit: th.Subreddit,
					Title:     th.Title,
					Score:     th.Score,
					Comments:  th.NumComments,
					URL:       th.URL,
					Permalink: "https://www.reddit.com" + th.Permalink,
					Body:      th.Body,
				})
			}
			if len(out.Quotes) == 0 {
				out.Note = "No matching Reddit threads in the local store. Run sync-city <slug> first."
				_ = printJSONFiltered(cmd.OutOrStdout(), out, flags)
				return notFoundErr(fmt.Errorf("no Reddit threads mention %q", name))
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

type redditQuotesReport struct {
	Query     string        `json:"query"`
	NameLocal string        `json:"name_local,omitempty"`
	Quotes    []redditQuote `json:"quotes"`
	Note      string        `json:"note,omitempty"`
}

type redditQuote struct {
	Subreddit string `json:"subreddit"`
	Title     string `json:"title"`
	Score     int    `json:"score"`
	Comments  int    `json:"comments"`
	URL       string `json:"url"`
	Permalink string `json:"permalink"`
	Body      string `json:"body"`
}
