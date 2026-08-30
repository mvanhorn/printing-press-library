// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: certificates command.

package cli

import (
	"github.com/spf13/cobra"
)

func newCertificatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "certificates",
		Short:   "List your API certificates",
		Long:    "List your API certificates. Data comes from the RapidAPI hub GraphQL gateway.",
		Example: "  rapidapi-pp-cli certificates",
		Annotations: map[string]string{"pp:endpoint": "certificates.list", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := "/gateway/graphql"
			_ = path
			variables := map[string]any{"providersId": []string{}}
			data, err := gqlExec(cmd, flags, "getCertificates", variables, "data.apiCertificates.nodes")
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
