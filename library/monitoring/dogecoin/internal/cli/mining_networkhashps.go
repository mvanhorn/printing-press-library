package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newMiningNetworkhashpsCmd(flags *rootFlags) *cobra.Command {
	var blocks int
	cmd := &cobra.Command{
		Use:         "networkhashps",
		Short:       "Get network hash rate in H/s",
		Example:     "  dogecoin-pp-cli mining networkhashps --json",
		Annotations: map[string]string{"pp:endpoint": "mining.networkhashps", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getnetworkhashps"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			params := []any{}
			if blocks > 0 {
				params = []any{blocks}
			}
			raw, err := c.Call(context.Background(), "getnetworkhashps", params)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&blocks, "blocks", 0, "Blocks to average (default 120)")
	return cmd
}
