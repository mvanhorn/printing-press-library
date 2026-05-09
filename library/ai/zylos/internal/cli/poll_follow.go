package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/ai/zylos/internal/store"
)

func newPollFollowCmd(flags *rootFlags) *cobra.Command {
	var follow bool
	var interval time.Duration
	var dbPath string

	cmd := &cobra.Command{
		Use:   "follow",
		Short: "Stream new messages in real-time from the local store (NDJSON)",
		Example: strings.Trim(`
  zylos-pp-cli conversations follow --follow
  zylos-pp-cli conversations follow --follow --interval 5s
  zylos-pp-cli conversations follow --follow --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !follow {
				return cmd.Help()
			}
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

			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			// Track the highest timestamp we've seen
			var lastTS string
			// Bootstrap: find the latest message timestamp
			db.DB().QueryRowContext(cmd.Context(),
				`SELECT json_extract(data, '$.timestamp') FROM resources
				 WHERE resource_type = 'conversations'
				 ORDER BY json_extract(data, '$.timestamp') DESC
				 LIMIT 1`,
			).Scan(&lastTS)

			fmt.Fprintf(os.Stderr, "Following for new messages every %s (Ctrl+C to stop)\n", interval)

			enc := json.NewEncoder(cmd.OutOrStdout())

			for {
				select {
				case <-sig:
					fmt.Fprintln(os.Stderr, "\nShutting down gracefully...")
					return nil
				case <-ticker.C:
					rows, err := db.DB().QueryContext(cmd.Context(),
						`SELECT data FROM resources
						 WHERE resource_type = 'conversations'
						 AND json_extract(data, '$.timestamp') > ?
						 ORDER BY json_extract(data, '$.timestamp') ASC`,
						lastTS,
					)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: poll failed: %v\n", err)
						continue
					}

					for rows.Next() {
						var dataStr string
						if rows.Scan(&dataStr) != nil {
							continue
						}
						var msg map[string]any
						if json.Unmarshal([]byte(dataStr), &msg) != nil {
							continue
						}
						event := map[string]any{
							"event":     "message",
							"timestamp": time.Now().UTC().Format(time.RFC3339),
							"data":      msg,
						}
						if err := enc.Encode(event); err != nil {
							rows.Close()
							return err
						}
						if ts, ok := msg["timestamp"].(string); ok && ts > lastTS {
							lastTS = ts
						}
					}
					rows.Close()
				}
			}
		},
	}

	cmd.Flags().BoolVar(&follow, "follow", false, "Enable continuous polling for new messages")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "Poll interval")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")

	return cmd
}
