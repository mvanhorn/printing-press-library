// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: entry/exit/passthrough page roles, absorbed from
// frontedu's get_content_performance MCP tool, using v3's native entry/exit
// metric types.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newPagesCmd(flags *rootFlags) *cobra.Command {
	var period string
	var limit int
	cmd := &cobra.Command{
		Use:   "pages [site]",
		Short: "Classify top pages as entry doors, exit doors, or passthrough content",
		Long: `Join the path, entry, and exit breakdowns for one site and classify each
top page by its role: entry-door (high share of entries), exit-door (high
share of exits), or passthrough. Shares are relative to the page's own views.`,
		Example:     "  umami-pp-cli pages restodom.fr --period 30d --limit 15",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574;--period=30d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would classify top pages by entry/exit/passthrough role")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			websiteID, err := resolveWebsiteID(ctx, c, args[0])
			if err != nil {
				return err
			}
			w, err := parsePeriod(period, time.Now())
			if err != nil {
				return usageErr(err)
			}
			metrics, failures := fanoutMetricsTypes(ctx, c, websiteID, w, []string{"path", "entry", "exit"}, 200)
			if len(failures) > 0 {
				return apiErr(fmt.Errorf("fetching page breakdowns: %s", failures[0].Error))
			}
			entries := map[string]float64{}
			for _, r := range metrics["entry"] {
				entries[r.X] = r.Y
			}
			exits := map[string]float64{}
			for _, r := range metrics["exit"] {
				exits[r.X] = r.Y
			}
			type pageRole struct {
				Path         string  `json:"path"`
				Views        float64 `json:"views"`
				EntrySharePct float64 `json:"entry_share_pct"`
				ExitSharePct  float64 `json:"exit_share_pct"`
				Role         string  `json:"role"`
			}
			pages := make([]pageRole, 0, len(metrics["path"]))
			for _, r := range metrics["path"] {
				if r.Y == 0 {
					continue
				}
				entryShare := entries[r.X] / r.Y * 100
				exitShare := exits[r.X] / r.Y * 100
				role := "passthrough"
				switch {
				case entryShare >= 50 && exitShare >= 50:
					role = "landing-and-leaving"
				case entryShare >= 50:
					role = "entry-door"
				case exitShare >= 50:
					role = "exit-door"
				}
				pages = append(pages, pageRole{
					Path: r.X, Views: r.Y,
					EntrySharePct: round1(entryShare), ExitSharePct: round1(exitShare), Role: role,
				})
			}
			sort.Slice(pages, func(i, j int) bool { return pages[i].Views > pages[j].Views })
			if len(pages) > limit {
				pages = pages[:limit]
			}
			out := map[string]any{"website": websiteID, "period": period, "pages": pages}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&period, "period", "30d", "Period (24h, 7d, 30d, this-month, ...)")
	cmd.Flags().IntVar(&limit, "limit", 15, "Max pages returned")
	return cmd
}
