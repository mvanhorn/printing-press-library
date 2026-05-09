// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

type analyticsSpeaker struct {
	SpeakerID       int     `json:"speaker_id"`
	Name            string  `json:"name"`
	Duration        float64 `json:"duration"`
	WordCount       int     `json:"word_count"`
	LongestMono     float64 `json:"longest_monologue"`
	MonologueCount  int     `json:"monologues_count"`
	FillerWords     int     `json:"filler_words"`
	Questions       int     `json:"questions"`
	DurationPct     float64 `json:"duration_pct"`
	WordsPerMinute  float64 `json:"words_per_minute"`
}

type meetingAnalytics struct {
	Sentiments struct {
		PositivePct float64 `json:"positive_pct"`
		NeutralPct  float64 `json:"neutral_pct"`
		NegativePct float64 `json:"negative_pct"`
	} `json:"sentiments"`
	Speakers []analyticsSpeaker `json:"speakers"`
}

func newSpeakersCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "speakers <id>",
		Short: "Show per-speaker analytics for a transcript",
		Long:  "Shows talk time, word count, filler words, questions, and WPM per speaker. Run 'transcripts pull <id>' first.",
		Example: strings.Trim(`
  fireflies-pp-cli speakers abc123
  fireflies-pp-cli speakers abc123 --json`, "\n"),
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
				return fmt.Errorf("no analytics data — run 'transcripts pull %s' to hydrate", args[0])
			}

			var analytics meetingAnalytics
			if err := json.Unmarshal(t.Analytics, &analytics); err != nil {
				return fmt.Errorf("parsing analytics: %w", err)
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), analytics, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Speakers in %q:\n\n", t.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s  %6s  %7s  %5s  %6s  %5s  %6s\n",
				"SPEAKER", "TALK%", "WORDS", "WPM", "FILLER", "QUEST", "MONO")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 78))
			for _, s := range analytics.Speakers {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s  %5.1f%%  %7d  %5.0f  %6d  %5d  %6.0fs\n",
					truncate(s.Name, 25),
					s.DurationPct,
					s.WordCount,
					s.WordsPerMinute,
					s.FillerWords,
					s.Questions,
					s.LongestMono,
				)
			}
			if len(analytics.Speakers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No speaker data available.")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nSentiment: %.0f%% positive / %.0f%% neutral / %.0f%% negative\n",
				analytics.Sentiments.PositivePct,
				analytics.Sentiments.NeutralPct,
				analytics.Sentiments.NegativePct,
			)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
