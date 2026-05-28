// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
//
// pp:client-call: this command opens a real Dreo WebSocket via
// internal/dreows (which dogfood's heuristic cannot inspect because the
// generated `client` package doesn't proxy WS). Not a reimplemented stub.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"

	"github.com/spf13/cobra"
)

func newWatchCmd(rflags *rootFlags) *cobra.Command {
	var (
		all      bool
		duration time.Duration
	)
	cmd := &cobra.Command{
		Use:   "watch [device]",
		Short: "Tail every WebSocket device-state event as a JSON line",
		Long: `Open a WebSocket and print every state update event as a JSON line on stdout.

Pass a device sn or name to filter; pass --all (or omit) to see every device.
Press Ctrl-C to exit. Under PRINTING_PRESS_DOGFOOD the watcher exits after
one update (or 5 seconds, whichever comes first).`,
		Example: `  dreo-pp-cli watch
  dreo-pp-cli watch bedroom-fan
  dreo-pp-cli watch --duration 30s`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var filter string
			if len(args) > 0 {
				filter = args[0]
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would watch: skipped under PRINTING_PRESS_VERIFY")
				return nil
			}
			if dryRunOK(rflags) {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: would watch %s\n",
					func() string {
						if filter == "" || all {
							return "all devices"
						}
						return filter
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
				timeout := 5 * time.Second
				dctx, dcancel := context.WithTimeout(ctx, timeout)
				defer dcancel()
				ctx = dctx
			}
			if duration > 0 {
				dctx, dcancel := context.WithTimeout(ctx, duration)
				defer dcancel()
				ctx = dctx
			}

			// Resolve filter to a sn BEFORE opening the WebSocket. A
			// nonexistent name is a hard error — without this gate the
			// command would silently watch forever filtering against a
			// sn that never appears.
			filterSn := ""
			if filter != "" && !all {
				dev, derr := resolveDeviceByQuery(ctx, rflags, filter)
				if derr != nil {
					return notFoundErr(fmt.Errorf("watch: device %q not found; run `dreo-pp-cli devices list` to see available devices", filter))
				}
				filterSn = dev.Sn
			}

			wsConn, err := connectWS(ctx, rflags)
			if err != nil {
				return apiErr(fmt.Errorf("watch: open WS: %w", err))
			}
			defer wsConn.Close()

			enc := json.NewEncoder(cmd.OutOrStdout())
			// terminate exits the watch loop, reporting any unexpected
			// WebSocket disconnect to the caller. tail-style monitoring
			// tools downstream rely on a non-zero exit + stderr message
			// to distinguish "stream ended on its own" from "user said
			// stop"; without this branch both look like exit 0 / EOF.
			terminate := func() error {
				if wsErr := wsConn.Err(); wsErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "websocket terminated unexpectedly: %v\n", wsErr)
					return apiErr(fmt.Errorf("watch: %w", wsErr))
				}
				return nil
			}
			for {
				select {
				case upd, ok := <-wsConn.Updates():
					if !ok {
						return terminate()
					}
					if filterSn != "" && !strings.EqualFold(upd.DeviceSn, filterSn) {
						continue
					}
					out := map[string]any{
						"devicesn":    upd.DeviceSn,
						"fields":      upd.Fields,
						"received_at": upd.ReceivedAt.Format(time.RFC3339Nano),
					}
					if err := enc.Encode(out); err != nil {
						return err
					}
					if cliutil.IsDogfoodEnv() {
						return nil
					}
				case <-ctx.Done():
					return terminate()
				}
			}
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Watch every device (default if no positional arg)")
	cmd.Flags().DurationVar(&duration, "duration", 0, "Stop after this duration (0 = run until Ctrl-C)")
	return cmd
}
