// Copyright 2026 nikica-jokic. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/notion/internal/store"
)

type changedPage struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	LastEditedTime time.Time `json:"last_edited_time"`
	EditedBy      string    `json:"edited_by,omitempty"`
	URL           string    `json:"url,omitempty"`
}

func newChangedCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "changed",
		Short: "Show pages edited since a timestamp or duration",
		Long:  "Show pages edited since a given timestamp or relative duration (e.g. 2h, 24h, 7d). Requires a local sync — run 'notion-pp-cli sync' first.",
		Example: strings.Trim(`
  notion-pp-cli changed --since 2h
  notion-pp-cli changed --since 24h --json
  notion-pp-cli changed --since 2026-05-07T00:00:00Z --json --select id,title,last_edited_time`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[]`)
				return nil
			}

			cutoff, err := parseSinceDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since value %q: use a duration like 2h, 24h, 7d, or an RFC3339 timestamp", since)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("notion-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w\nRun 'notion-pp-cli sync' first to populate the local store.", err)
			}
			defer db.Close()

			query := `
				SELECT id, data
				FROM resources
				WHERE resource_type = 'pages'
				  AND json_extract(data, '$.last_edited_time') >= ?
				ORDER BY json_extract(data, '$.last_edited_time') DESC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, cutoff.Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("querying changed pages: %w", err)
			}
			defer rows.Close()

			var results []changedPage
			for rows.Next() {
				var id string
				var dataRaw []byte
				if err := rows.Scan(&id, &dataRaw); err != nil {
					continue
				}
				var data map[string]any
				if err := json.Unmarshal(dataRaw, &data); err != nil {
					continue
				}

				title := extractPageTitle(data)
				lastEditedStr, _ := data["last_edited_time"].(string)
				lastEdited, _ := time.Parse(time.RFC3339, lastEditedStr)
				if lastEdited.IsZero() {
					lastEdited, _ = time.Parse("2006-01-02T15:04:05.999Z", lastEditedStr)
				}

				editedBy := ""
				if leb, ok := data["last_edited_by"].(map[string]any); ok {
					if name, ok := leb["name"].(string); ok {
						editedBy = name
					} else if id, ok := leb["id"].(string); ok {
						editedBy = id
					}
				}
				url, _ := data["url"].(string)

				results = append(results, changedPage{
					ID:            id,
					Title:         title,
					LastEditedTime: lastEdited,
					EditedBy:      editedBy,
					URL:           url,
				})
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("reading results: %w", err)
			}

			if len(results) == 0 {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No pages changed since %s.\n", cutoff.Format("2006-01-02 15:04"))
				}
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pages changed since %s (%d found):\n\n", cutoff.Format("2006-01-02 15:04"), len(results))
			for _, p := range results {
				by := ""
				if p.EditedBy != "" {
					by = "  by " + p.EditedBy
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %-50s%s\n",
					p.LastEditedTime.Format("2006-01-02 15:04"),
					truncate(p.Title, 50),
					by,
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "24h", "Show pages edited since this duration (e.g. 2h, 7d) or RFC3339 timestamp")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/notion-pp-cli/data.db)")

	return cmd
}

