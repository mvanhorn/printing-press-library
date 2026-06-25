// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/kdpnichefinder/internal/kdpsource"
)

func newNichesPromotedCmd(flags *rootFlags) *cobra.Command {
	var flagSearch string
	var flagPage string
	var flagAll bool

	cmd := &cobra.Command{
		Use:         "niches <type>",
		Short:       "Browse a niche bucket: evergreen, fresh_money, hidden_gems, or high_ticket",
		Long:        "Browse a niche bucket: evergreen, fresh_money, hidden_gems, or high_ticket",
		Example:     "  kdpnichefinder-pp-cli niches evergreen",
		Annotations: map[string]string{"pp:endpoint": "niches.browse", "pp:method": "GET", "pp:path": "/app/category/{type}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				if flags.asJSON {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "type is required",
						"usage": fmt.Sprintf("%s <%s>", cmd.CommandPath(), "type"),
					}, flags); printErr != nil {
						return printErr
					}
				}
				return usageErr(fmt.Errorf("type is required\nUsage: %s <%s>", cmd.CommandPath(), "type"))
			}

			bucket := args[0]
			valid := false
			for _, b := range kdpsource.Buckets {
				if b == bucket {
					valid = true
					break
				}
			}
			if !valid {
				return usageErr(fmt.Errorf("unknown bucket %q (valid: %v)", bucket, kdpsource.Buckets))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			page := 1
			if flagPage != "" {
				if p, perr := strconv.Atoi(flagPage); perr == nil && p > 0 {
					page = p
				}
			}

			type bookRow struct {
				ID                      int     `json:"id"`
				Title                   string  `json:"title"`
				ASIN                    string  `json:"asin"`
				Price                   string  `json:"price"`
				Publisher               string  `json:"publisher"`
				EstimatedMonthlySales   int     `json:"estimated_monthly_sales"`
				EstimatedMonthlyRevenue float64 `json:"estimated_monthly_revenue"`
				AmazonURL               string  `json:"amazon_url"`
			}
			out := make([]bookRow, 0)

			maxPages := page
			if flagAll {
				maxPages = paginatedGetMaxPages
			}
			for ; page <= maxPages; page++ {
				raw, err := c.GetWithHeaders(ctx, "/app/category/"+bucket, map[string]string{
					"search": flagSearch,
					"page":   strconv.Itoa(page),
				}, nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				books, _, lastPage, err := kdpsource.ParseDataPage(raw)
				if err != nil {
					return apiErr(fmt.Errorf("parsing %s page %d: %w", bucket, page, err))
				}
				for _, b := range books {
					out = append(out, bookRow{
						ID:                      b.ID,
						Title:                   b.Title,
						ASIN:                    kdpsource.ASIN(b.AmazonURL),
						Price:                   b.Price,
						Publisher:               b.Publisher,
						EstimatedMonthlySales:   b.EstimatedMonthlySales,
						EstimatedMonthlyRevenue: b.EstimatedMonthlyRevenue,
						AmazonURL:               b.AmazonURL,
					})
				}
				if !flagAll || lastPage <= page {
					break
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagSearch, "search", "", "Filter the bucket to titles matching this term")
	cmd.Flags().StringVar(&flagPage, "page", "1", "Page number (Laravel paginator)")
	cmd.Flags().BoolVar(&flagAll, "all", false, "Fetch all pages")

	return cmd
}
