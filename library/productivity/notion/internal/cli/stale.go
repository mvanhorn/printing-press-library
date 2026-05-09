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

type stalePage struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	LastEditedTime time.Time `json:"last_edited_time"`
	DaysSinceEdit  int       `json:"days_since_edit"`
	URL           string    `json:"url,omitempty"`
}

func newStaleCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string
	var resourceType string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List pages not edited in N days",
		Long:  "List pages (and optionally database records) that haven't been edited in N days. Requires a local sync — run 'notion-pp-cli sync' first.",
		Example: strings.Trim(`
  notion-pp-cli stale --days 30
  notion-pp-cli stale --days 7 --json
  notion-pp-cli stale --days 14 --json --select id,title,days_since_edit`, "\n"),
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

			if dbPath == "" {
				dbPath = defaultDBPath("notion-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local store: %w\nRun 'notion-pp-cli sync' first to populate the local store.", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -days)

			query := `
				SELECT id, data
				FROM resources
				WHERE resource_type = ?
				  AND json_extract(data, '$.last_edited_time') < ?
				  AND (json_extract(data, '$.in_trash') = 0 OR json_extract(data, '$.in_trash') IS NULL)
				  AND (json_extract(data, '$.archived') = 0 OR json_extract(data, '$.archived') IS NULL)
				ORDER BY json_extract(data, '$.last_edited_time') ASC`

			rows, err := db.DB().QueryContext(cmd.Context(), query, resourceType, cutoff.Format(time.RFC3339))
			if err != nil {
				return fmt.Errorf("querying stale pages: %w", err)
			}
			defer rows.Close()

			var results []stalePage
			now := time.Now()
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
				lastEdited, err := time.Parse(time.RFC3339, lastEditedStr)
				if err != nil {
					lastEdited, _ = time.Parse("2006-01-02T15:04:05.999Z", lastEditedStr)
				}
				daysSince := int(now.Sub(lastEdited).Hours() / 24)

				url, _ := data["url"].(string)
				results = append(results, stalePage{
					ID:            id,
					Title:         title,
					LastEditedTime: lastEdited,
					DaysSinceEdit:  daysSince,
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
					fmt.Fprintf(cmd.OutOrStdout(), "No pages stale for more than %d days.\n", days)
				}
				return nil
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Pages not edited in %d+ days (%d found):\n\n", days, len(results))
			for _, p := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d days  %-50s  %s\n",
					p.DaysSinceEdit,
					truncate(p.Title, 50),
					p.LastEditedTime.Format("2006-01-02"),
				)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&days, "days", 30, "Flag pages not edited for this many days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/notion-pp-cli/data.db)")
	cmd.Flags().StringVar(&resourceType, "type", "pages", "Resource type to check: pages")

	return cmd
}

// extractPageTitle extracts the plain-text title from a Notion page object.
func extractPageTitle(data map[string]any) string {
	// Try properties.title (database page)
	if props, ok := data["properties"].(map[string]any); ok {
		for _, v := range props {
			prop, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if prop["type"] == "title" {
				if titleArr, ok := prop["title"].([]any); ok && len(titleArr) > 0 {
					if rt, ok := titleArr[0].(map[string]any); ok {
						if pt, ok := rt["plain_text"].(string); ok && pt != "" {
							return pt
						}
					}
				}
			}
		}
	}
	// Try child_page.title (nested page block)
	if cp, ok := data["child_page"].(map[string]any); ok {
		if t, ok := cp["title"].(string); ok && t != "" {
			return t
		}
	}
	// Fallback
	if id, ok := data["id"].(string); ok {
		return id
	}
	return "(untitled)"
}

