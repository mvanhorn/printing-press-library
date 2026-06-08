// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type snapshotResult struct {
	Target    string         `json:"target"`
	Country   string         `json:"country,omitempty"`
	Date      string         `json:"date"`
	Authority map[string]any `json:"authority"`
	Backlinks map[string]any `json:"backlinks"`
	Organic   map[string]any `json:"organic"`
	Warnings  []string       `json:"warnings,omitempty"`
}

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	var flagTarget string
	var flagCountry string
	var flagDate string

	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Build a Site Explorer report card for one target",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  ahrefs-pp-cli snapshot --target bestself.co --country us
  ahrefs-pp-cli snapshot --target bestself.co --country us --date 2026-06-03`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagTarget == "" && !flags.dryRun {
				return fmt.Errorf("required flag %q not set", "target")
			}
			if flagDate == "" {
				flagDate = todayUTCDate()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := snapshotResult{
				Target:   flagTarget,
				Country:  flagCountry,
				Date:     flagDate,
				Warnings: []string{},
			}
			provs := []DataProvenance{}

			authorityParams := map[string]string{
				"target":   flagTarget,
				"protocol": "both",
				"date":     flagDate,
			}
			authority, prov, err := fetchCompositeObject(cmd, c, flags, "/site-explorer/domain-rating", authorityParams)
			provs = append(provs, prov)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("domain-rating: %v", err))
			} else {
				result.Authority = keepSnapshotFields(authority, "domain_rating", "ahrefs_rank")
			}

			backlinksParams := map[string]string{
				"target":   flagTarget,
				"protocol": "both",
				"mode":     "subdomains",
				"date":     flagDate,
			}
			backlinks, prov, err := fetchCompositeObject(cmd, c, flags, "/site-explorer/backlinks-stats", backlinksParams)
			provs = append(provs, prov)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("backlinks-stats: %v", err))
			} else {
				result.Backlinks = keepSnapshotFields(backlinks, "live", "all_time", "live_refdomains", "all_time_refdomains")
			}

			metricsParams := map[string]string{
				"target":      flagTarget,
				"protocol":    "both",
				"mode":        "subdomains",
				"date":        flagDate,
				"volume_mode": "monthly",
			}
			if flagCountry != "" {
				metricsParams["country"] = flagCountry
			}
			organic, prov, err := fetchCompositeObject(cmd, c, flags, "/site-explorer/metrics", metricsParams)
			provs = append(provs, prov)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("metrics: %v", err))
			} else {
				result.Organic = keepSnapshotFields(organic, "org_keywords", "org_traffic", "org_cost", "paid_keywords", "paid_traffic")
			}
			if flags.dryRun {
				return nil
			}
			if len(result.Warnings) == 0 {
				result.Warnings = nil
			}
			return printCompositeOutputWithCompact(cmd, result, compactSnapshotResult(result), 1, mergeCompositeProvenance(provs...), flags)
		},
	}
	cmd.Flags().StringVar(&flagTarget, "target", "", "Target domain or URL.")
	cmd.Flags().StringVar(&flagCountry, "country", "", "A two-letter country code (ISO 3166-1 alpha-2).")
	cmd.Flags().StringVar(&flagDate, "date", "", "Snapshot date in YYYY-MM-DD format. Defaults to today's UTC date.")
	return cmd
}

func keepSnapshotFields(obj map[string]any, fields ...string) map[string]any {
	if obj == nil {
		return nil
	}
	out := make(map[string]any, len(fields))
	for _, field := range fields {
		out[field] = obj[field]
	}
	return out
}

func compactSnapshotResult(result snapshotResult) map[string]any {
	compact := map[string]any{
		"domain_rating":   nil,
		"org_traffic":     nil,
		"live_refdomains": nil,
	}
	if result.Authority != nil {
		compact["domain_rating"] = result.Authority["domain_rating"]
	}
	if result.Organic != nil {
		compact["org_traffic"] = result.Organic["org_traffic"]
	}
	if result.Backlinks != nil {
		compact["live_refdomains"] = result.Backlinks["live_refdomains"]
	}
	if len(result.Warnings) > 0 {
		compact["warnings"] = result.Warnings
	}
	return compact
}
