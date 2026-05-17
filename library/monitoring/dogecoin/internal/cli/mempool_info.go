package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newMempoolInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "info",
		Short:       "Get mempool statistics (size, bytes)",
		Example:     "  dogecoin-pp-cli mempool info --json",
		Annotations: map[string]string{"pp:endpoint": "mempool.info", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getmempoolinfo"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getmempoolinfo", nil)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
