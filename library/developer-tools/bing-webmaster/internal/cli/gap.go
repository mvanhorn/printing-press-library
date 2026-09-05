// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// gap: Bing-vs-Google coverage reconciliation. Joins live Bing query data with
// a Google Search Console performance CSV export to find queries you rank for
// on one engine but not the other. Hand-authored transcendence command.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newGapCmd(flags *rootFlags) *cobra.Command {
	var site string
	var gscPath string
	cmd := &cobra.Command{
		Use:         "gap",
		Short:       "Reconcile Bing queries against a Google Search Console export to find cross-engine gaps",
		Long:        "Compare your Bing query coverage against a Google Search Console performance CSV export (--gsc). Surfaces queries present on Bing but not Google, and on Google but not Bing — the low-effort wins one engine already validates. No extra OAuth: you supply the GSC export file.",
		Example:     "  bing-webmaster-pp-cli gap --site https://example.com --gsc ./gsc-queries.csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				cmd.Println(fmt.Sprintf("would reconcile Bing queries for %q against GSC export %q", site, gscPath))
				return nil
			}
			if site == "" {
				return errRequiredFlag("site")
			}
			if gscPath == "" {
				return fmt.Errorf("required flag %q not set (path to a Google Search Console performance CSV export)", "gsc")
			}
			f, err := os.Open(gscPath)
			if err != nil {
				return fmt.Errorf("opening GSC export %q: %w", gscPath, err)
			}
			defer f.Close()
			gscQueries, err := bParseGSCQueries(f)
			if err != nil {
				return fmt.Errorf("parsing GSC export: %w", err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get(cmd.Context(), "/json/GetQueryStats", map[string]string{"siteUrl": site})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			bingRows := bParseQueryRows(data)
			bingQueries := make([]string, 0, len(bingRows))
			for _, r := range bingRows {
				bingQueries = append(bingQueries, r.Query)
			}

			result := bComputeGap(bingQueries, gscQueries)
			return emitIntel(cmd, flags, result, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "Coverage gap for %s\n", site)
				fmt.Fprintf(cmd.OutOrStdout(), "  on both: %d   bing-only: %d   google-only: %d\n",
					result.Summary.BothCount, result.Summary.BingOnlyCount, result.Summary.GoogleOnlyCount)
				if len(bingQueries) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "  (Bing returned no query data — common below its data threshold)")
				}
			})
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Verified site URL (required)")
	cmd.Flags().StringVar(&gscPath, "gsc", "", "Path to a Google Search Console performance CSV export (required)")
	return cmd
}
