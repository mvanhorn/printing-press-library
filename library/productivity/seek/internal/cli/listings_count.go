// Copyright 2026 Paul Siola and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: match count for a filter set without fetching rows.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type listingsCountView struct {
	Keywords       string `json:"keywords,omitempty"`
	Where          string `json:"where,omitempty"`
	Site           string `json:"site"`
	Classification string `json:"classification,omitempty"`
	Worktype       string `json:"worktype,omitempty"`
	Count          int    `json:"count"`
}

func newNovelListingsCountCmd(flags *rootFlags) *cobra.Command {
	var flagKeywords, flagWhere, flagSite, flagClassification, flagWorktype, flagPostedWithin string

	cmd := &cobra.Command{
		Use:   "count",
		Short: "Count matching SEEK jobs for a filter set without fetching rows",
		Long: "Use 'listings count' as a cheap probe — how many jobs match a query — before\n" +
			"committing to a full 'listings search'. It makes one request and reads the total.",
		Example: strings.Trim(`
  seek-pp-cli listings count --keywords "data analyst" --where "Brisbane QLD"
  seek-pp-cli listings count --keywords "nurse" --where "All Australia" --posted-within-days 3`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "listings count")
			}
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			if strings.TrimSpace(flagKeywords) == "" && strings.TrimSpace(flagWhere) == "" && strings.TrimSpace(flagClassification) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide --keywords, --where, or --classification"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			opts := seekSearchOpts{
				Keywords: flagKeywords, Where: flagWhere, SiteKey: flagSite,
				Classification: flagClassification, Worktype: flagWorktype, PageSize: 1,
			}
			if strings.Contains(flagSite, "NZ") {
				opts.Locale = "en-NZ"
			}
			params := opts.params(1)
			if v := strings.TrimSpace(flagPostedWithin); v != "" {
				params["daterange"] = v
			}
			raw, err := c.Get(ctx, seekSearchPath, params)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			var resp seekSearchResponse
			if err := json.Unmarshal(raw, &resp); err != nil {
				return fmt.Errorf("decoding search response: %w", err)
			}
			view := listingsCountView{
				Keywords: flagKeywords, Where: flagWhere,
				Site:           firstNonEmpty(flagSite, "AU-Main"),
				Classification: flagClassification, Worktype: flagWorktype,
				Count: resp.TotalCount,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d matching job(s)\n", view.Count)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagKeywords, "keywords", "", "Keyword query")
	cmd.Flags().StringVar(&flagWhere, "where", "", "Location")
	cmd.Flags().StringVar(&flagSite, "site", "AU-Main", "Marketplace: AU-Main or NZ-Main")
	cmd.Flags().StringVar(&flagClassification, "classification", "", "Classification ID (see 'classifications')")
	cmd.Flags().StringVar(&flagWorktype, "work-type", "", "Work-type ID(s): 242 full-time, 243 part-time, 244 contract, 245 casual")
	cmd.Flags().StringVar(&flagPostedWithin, "posted-within-days", "", "Only jobs listed within the last N days")
	return cmd
}
