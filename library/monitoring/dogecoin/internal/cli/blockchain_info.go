package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newBlockchainInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "info",
		Short:       "Get blockchain state (height, difficulty, sync progress)",
		Example:     "  dogecoin-pp-cli blockchain info --json",
		Annotations: map[string]string{"pp:endpoint": "blockchain.info", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getblockchaininfo"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getblockchaininfo", nil)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
