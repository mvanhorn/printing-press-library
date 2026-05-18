// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/techmeme/internal/store"
	"github.com/spf13/cobra"
)

func newTrackCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Track topics and get alerts when they hit Techmeme",
		Long: `Save topics to monitor and check when they appear in Techmeme headlines.
Persistent monitoring without browser tabs.`,
		Example: `  # Add a topic to track
  techmeme-pp-cli track add "artificial intelligence"

  # Remove a tracked topic
  techmeme-pp-cli track remove "artificial intelligence"

  # List all tracked topics
  techmeme-pp-cli track list

  # Check for matches since last check
  techmeme-pp-cli track check`,
	}

	addCmd := &cobra.Command{
		Use:   "add <topic>",
		Short: "Add a topic to track",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			topic := strings.Join(args, " ")

			if dbPath == "" {
				dbPath = defaultDBPath("techmeme-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := db.AddTrackedTopic(topic); err != nil {
				return fmt.Errorf("adding topic: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]string{
					"status": "added",
					"topic":  topic,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Now tracking: %s\n", topic)
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:     "remove <topic>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove a tracked topic",
		Example: `  # Stop tracking a topic
  techmeme-pp-cli track remove "artificial intelligence"

  # Using alias
  techmeme-pp-cli track rm "Apple"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			topic := strings.Join(args, " ")

			if dbPath == "" {
				dbPath = defaultDBPath("techmeme-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if err := db.RemoveTrackedTopic(topic); err != nil {
				return fmt.Errorf("removing topic: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]string{
					"status": "removed",
					"topic":  topic,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stopped tracking: %s\n", topic)
			return nil
		},
	}

	listCmd := &cobra.Command{
		Use:         "list",
		Short:       "List all tracked topics",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("techmeme-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			topics, err := db.ListTrackedTopics()
			if err != nil {
				return fmt.Errorf("listing topics: %w", err)
			}

			if len(topics) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tracked topics. Use 'techmeme-pp-cli track add <topic>' to start.")
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), topics, flags)
			}

			headers := []string{"TOPIC", "ADDED", "LAST CHECKED"}
			rows := make([][]string, 0, len(topics))
			for _, t := range topics {
				lastChecked := "never"
				if t.LastCheckedAt != nil {
					lastChecked = t.LastCheckedAt.Local().Format("2006-01-02 15:04")
				}
				rows = append(rows, []string{
					t.Topic,
					t.AddedAt.Local().Format("2006-01-02 15:04"),
					lastChecked,
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Check cached headlines for tracked topics",
		Long:  "Search cached headlines for each tracked topic and show matches since the last check.",
		Example: `  # Check for new matches on all tracked topics
  techmeme-pp-cli track check

  # Check and output as JSON for agents
  techmeme-pp-cli track check --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("techmeme-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			topics, err := db.ListTrackedTopics()
			if err != nil {
				return fmt.Errorf("listing topics: %w", err)
			}

			if len(topics) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No tracked topics. Use 'techmeme-pp-cli track add <topic>' to start.")
				return nil
			}

			type topicMatch struct {
				Topic   string            `json:"topic"`
				Matches []json.RawMessage `json:"matches"`
			}

			var allMatches []topicMatch
			for _, t := range topics {
				since := time.Now().Add(-24 * time.Hour)
				if t.LastCheckedAt != nil {
					since = *t.LastCheckedAt
				}

				matches, err := db.SearchHeadlines(t.Topic, since)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: search for %q failed: %v\n", t.Topic, err)
					continue
				}

				if len(matches) > 0 {
					allMatches = append(allMatches, topicMatch{
						Topic:   t.Topic,
						Matches: matches,
					})
				}

				_ = db.UpdateTopicLastChecked(t.Topic)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), allMatches, flags)
			}

			if len(allMatches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No new matches for tracked topics.")
				return nil
			}

			for _, tm := range allMatches {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%d matches):\n", bold(tm.Topic), len(tm.Matches))
				for _, m := range tm.Matches {
					var obj map[string]any
					if json.Unmarshal(m, &obj) != nil {
						continue
					}
					title := ""
					if v, ok := obj["title"].(string); ok {
						title = v
					} else if v, ok := obj["headline"].(string); ok {
						title = v
					}
					source := ""
					if v, ok := obj["source"].(string); ok {
						source = v
					} else if v, ok := obj["author"].(string); ok {
						source = v
					}
					if title != "" {
						if source != "" {
							fmt.Fprintf(cmd.OutOrStdout(), "  - [%s] %s\n", source, title)
						} else {
							fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", title)
						}
					}
				}
			}
			return nil
		},
	}

	cmd.PersistentFlags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/techmeme-pp-cli/data.db)")

	cmd.AddCommand(addCmd)
	cmd.AddCommand(removeCmd)
	cmd.AddCommand(listCmd)
	cmd.AddCommand(checkCmd)

	return cmd
}
