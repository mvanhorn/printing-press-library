package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newMiningInfoCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "info",
		Short:       "Get mining info (difficulty, hashrate, mempool)",
		Example:     "  dogecoin-pp-cli mining info --json",
		Annotations: map[string]string{"pp:endpoint": "mining.info", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getmininginfo"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getmininginfo", nil)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
