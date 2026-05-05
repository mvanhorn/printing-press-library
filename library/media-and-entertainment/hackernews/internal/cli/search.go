package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/algolia"
)

// hnSearchCmd is a hand-built replacement for the generated search.
// We register it as `algolia-search` so we don't collide with the
// generated `search` command (which uses local FTS5). The generated
// search remains the local-FTS path; this command hits Algolia live.
//
// We register it as `live-search` in root.go to keep names obvious.
func newSearchCmd(flags *rootFlags) *cobra.Command {
	var tag string
	var since string
	var until string
	var minPoints int
	var minComments int
	var byDate bool
	var page int
	var hitsPerPage int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Hacker News stories and comments live via the Algolia API",
		Long: `Run a live search against the Hacker News Algolia index.

This is the live counterpart to 'search' (which uses local FTS5).
Use it for the full HN history; Algolia keeps everything since
2006. Tag and numeric filters are passed straight through.`,
		Example: strings.Trim(`
  # Relevance-sorted search
  hackernews-pp-cli search "rust async"

  # Stories only, last 7 days, sorted by date
  hackernews-pp-cli search openai --tag story --since 7d --by-date

  # Top-scoring submissions for a query
  hackernews-pp-cli search "kubernetes" --min-points 100 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := args[0]
			ac := algolia.New(flags.timeout)

			var numerics []string
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since: %w", err))
				}
				numerics = append(numerics, "created_at_i>"+strconv.FormatInt(ts.Unix(), 10))
			}
			if until != "" {
				ts, err := parseSinceDuration(until)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --until: %w", err))
				}
				numerics = append(numerics, "created_at_i<"+strconv.FormatInt(ts.Unix(), 10))
			}
			if minPoints > 0 {
				numerics = append(numerics, "points>"+strconv.Itoa(minPoints))
			}
			if minComments > 0 {
				numerics = append(numerics, "num_comments>"+strconv.Itoa(minComments))
			}

			if hitsPerPage <= 0 {
				hitsPerPage = 20
			}
			if hitsPerPage > 100 {
				hitsPerPage = 100
			}

			resp, err := ac.Search(query, algolia.SearchOpts{
				Tags:           tag,
				NumericFilters: strings.Join(numerics, ","),
				HitsPerPage:    hitsPerPage,
				Page:           page,
				ByDate:         byDate,
			})
			if err != nil {
				return apiErr(err)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out, _ := json.MarshalIndent(resp, "", "  ")
				return printOutput(cmd.OutOrStdout(), out, true)
			}
			// Human-friendly table.
			rows := make([][]string, 0, len(resp.Hits))
			for _, h := range resp.Hits {
				title := h.Title
				if title == "" {
					title = truncateAtRune(stripHTML(h.CommentText), 60)
				}
				url := h.URL
				if url == "" && h.StoryURL != "" {
					url = h.StoryURL
				}
				rows = append(rows, []string{
					strconv.Itoa(h.Points),
					strconv.Itoa(h.NumComments),
					h.Author,
					formatAlgoliaTime(h.CreatedAtI),
					truncateAtRune(title, 70),
					url,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no results")
				return nil
			}
			if err := flags.printTable(cmd, []string{"PTS", "CMTS", "BY", "AGE", "TITLE", "URL"}, rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d hits (showing %d) — page %d/%d\n", resp.NbHits, len(resp.Hits), resp.Page+1, resp.NbPages)
			return nil
		},
	}
	cmd.Flags().StringVar(&tag, "tag", "", "Algolia tag filter: story, comment, ask_hn, show_hn, job, poll, author_<name>")
	cmd.Flags().StringVar(&since, "since", "", "Only include items newer than this duration (e.g., 7d, 24h)")
	cmd.Flags().StringVar(&until, "until", "", "Only include items older than this duration ago")
	cmd.Flags().IntVar(&minPoints, "min-points", 0, "Only include stories with at least this many points")
	cmd.Flags().IntVar(&minComments, "min-comments", 0, "Only include items with at least this many comments")
	cmd.Flags().BoolVar(&byDate, "by-date", false, "Sort by date (newest first) instead of relevance")
	cmd.Flags().IntVar(&page, "page", 0, "Page number (0-indexed)")
	cmd.Flags().IntVar(&hitsPerPage, "hits-per-page", 20, "Results per page (max 100)")
	return cmd
}

func formatAlgoliaTime(ts int64) string {
	if ts <= 0 {
		return "?"
	}
	return relativeTimeFromUnix(ts)
}

func truncateAtRune(s string, n int) string {
	if n <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// helper used by other algolia commands
func formatHumanTime(ts int64) string {
	if ts <= 0 {
		return "—"
	}
	t := time.Unix(ts, 0).UTC()
	return t.Format("2006-01-02")
}
