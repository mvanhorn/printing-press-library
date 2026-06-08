// Copyright 2026 Vincent Lauriat and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/store"
)

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var flagType string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "since <duration-or-timestamp>",
		Short: "See everything that changed in your Notion workspace since a timestamp — new pages, property edits, archived items.",
		Long: `Show all Notion pages and resources edited since a given time.
Reads entirely from the local sync store — no API calls required.

Duration formats: 7d, 24h, 1w, 30m
Timestamp formats: 2024-01-01, 2024-01-01T15:04:05Z

Run 'notion-pp-cli sync' first to populate the local database.

Examples:
  notion-pp-cli since 7d
  notion-pp-cli since 1w --type pages
  notion-pp-cli since 2024-06-01 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			since, err := parseSinceArg(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid timestamp %q: %w\nExpected duration (7d, 24h, 1w) or timestamp (2024-01-01)", args[0], err))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("notion-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'notion-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			sinceStr := since.UTC().Format(time.RFC3339)
			editedPath := "$.last_edited_time"

			var (
				querySQL  string
				queryArgs []any
			)
			if flagType != "" {
				querySQL = fmt.Sprintf(
					`SELECT data FROM resources
					 WHERE resource_type = ?
					 AND json_extract(data, '%s') IS NOT NULL
					 AND json_extract(data, '%s') >= ?
					 ORDER BY json_extract(data, '%s') DESC
					 LIMIT ?`,
					editedPath, editedPath, editedPath,
				)
				queryArgs = []any{flagType, sinceStr, flagLimit}
			} else {
				querySQL = fmt.Sprintf(
					`SELECT data FROM resources
					 WHERE resource_type IN ('pages', 'data_sources', 'databases', 'blocks')
					 AND json_extract(data, '%s') IS NOT NULL
					 AND json_extract(data, '%s') >= ?
					 ORDER BY json_extract(data, '%s') DESC
					 LIMIT ?`,
					editedPath, editedPath, editedPath,
				)
				queryArgs = []any{sinceStr, flagLimit}
			}

			rows, err := db.DB().QueryContext(cmd.Context(), querySQL, queryArgs...)
			if err != nil {
				return fmt.Errorf("since query failed: %w", err)
			}
			defer rows.Close()

			var results []json.RawMessage
			for rows.Next() {
				var data string
				if err := rows.Scan(&data); err != nil {
					return err
				}
				results = append(results, json.RawMessage(data))
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if len(results) == 0 {
				if flags.asJSON {
					return flags.printJSON(cmd, []json.RawMessage{})
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "No changes since %s.\n", since.Format("2006-01-02 15:04:05 UTC"))
				return nil
			}

			prov := localProvenance(db, flagType, "user_requested")
			printProvenance(cmd, len(results), prov)

			raw, err := json.Marshal(results)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().StringVar(&flagType, "type", "", "Filter by resource type (e.g. pages, data_sources)")
	cmd.Flags().IntVar(&flagLimit, "limit", 500, "Maximum results to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/github.com/mvanhorn/printing-press-library/library/productivity/notion/data.db)")
	return cmd
}

// parseSinceArg parses a duration ("7d", "24h", "1w", "30m") or ISO timestamp string.
func parseSinceArg(s string) (time.Time, error) {
	if t, err := parseSinceDuration(s); err == nil {
		return t, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized format")
}
