// Copyright 2026 bossriceshark. Licensed under Apache-2.0. See LICENSE.
// PATCH: Add agent-friendly live Search Analytics workflows for brand splits and page query breakdowns.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type searchAnalyticsResponse struct {
	Rows []searchAnalyticsRow `json:"rows"`
}

const maxSearchAnalyticsRowLimit = 25000

type searchAnalyticsRow struct {
	Keys        []string `json:"keys"`
	Clicks      float64  `json:"clicks"`
	Impressions float64  `json:"impressions"`
	CTR         float64  `json:"ctr"`
	Position    float64  `json:"position"`
}

type searchMetricSummary struct {
	Clicks      float64 `json:"clicks"`
	Impressions float64 `json:"impressions"`
	CTR         float64 `json:"ctr"`
	Position    float64 `json:"position"`
	Rows        int     `json:"rows"`
}

type classifiedSearchRow struct {
	Query       string  `json:"query,omitempty"`
	Page        string  `json:"page,omitempty"`
	Bucket      string  `json:"bucket,omitempty"`
	Clicks      float64 `json:"clicks"`
	Impressions float64 `json:"impressions"`
	CTR         float64 `json:"ctr"`
	Position    float64 `json:"position"`
}

type searchAnalyticsFilter struct {
	Dimension  string
	Operator   string
	Expression string
}

func newBrandVsNonbrandSplitCmd(flags *rootFlags) *cobra.Command {
	var startDate, endDate, typeFlag, brandRegex string
	var brands []string
	var rowLimit int
	var includeRows bool

	cmd := &cobra.Command{
		Use:         "brand-vs-nonbrand-split <siteUrl>",
		Aliases:     []string{"branded-split", "brand-split"},
		Short:       "Split query performance into branded and non-branded buckets",
		Long:        "Fetches query-level Search Analytics rows for a date range, classifies each query with --brand terms and/or --brand-regex, and returns weighted branded vs non-branded summaries. This is a live API read-only workflow; run sync separately for local-corpus workflows.",
		Example:     "  google-search-console-pp-cli brand-vs-nonbrand-split sc-domain:example.com --brand example --start-date 2026-04-01 --end-date 2026-04-30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if startDate == "" || endDate == "" {
				return usageErr(fmt.Errorf("--start-date and --end-date are required"))
			}
			rowLimit, err := normalizeSearchAnalyticsRowLimit(rowLimit)
			if err != nil {
				return usageErr(err)
			}
			matcher, err := buildBrandMatcher(brands, brandRegex)
			if err != nil {
				return err
			}
			rows, err := runSearchAnalyticsQuery(flags, args[0], startDate, endDate, []string{"query"}, typeFlag, rowLimit, 0, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			branded, nonBranded := searchMetricSummary{}, searchMetricSummary{}
			outRows := []classifiedSearchRow{}
			for _, row := range rows {
				query := searchAnalyticsKeyAt(row, 0)
				bucket := "non_branded"
				if matcher(query) {
					bucket = "branded"
					addSearchSummary(&branded, row)
				} else {
					addSearchSummary(&nonBranded, row)
				}
				if includeRows {
					outRows = append(outRows, classifiedSearchRow{Query: query, Bucket: bucket, Clicks: row.Clicks, Impressions: row.Impressions, CTR: row.CTR, Position: row.Position})
				}
			}
			finalizeSearchSummary(&branded)
			finalizeSearchSummary(&nonBranded)

			result := map[string]any{
				"site_url": args[0],
				"date_range": map[string]string{
					"start_date": startDate,
					"end_date":   endDate,
				},
				"brand_terms": brands,
				"brand_regex": brandRegex,
				"summary": map[string]any{
					"branded":     branded,
					"non_branded": nonBranded,
				},
			}
			addSearchAnalyticsMetadata(result, rows, rowLimit)
			if includeRows {
				result["rows"] = outRows
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringArrayVar(&brands, "brand", nil, "Brand term to classify as branded; repeat for variants")
	cmd.Flags().StringVar(&brandRegex, "brand-regex", "", "Regex used to classify branded queries; evaluated case-insensitively")
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date YYYY-MM-DD")
	cmd.Flags().StringVar(&typeFlag, "type", "WEB", "Search property type (WEB, IMAGE, VIDEO, NEWS, DISCOVER, GOOGLE_NEWS)")
	cmd.Flags().IntVar(&rowLimit, "row-limit", maxSearchAnalyticsRowLimit, "Rows to request from Search Analytics (max 25000)")
	cmd.Flags().BoolVar(&includeRows, "include-rows", false, "Include classified query rows")
	return cmd
}

func newPageQueriesCmd(flags *rootFlags) *cobra.Command {
	var startDate, endDate, typeFlag string
	var rowLimit int

	cmd := &cobra.Command{
		Use:         "page-queries <siteUrl> <pageUrl>",
		Aliases:     []string{"page-query-breakdown"},
		Short:       "Return query performance for one page",
		Long:        "Fetches Search Analytics query rows filtered to a single page URL, returning the queries, page URL, clicks, impressions, CTR, and average position for the requested date range.",
		Example:     "  google-search-console-pp-cli page-queries sc-domain:example.com https://example.com/page --start-date 2026-04-01 --end-date 2026-04-30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if startDate == "" || endDate == "" {
				return usageErr(fmt.Errorf("--start-date and --end-date are required"))
			}
			rowLimit, err := normalizeSearchAnalyticsRowLimit(rowLimit)
			if err != nil {
				return usageErr(err)
			}
			filter := &searchAnalyticsFilter{Dimension: "page", Operator: "equals", Expression: args[1]}
			rows, err := runSearchAnalyticsQuery(flags, args[0], startDate, endDate, []string{"query", "page"}, typeFlag, rowLimit, 0, filter)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			outRows := make([]classifiedSearchRow, 0, len(rows))
			for _, row := range rows {
				outRows = append(outRows, classifiedSearchRow{
					Query:       searchAnalyticsKeyAt(row, 0),
					Page:        searchAnalyticsKeyAt(row, 1),
					Clicks:      row.Clicks,
					Impressions: row.Impressions,
					CTR:         row.CTR,
					Position:    row.Position,
				})
			}
			result := map[string]any{
				"site_url": args[0],
				"page_url": args[1],
				"date_range": map[string]string{
					"start_date": startDate,
					"end_date":   endDate,
				},
				"rows": outRows,
			}
			addSearchAnalyticsMetadata(result, rows, rowLimit)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&startDate, "start-date", "", "Start date YYYY-MM-DD")
	cmd.Flags().StringVar(&endDate, "end-date", "", "End date YYYY-MM-DD")
	cmd.Flags().StringVar(&typeFlag, "type", "WEB", "Search property type (WEB, IMAGE, VIDEO, NEWS, DISCOVER, GOOGLE_NEWS)")
	cmd.Flags().IntVar(&rowLimit, "row-limit", maxSearchAnalyticsRowLimit, "Rows to request from Search Analytics (max 25000)")
	return cmd
}

func runSearchAnalyticsQuery(flags *rootFlags, siteURL, startDate, endDate string, dimensions []string, searchType string, rowLimit, startRow int, filter *searchAnalyticsFilter) ([]searchAnalyticsRow, error) {
	if rowLimit > maxSearchAnalyticsRowLimit {
		rowLimit = maxSearchAnalyticsRowLimit
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	body := map[string]any{
		"startDate":  startDate,
		"endDate":    endDate,
		"dimensions": dimensions,
		"rowLimit":   rowLimit,
		"startRow":   startRow,
	}
	if searchType != "" {
		body["type"] = searchType
	}
	if filter != nil {
		body["dimensionFilterGroups"] = []map[string]any{{
			"filters": []map[string]any{{
				"dimension":  filter.Dimension,
				"operator":   filter.Operator,
				"expression": filter.Expression,
			}},
		}}
	}
	path := replacePathParam("/webmasters/v3/sites/{siteUrl}/searchAnalytics/query", "siteUrl", siteURL)
	data, _, err := c.Post(path, body)
	if err != nil {
		return nil, err
	}
	var resp searchAnalyticsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing search analytics response: %w", err)
	}
	return resp.Rows, nil
}

func normalizeSearchAnalyticsRowLimit(rowLimit int) (int, error) {
	if rowLimit <= 0 {
		return 0, fmt.Errorf("--row-limit must be greater than 0")
	}
	if rowLimit > maxSearchAnalyticsRowLimit {
		return maxSearchAnalyticsRowLimit, nil
	}
	return rowLimit, nil
}

func addSearchAnalyticsMetadata(result map[string]any, rows []searchAnalyticsRow, rowLimit int) {
	result["rows_returned"] = len(rows)
	result["row_limit"] = rowLimit
	result["truncated"] = rowLimit > 0 && len(rows) == rowLimit
}

func buildBrandMatcher(brands []string, brandRegex string) (func(string) bool, error) {
	terms := make([]string, 0, len(brands))
	for _, brand := range brands {
		brand = strings.ToLower(strings.TrimSpace(brand))
		if brand != "" {
			terms = append(terms, brand)
		}
	}

	var re *regexp.Regexp
	if strings.TrimSpace(brandRegex) != "" {
		compiled, err := regexp.Compile("(?i)" + brandRegex)
		if err != nil {
			return nil, err
		}
		re = compiled
	}
	if len(terms) == 0 && re == nil {
		return nil, usageErr(fmt.Errorf("provide at least one --brand or --brand-regex"))
	}

	return func(query string) bool {
		if re != nil && re.MatchString(query) {
			return true
		}
		query = strings.ToLower(query)
		for _, term := range terms {
			if strings.Contains(query, term) {
				return true
			}
		}
		return false
	}, nil
}

func searchAnalyticsKeyAt(row searchAnalyticsRow, idx int) string {
	if idx >= 0 && idx < len(row.Keys) {
		return row.Keys[idx]
	}
	return ""
}

func addSearchSummary(summary *searchMetricSummary, row searchAnalyticsRow) {
	summary.Clicks += row.Clicks
	summary.Impressions += row.Impressions
	summary.Position += row.Position * row.Impressions
	summary.Rows++
}

func finalizeSearchSummary(summary *searchMetricSummary) {
	if summary.Impressions > 0 {
		summary.CTR = summary.Clicks / summary.Impressions
		summary.Position = summary.Position / summary.Impressions
	}
}
