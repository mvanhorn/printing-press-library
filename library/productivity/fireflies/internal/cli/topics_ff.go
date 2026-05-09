// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/fireflies/internal/store"

	"github.com/spf13/cobra"
)

func newTopicsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "topics",
		Short: "Extract and analyze topics from transcripts",
	}
	cmd.AddCommand(newTopicsGetCmd(flags))
	cmd.AddCommand(newTopicsListCmd(flags))
	return cmd
}

func newTopicsGetCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Get topics discussed in a specific transcript",
		Example: `  fireflies-pp-cli topics get abc123 --json`,
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

			topics := []string{}
			source := "topics_discussed"
			if t.Summary != nil {
				topics = t.Summary.Topics
				if len(topics) == 0 && len(t.Summary.Keywords) > 0 {
					topics = t.Summary.Keywords
					source = "keywords"
				}
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), topics, flags)
			}

			label := "Topics"
			if source == "keywords" {
				label = "Keywords (topics_discussed not available)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s in %q:\n\n", label, t.Title)
			for _, topic := range topics {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", topic)
			}
			if len(topics) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No topics or keywords available — run 'transcripts pull <id>' to hydrate.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newTopicsListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var fromDays int
	var topN int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List most frequent topics across all transcripts in a date range",
		Long: `Aggregates topic signals from synced transcript summaries, ranked by frequency.
Uses topics_discussed when available; falls back to summary.keywords (always populated).
Keywords are per-meeting AI extractions — deduplicated case-insensitively across meetings.`,
		Example: strings.Trim(`
  fireflies-pp-cli topics list --from 30
  fireflies-pp-cli topics list --from 90 --top 20 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
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

			// freq maps lowercase key → (canonical display name, count)
			type entry struct {
				display string
				count   int
			}
			freq := map[string]*entry{}
			total := 0
			source := "topics_discussed"

			for rows.Next() {
				var raw []byte
				if err := rows.Scan(&raw); err != nil {
					continue
				}
				var t transcriptRow
				if err := json.Unmarshal(raw, &t); err != nil {
					continue
				}
				if t.Summary == nil {
					continue
				}
				total++

				// Prefer topics_discussed; fall back to keywords
				terms := t.Summary.Topics
				if len(terms) == 0 {
					terms = t.Summary.Keywords
					if len(terms) > 0 {
						source = "keywords"
					}
				}
				for _, term := range terms {
					term = strings.TrimSpace(term)
					if term == "" {
						continue
					}
					key := strings.ToLower(term)
					if e, ok := freq[key]; ok {
						e.count++
					} else {
						freq[key] = &entry{display: term, count: 1}
					}
				}
			}

			type topicEntry struct {
				Topic  string `json:"topic"`
				Count  int    `json:"count"`
				Source string `json:"source"`
			}
			ranked := []topicEntry{}
			for _, e := range freq {
				ranked = append(ranked, topicEntry{Topic: e.display, Count: e.count, Source: source})
			}
			sort.Slice(ranked, func(i, j int) bool {
				return ranked[i].Count > ranked[j].Count
			})
			if topN > 0 && len(ranked) > topN {
				ranked = ranked[:topN]
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), ranked, flags)
			}

			label := "topics"
			if source == "keywords" {
				label = "keywords (topics_discussed not available)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Top %s across %d meetings (last %d days):\n\n", label, total, fromDays)
			for i, r := range ranked {
				fmt.Fprintf(cmd.OutOrStdout(), "  %2d. %-50s  %d meeting(s)\n", i+1, r.Topic, r.Count)
			}
			if len(ranked) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No topics or keywords found in synced summaries.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&fromDays, "from", 30, "Look back N days")
	cmd.Flags().IntVar(&topN, "top", 25, "Number of top topics to show")
	return cmd
}
