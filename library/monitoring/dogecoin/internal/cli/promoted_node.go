package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNodePromotedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Node uptime and health",
	}
	cmd.AddCommand(newNodeUptimeCmd(flags))
	cmd.AddCommand(newNodeHealthCmd(flags))
	return cmd
}

func newNodeUptimeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "uptime",
		Short:       "Get node uptime in seconds",
		Example:     "  dogecoin-pp-cli node uptime --json",
		Annotations: map[string]string{"pp:endpoint": "node.uptime", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"uptime"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			ctx := context.Background()
			raw, err := c.Call(ctx, "uptime", nil)
			if err != nil {
				// uptime RPC method added in Dogecoin Core 1.14.x; 1.10.x returns 404
				result := map[string]any{
					"uptime_seconds": nil,
					"uptime_human":   "unavailable",
					"note":           "uptime RPC not supported by this node version (requires Dogecoin Core >= 1.14.x)",
				}
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			var seconds int64
			if err := json.Unmarshal(raw, &seconds); err != nil {
				return fmt.Errorf("parsing uptime: %w", err)
			}
			hours := seconds / 3600
			minutes := (seconds % 3600) / 60
			result := map[string]any{
				"uptime_seconds": seconds,
				"uptime_human":   fmt.Sprintf("%dh %dm", hours, minutes),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}
