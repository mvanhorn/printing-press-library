// Copyright 2026 David Barbier and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored composite: realtime activity vs same-time-yesterday baseline,
// absorbed from frontedu's get_realtime_vs_baseline MCP tool.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newPulseCmd(flags *rootFlags) *cobra.Command {
	var timezone string
	cmd := &cobra.Command{
		Use:   "pulse [site]",
		Short: "Realtime activity vs the same 30-minute window yesterday — spike or drop verdict",
		Long: `Compare the last 30 minutes of visitors against the same 30-minute window
yesterday and report a spike/drop/normal verdict.

Use this command for last-30-minutes realtime spike checks. Do NOT use it
for daily/weekly anomaly detection across sites; use 'watch' instead.`,
		Example:     "  umami-pp-cli pulse restodom.fr",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "site=f1fa838a-fc6a-49c1-bdef-c3688dc92574"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare realtime activity against yesterday's baseline")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("site argument (UUID or domain) is required"))
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
			data, err := c.Get(ctx, "/api/realtime/"+websiteID, map[string]string{"timezone": timezone})
			if err != nil {
				return apiErr(fmt.Errorf("fetching realtime: %w", err))
			}
			var rt struct {
				Totals struct {
					Views    float64 `json:"views"`
					Visitors float64 `json:"visitors"`
					Events   float64 `json:"events"`
				} `json:"totals"`
			}
			if err := json.Unmarshal(data, &rt); err != nil {
				return apiErr(fmt.Errorf("parsing realtime: %w", err))
			}
			// Baseline: the same 30-minute window yesterday, via a stats call.
			now := time.Now()
			base := periodWindow{ms(now.Add(-24*time.Hour - 30*time.Minute)), ms(now.Add(-24 * time.Hour))}
			baseline, err := getStats(ctx, c, websiteID, base, "")
			if err != nil {
				return apiErr(fmt.Errorf("fetching baseline window: %w", err))
			}
			verdict := "normal"
			deviation := 0.0
			if baseline.Visitors > 0 {
				deviation = pctChange(rt.Totals.Visitors, baseline.Visitors)
				switch {
				case deviation >= 100:
					verdict = "spike"
				case deviation <= -60:
					verdict = "drop"
				}
			} else if rt.Totals.Visitors > 5 {
				verdict = "spike"
			}
			out := map[string]any{
				"website":            websiteID,
				"realtime_visitors":  rt.Totals.Visitors,
				"realtime_views":     rt.Totals.Views,
				"baseline_visitors":  baseline.Visitors,
				"deviation_pct":      round1(deviation),
				"verdict":            verdict,
				"baseline_window":    "same 30 minutes yesterday",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&timezone, "timezone", "UTC", "IANA timezone")
	return cmd
}
