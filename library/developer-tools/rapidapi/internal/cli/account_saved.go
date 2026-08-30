// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic account saved command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountSavedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "saved",
		Short:       "List APIs you saved as favorites",
		Long:        "List the APIs the logged-in user saved as favorites on the RapidAPI hub.",
		Example:     "  rapidapi-pp-cli account saved",
		Annotations: map[string]string{"pp:endpoint": "account.saved", "pp:method": "POST", "pp:path": "/gateway/graphql"},
		RunE: func(cmd *cobra.Command, args []string) error {
			variables := map[string]any{}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getUserSavedApis", variables, gqlResponsePaths["getUserSavedApis"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"api": true})
		},
	}
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
