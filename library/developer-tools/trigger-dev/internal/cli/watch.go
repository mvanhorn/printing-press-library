package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var taskFilter string
	var notify bool
	var sound bool
	var maxRuns int

	cmd := &cobra.Command{
		Use:   "watch",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Monitor runs in real time and alert on failures",
		Long: `Watch for run failures in real time by polling the API at a configurable interval.
Displays new failures as they occur and optionally sends desktop notifications.

This is the feature Trigger.dev's dashboard alerts can't do: instant, terminal-based
failure monitoring with sound and desktop notifications.`,
		Example: `  # Watch for all failures, poll every 10 seconds
  trigger-dev-pp-cli watch

  # Watch specific task with desktop notifications
  trigger-dev-pp-cli watch --task my-email-task --notify

  # Watch with sound alerts and 5-second polling
  trigger-dev-pp-cli watch --interval 5s --sound --notify

  # JSON output for piping to other tools
  trigger-dev-pp-cli watch --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if flags.dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would poll %s/api/v1/runs every %s for failures\n", c.BaseURL, interval)
				return nil
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Watching for run failures (polling every %s)...\n", interval)
			if taskFilter != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Filtering: task=%s\n", taskFilter)
			}
			if notify {
				fmt.Fprintf(cmd.ErrOrStderr(), "Desktop notifications: enabled\n")
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Press Ctrl+C to stop.\n\n")

			seen := make(map[string]bool)
			firstPoll := true

			for {
				params := map[string]string{
					"status":                    "FAILED,CRASHED,SYSTEM_FAILURE",
					"page[size]":                fmt.Sprintf("%d", maxRuns),
					"filter[createdAt][period]": "1d",
				}
				if taskFilter != "" {
					params["taskIdentifier"] = taskFilter
				}

				resp, err := c.Get("/api/v1/runs", params)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Error polling: %v\n", err)
					time.Sleep(interval)
					continue
				}

				var envelope struct {
					Data []json.RawMessage `json:"data"`
				}
				if err := json.Unmarshal(resp, &envelope); err != nil {
					// Try as direct array
					var arr []json.RawMessage
					if err2 := json.Unmarshal(resp, &arr); err2 == nil {
						envelope.Data = arr
					}
				}

				for _, raw := range envelope.Data {
					var run struct {
						ID             string     `json:"id"`
						Status         string     `json:"status"`
						TaskIdentifier string     `json:"taskIdentifier"`
						CreatedAt      time.Time  `json:"createdAt"`
						FinishedAt     *time.Time `json:"finishedAt"`
						DurationMs     int        `json:"durationMs"`
						CostInCents    float64    `json:"costInCents"`
						Tags           []string   `json:"tags"`
					}
					if err := json.Unmarshal(raw, &run); err != nil {
						continue
					}

					if seen[run.ID] {
						continue
					}
					seen[run.ID] = true

					if firstPoll {
						continue // Don't alert on pre-existing failures
					}

					ts := run.CreatedAt.Local().Format("15:04:05")

					if flags.asJSON {
						flags.printJSON(cmd, run)
					} else {
						symbol := "FAIL"
						if run.Status == "CRASHED" {
							symbol = "CRASH"
						} else if run.Status == "SYSTEM_FAILURE" {
							symbol = "SYSFAIL"
						}
						fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s  %s  %s  (%.1fms, $%.4f)\n",
							ts, symbol, run.ID, run.TaskIdentifier, float64(run.DurationMs), run.CostInCents/100)
						if len(run.Tags) > 0 {
							fmt.Fprintf(cmd.OutOrStdout(), "         tags: %s\n", strings.Join(run.Tags, ", "))
						}
					}

					if notify {
						sendDesktopNotification(run.TaskIdentifier, run.ID, run.Status)
					}
					if sound {
						playAlertSound()
					}
				}

				firstPoll = false
				time.Sleep(interval)
			}
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Polling interval (e.g., 5s, 30s, 1m)")
	cmd.Flags().StringVar(&taskFilter, "task", "", "Filter by task identifier")
	cmd.Flags().BoolVar(&notify, "notify", false, "Send desktop notifications on failure")
	cmd.Flags().BoolVar(&sound, "sound", false, "Play alert sound on failure")
	cmd.Flags().IntVar(&maxRuns, "max", 50, "Max runs to check per poll")

	return cmd
}

func sendDesktopNotification(task, runID, status string) {
	title := fmt.Sprintf("Trigger.dev: %s %s", status, task)
	body := fmt.Sprintf("Run %s failed", runID)

	switch runtime.GOOS {
	case "darwin":
		exec.Command("osascript", "-e", fmt.Sprintf(
			`display notification %q with title %q sound name "Basso"`, body, title)).Run()
	case "linux":
		exec.Command("notify-send", "-u", "critical", title, body).Run()
	}
}

func playAlertSound() {
	switch runtime.GOOS {
	case "darwin":
		exec.Command("afplay", "/System/Library/Sounds/Basso.aiff").Start()
	case "linux":
		exec.Command("paplay", "/usr/share/sounds/freedesktop/stereo/dialog-error.oga").Start()
	}
}
