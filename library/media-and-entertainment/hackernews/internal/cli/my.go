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

type submissionsRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	Points    int    `json:"points"`
	Comments  int    `json:"num_comments"`
	CreatedAt string `json:"created_at"`
}

type submissionsResult struct {
	User        string           `json:"user"`
	Total       int              `json:"total"`
	HighScore   int              `json:"high_score"`
	MedianScore int              `json:"median_score"`
	Buckets     map[string]int   `json:"score_buckets"`
	Best        []submissionsRow `json:"best"`
	Recent      []submissionsRow `json:"recent"`
	HourHisto   map[int]int      `json:"hour_of_day_histogram"`
	BestHour    int              `json:"best_hour_utc"`
}

func newMyCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "my <username>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Track a user's submission history with score buckets, traction rate, and best posting time",
		Long: `Pull a user's recent submissions from Algolia and compute structured stats.

This is read-only — works against any HN username, no auth needed.
Useful for profiling contributors or checking your own posting timing.`,
		Example: strings.Trim(`
  hackernews-pp-cli my pg
  hackernews-pp-cli my dang --limit 200 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			user := args[0]
			if limit <= 0 {
				limit = 50
			}
			if limit > 1000 {
				limit = 1000
			}
			ac := algolia.New(flags.timeout)
			resp, err := ac.Search("", algolia.SearchOpts{
				Tags:        "story,author_" + user,
				ByDate:      true,
				HitsPerPage: limit,
			})
			if err != nil {
				return apiErr(err)
			}

			rows := make([]submissionsRow, 0, len(resp.Hits))
			scores := make([]int, 0, len(resp.Hits))
			hourHist := map[int]int{}
			best := submissionsRow{}
			for _, h := range resp.Hits {
				r := submissionsRow{
					ID:        h.ObjectID,
					Title:     h.Title,
					URL:       h.URL,
					Points:    h.Points,
					Comments:  h.NumComments,
					CreatedAt: h.CreatedAt,
				}
				rows = append(rows, r)
				scores = append(scores, h.Points)
				hour := time.Unix(h.CreatedAtI, 0).UTC().Hour()
				hourHist[hour]++
				if h.Points > best.Points {
					best = r
				}
			}

			buckets := map[string]int{
				"0-1":     0,
				"2-9":     0,
				"10-29":   0,
				"30-99":   0,
				"100-499": 0,
				"500+":    0,
			}
			for _, s := range scores {
				switch {
				case s < 2:
					buckets["0-1"]++
				case s < 10:
					buckets["2-9"]++
				case s < 30:
					buckets["10-29"]++
				case s < 100:
					buckets["30-99"]++
				case s < 500:
					buckets["100-499"]++
				default:
					buckets["500+"]++
				}
			}

			med := median(scores)
			result := submissionsResult{
				User:        user,
				Total:       len(rows),
				HighScore:   best.Points,
				MedianScore: med,
				Buckets:     buckets,
				Best:        []submissionsRow{best},
				HourHisto:   hourHist,
				BestHour:    bestHour(hourHist, scores, resp.Hits),
			}
			// Recent (top 5 by date).
			sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt > rows[j].CreatedAt })
			if len(rows) > 5 {
				result.Recent = rows[:5]
			} else {
				result.Recent = rows
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				j, _ := json.MarshalIndent(result, "", "  ")
				return printOutput(cmd.OutOrStdout(), j, true)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %d recent submissions, median %d, top %d\n", user, result.Total, result.MedianScore, result.HighScore)
			fmt.Fprintln(cmd.OutOrStdout(), "\nScore buckets:")
			for _, k := range []string{"0-1", "2-9", "10-29", "30-99", "100-499", "500+"} {
				fmt.Fprintf(cmd.OutOrStdout(), "  %-9s %d\n", k, buckets[k])
			}
			if best.ID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nBest submission: [%d] %s\n  https://news.ycombinator.com/item?id=%s\n", best.Points, truncateAtRune(best.Title, 80), best.ID)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nBest hour of day to post (UTC, weighted by points): %02d:00\n", result.BestHour)
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max submissions to consider (capped at 1000)")
	return cmd
}

func median(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]int(nil), xs...)
	sort.Ints(cp)
	if len(cp)%2 == 0 {
		return (cp[len(cp)/2-1] + cp[len(cp)/2]) / 2
	}
	return cp[len(cp)/2]
}

// bestHour weighs hour buckets by total points scored in that hour.
// Empty hours never win. If everything is zero-points, it falls back to
// the most-frequently-posted hour.
func bestHour(hist map[int]int, scores []int, hits []algolia.SearchHit) int {
	weighted := map[int]int{}
	for i, h := range hits {
		if i >= len(scores) {
			break
		}
		hour := time.Unix(h.CreatedAtI, 0).UTC().Hour()
		weighted[hour] += scores[i]
	}
	bestH := 0
	bestW := -1
	for h, w := range weighted {
		if w > bestW {
			bestW = w
			bestH = h
		}
	}
	if bestW <= 0 {
		bestF := 0
		bestC := -1
		for h, c := range hist {
			if c > bestC {
				bestC = c
				bestF = h
			}
		}
		return bestF
	}
	return bestH
}

// formatSubmissionDate keeps the YYYY-MM-DD slice from Algolia's
// stringified timestamp; the rest is unhelpful for human display.
func formatSubmissionDate(s string) string {
	if i := strings.Index(s, "T"); i > 0 {
		return s[:i]
	}
	return s
}

var _ = formatSubmissionDate // currently unused; kept for future detail-table rendering
var _ = strconv.Itoa         // strconv used only in error-handling callers; reserved for table extension
