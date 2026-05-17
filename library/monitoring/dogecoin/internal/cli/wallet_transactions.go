package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newWalletTransactionsCmd(flags *rootFlags) *cobra.Command {
	var count int
	var skip int
	cmd := &cobra.Command{
		Use:         "transactions",
		Short:       "List recent wallet transactions",
		Example:     "  dogecoin-pp-cli wallet transactions --count 20 --json",
		Annotations: map[string]string{"pp:endpoint": "wallet.transactions", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"listtransactions"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			// listtransactions [account] [count] [from]
			raw, err := c.Call(context.Background(), "listtransactions", []any{"*", count, skip})
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&count, "count", 10, "Number of transactions to return")
	cmd.Flags().IntVar(&skip, "skip", 0, "Transactions to skip")
	return cmd
}
