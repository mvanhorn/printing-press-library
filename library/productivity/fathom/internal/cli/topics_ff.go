package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fathom/internal/store"
	"github.com/spf13/cobra"
)

func newTopicsCmd(flags *rootFlags) *cobra.Command {
	var terms string
	var weeks int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "topics",
		Short: "Topic frequency tracker — see how often keywords appear in meeting transcripts over time",
		Long: `Search all synced transcripts and summaries for specified keywords and
show how often each term appears, week by week. Useful for identifying
recurring themes across customer calls, team meetings, or any topic over time.

Run 'sync --full' first to populate transcripts locally.`,
		Example: strings.Trim(`
  fathom-pp-cli topics --terms pricing
  fathom-pp-cli topics --terms "pricing,onboarding,churn" --weeks 12
  fathom-pp-cli topics --terms roadmap --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if terms == "" && len(args) > 0 {
				terms = args[0]
			}
			if terms == "" {
				return cmd.Help()
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fathom-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			termList := strings.Split(terms, ",")
			for i, t := range termList {
				termList[i] = strings.TrimSpace(t)
			}

			meetings, err := loadAllMeetings(cmd.Context(), db)
			if err != nil {
				return err
			}

			cutoffWeeks := weeks
			if cutoffWeeks <= 0 {
				cutoffWeeks = 52
			}
			cutoff, _ := parseSince(fmt.Sprintf("%dd", cutoffWeeks*7))

			type weekCount map[string]int // term -> count
			weekData := map[string]weekCount{} // week -> term counts
			totalCounts := map[string]int{}
			meetingCounts := map[string]int{} // term -> meetings mentioning it

			for _, m := range meetings {
				if !cutoff.IsZero() {
					t, err := parseFlexTime(m.CreatedAt)
					if err != nil || t.Before(cutoff) {
						continue
					}
				}
				mt, err := parseFlexTime(m.CreatedAt)
				if err != nil {
					continue
				}
				week := isoWeek(mt)

				// Build full text to search (transcript + summary)
				var sb strings.Builder
				for _, seg := range m.Transcript {
					sb.WriteString(seg.Text)
					sb.WriteString(" ")
				}
				if m.DefaultSummary != nil && m.DefaultSummary.MarkdownFormatted != nil {
					sb.WriteString(*m.DefaultSummary.MarkdownFormatted)
				}
				text := strings.ToLower(sb.String())

				if _, ok := weekData[week]; !ok {
					weekData[week] = weekCount{}
				}
				for _, term := range termList {
					lc := strings.ToLower(term)
					count := strings.Count(text, lc)
					if count > 0 {
						weekData[week][term] += count
						totalCounts[term] += count
						meetingCounts[term]++
					}
				}
			}

			// Build sorted weeks list
			weekKeys := make([]string, 0, len(weekData))
			for w := range weekData {
				weekKeys = append(weekKeys, w)
			}
			sort.Strings(weekKeys)

			type termWeek struct {
				Week   string `json:"week"`
				Term   string `json:"term"`
				Count  int    `json:"count"`
			}
			type topicResult struct {
				Term          string     `json:"term"`
				TotalCount    int        `json:"total_count"`
				MeetingCount  int        `json:"meeting_count"`
				WeeklyTrend   []termWeek `json:"weekly_trend"`
			}

			var results []topicResult
			for _, term := range termList {
				var trend []termWeek
				for _, w := range weekKeys {
					count := weekData[w][term]
					if count > 0 {
						trend = append(trend, termWeek{Week: w, Term: term, Count: count})
					}
				}
				results = append(results, topicResult{
					Term:         term,
					TotalCount:   totalCounts[term],
					MeetingCount: meetingCounts[term],
					WeeklyTrend:  trend,
				})
			}

			// Sort by total count desc
			sort.Slice(results, func(i, j int) bool {
				return results[i].TotalCount > results[j].TotalCount
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			// Human output
			fmt.Fprintf(cmd.OutOrStdout(), "Topic frequency (last %d weeks)\n\n", cutoffWeeks)
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s  %d mentions across %d meetings\n", r.Term, r.TotalCount, r.MeetingCount)
				for _, w := range r.WeeklyTrend {
					// PATCH(topics-bar-utf8): cap by rune count, not byte length, to avoid mid-codepoint truncation on █ (3 bytes)
					barCount := w.Count
					if barCount > 40 {
						barCount = 40
					}
					bar := strings.Repeat("█", barCount)
					if w.Count > 40 {
						bar += "…"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s (%d)\n", w.Week, bar, w.Count)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&terms, "terms", "", "Comma-separated keywords to track (e.g. pricing,onboarding)")
	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of weeks to look back")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
