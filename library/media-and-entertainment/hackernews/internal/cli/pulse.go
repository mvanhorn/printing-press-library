package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/hackernews/internal/algolia"
)

// pulse aggregates Algolia hits per day to give a velocity view of a
// topic. We pull stories AND comments mentioning the topic in the
// configured window, bucket by UTC date, then return mentions and
// summed points per bucket.

type pulseDay struct {
	Date     string `json:"date"`
	Mentions int    `json:"mentions"`
	Points   int    `json:"total_points"`
}

type pulseResult struct {
	Topic      string     `json:"topic"`
	WindowDays int        `json:"window_days"`
	TotalHits  int        `json:"total_hits"`
	TopStories []topStory `json:"top_stories"`
	Days       []pulseDay `json:"days"`
}

type topStory struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	URL    string `json:"url"`
	Points int    `json:"points"`
	Author string `json:"author"`
}

func newPulseCmd(flags *rootFlags) *cobra.Command {
	var days int
	var hitsPerPage int

	cmd := &cobra.Command{
		Use:   "pulse <topic>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Show what HN is saying about a topic this week — score, comment, frequency by day",
		Long: `Run an Algolia query for a topic, scoped to the last N days, and bucket
hits by UTC date.

Output shows mentions per day, summed points, and the top 5 stories
(by points) in the window. Useful for tracking topic velocity without
clicking through paginated search results.`,
		Example: strings.Trim(`
  hackernews-pp-cli pulse rust --days 7
  hackernews-pp-cli pulse openai --days 30 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			topic := args[0]
			if days <= 0 {
				days = 7
			}
			cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour).Unix()

			ac := algolia.New(flags.timeout)
			resp, err := ac.Search(topic, algolia.SearchOpts{
				Tags:           "story",
				NumericFilters: "created_at_i>" + strconv.FormatInt(cutoff, 10),
				HitsPerPage:    hitsPerPage,
				ByDate:         true,
			})
			if err != nil {
				return apiErr(err)
			}

			byDay := map[string]*pulseDay{}
			top := []topStory{}
			topicLower := strings.ToLower(topic)
			for _, h := range resp.Hits {
				date := time.Unix(h.CreatedAtI, 0).UTC().Format("2006-01-02")
				if _, ok := byDay[date]; !ok {
					byDay[date] = &pulseDay{Date: date}
				}
				byDay[date].Mentions++
				byDay[date].Points += h.Points
				// Algolia returns story records that contain the
				// query *anywhere* — including in story_text that's
				// often a tangentially-related comment quote. For the
				// "top stories" list we want only stories whose title
				// or URL actually contains the topic, otherwise an
				// agent gets misleading results (e.g. searching for
				// "rust" surfaces a Mercor breach story because the
				// term appears in the body).
				if strings.Contains(strings.ToLower(h.Title), topicLower) ||
					strings.Contains(strings.ToLower(h.URL), topicLower) {
					top = append(top, topStory{
						ID: h.ObjectID, Title: h.Title, URL: h.URL,
						Points: h.Points, Author: h.Author,
					})
				}
			}
			// Top 5 by points.
			sort.Slice(top, func(i, j int) bool { return top[i].Points > top[j].Points })
			if len(top) > 5 {
				top = top[:5]
			}

			// Sorted day list.
			out := pulseResult{
				Topic:      topic,
				WindowDays: days,
				TotalHits:  resp.NbHits,
				TopStories: top,
			}
			for _, d := range byDay {
				out.Days = append(out.Days, *d)
			}
			sort.Slice(out.Days, func(i, j int) bool { return out.Days[i].Date < out.Days[j].Date })

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(out, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%q over the last %d days — %d total hits\n\n", topic, days, resp.NbHits)
			rows := make([][]string, 0, len(out.Days))
			for _, d := range out.Days {
				rows = append(rows, []string{d.Date, strconv.Itoa(d.Mentions), strconv.Itoa(d.Points)})
			}
			if err := flags.printTable(cmd, []string{"DATE", "MENTIONS", "POINTS"}, rows); err != nil {
				return err
			}
			if len(out.TopStories) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nTop stories in window:")
				for _, t := range out.TopStories {
					url := t.URL
					if url == "" {
						url = "https://news.ycombinator.com/item?id=" + t.ID
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  [%d] %s — %s\n", t.Points, truncateAtRune(t.Title, 70), url)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 7, "Window in days (1-180)")
	cmd.Flags().IntVar(&hitsPerPage, "hits-per-page", 100, "Max hits to fetch (capped at 1000 by Algolia)")
	return cmd
}
