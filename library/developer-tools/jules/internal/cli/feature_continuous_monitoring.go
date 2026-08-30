// Feature 2: Continuous Session Monitoring with reconciliation
// pp:data-source live
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/jules/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		monitorCmd := newMonitorCmd(flags)
		addNovelCommandIfAbsent(root, monitorCmd)
	})
}

func newMonitorCmd(flags *rootFlags) *cobra.Command {
	var reconcile bool
	var interval time.Duration
	var timeout time.Duration
	var sessionID string

	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Monitor session progress with reconciliation support",
		Long: `Monitor one or more sessions for progress and state changes.

Use --reconcile to detect and report stalled sessions (no activity for specified duration).
Use --session-id to monitor a specific session, or omit to monitor all active sessions.`,
		Example: `  # Monitor all sessions with reconciliation every 30s
  jules-pp-cli monitor --reconcile --interval 30s

  # Monitor specific session, detect stalled states (no activity for 5m)
  jules-pp-cli monitor --session-id abc123 --reconcile --timeout 5m`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			if flags.dryRun {
				// Honor the global --dry-run convention: run a single
				// poll-and-report pass instead of the unbounded loop so
				// dry-run invocations complete promptly.
				if !flags.asJSON {
					fmt.Fprintf(cmd.OutOrStdout(), "Dry run: polling sessions once (reconcile=%v)\n", reconcile)
				}
				return pollAndReconcile(ctx, c, cmd.OutOrStdout(), sessionID, reconcile, make(map[string]time.Time), flags.asJSON)
			}

			return monitorSessions(ctx, c, cmd.OutOrStdout(), sessionID, reconcile, interval, flags.asJSON)
		},
	}

	cmd.Flags().BoolVar(&reconcile, "reconcile", false, "Detect and report stalled sessions")
	cmd.Flags().DurationVar(&interval, "interval", 10*time.Second, "Poll interval")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Total monitor timeout (0 = unlimited)")
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Monitor specific session (omit for all)")

	return cmd
}

func monitorSessions(ctx context.Context, c *client.Client, out io.Writer, sessionID string, reconcile bool, interval time.Duration, asJSON bool) error {
	stalledSessions := make(map[string]time.Time) // sessionID -> lastActivityTime
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if !asJSON {
		fmt.Fprintf(out, "Starting session monitoring (reconcile=%v, interval=%v)\n", reconcile, interval)
	}

	for {
		select {
		case <-ctx.Done():
			// A self-imposed --timeout elapsing is expected, successful
			// completion of the monitor run, not a failure -- report it
			// as such instead of surfacing context.DeadlineExceeded as
			// a hard error. An externally-cancelled context (e.g. the
			// parent command context) still exits cleanly here; the
			// caller already knows why it cancelled.
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				if !asJSON {
					fmt.Fprintf(out, "\nMonitor timeout reached; exiting.\n")
				}
				return nil
			}
			return nil
		case <-ticker.C:
			if err := pollAndReconcile(ctx, c, out, sessionID, reconcile, stalledSessions, asJSON); err != nil {
				if !asJSON {
					fmt.Fprintf(out, "Error polling: %v\n", err)
				}
			}
		}
	}
}

type monitorSessionStatus struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Updated string `json:"updated,omitempty"`
	Stalled bool   `json:"stalled,omitempty"`
}

func pollAndReconcile(ctx context.Context, c *client.Client, out io.Writer, sessionID string, reconcile bool, stalledSessions map[string]time.Time, asJSON bool) error {
	path := "/sessions"
	params := map[string]string{"pageSize": "100"}

	data, err := c.Get(ctx, path, params)
	if err != nil {
		return err
	}

	sessions := decodeJSONList(data, "sessions")

	now := time.Now()
	checkedSessions := make(map[string]bool)
	var statuses []monitorSessionStatus
	var completed []string

	for _, s := range sessions {
		sessionMap, ok := s.(map[string]any)
		if !ok {
			continue
		}

		id, ok := sessionMap["id"].(string)
		if !ok {
			continue
		}
		checkedSessions[id] = true

		// Filter by session ID if specified
		if sessionID != "" && id != sessionID {
			continue
		}

		state, _ := sessionMap["state"].(string)
		updateTime, _ := sessionMap["updateTime"].(string)

		// Parse updateTime and detect staleness
		var lastUpdate time.Time
		if updateTime != "" {
			t, err := time.Parse(time.RFC3339, updateTime)
			if err == nil {
				lastUpdate = t
			}
		}

		stalledNow := false
		// Check for stalled sessions
		if reconcile && !lastUpdate.IsZero() {
			stalled := false
			lastActivity, seen := stalledSessions[id]

			// Detect stalled state: no update for 30 minutes
			if now.Sub(lastUpdate) > 30*time.Minute {
				if !seen || lastActivity.Equal(lastUpdate) {
					if !asJSON {
						fmt.Fprintf(out, "⚠️  Stalled session: %s (state=%s, last_update=%s, duration=%v)\n",
							id, state, updateTime, now.Sub(lastUpdate))
					}
					stalled = true
					stalledNow = true
				}
			}

			if stalled {
				stalledSessions[id] = lastUpdate
			} else if seen {
				delete(stalledSessions, id)
			}
		}

		// Reconcile: update tracking for active sessions
		if !lastUpdate.IsZero() {
			stalledSessions[id] = lastUpdate
		}

		// Print status
		if sessionID == id || sessionID == "" {
			if asJSON {
				statuses = append(statuses, monitorSessionStatus{ID: id, State: state, Updated: updateTime, Stalled: stalledNow})
			} else {
				fmt.Fprintf(out, "Session %s: state=%s, updated=%s\n", id, state, updateTime)
			}
		}
	}

	// Report sessions that completed/disappeared
	for id := range stalledSessions {
		if !checkedSessions[id] {
			if asJSON {
				completed = append(completed, id)
			} else {
				fmt.Fprintf(out, "✓ Session %s: completed or removed\n", id)
			}
			delete(stalledSessions, id)
		}
	}

	if asJSON {
		if statuses == nil {
			statuses = []monitorSessionStatus{}
		}
		if completed == nil {
			completed = []string{}
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(map[string]any{"sessions": statuses, "completed": completed})
	}

	return nil
}
