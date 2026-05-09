package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/zylos/internal/store"
)

type exportMessage struct {
	ID        int    `json:"id"`
	Direction string `json:"direction"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func newExportCmd(flags *rootFlags) *cobra.Command {
	var format string
	var output string
	var today bool
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export conversations to JSON or Markdown",
		Example: strings.Trim(`
  zylos-pp-cli export --format json
  zylos-pp-cli export --format markdown --output chat.md
  zylos-pp-cli export --today --format markdown`, "\n"),
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

			rows, err := db.DB().QueryContext(cmd.Context(), query)
			if err != nil {
				return fmt.Errorf("querying conversations: %w", err)
			}
			defer rows.Close()

			var messages []exportMessage
			for rows.Next() {
				var dataStr string
				if err := rows.Scan(&dataStr); err != nil {
					continue
				}
				var msg exportMessage
				if err := json.Unmarshal([]byte(dataStr), &msg); err != nil {
					continue
				}
				messages = append(messages, msg)
			}

			w := cmd.OutOrStdout()
			if output != "" {
				f, err := os.Create(output)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer f.Close()
				w = f
			}

			switch format {
			case "markdown", "md":
				return exportMarkdown(w, messages)
			default:
				return exportJSON(w, messages)
			}
		},
	}

	cmd.Flags().StringVar(&format, "format", "json", "Export format: json or markdown")
	cmd.Flags().StringVar(&output, "output", "", "Write to file instead of stdout")
	cmd.Flags().BoolVar(&today, "today", false, "Export only today's messages")
	cmd.Flags().IntVar(&days, "days", 0, "Export messages from last N days (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}

func exportJSON(w io.Writer, messages any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(messages)
}

func exportMarkdown(w io.Writer, messages []exportMessage) error {
	fmt.Fprintln(w, "# Zylos Conversation Export")
	fmt.Fprintf(w, "Generated: %s\n\n", time.Now().Format(time.RFC3339))

	for _, msg := range messages {
		speaker := "AI"
		if msg.Direction == "in" {
			speaker = "User"
		}
		ts := msg.Timestamp
		if t, err := time.Parse(time.RFC3339, msg.Timestamp); err == nil {
			ts = t.Format("2006-01-02 15:04:05")
		}
		fmt.Fprintf(w, "### %s (%s)\n\n%s\n\n---\n\n", speaker, ts, msg.Content)
	}
	return nil
}
