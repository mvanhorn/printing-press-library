// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newNovelOpenAdviceCmd(flags *rootFlags) *cobra.Command {
	var site string
	var checkID string

	cmd := &cobra.Command{
		Use:   "open-advice",
		Short: "List every still-failing check across all your scanned sites with its fix prompt",
		Long: "Read your local scan history and, for each site's most recent scan, list the checks that\n" +
			"are still failing along with the fix prompt for each. This is the cross-site backlog of\n" +
			"what is left to do, which the stateless web UI cannot show. Narrow with --site or --check.\n" +
			"Reads local data only; run 'check <url>' on your sites first.",
		Example: "  isitagentready-pp-cli open-advice\n" +
			"  isitagentready-pp-cli open-advice --site https://example.com --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// open-advice has no required input (it summarizes the whole store),
			// so a bare invocation runs rather than printing help.
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list still-open fixes across stored scans")
				return nil
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("open-advice reads local scan history only; it has no live equivalent (remove --data-source live)"))
			}

			recs, err := loadStore()
			if err != nil {
				return err
			}
			if len(recs) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no scans yet; run: isitagentready-pp-cli check <url>")
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}

			items := store.OpenAdvice(store.LatestPerURL(recs), site, checkID)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), items, flags)
			}
			return renderOpenAdvice(cmd, items)
		},
	}
	cmd.Flags().StringVar(&site, "site", "", "Limit to one site (URL)")
	cmd.Flags().StringVar(&checkID, "check", "", "Limit to one check id (e.g. mcpServerCard)")
	return cmd
}

func renderOpenAdvice(cmd *cobra.Command, items []store.OpenItem) error {
	out := cmd.OutOrStdout()
	if len(items) == 0 {
		fmt.Fprintln(out, "No open fixes: every scanned site is at its top listed level (or your filter matched nothing).")
		return nil
	}
	bySite := map[string][]store.OpenItem{}
	var order []string
	for _, it := range items {
		if _, ok := bySite[it.URL]; !ok {
			order = append(order, it.URL)
		}
		bySite[it.URL] = append(bySite[it.URL], it)
	}
	for _, url := range order {
		list := bySite[url]
		fmt.Fprintf(out, "%s  (level %d — %s, %d open)\n", bold(url), list[0].Level, list[0].LevelName, len(list))
		for _, it := range list {
			fmt.Fprintf(out, "  %s %s\n", red("[fail]"), it.Check)
			if it.Description != "" {
				fmt.Fprintf(out, "     %s\n", it.Description)
			}
		}
	}
	fmt.Fprintf(out, "\n%d open fix(es) across %d site(s). Run 'isitagentready-pp-cli advice <url> --copy' for the prompts.\n", len(items), len(order))
	return nil
}
