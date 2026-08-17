// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "compare <url> [<url>...]",
		Short: "Compare which agent standards several sites implement, side by side",
		Long: "Scan several sites (or read their latest stored scans with --data-source local) and print\n" +
			"a check-by-check matrix of which agent-readiness standards each site implements, plus each\n" +
			"site's readiness level. Use this to see exactly which standards a competitor implemented\n" +
			"that you have not.\n\n" +
			"Use compare for DIFFERENT sites side by side. For one site across time use 'diff' or\n" +
			"'history'.",
		Example: "  isitagentready-pp-cli compare https://example.com https://stripe.com\n" +
			"  isitagentready-pp-cli compare https://example.com https://stripe.com --agent",
		// Not mcp:read-only: in auto/live mode compare scans each site and appends
		// the result to the local scan store (persistScan), an observable side
		// effect (consistent with gate/batch). Any URL scans to a 200 result
		// (siteError for unreachable), so there is no bad-input error path to
		// probe; skip it (see check.go).
		Annotations: map[string]string{"pp:no-error-path-probe": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare agent readiness across sites")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one URL argument is required, e.g. compare https://example.com https://stripe.com"))
			}
			// De-duplicate so the same site passed twice is scanned and
			// persisted once (avoids a double scan + double history row).
			urls := dedupURLs(args)
			// Live dogfood runs under a flat per-command timeout; cap the
			// number of real scans so the matrix fits.
			if cliutil.IsDogfoodEnv() && len(urls) > 2 {
				urls = urls[:2]
			}

			// Scan the sites concurrently (each scan is several seconds) so the
			// matrix returns in roughly one scan's time rather than the sum.
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			results, ferrs := cliutil.FanoutRun(ctx, urls,
				func(u string) string { return u },
				func(c context.Context, u string) (*store.Report, error) {
					raw, err := resolveReportCtx(c, flags, u)
					if err != nil {
						return nil, err
					}
					return store.ParseReport(raw)
				})
			for _, fe := range ferrs {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not scan %s: %v\n", fe.Source, fe.Err)
			}
			reports := make([]*store.Report, 0, len(results))
			for _, r := range results {
				if r.Value != nil {
					reports = append(reports, r.Value)
				}
			}
			if len(reports) == 0 {
				return apiErr(fmt.Errorf("none of the %d site(s) could be scanned", len(urls)))
			}

			result := store.BuildCompare(reports)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return renderCompare(cmd, result)
		},
	}
	return cmd
}

func renderCompare(cmd *cobra.Command, res store.CompareResult) error {
	out := cmd.OutOrStdout()
	tw := newTabWriter(out)

	// Header: check column + one column per site (host + level).
	header := "CHECK"
	for _, s := range res.Sites {
		header += "\t" + store.NormalizeURL(s.URL) + " (L" + itoa(s.Level) + ")"
	}
	fmt.Fprintln(tw, bold(header))

	lastCat := ""
	for _, row := range res.Checks {
		if row.Category != lastCat {
			fmt.Fprintf(tw, "%s\n", bold("["+labelFor(row.Category)+"]"))
			lastCat = row.Category
		}
		line := "  " + row.Check
		for _, s := range res.Sites {
			st := row.Statuses[store.NormalizeURL(s.URL)]
			line += "\t" + statusMark(st)
		}
		fmt.Fprintln(tw, line)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	// Note any sites the scanner could not fetch.
	for _, s := range res.Sites {
		if s.SiteError {
			fmt.Fprintf(out, "\nnote: %s could not be fetched by the scanner (siteError)\n", s.URL)
		}
	}
	return nil
}

func itoa(n int) string { return strings.TrimSpace(fmt.Sprintf("%d", n)) }

// dedupURLs returns urls with later duplicates removed, comparing by
// normalized form so example.com and https://example.com/ count as one.
func dedupURLs(urls []string) []string {
	seen := make(map[string]bool, len(urls))
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		k := store.NormalizeURL(u)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, u)
	}
	return out
}
