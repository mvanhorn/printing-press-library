package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newNetworkPeersCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "peers",
		Short:       "Get detailed peer connection info",
		Example:     "  dogecoin-pp-cli network peers --json",
		Annotations: map[string]string{"pp:endpoint": "network.peers", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getpeerinfo"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getpeerinfo", nil)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
