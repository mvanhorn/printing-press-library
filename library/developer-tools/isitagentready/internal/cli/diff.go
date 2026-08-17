// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local
package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/store"
)

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var showAll bool

	cmd := &cobra.Command{
		Use:   "diff <url>",
		Short: "Diff a site's two most recent scans (what regressed, fixed, or changed)",
		Long: "Compare the two most recent stored scans of a site and print which checks regressed,\n" +
			"improved, or changed, plus the readiness-level delta. Reads local data only; run\n" +
			"'check <url>' at least twice (before and after a change). Use --all to include unchanged\n" +
			"checks.\n\n" +
			"Use diff for two scans of the SAME site over time. For two DIFFERENT sites at one moment,\n" +
			"use 'compare'.",
		Example: "  isitagentready-pp-cli diff https://example.com\n" +
			"  isitagentready-pp-cli diff https://example.com --agent",
		// An "invalid" URL still has stored siteError scans to diff (exit 0), so
		// there is no bad-input error path to probe; skip it (see check.go).
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		Args:        cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff a site's two most recent scans")
				return nil
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("diff reads local scan history only; it has no live equivalent (remove --data-source live)"))
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a URL argument is required, e.g. diff https://example.com"))
			}
			url := args[0]

			recs, err := loadStore()
			if err != nil {
				return err
			}
			hist := store.HistoryFor(recs, url)
			if len(hist) < 2 {
				note := fmt.Sprintf("need at least 2 scans of %s to diff (have %d); run 'isitagentready-pp-cli check %s' again after changes", url, len(hist), url)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"url": url, "scans": len(hist), "changes": []any{}, "note": note,
					}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}
			fromRec := hist[len(hist)-2]
			toRec := hist[len(hist)-1]
			from, err := store.ParseReport(fromRec.Raw)
			if err != nil {
				return err
			}
			to, err := store.ParseReport(toRec.Raw)
			if err != nil {
				return err
			}

			transitions := store.DiffChecks(from, to)
			shown := transitions
			if !showAll {
				shown = shown[:0:0]
				for _, t := range transitions {
					if t.Change != "unchanged" {
						shown = append(shown, t)
					}
				}
			}

			result := map[string]any{
				"url":        url,
				"from":       map[string]any{"scannedAt": fromRec.ScannedAt, "level": from.Level, "levelName": from.LevelName},
				"to":         map[string]any{"scannedAt": toRec.ScannedAt, "level": to.Level, "levelName": to.LevelName},
				"levelDelta": to.Level - from.Level,
				"changes":    shown,
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return renderDiff(cmd, url, from, to, shown, len(transitions))
		},
	}
	cmd.Flags().BoolVar(&showAll, "all", false, "Include unchanged checks in the output")
	return cmd
}

func renderDiff(cmd *cobra.Command, url string, from, to *store.Report, shown []store.CheckTransition, total int) error {
	out := cmd.OutOrStdout()
	delta := to.Level - from.Level
	arrow := "="
	if delta > 0 {
		arrow = green(fmt.Sprintf("+%d", delta))
	} else if delta < 0 {
		arrow = red(fmt.Sprintf("%d", delta))
	}
	fmt.Fprintf(out, "%s\n", bold(url))
	fmt.Fprintf(out, "  level %d → %d (%s)\n", from.Level, to.Level, arrow)
	if len(shown) == 0 {
		fmt.Fprintf(out, "  no check changes across %d checks\n", total)
		return nil
	}
	for _, t := range shown {
		mark := yellow(t.Change)
		switch t.Change {
		case "regressed", "removed":
			mark = red(t.Change)
		case "improved", "added":
			mark = green(t.Change)
		}
		fmt.Fprintf(out, "  %-10s %s (%s → %s)\n", mark, t.Check, t.From, t.To)
	}
	return nil
}
