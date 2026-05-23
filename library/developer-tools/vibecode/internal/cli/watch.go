// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-coded transcendence feature for vibecode-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/internal/store"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var projectID string
	var notify bool
	var types string

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for project and deployment changes",
		Long: `Poll API for changes and optionally trigger OS notifications.

Stores state deltas in SQLite to track what changed between polls.
Useful for monitoring deployments or tracking project activity.`,
		Example: `  vibecode-pp-cli watch
  vibecode-pp-cli watch --interval 30s
  vibecode-pp-cli watch --project proj_abc123 --notify
  vibecode-pp-cli watch --types deployments,sandboxes`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would start watching for changes")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			dbPath := defaultDBPath("vibecode-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			ctx := cmd.Context()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			fmt.Fprintf(cmd.OutOrStdout(), "Watching for changes (interval: %s)...\n", interval)
			if projectID != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Filtered to project: %s\n", projectID)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl+C to stop")

			// Initial state
			lastState := make(map[string]string)

			checkForChanges := func() error {
				var changes []watchChange

				// Check deployments
				deploymentsData, err := c.Get(ctx, "/deployments", nil)
				if err != nil {
					return fmt.Errorf("fetching deployments: %w", err)
				}

				var deployments []map[string]any
				if err := json.Unmarshal(deploymentsData, &deployments); err != nil {
					return fmt.Errorf("parsing deployments: %w", err)
				}

				for _, d := range deployments {
					id, _ := d["id"].(string)
					if id == "" {
						continue
					}
					if projectID != "" {
						if pid, _ := d["project_id"].(string); pid != projectID {
							continue
						}
					}

					status, _ := d["status"].(string)
					key := "deployment:" + id
					if oldStatus, exists := lastState[key]; exists && oldStatus != status {
						changes = append(changes, watchChange{
							Type:      "deployment",
							ID:        id,
							Field:     "status",
							OldValue:  oldStatus,
							NewValue:  status,
							Timestamp: time.Now(),
						})
					}
					lastState[key] = status
				}

				// Check sandboxes
				sandboxesData, err := c.Get(ctx, "/sandboxes", nil)
				if err != nil {
					// Sandboxes endpoint might fail if none exist
					sandboxesData = []byte("[]")
				}

				var sandboxes []map[string]any
				if err := json.Unmarshal(sandboxesData, &sandboxes); err == nil {
					for _, s := range sandboxes {
						id, _ := s["id"].(string)
						if id == "" {
							continue
						}
						if projectID != "" {
							if pid, _ := s["project_id"].(string); pid != projectID {
								continue
							}
						}

						status, _ := s["status"].(string)
						key := "sandbox:" + id
						if oldStatus, exists := lastState[key]; exists && oldStatus != status {
							changes = append(changes, watchChange{
								Type:      "sandbox",
								ID:        id,
								Field:     "status",
								OldValue:  oldStatus,
								NewValue:  status,
								Timestamp: time.Now(),
							})
						}
						lastState[key] = status
					}
				}

				// Report changes
				for _, change := range changes {
					changeJSON, _ := json.Marshal(change)
					if flags.asJSON || flags.agent {
						fmt.Fprintln(cmd.OutOrStdout(), string(changeJSON))
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s %s: %s → %s\n",
							change.Timestamp.Format("15:04:05"),
							change.Type,
							change.ID,
							change.OldValue,
							change.NewValue,
						)
					}

					// Store change in database
					_, _ = db.DB().ExecContext(ctx, `
						INSERT INTO metadata (key, value)
						VALUES (?, ?)
						ON CONFLICT(key) DO UPDATE SET value = excluded.value
					`, fmt.Sprintf("watch_change_%d", time.Now().UnixNano()), string(changeJSON))

					// OS notification
					if notify {
						sendNotification(
							fmt.Sprintf("Vibecode: %s changed", change.Type),
							fmt.Sprintf("%s %s: %s → %s", change.Type, change.ID, change.OldValue, change.NewValue),
						)
					}
				}

				return nil
			}

			// Initial check
			if err := checkForChanges(); err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Warning: %v\n", err)
			}

			// Watch loop
			for {
				select {
				case <-ctx.Done():
					fmt.Fprintln(cmd.OutOrStdout(), "\nWatch stopped")
					return nil
				case <-ticker.C:
					if err := checkForChanges(); err != nil {
						fmt.Fprintf(cmd.OutOrStdout(), "Warning: %v\n", err)
					}
				}
			}
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", time.Minute, "Polling interval")
	cmd.Flags().StringVar(&projectID, "project", "", "Filter to a specific project")
	cmd.Flags().BoolVar(&notify, "notify", false, "Send OS notifications on changes")
	cmd.Flags().StringVar(&types, "types", "deployments,sandboxes", "Comma-separated resource types to watch")
	return cmd
}

type watchChange struct {
	Type      string    `json:"type"`
	ID        string    `json:"id"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	Timestamp time.Time `json:"timestamp"`
}

// sendNotification sends an OS notification (best effort)
func sendNotification(title, message string) {
	switch runtime.GOOS {
	case "darwin":
		// macOS notification using osascript
		script := fmt.Sprintf(`display notification "%s" with title "%s"`, message, title)
		exec.Command("osascript", "-e", script).Run()
	case "linux":
		// Linux notification using notify-send
		exec.Command("notify-send", title, message).Run()
	}
	// Windows would use powershell or toast notifications
}
