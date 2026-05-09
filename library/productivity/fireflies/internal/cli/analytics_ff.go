// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/gql"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

const analyticsQuery = `
query Analytics($start_time: String, $end_time: String) {
  analytics(start_time: $start_time, end_time: $end_time) {
    team_meetings
    team_hours
    avg_duration
    users {
      user_id name email num_meetings hours
      words_per_minute questions filler_words longest_monologue talk_ratio
    }
  }
}`

func newAnalyticsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Query meeting analytics",
	}
	cmd.AddCommand(newAnalyticsTeamCmd(flags))
	cmd.AddCommand(newAnalyticsMeetingCmd(flags))
	return cmd
}

func newAnalyticsTeamCmd(flags *rootFlags) *cobra.Command {
	var from string
	var to string

	cmd := &cobra.Command{
		Use:   "team",
		Short: "Get team-wide analytics (requires Business+ plan)",
		Example: strings.Trim(`
  fireflies-pp-cli analytics team
  fireflies-pp-cli analytics team --from 2026-01-01 --to 2026-03-31 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{"team_meetings":0}`)
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return err
			}
			client, err := gql.New(cfg)
			if err != nil {
				return err
			}
			vars := map[string]any{}
			if from != "" {
				vars["start_time"] = from
			}
			if to != "" {
				vars["end_time"] = to
			}
			data, err := client.Query(cmd.Context(), analyticsQuery, vars, "analytics")
			if err != nil {
				return fmt.Errorf("fetching analytics: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), data, flags)
			}

			var result struct {
				TeamMeetings int     `json:"team_meetings"`
				TeamHours    float64 `json:"team_hours"`
				AvgDuration  float64 `json:"avg_duration"`
				Users        []struct {
					Name        string  `json:"name"`
					Email       string  `json:"email"`
					NumMeetings int     `json:"num_meetings"`
					Hours       float64 `json:"hours"`
					WPM         float64 `json:"words_per_minute"`
					Questions   int     `json:"questions"`
					FillerWords int     `json:"filler_words"`
					TalkRatio   float64 `json:"talk_ratio"`
				} `json:"users"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parsing: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Team: %d meetings / %.1fh total / %.0f min avg\n\n",
				result.TeamMeetings, result.TeamHours, result.AvgDuration)
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s  %8s  %5s  %6s  %6s  %6s\n",
				"USER", "MEETINGS", "HRS", "WPM", "QUEST", "FILLER")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 65))
			for _, u := range result.Users {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s  %8d  %5.1f  %6.0f  %6d  %6d\n",
					truncate(u.Name, 25),
					u.NumMeetings,
					u.Hours,
					u.WPM,
					u.Questions,
					u.FillerWords,
				)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Start date (ISO 8601, e.g. 2026-01-01)")
	cmd.Flags().StringVar(&to, "to", "", "End date (ISO 8601)")
	return cmd
}

func newAnalyticsMeetingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "meeting <id>",
		Short:   "Get per-meeting speaker analytics from local store",
		Example: `  fireflies-pp-cli analytics meeting abc123 --json`,
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

			raw, err := db.Get("transcripts", args[0])
			if err != nil {
				return fmt.Errorf("transcript not found — run 'transcripts pull %s' first", args[0])
			}
			var t transcriptRow
			if err := json.Unmarshal(raw, &t); err != nil {
				return fmt.Errorf("parsing: %w", err)
			}
			if t.Analytics == nil {
				return fmt.Errorf("no analytics — run 'transcripts pull %s' to hydrate", args[0])
			}
			var analytics meetingAnalytics
			if err := json.Unmarshal(t.Analytics, &analytics); err != nil {
				return fmt.Errorf("parsing analytics: %w", err)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), analytics, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Speaker analytics for %q:\n\n", t.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s  %6s  %7s  %5s  %6s  %5s\n",
				"SPEAKER", "TALK%", "WORDS", "WPM", "FILLER", "QUEST")
			for _, s := range analytics.Speakers {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s  %5.1f%%  %7d  %5.0f  %6d  %5d\n",
					truncate(s.Name, 25), s.DurationPct, s.WordCount,
					s.WordsPerMinute, s.FillerWords, s.Questions)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
