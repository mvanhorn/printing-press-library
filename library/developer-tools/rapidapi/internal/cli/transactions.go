// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: transactions command.

package cli

import (
	"github.com/spf13/cobra"
)

func newTransactionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "transactions",
		Short:   "List your API transactions",
		Long:    "List your API transactions. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli transactions",
		Annotations: map[string]string{"pp:endpoint": "transactions.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"input": map[string]any{}}
			data, err := gqlExec(cmd, flags, "getConsumerApiTransactions", variables, "data.getConsumerApiTransactions.rows")
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "name": true})
		},
	}
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")
	return cmd
}
