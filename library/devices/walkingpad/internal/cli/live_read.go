package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/devices/walkingpad/internal/wpble"
)

func newMonitorCmd(flags *rootFlags) *cobra.Command {
	var durationStr string
	cmd := &cobra.Command{
		Use:     "monitor",
		Short:   "Stream live belt telemetry until stopped",
		Long:    "Continuously poll and print belt status (speed, distance, steps, time). Requires --live. Use --duration to bound the run; otherwise it streams until interrupted.",
		Example: "  walkingpad-pp-cli monitor --live --duration 30s --json",
		// Read-only, but a long-running stream that holds a BLE connection — hidden
		// from MCP, which is request-response. Agents use `status` for a snapshot.
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var duration time.Duration
			if durationStr != "" {
				d, err := time.ParseDuration(durationStr)
				if err != nil {
					return fmt.Errorf("--duration: %w", err)
				}
				duration = d
			}
			if stop, err := offlineGuard(cmd, flags, "monitor the belt"); stop {
				return err
			}
			ctx := cmd.Context()
			if d, bounded := boundedContext(cmd, duration); bounded {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, d)
				defer cancel()
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			return dialAndMonitor(ctx, flags, statsPollInterval, func(s wpble.Status) error {
				if flags.asJSON {
					return enc.Encode(s)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%.1f km/h  %dm  %d steps  %ds  mode=%s\n",
					s.SpeedKmh, s.DistanceM, s.Steps, s.TimeS, s.ModeName)
				return nil
			})
		},
	}
	cmd.Flags().StringVar(&durationStr, "duration", "", "How long to stream (e.g. 30s, 5m); empty = until interrupted")
	return cmd
}

func newLastRecordCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "last-record",
		Short:       "Read the belt's last stored run (time, distance, steps)",
		Long:        "Read the run the belt holds in memory from its last session. The belt clears it on the next request and loses it on power-cut. Requires --live.",
		Example:     "  walkingpad-pp-cli last-record --live --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stop, err := offlineGuard(cmd, flags, "read the last record"); stop {
				return err
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 12*time.Second)
			defer cancel()
			rec, err := lastRecord(ctx, flags)
			if err != nil {
				return err
			}
			return emit(cmd, flags, rec,
				fmt.Sprintf("last run: %dm, %d steps, %ds", rec.DistanceM, rec.Steps, rec.TimeS))
		},
	}
}

// lastRecord opens a BLE link, requests the last-record frame, and returns it.
func lastRecord(ctx context.Context, flags *rootFlags) (wpble.LastRecord, error) {
	link, err := dialBelt(ctx, flags)
	if err != nil {
		return wpble.LastRecord{}, err
	}
	defer func() { _ = link.Close() }()

	done := make(chan wpble.LastRecord, 1)
	if err := link.Subscribe(wpble.NotifyUUID, func(data []byte) {
		rec, ok := wpble.ParseLastRecord(data)
		if ok {
			select {
			case done <- rec:
			default:
			}
		}
	}); err != nil {
		return wpble.LastRecord{}, err
	}
	if err := link.Write(wpble.WriteUUID, wpble.CmdAskLastRecord()); err != nil {
		return wpble.LastRecord{}, err
	}
	select {
	case rec := <-done:
		return rec, nil
	case <-ctx.Done():
		return wpble.LastRecord{}, ctx.Err()
	}
}
