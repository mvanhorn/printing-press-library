package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newBlockchainHashCmd(flags *rootFlags) *cobra.Command {
	var height int64
	cmd := &cobra.Command{
		Use:         "hash",
		Short:       "Get block hash at height",
		Example:     "  dogecoin-pp-cli blockchain hash --height 6209302",
		Annotations: map[string]string{"pp:endpoint": "blockchain.hash", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				if _, err := fmt.Sscanf(args[0], "%d", &height); err != nil {
					return fmt.Errorf("invalid height %q", args[0])
				}
			}
			if height == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getblockhash"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getblockhash", []any{height})
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().Int64Var(&height, "height", 0, "Block height")
	return cmd
}
