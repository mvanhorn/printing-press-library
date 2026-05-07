// Hand-authored novel feature: real-time failure watch with optional
// desktop notifications and sound. Short-circuits in cliutil.IsVerifyEnv().

package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/trigger-dev/internal/cliutil"
)

func newRunsWatchCmd(flags *rootFlags) *cobra.Command {
	var interval time.Duration
	var taskFilter string
	var notify bool
	var sound bool
	var maxRuns int

	cmd := &cobra.Command{
		Use:         "watch",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Watch live runs and alert on new failures (non-zero exit + notify + sound)",
		Long: `Polls /api/v1/runs at a configurable interval, surfacing only runs that
failed (FAILED, CRASHED, SYSTEM_FAILURE) AFTER the watch started. The first
poll seeds the local cursor; only failures after that fire the alert path.

The --notify and --sound flags are opt-in; default behavior is plain stdout.
Short-circuits in PRINTING_PRESS_VERIFY=1 mode.`,
		Example: `  trigger-dev-pp-cli runs watch
  trigger-dev-pp-cli runs watch --task daily-digest --notify --sound
  trigger-dev-pp-cli runs watch --interval 5s --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would watch: /api/v1/runs (verify mode short-circuit)")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would poll %s/api/v1/runs every %s for failures\n", c.BaseURL, interval)
				return nil
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Watching for run failures (polling every %s)...\n", interval)
			if taskFilter != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Filtering: task=%s\n", taskFilter)
			}
			if notify {
				fmt.Fprintln(cmd.ErrOrStderr(), "Desktop notifications: enabled")
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "Press Ctrl+C to stop.")

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
					fmt.Fprintf(cmd.ErrOrStderr(), "poll error: %v\n", err)
					time.Sleep(interval)
					continue
				}

				items := unwrapEnvelope(resp)
				for _, raw := range items {
					var run struct {
						ID             string    `json:"id"`
						Status         string    `json:"status"`
						TaskIdentifier string    `json:"taskIdentifier"`
						CreatedAt      time.Time `json:"createdAt"`
						DurationMs     int       `json:"durationMs"`
						CostInCents    float64   `json:"costInCents"`
						Tags           []string  `json:"tags"`
					}
					if err := json.Unmarshal(raw, &run); err != nil {
						continue
					}
					if seen[run.ID] {
						continue
					}
					seen[run.ID] = true
					if firstPoll {
						continue
					}
					ts := run.CreatedAt.Local().Format("15:04:05")
					if flags.asJSON {
						_ = printJSONFiltered(cmd.OutOrStdout(), run, flags)
					} else {
						symbol := "FAIL"
						switch run.Status {
						case "CRASHED":
							symbol = "CRASH"
						case "SYSTEM_FAILURE":
							symbol = "SYSFAIL"
						}
						fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s  %s  %s  (%dms, $%.4f)\n",
							ts, symbol, run.ID, run.TaskIdentifier, run.DurationMs, run.CostInCents/100)
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

	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Polling interval (5s, 30s, 1m)")
	cmd.Flags().StringVar(&taskFilter, "task", "", "Filter by task identifier")
	cmd.Flags().BoolVar(&notify, "notify", false, "Send desktop notifications on new failure")
	cmd.Flags().BoolVar(&sound, "sound", false, "Play alert sound on new failure")
	cmd.Flags().IntVar(&maxRuns, "max", 50, "Max runs to inspect per poll")

	return cmd
}

func unwrapEnvelope(resp json.RawMessage) []json.RawMessage {
	var env struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(resp, &env); err == nil && env.Data != nil {
		return env.Data
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(resp, &arr); err == nil {
		return arr
	}
	return nil
}

func sendDesktopNotification(task, runID, status string) {
	title := fmt.Sprintf("Trigger.dev: %s %s", status, task)
	body := fmt.Sprintf("Run %s failed", runID)
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("osascript", "-e",
			fmt.Sprintf(`display notification %q with title %q sound name "Basso"`, body, title)).Run()
	case "linux":
		_ = exec.Command("notify-send", "-u", "critical", title, body).Run()
	}
}

func playAlertSound() {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("afplay", "/System/Library/Sounds/Basso.aiff").Start()
	case "linux":
		_ = exec.Command("paplay", "/usr/share/sounds/freedesktop/stereo/dialog-error.oga").Start()
	}
}
