// Copyright 2026 jmbernabotto. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

func newDisruptionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disruptions",
		Short: "Query disruptions and service alerts",
	}
	cmd.AddCommand(newDisruptionsDigestCmd(flags))
	cmd.AddCommand(newDisruptionsListCmd(flags))
	return cmd
}

func newDisruptionsListCmd(flags *rootFlags) *cobra.Command {
	var coverage, uri string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List disruptions for a coverage region",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  sncf-connect-pp-cli disruptions list --coverage sncf
  sncf-connect-pp-cli disruptions list --coverage sncf --uri stop_area:OCE:SA:87391003`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var path string
			if uri != "" {
				path = "/coverage/" + coverage + "/" + uri + "/disruptions"
			} else {
				path = "/coverage/" + coverage + "/disruptions"
			}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "disruptions", true, path, nil, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			printProvenance(cmd, -1, prov)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&coverage, "coverage", "sncf", "Navitia coverage region")
	cmd.Flags().StringVar(&uri, "uri", "", "Filter to disruptions affecting a specific stop area or line URI")
	return cmd
}
