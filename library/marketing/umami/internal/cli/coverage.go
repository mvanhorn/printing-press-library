// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: tracker health check. Flags sites with no traffic in the
// recent window and reports when each silent site last received data.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var hours int
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Flag sites whose tracking script went silent, with the timestamp of their last data",
		Long: `Check every website for tracking coverage: any site with zero pageviews in
the recent window is flagged, and its collected-data date range is fetched
to show when the tracker last reported. A dead tracker on a many-site
portfolio otherwise goes unnoticed for weeks.

Use this command to detect client sites whose tracking script stopped
sending data. Do NOT use it to check CLI auth or API connectivity; use
'doctor' instead.`,
		Example:     "  umami-pp-cli coverage --json\n  umami-pp-cli coverage --hours 48",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--hours=48"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would check every site for tracking coverage")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sites, err := fetchAllWebsites(ctx, c)
			if err != nil {
				return err
			}
			now := time.Now()
			w := periodWindow{ms(now.Add(-time.Duration(hours) * time.Hour)), ms(now)}
			results := fanoutStats(ctx, c, sites, w, "")
			ok, failures := splitStatsResults(results)
			type silentSite struct {
				Name       string `json:"name"`
				Domain     string `json:"domain"`
				ID         string `json:"id"`
				LastData   string `json:"last_data"`
				SilentDays float64 `json:"silent_days"`
			}
			healthy := 0
			silent := make([]silentSite, 0)
			for _, r := range ok {
				if r.Stats.Pageviews > 0 {
					healthy++
					continue
				}
				entry := silentSite{Name: r.Site.Name, Domain: r.Site.Domain, ID: r.Site.ID}
				if data, err := c.Get(ctx, "/api/websites/"+r.Site.ID+"/daterange", nil); err == nil {
					var dr struct {
						EndDate string `json:"endDate"`
						Maxdate string `json:"maxdate"`
					}
					if json.Unmarshal(data, &dr) == nil {
						last := dr.EndDate
						if last == "" {
							last = dr.Maxdate
						}
						entry.LastData = last
						if t, err := time.Parse(time.RFC3339, last); err == nil {
							entry.SilentDays = round1(now.Sub(t).Hours() / 24)
						}
					}
				}
				silent = append(silent, entry)
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d site checks failed; coverage computed over the remaining %d\n", len(failures), len(sites), healthy+len(silent))
			}
			out := map[string]any{
				"window_hours":   hours,
				"checked_sites":  healthy + len(silent),
				"healthy_sites":  healthy,
				"silent_sites":   silent,
				"fetch_failures": failures,
			}
			if len(silent) == 0 && len(failures) == 0 {
				out["note"] = "every tracker reported data in the window"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&hours, "hours", 24, "Window (hours) a site may be silent before being flagged")
	return cmd
}
