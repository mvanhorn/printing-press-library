package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/zylos/internal/store"
)

func newTimelineCmd(flags *rootFlags) *cobra.Command {
	var today bool
	var days int
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Chronological timeline of conversations with gap detection",
		Example: strings.Trim(`
  zylos-pp-cli timeline
  zylos-pp-cli timeline --today
  zylos-pp-cli timeline --days 3 --limit 50 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("zylos-pp-cli")
			}
			db, err := store.OpenReadOnly(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zylos-pp-cli sync' first.", err)
			}
			defer db.Close()

			if limit <= 0 {
				limit = 200
			}

			query := `SELECT data FROM resources
			  WHERE resource_type = 'conversations'`

			var since time.Time
			if today {
				now := time.Now()
				since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
				query += fmt.Sprintf(` AND json_extract(data, '$.timestamp') >= '%s'`, since.Format(time.RFC3339))
			} else if days > 0 {
				since = time.Now().AddDate(0, 0, -days)
				query += fmt.Sprintf(` AND json_extract(data, '$.timestamp') >= '%s'`, since.Format(time.RFC3339))
			}

			query += ` ORDER BY json_extract(data, '$.timestamp') ASC`

			if limit > 0 {
				query += fmt.Sprintf(` LIMIT %d`, limit)
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("querying timeline: %w", err)
			}
			defer rows.Close()

			type message struct {
				Direction string `json:"direction"`
				Content   string `json:"content"`
				Timestamp string `json:"timestamp"`
			}

			type timelineEntry struct {
				Timestamp      string  `json:"timestamp"`
				Direction      string  `json:"direction"`
				ContentPreview string  `json:"content_preview"`
				GapBefore      float64 `json:"gap_before_seconds,omitempty"`
			}

			entries := make([]timelineEntry, 0)
			var prevTimestamp *time.Time

			for rows.Next() {
				var dataStr string
				if err := rows.Scan(&dataStr); err != nil {
					continue
				}
				var msg message
				if err := json.Unmarshal([]byte(dataStr), &msg); err != nil {
					continue
				}

				entry := timelineEntry{
					Timestamp:      msg.Timestamp,
					Direction:      msg.Direction,
					ContentPreview: truncate(msg.Content, 100),
				}

				if prevTimestamp != nil {
					ts, err := time.Parse(time.RFC3339, msg.Timestamp)
					if err == nil {
						gap := ts.Sub(*prevTimestamp).Seconds()
						if gap > 300 { // > 5 minutes
							entry.GapBefore = gap
						}
					}
				}

				ts, err := time.Parse(time.RFC3339, msg.Timestamp)
				if err == nil {
					prevTimestamp = &ts
				}

				entries = append(entries, entry)
			}

			return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
		},
	}

	cmd.Flags().BoolVar(&today, "today", false, "Show only today's messages")
	cmd.Flags().IntVar(&days, "days", 7, "Show messages from last N days")
	cmd.Flags().IntVar(&limit, "limit", 200, "Maximum number of timeline entries")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
