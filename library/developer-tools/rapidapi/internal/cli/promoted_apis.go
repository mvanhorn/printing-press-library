// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic apis command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newApisPromotedCmd(flags *rootFlags) *cobra.Command {
	var owner string
	var api string
	var withEndpoints bool

	cmd := &cobra.Command{
		Use:     "apis [OWNER/API] [flags]",
		Short:   "Show a single API's full detail: endpoints, versions, billing plans, rating, owner",
		Long:    "Show a RapidAPI hub API by owner/slug: description, endpoints (route/method), versions, billing plans, rating, quality score, subscriptions count, and website.",
		Example: "  rapidapi-pp-cli apis meteostat/meteostat\n  rapidapi-pp-cli apis --owner meteostat --api meteostat --with-endpoints",
		Annotations: map[string]string{"pp:endpoint": "apis.show", "pp:method": "POST", "pp:path": "/gateway/graphql", "mcp:read-only": "true", "pp:happy-args": "owner=meteostat;api=meteostat"},
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
			if len(args) == 1 && owner == "" {
				// Accept "owner/api" or "owner-slug/api-slug"
				part := args[0]
				for i := 0; i < len(part); i++ {
					if part[i] == '/' {
						owner = part[:i]
						api = part[i+1:]
						break
					}
				}
				if api == "" {
					return fmt.Errorf("expected OWNER/API format, got %q", part)
				}
			} else if len(args) == 2 {
				if owner == "" {
					owner = args[0]
				}
				if api == "" {
					api = args[1]
				}
			}
			if owner == "" || api == "" {
				if !flags.dryRun {
					return fmt.Errorf("owner and api required (positional OWNER/API or --owner/--api)")
				}
			}
			variables := map[string]any{"apiOwnerSlug": owner, "apiSlug": api, "withEndpoints": withEndpoints}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getApiBySlugAndOwner", variables, gqlResponsePaths["getApiBySlugAndOwner"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "name": true, "title": true, "description": true, "longDescription": true, "slugifiedName": true, "thumbnail": true, "apiType": true, "status": true, "createdAt": true, "quality": true, "owner": true, "versions": true, "version": true, "billingPlans": true, "rating": true, "subscriptionsCount": true, "websiteUrl": true})
		},
	}
	cmd.Flags().StringVar(&owner, "owner", "", "API owner slugified name (also accepted as OWNER/API positional)")
	cmd.Flags().StringVar(&api, "api", "", "API slugified name")
	cmd.Flags().BoolVar(&withEndpoints, "with-endpoints", false, "Include the API's endpoints (route, method, name)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
