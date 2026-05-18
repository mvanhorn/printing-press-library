// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/cliutil"
)

// newWarmCmd activates the Granola desktop app and types a search query
// into it via AppleScript. Default behavior: print only. --launch
// actually executes osascript. Always short-circuits under verify.
func newWarmCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:   "warm <id> <query>",
		Short: "Open the Granola desktop app with a meeting + search query (macOS)",
		Annotations: map[string]string{
			"mcp:hidden": "true",
			// mutates user-visible OS state on --launch; intentionally NOT
			// marked read-only.
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, query := args[0], args[1]
			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(cmd.OutOrStdout(), "verify: would warm %s with %q (no-op)\n", id, query)
				return nil
			}
			if runtime.GOOS != "darwin" {
				fmt.Fprintln(cmd.OutOrStdout(), "warm: not supported on this platform; skipping")
				return nil
			}
			if !launch {
				fmt.Fprintf(cmd.OutOrStdout(), "would warm: %s with query %q (re-run with --launch to execute)\n", id, query)
				return nil
			}
			// Activate + paste query.
			script := fmt.Sprintf(`tell application "Granola" to activate
delay 0.3
tell application "System Events"
  keystroke "%s"
end tell`, escapeAppleScript(query))
			out, err := exec.Command("osascript", "-e", script).CombinedOutput()
			if err != nil {
				return ioErr(fmt.Errorf("osascript: %w: %s", err, string(out)))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "{\"warmed\":true,\"id\":%q,\"query\":%q}\n", id, query)
			return nil
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Actually open Granola and type the query (default: print only)")
	return cmd
}

func escapeAppleScript(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return string(out)
}
