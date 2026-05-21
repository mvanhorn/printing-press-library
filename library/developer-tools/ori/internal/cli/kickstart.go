// Copyright 2026 error. Licensed under Apache-2.0.
// Transcendence command: wraps `launchctl kickstart` + /healthz poll into one
// verb. Replaces the memorized post-`compose down/up` recovery ritual.

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/cliutil"
)

func newKickstartCmd(flags *rootFlags) *cobra.Command {
	var wait bool
	var waitTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "kickstart",
		Short: "Restart the launchd a2a-server bridge and poll /healthz until ready",
		Long: `Runs:

  launchctl kickstart -k gui/$(id -u)/dev.error2.openclaw-a2a-server

then, when --wait is set, polls /healthz every 500ms until {ok:true} or the
timeout expires. Use this after every NAS compose down/up to recover from the
"chat_ori returns empty failed" state without remembering the launchctl
incantation.`,
		Example: `  ori-pp-cli kickstart
  ori-pp-cli kickstart --wait
  ori-pp-cli kickstart --wait --wait-timeout 30s
  ori-pp-cli kickstart --dry-run`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			label := "dev.error2.openclaw-a2a-server"
			target := fmt.Sprintf("gui/%d/%s", os.Getuid(), label)
			out := cmd.OutOrStdout()

			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(out, "would kickstart:", target)
				return nil
			}
			if flags.dryRun {
				fmt.Fprintln(out, "would run: launchctl kickstart -k", target)
				return nil
			}

			report := map[string]any{"target": target, "kickstarted": false, "ready": false}
			emit := func() error {
				if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
					return printJSONFiltered(out, report, flags)
				}
				return nil
			}

			if !flags.asJSON {
				fmt.Fprintln(out, "kickstarting", target+"...")
			}
			ksCmd := exec.Command("launchctl", "kickstart", "-k", target)
			if kerr := ksCmd.Run(); kerr != nil {
				report["error"] = kerr.Error()
				_ = emit()
				return apiErr(fmt.Errorf("launchctl kickstart failed: %w", kerr))
			}
			report["kickstarted"] = true
			if !flags.asJSON {
				fmt.Fprintln(out, "kickstart issued")
			}

			if !wait {
				return emit()
			}

			c, cerr := flags.newClient()
			if cerr != nil {
				return cerr
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), waitTimeout)
			defer cancel()
			deadline := time.Now().Add(waitTimeout)
			if !flags.asJSON {
				fmt.Fprintln(out, "polling /healthz...")
			}
			for {
				if _, perr := c.Get("/healthz", nil); perr == nil {
					elapsed := time.Since(deadline.Add(-waitTimeout))
					report["ready"] = true
					report["wait_seconds"] = elapsed.Seconds()
					if !flags.asJSON {
						fmt.Fprintf(out, "%s ready (%.1fs)\n", green("OK"), elapsed.Seconds())
					}
					return emit()
				} else {
					var apiE *client.APIError
					if errors.As(perr, &apiE) {
						// got a response but non-2xx — still booting
					}
				}
				select {
				case <-ctx.Done():
					report["error"] = "timed out waiting for /healthz"
					_ = emit()
					return apiErr(fmt.Errorf("timed out after %s waiting for /healthz", waitTimeout))
				case <-time.After(500 * time.Millisecond):
				}
			}
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "Poll /healthz until ready")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 30*time.Second, "Max time to wait for /healthz when --wait is set")
	return cmd
}
