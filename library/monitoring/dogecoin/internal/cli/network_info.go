package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newNetworkInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "info",
		Short:       "Get network info (version, peers, services)",
		Example:     "  dogecoin-pp-cli network info --json",
		Annotations: map[string]string{"pp:endpoint": "network.info", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getnetworkinfo"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getnetworkinfo", nil)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
