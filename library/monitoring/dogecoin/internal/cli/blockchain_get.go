package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newBlockchainGetCmd(flags *rootFlags) *cobra.Command {
	var verbose int
	cmd := &cobra.Command{
		Use:         "get [hash]",
		Short:       "Get block data by hash",
		Example:     "  dogecoin-pp-cli blockchain get 2ad5eaa2fb529ec787889b7c1d1abc70a3a3ce586fc8d0c2658fd2ffc68c4e62 --json",
		Annotations: map[string]string{"pp:endpoint": "blockchain.get", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"call":"getblock"}`)
				return nil
			}
			c, err := flags.newRPCClient()
			if err != nil {
				return err
			}
			raw, err := c.Call(context.Background(), "getblock", []any{args[0], verbose})
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), raw, flags)
		},
	}
	cmd.Flags().IntVar(&verbose, "verbose", 1, "Verbosity: 0=hex, 1=json, 2=json+tx")
	return cmd
}
