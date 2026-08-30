// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic account whoami command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountWhoamiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "whoami",
		Short:       "Show the active logged-in user, their entities, and tenant",
		Long:        "Show the currently logged-in RapidAPI user: name, email, username, entity, billing provider, and organizations.",
		Example:     "  rapidapi-pp-cli account whoami",
		Annotations: map[string]string{"pp:endpoint": "account.whoami", "pp:method": "POST", "pp:path": "/gateway/graphql"},
		RunE: func(cmd *cobra.Command, args []string) error {
			variables := map[string]any{}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "activeUser", variables, gqlResponsePaths["activeUser"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"name": true, "id": true, "mashapeId": true, "email": true, "username": true, "entity": true, "thumbnail": true, "billingType": true, "paymentProvider": true, "verified": true, "organizations": true})
		},
	}
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
