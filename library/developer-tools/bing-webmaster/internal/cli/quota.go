// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.
//
// quota: unified URL + content submission quota with a pacing recommendation.
// Hand-authored transcendence command.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type bQuotaSide struct {
	DailyRemaining   int `json:"daily_remaining"`
	MonthlyRemaining int `json:"monthly_remaining"`
}

type bQuotaResult struct {
	Site    string     `json:"site"`
	URL     bQuotaSide `json:"url"`
	Content bQuotaSide `json:"content"`
	Pacing  struct {
		RecommendedPerHour int `json:"recommended_per_hour"`
	} `json:"pacing"`
}

func newQuotaCmd(flags *rootFlags) *cobra.Command {
	var site string
	cmd := &cobra.Command{
		Use:         "quota",
		Short:       "Remaining URL + content submission quota and a safe pacing recommendation",
		Long:        "Combine the URL and content submission quotas into one view (daily and monthly remaining) plus a recommended hourly submission rate, so you can size a bulk push without hitting the wall. Quota resets at midnight GMT.",
		Example:     "  bing-webmaster-pp-cli quota --site https://example.com",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			proceed, err := siteOrDryRun(cmd, flags, site, fmt.Sprintf("would read URL + content quota for %q", site))
			if err != nil || !proceed {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var result bQuotaResult
			result.Site = site

			if data, err := c.Get(cmd.Context(), "/json/GetUrlSubmissionQuota", map[string]string{"siteUrl": site}); err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			} else {
				m := bCIMap(data)
				d, _ := bNum(m, "DailyQuota")
				mo, _ := bNum(m, "MonthlyQuota")
				result.URL = bQuotaSide{DailyRemaining: int(d), MonthlyRemaining: int(mo)}
			}
			if data, err := c.Get(cmd.Context(), "/json/GetContentSubmissionQuota", map[string]string{"siteUrl": site}); err != nil {
				// Content quota is less universally available; treat its failure as non-fatal.
				result.Content = bQuotaSide{}
			} else {
				m := bCIMap(data)
				d, _ := bNum(m, "DailyQuota")
				mo, _ := bNum(m, "MonthlyQuota")
				result.Content = bQuotaSide{DailyRemaining: int(d), MonthlyRemaining: int(mo)}
			}
			if result.URL.DailyRemaining > 0 {
				per := result.URL.DailyRemaining / 24
				if per < 1 {
					per = 1
				}
				result.Pacing.RecommendedPerHour = per
			}
			return emitIntel(cmd, flags, result, func() {
				fmt.Fprintf(cmd.OutOrStdout(), "Submission quota for %s\n", site)
				fmt.Fprintf(cmd.OutOrStdout(), "  URL     daily %d   monthly %d\n", result.URL.DailyRemaining, result.URL.MonthlyRemaining)
				fmt.Fprintf(cmd.OutOrStdout(), "  content daily %d   monthly %d\n", result.Content.DailyRemaining, result.Content.MonthlyRemaining)
				fmt.Fprintf(cmd.OutOrStdout(), "  pace    ~%d URLs/hour\n", result.Pacing.RecommendedPerHour)
			})
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Verified site URL (required)")
	return cmd
}
