// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: invoices command.

package cli

import (
	"github.com/spf13/cobra"
)

func newInvoicesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "invoices",
		Short:   "Show invoice details",
		Long:    "Show invoice details. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli invoices --id inv_x",
		Annotations: map[string]string{"pp:endpoint": "invoices.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"input": map[string]any{"invoiceId": "inv_x"}}
			data, err := gqlExec(cmd, flags, "getInvoice", variables, "data.getInvoice")
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
