// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: ergonomic account subscriptions command for the RapidAPI hub GraphQL gateway.

package cli

import (
	"github.com/spf13/cobra"
)

func newAccountSubscriptionsCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var status string

	cmd := &cobra.Command{
		Use:         "subscriptions",
		Short:       "List your API subscriptions with plan details and status",
		Long:        "List the logged-in user's API subscriptions: plan name, price, period, status, and API.",
		Example:     "  rapidapi-pp-cli account subscriptions --limit 20",
		Annotations: map[string]string{"pp:endpoint": "account.subscriptions", "pp:method": "POST", "pp:path": "/gateway/graphql", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			statuses := []string{}
			if status != "" {
				statuses = []string{status}
			}
			variables := map[string]any{"input": map[string]any{"statuses": statuses, "limit": limit}}
			path := "/gateway/graphql"
			_ = path
			data, err := gqlExec(cmd, flags, "getApiSubscriptions", variables, gqlResponsePaths["getApiSubscriptions"])
			if err != nil {
				return err
			}
			return gqlOutput(cmd, flags, data, map[string]bool{"id": true, "status": true, "userId": true, "apiId": true, "billingPlanVersion": true})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of subscriptions")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status (ACTIVE, BLOCKED, CANCELLED, ...)")
	cmd.Flags().String("query", "", "Raw GraphQL query override (advanced)")
	cmd.Flags().String("variables", "", "Raw GraphQL variables override (advanced)")

	return cmd
}
