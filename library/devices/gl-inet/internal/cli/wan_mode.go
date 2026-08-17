// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelWanModeCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "mode <ethernet|repeater|tethering>",
		Short:       "Switch the router's WAN source and verify it reconnected.",
		Long:        "Switch the WAN uplink source via netmode.set_mode and confirm the new mode. Mapping: ethernet->router, repeater->repeater, tethering->tethering (passed literally to the firmware).",
		Example:     "  gl-inet-pp-cli wan mode repeater --agent",
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			mode, ok := mapWanMode(args[0])
			if !ok {
				return usageErr(fmt.Errorf("invalid WAN source %q: expected ethernet, repeater, or tethering", args[0]))
			}
			out := cmd.OutOrStdout()

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would set WAN mode to %q (netmode %q)\n", args[0], mode)
				return nil
			}
			if dryRunOK(flags) {
				fmt.Fprintf(out, "dry-run: netmode.set_mode {\"mode\":%q}\n", mode)
				return nil
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			setRes, err := c.Call(ctx, "netmode", "set_mode", map[string]any{"mode": mode})
			if err != nil {
				return classifyGLError(err, flags)
			}

			result := map[string]any{
				"requested": args[0],
				"mode":      mode,
				"set":       json.RawMessage(setRes),
			}
			if got, gerr := c.Call(ctx, "netmode", "get_mode", nil); gerr == nil {
				result["current"] = json.RawMessage(got)
			}
			if status, serr := c.Call(ctx, "system", "get_status", nil); serr == nil {
				result["status"] = json.RawMessage(status)
			}
			raw, _ := json.Marshal(result)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	return cmd
}
