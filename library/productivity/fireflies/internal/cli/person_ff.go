// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

func newPersonCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "person",
		Short: "Person-centric views across all meetings",
	}
	cmd.AddCommand(newPersonTimelineCmd(flags))
	cmd.AddCommand(newPersonComplaintsCmd(flags))
	return cmd
}

func newPersonTimelineCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var fromDays int

	cmd := &cobra.Command{
		Use:   "timeline <email-or-name>",
		Short: "Chronological meeting history with a specific person",
		Long: `Shows every meeting where the given person (by email or name) appeared,
with per-meeting: topics, action items, and speaker talk ratio if available.

Use an email address for reliable matching — meeting titles don't always
contain participant names.`,
		Example: strings.Trim(`
  fireflies-pp-cli person timeline danijel.latin@verybigthings.com
  fireflies-pp-cli person timeline "Jane Smith" --from 90
  fireflies-pp-cli person timeline client@company.com --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w\nRun 'fireflies-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := `SELECT data FROM transcripts ORDER BY CAST(date AS REAL) ASC`
			args2 := []any{}
			if fromDays > 0 {
				cutoff := time.Now().AddDate(0, 0, -fromDays).UnixMilli()
				query = `SELECT data FROM transcripts WHERE CAST(date AS REAL) >= ? ORDER BY CAST(date AS REAL) ASC`
				args2 = append(args2, cutoff)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return fmt.Errorf("querying: %w", err)
			}
			defer rows.Close()

			search := strings.ToLower(args[0])
			isEmail := strings.Contains(search, "@")

			type timelineEntry struct {
				ID          string   `json:"id"`
				Title       string   `json:"title"`
				Date        string   `json:"date"`
				Status      string   `json:"status"`
				Topics      []string `json:"topics,omitempty"`
				ActionItems string   `json:"action_items,omitempty"`
				TalkRatioPct float64 `json:"talk_ratio_pct,omitempty"`
			}
			var timeline []timelineEntry

			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}

				matched := false
				if isEmail {
					matched = t.hasParticipant(search)
				} else {
					// Name-based match: check speakers and participant emails
					for _, s := range t.Speakers {
						if strings.Contains(strings.ToLower(s.Name), search) {
							matched = true
							break
						}
					}
					if !matched {
						for _, p := range t.Participants {
							if strings.Contains(strings.ToLower(p), search) {
								matched = true
								break
							}
						}
					}
				}
				if !matched {
					continue
				}

				entry := timelineEntry{
					ID:     t.ID,
					Title:  t.Title,
					Date:   t.dateFormatted(),
					Status: t.summaryStatus(),
				}
				if t.Summary != nil {
					entry.Topics = t.Summary.Topics
					entry.ActionItems = t.Summary.ActionItems
				}

				// Find their talk ratio from analytics if available
				if t.Analytics != nil {
					var analytics meetingAnalytics
					if json.Unmarshal(t.Analytics, &analytics) == nil {
						for _, s := range analytics.Speakers {
							if strings.Contains(strings.ToLower(s.Name), search) {
								entry.TalkRatioPct = s.DurationPct
								break
							}
						}
					}
				}

				timeline = append(timeline, entry)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading rows: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), timeline, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Meeting history with %q (%d meetings):\n\n", args[0], len(timeline))
			for _, entry := range timeline {
				fmt.Fprintf(cmd.OutOrStdout(), "── %s  %s  [%s]\n", entry.Date, entry.Title, entry.Status)
				if len(entry.Topics) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "   Topics: %s\n", strings.Join(entry.Topics, ", "))
				}
				if entry.TalkRatioPct > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "   Talk:   %.1f%%\n", entry.TalkRatioPct)
				}
				if entry.ActionItems != "" {
					lines := strings.Split(strings.TrimSpace(entry.ActionItems), "\n")
					if len(lines) > 0 {
						fmt.Fprintf(cmd.OutOrStdout(), "   Actions: %s\n", lines[0])
					}
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
			if len(timeline) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No meetings found with %q.\n", args[0])
				if !isEmail {
					fmt.Fprintln(cmd.OutOrStdout(), "Tip: use an email address for more reliable matching.")
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&fromDays, "from", 0, "Look back N days (0 = all time)")
	return cmd
}

func newPersonComplaintsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var fromDays int

	cmd := &cobra.Command{
		Use:   "complaints <email-or-name>",
		Short: "Extract negative-sentiment sentences from a person across all meetings",
		Example: strings.Trim(`
  fireflies-pp-cli person complaints "Acme" --from 90
  fireflies-pp-cli person complaints client@company.com --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("fireflies-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer db.Close()

			query := `SELECT data FROM transcripts ORDER BY CAST(date AS REAL) DESC`
			args2 := []any{}
			if fromDays > 0 {
				cutoff := time.Now().AddDate(0, 0, -fromDays).UnixMilli()
				query = `SELECT data FROM transcripts WHERE CAST(date AS REAL) >= ? ORDER BY CAST(date AS REAL) DESC`
				args2 = append(args2, cutoff)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, args2...)
			if err != nil {
				return fmt.Errorf("querying: %w", err)
			}
			defer rows.Close()

			search := strings.ToLower(args[0])

			type complaint struct {
				TranscriptTitle string `json:"transcript_title"`
				Date            string `json:"date"`
				Speaker         string `json:"speaker"`
				Text            string `json:"text"`
				Time            string `json:"time"`
			}
			results := []complaint{}

			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				if t.Sentences == nil {
					continue
				}

				var sentences []struct {
					Text        string `json:"text"`
					SpeakerName string `json:"speaker_name"`
					StartTime   string `json:"start_time"`
					AIFilters   *struct {
						Sentiment string `json:"sentiment"`
					} `json:"ai_filters"`
				}
				if json.Unmarshal(t.Sentences, &sentences) != nil {
					continue
				}

				for _, s := range sentences {
					if s.AIFilters == nil || strings.ToLower(s.AIFilters.Sentiment) != "negative" {
						continue
					}
					// Match speaker name or any participant
					if !strings.Contains(strings.ToLower(s.SpeakerName), search) &&
						!strings.Contains(strings.ToLower(t.Title), search) &&
						!t.hasParticipant(search) {
						continue
					}
					results = append(results, complaint{
						TranscriptTitle: t.Title,
						Date:            t.dateFormatted(),
						Speaker:         s.SpeakerName,
						Text:            s.Text,
						Time:            s.StartTime,
					})
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Negative-sentiment mentions of %q (%d found):\n\n", args[0], len(results))
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s (%s)\n    %s\n\n",
					r.Date, r.TranscriptTitle, r.Speaker, r.Text)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No negative-sentiment mentions found.")
				fmt.Fprintln(cmd.OutOrStdout(), "Hint: requires sentence-level data. Run 'transcripts pull <id>' on relevant meetings first.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&fromDays, "from", 90, "Look back N days")
	return cmd
}
