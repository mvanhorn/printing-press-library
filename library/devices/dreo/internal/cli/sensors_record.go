// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"

	"github.com/spf13/cobra"
)

func newSensorsRecordCmd(rflags *rootFlags) *cobra.Command {
	var (
		duration time.Duration
		deviceSn string
	)
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Record WebSocket sensor events to local sensor_readings table",
		Long: `Subscribe to the Dreo control-plane WebSocket and append every
numeric sensor reading (temperature, humidity, pm25, etc.) to the local
sensor_readings table. Long-running; press Ctrl-C to stop.

Under PRINTING_PRESS_DOGFOOD the recorder caps at 10 seconds.`,
		Example: `  dreo-pp-cli sensors record --duration 1h
  dreo-pp-cli sensors record --device HTF008S-...`,
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would record: skipped under PRINTING_PRESS_VERIFY")
				return nil
			}
			if dryRunOK(rflags) {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: would record for %s\n",
					func() string {
						if duration == 0 {
							return "until Ctrl-C"
						}
						return duration.String()
					}())
				return nil
			}

			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			if cliutil.IsDogfoodEnv() {
				dctx, dcancel := context.WithTimeout(ctx, 10*time.Second)
				defer dcancel()
				ctx = dctx
			}
			if duration > 0 {
				dctx, dcancel := context.WithTimeout(ctx, duration)
				defer dcancel()
				ctx = dctx
			}

			wsConn, err := connectWS(ctx, rflags)
			if err != nil {
				return apiErr(fmt.Errorf("sensors record: open WS: %w", err))
			}
			defer wsConn.Close()

			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			count := 0
			writeErrs := 0
			metrics := []string{"temperature", "humidity", "pm25", "targetTemperature", "targetHumidity", "tvoc", "co2"}

			for {
				select {
				case upd, ok := <-wsConn.Updates():
					if !ok {
						goto done
					}
					if deviceSn != "" && upd.DeviceSn != deviceSn {
						continue
					}
					for _, metric := range metrics {
						v, has := asFloat(upd.Fields, metric)
						if !has {
							continue
						}
						// Only increment count when the row actually
						// landed — on disk-full / sqlite-locked, inserts
						// silently fail and the user would otherwise see
						// "Recorded N readings" with zero rows persisted.
						// First failure also logs to stderr so unattended
						// cron jobs surface the disk problem.
						if err := st.AppendSensorReading(ctx, upd.DeviceSn, upd.ReceivedAt, metric, v); err != nil {
							if writeErrs == 0 {
								fmt.Fprintf(cmd.ErrOrStderr(), "sensors record: failed to persist reading (%s for %s): %v\n", metric, upd.DeviceSn, err)
							}
							writeErrs++
							continue
						}
						count++
					}
				case <-ctx.Done():
					goto done
				}
			}
		done:
			// Distinguish clean stop (user Ctrl-C, --duration elapsed) from
			// an unexpected WebSocket disconnect. For unattended cron use
			// the silent-exit case is indistinguishable from success
			// otherwise — the operator only learns recording died when
			// they query the empty store hours later.
			wsErr := wsConn.Err()
			result := map[string]any{"recorded": count}
			if writeErrs > 0 {
				result["write_errors"] = writeErrs
			}
			if wsErr != nil {
				result["websocket_error"] = wsErr.Error()
			}
			if rflags.asJSON {
				_ = printJSONFiltered(cmd.OutOrStdout(), result, rflags)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Recorded %d sensor readings.\n", count)
				if writeErrs > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d sensor readings failed to persist (see first failure above)\n", writeErrs)
				}
				if wsErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "websocket terminated unexpectedly: %v\n", wsErr)
				}
			}
			if wsErr != nil {
				return apiErr(fmt.Errorf("sensors record: %w", wsErr))
			}
			return nil
		},
	}
	cmd.Flags().DurationVar(&duration, "duration", 0, "Recording duration (0 = until Ctrl-C)")
	cmd.Flags().StringVar(&deviceSn, "device", "", "Restrict to a single device sn")
	return cmd
}
