// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic collections show command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCollectionsShowCmd(flags *rootFlags) *cobra.Command {
	var flagSlug string

	cmd := &cobra.Command{
		Use:         "show [slug] [flags]",
		Short:       "Show a collection's detail and its APIs",
		Example:     "  rapidapi-pp-cli collections show recommended-apis",
		Annotations: map[string]string{"pp:endpoint": "collections.show", "pp:method": "POST", "pp:path": "/gateway/graphql", "pp:happy-args": "slug=recommended-apis"},
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
			if len(args) > 0 && flagSlug == "" {
				flagSlug = args[0]
			}
			if flagSlug == "" && !flags.dryRun {
				return fmt.Errorf("collection slug required (positional or --slug)")
			}
			variables := map[string]any{"slug": flagSlug}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getCollectionBySlug", variables, gqlResponsePaths["getCollectionBySlug"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "title": true, "thumbnail": true, "shortDescription": true, "longDescription": true, "slugifiedKey": true, "collectionType": true, "apis": true})
		},
	}
	cmd.Flags().StringVar(&flagSlug, "slug", "", "Collection slugified key (also accepted as positional arg)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
