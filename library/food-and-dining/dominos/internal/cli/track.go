package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/client"

	"github.com/spf13/cobra"
)

// newTrackCmd is the user-facing tracker command. The manifest calls it
// `track --phone ...`; we keep the existing `tracking` command in place
// (it's the endpoint-mirror name) and route both names through the same
// fetch + render path. With --watch, this polls the tracker every
// --interval and emits one JSON line per status transition until the
// status reaches "delivered" or the iteration cap is hit.
func newTrackCmd(flags *rootFlags) *cobra.Command {
	var phone string
	var watch bool
	var interval time.Duration

	cmd := &cobra.Command{
		Use:   "track",
		Short: "Track an order by phone (single-shot or --watch)",
		Long: `Track an order by phone.

Without --watch, fetches the current tracker payload once and prints it.
With --watch, polls every --interval and emits one JSON line per
status transition. Exits 0 when the order reaches 'delivered'.

Iteration is capped at 60 polls so the loop cannot run forever.`,
		Example:     "  dominos-pp-cli track --phone 2065551234 --watch --interval 30s",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if phone == "" && !flags.dryRun {
				return usageErr(fmt.Errorf(`required flag(s) "phone" not set`))
			}
			if interval <= 0 {
				interval = 30 * time.Second
			}
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "track",
					"dry_run": true,
					"phone":   phone,
					"watch":   watch,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if !watch {
				data, err := c.Get("/power/tracker", map[string]string{"Phone": phone})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
			}
			return watchTracker(cmd, c, phone, interval)
		},
	}
	cmd.Flags().StringVar(&phone, "phone", "", "Phone number associated with the order")
	cmd.Flags().BoolVar(&watch, "watch", false, "Poll the tracker until the order is delivered")
	cmd.Flags().DurationVar(&interval, "interval", 30*time.Second, "Polling interval when --watch is set")
	return cmd
}

// watchTracker polls /power/tracker every interval. It prints one JSON
// line per OrderStatus transition, exits 0 when the status reaches a
// terminal state (delivered/complete), and stops after 60 iterations
// to avoid infinite loops.
func watchTracker(cmd *cobra.Command, c *client.Client, phone string, interval time.Duration) error {
	const maxIters = 60
	var last string
	w := cmd.OutOrStdout()
	for i := 0; i < maxIters; i++ {
		data, err := c.Get("/power/tracker", map[string]string{"Phone": phone})
		if err != nil {
			return classifyAPIError(err, nil)
		}
		status, line := extractTrackerStatus(data)
		if status != last {
			fmt.Fprintln(w, string(line))
			last = status
		}
		if isTerminalStatus(status) {
			return nil
		}
		if i+1 < maxIters {
			time.Sleep(interval)
		}
	}
	return nil
}

// pollTrackerUntilDelivered is the helper used by composers like
// order-quick that already have a tracker phone number.
func pollTrackerUntilDelivered(cmd *cobra.Command, flags *rootFlags, c *client.Client, phone string) error {
	return watchTracker(cmd, c, phone, 30*time.Second)
}

func extractTrackerStatus(data json.RawMessage) (string, []byte) {
	// Tracker can be an array (one entry per active order) or a single
	// object. We pull the OrderStatus from the first entry and re-emit
	// the original bytes verbatim as the per-line JSON.
	var arr []map[string]any
	if json.Unmarshal(data, &arr) == nil && len(arr) > 0 {
		if s, ok := arr[0]["OrderStatus"].(string); ok {
			return strings.ToLower(s), []byte(data)
		}
	}
	var obj map[string]any
	if json.Unmarshal(data, &obj) == nil {
		if s, ok := obj["OrderStatus"].(string); ok {
			return strings.ToLower(s), []byte(data)
		}
	}
	return "", []byte(data)
}

func isTerminalStatus(s string) bool {
	switch strings.ToLower(s) {
	case "delivered", "complete", "completed":
		return true
	}
	return false
}
