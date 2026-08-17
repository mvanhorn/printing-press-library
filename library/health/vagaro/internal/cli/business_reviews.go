// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Phase 3 foundation: business reviews via the internal/vagaro
// sibling client.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newBusinessReviewsCmd(flags *rootFlags) *cobra.Command {
	var provider string
	var limit int

	cmd := &cobra.Command{
		Use:     "reviews <slug>",
		Short:   "List a business's customer reviews",
		Example: "  vagaro-pp-cli business reviews centralbarber --limit 5",
		// pp:data-source live
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "slug=centralbarber"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			slug := strings.Trim(strings.TrimSpace(args[0]), "/")
			if slug == "" {
				return usageErr(fmt.Errorf("slug is required\nUsage: %s <slug>", cmd.CommandPath()))
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c := newVagaroClient(flags)
			businessID, err := resolveBusinessID(ctx, c, flags, slug)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			pageSize := limit
			if pageSize <= 0 {
				pageSize = 20
			}
			reviews, err := c.Reviews(ctx, businessID, strings.TrimSpace(provider), pageSize)
			if err != nil {
				return classifyVagaroError(err, flags)
			}
			if limit > 0 && len(reviews) > limit {
				reviews = reviews[:limit]
			}
			return emitVagaro(cmd, flags, reviews)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "Filter to one provider's reviews (ServiceProviderId)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max reviews to return (default 20)")
	return cmd
}
