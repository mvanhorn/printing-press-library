package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newMempoolFeesCmd(flags *rootFlags) *cobra.Command {
	var blocks int
	cmd := &cobra.Command{
		Use:         "fees",
		Short:       "Estimate fee per KB for target confirmation blocks",
		Example:     "  dogecoin-pp-cli mempool fees --blocks 6 --json",
		Annotations: map[string]string{"pp:endpoint": "mempool.fees", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"estimatefee"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "estimatefee", []any{blocks})
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&blocks, "blocks", 6, "Target confirmation blocks")
	return cmd
}
