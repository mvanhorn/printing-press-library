package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newBlockchainCountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "count",
		Short:       "Get current block height",
		Example:     "  dogecoin-pp-cli blockchain count",
		Annotations: map[string]string{"pp:endpoint": "blockchain.count", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getblockcount"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getblockcount", nil)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
