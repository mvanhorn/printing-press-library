// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command `cost` — pre-flight credit estimator. Pure local
// lookup against the documented Scrape.do cost table (datacenter 1, +render 5,
// super 10, super+render 25, Google 10) plus per-domain overrides. Spends ZERO
// credits and makes no API call, so an agent can choose the cheapest mode that
// works before committing. Hand file (no generator header) so it survives regen.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelCostCmd(flags *rootFlags) *cobra.Command {
	var flagURL string
	var flagRender bool
	var flagSuper bool
	var flagGoogle bool

	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Estimate the credit cost of a scrape before spending — no API call, no credits",
		Long: `Print the exact credit cost a request will incur, accounting for JS render,
super (residential/mobile) proxy, Google endpoints, and documented per-domain
overrides. Makes no API call and spends no credits — use it to pick the
cheapest mode that works before dispatching a scrape or batch.

Cost table: datacenter=1, datacenter+render=5, super=10, super+render=25,
any Google endpoint=10. Domain overrides (e.g. linkedin.com=30) supersede the
proxy/render matrix.`,
		Example: "  scrape-do-pp-cli cost --url https://www.linkedin.com/company/example --render --super\n  scrape-do-pp-cli cost --google\n  scrape-do-pp-cli cost --url https://example.com --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--url=https://example.com",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagURL == "" && !flagGoogle {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			var credits int
			var mode string
			var target string
			if flagGoogle {
				credits, mode, target = 10, modeGoogle, "google endpoint"
			} else {
				credits, mode = estimateScrapeCost(flagURL, flagRender, flagSuper)
				target = flagURL
			}
			payload := map[string]any{
				"target":            target,
				"mode":              mode,
				"render":            flagRender,
				"super":             flagSuper,
				"estimated_credits": credits,
				"billed_on":         "success only (2xx/400-target/404/410); 401/429/502/510 are free",
			}
			text := fmt.Sprintf("estimated cost: %d credit(s)  [mode=%s]  target=%s", credits, mode, target)
			return emitGov(cmd, flags, payload, text)
		},
	}
	cmd.Flags().StringVar(&flagURL, "url", "", "Target URL to estimate a scrape for")
	cmd.Flags().BoolVar(&flagRender, "render", false, "Estimate with JS rendering enabled (raises cost)")
	cmd.Flags().BoolVar(&flagSuper, "super", false, "Estimate with the super (residential/mobile) proxy (raises cost)")
	cmd.Flags().BoolVar(&flagGoogle, "google", false, "Estimate a Google Scraper endpoint call (flat 10 credits)")
	return cmd
}
