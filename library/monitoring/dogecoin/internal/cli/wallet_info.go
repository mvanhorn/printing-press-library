package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newWalletInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "info",
		Short:       "Get wallet summary (balance, unconfirmed, immature)",
		Example:     "  dogecoin-pp-cli wallet info --json",
		Annotations: map[string]string{"pp:endpoint": "wallet.info", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getwalletinfo"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getwalletinfo", nil)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
