// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic users command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newUsersPromotedCmd(flags *rootFlags) *cobra.Command {
	var username string

	cmd := &cobra.Command{
		Use:     "users [username] [flags]",
		Short:   "Show a user profile and their published APIs",
		Long:    "Show a RapidAPI hub user profile: bio, company, location, joined date, and their published APIs.",
		Example: "  rapidapi-pp-cli users meteostat\n  rapidapi-pp-cli users --username meteostat",
		Annotations: map[string]string{"pp:endpoint": "users.show", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true", "pp:happy-args": "username=meteostat"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !hasChangedLocalFlags(cmd) && len(args) == 0 && !flags.dryRun {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "requires input",
						"usage": cmd.CommandPath() + " --help",
					}, flags); printErr != nil {
						return printErr
					}
					return usageErr(fmt.Errorf("%q requires input; run %q for usage", cmd.CommandPath(), cmd.CommandPath()+" --help"))
				}
				return cmd.Help()
			}
			if len(args) > 0 && username == "" {
				username = args[0]
			}
			if username == "" && !flags.dryRun {
				return fmt.Errorf("username required (positional or --username)")
			}
			variables := map[string]any{"username": username}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getUserProfile", variables, gqlResponsePaths["getUserProfile"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "thumbnail": true, "name": true, "username": true, "bio": true, "company": true, "location": true, "createdAt": true, "publishedApisList": true})
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "Username (also accepted as first positional arg)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
