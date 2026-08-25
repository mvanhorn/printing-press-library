// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"

	"github.com/spf13/cobra"
)

type lintReport struct {
	Account  string         `json:"account"`
	Entities int            `json:"entities_checked"`
	Findings []icp.Finding  `json:"findings"`
	Counts   map[string]int `json:"counts"`
	Note     string         `json:"note,omitempty"`
}

func newNovelLintCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		severity string
		limit    int
	)

	cmd := &cobra.Command{
		Use:   "lint [account]",
		Short: "Flag catalog quality problems: missing descriptions or images, expired registration windows, deleted-but-listed programs.",
		Long: strings.Trim(`
Audit the synced catalog for records that will render blank, dead, or misleading.

Use this command to audit catalog quality as it stands now. Do NOT use it to see
what changed since the last sync; use 'drift' instead.

Rules fire only on evidence present in the record. Nothing here infers a problem
from a field the portal never populates for that entity family, so a clean
catalog produces no findings at all.`, "\n"),
		Example:     "  iclasspro-pp-cli lint scaq --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<account>=scaq"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "lint")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			sev := strings.ToLower(strings.TrimSpace(severity))
			switch sev {
			case "", "all", "error", "warning", "info":
			default:
				return usageErr(fmt.Errorf("--severity accepts all, error, warning, or info"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			path := icpDBPath(dbPath)
			report := lintReport{Account: account, Findings: make([]icp.Finding, 0), Counts: map[string]int{}}
			s, ok, err := icpOpenStoreForRead(ctx, path)
			if err != nil {
				return err
			}
			if !ok {
				report.Note = fmt.Sprintf("no local mirror yet; run 'iclasspro-pp-cli sync %s'", account)
				return icpNoLocalData(cmd, flags, report, account)
			}
			defer func() { _ = s.Close() }()
			icpStaleHint(ctx, cmd.ErrOrStderr(), s, flags, account)

			ents, err := icpLatestEntities(ctx, s, account)
			if err != nil {
				return err
			}
			if len(ents) == 0 {
				report.Note = fmt.Sprintf("no synced entities for %s", account)
				return icpNoLocalData(cmd, flags, report, account)
			}
			report.Entities = len(ents)
			findings := icp.Lint(ents, icpNow())
			for _, f := range findings {
				report.Counts[f.Severity]++
			}
			if sev != "" && sev != "all" {
				filtered := make([]icp.Finding, 0, len(findings))
				for _, f := range findings {
					if f.Severity == sev {
						filtered = append(filtered, f)
					}
				}
				findings = filtered
			}
			if limit > 0 && len(findings) > limit {
				findings = findings[:limit]
			}
			report.Findings = findings

			if report.Entities == 0 {
				report.Note = fmt.Sprintf("no synced entities for %s; run 'iclasspro-pp-cli sync %s' first", account, account)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if report.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), report.Note)
				return nil
			}
			if len(findings) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Clean: %d entities checked, no findings.\n", report.Entities)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SEVERITY\tRULE\tKIND\tNAME\tDETAIL")
			for _, f := range findings {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					f.Severity, f.Rule, f.Entity, truncate(f.Name, 38), truncate(f.Detail, 56))
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d entities checked: %d errors, %d warnings, %d info\n",
				report.Entities, report.Counts["error"], report.Counts["warning"], report.Counts["info"])
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().StringVar(&severity, "severity", "", "Only show findings at this severity: error, warning, info")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum findings to return (0 = all)")
	return cmd
}
