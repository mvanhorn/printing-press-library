// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"

	"github.com/spf13/cobra"
)

type compareReport struct {
	Accounts []string         `json:"accounts"`
	Rows     []icp.CompareRow `json:"rows"`
	Missing  []string         `json:"accounts_without_local_data"`
	Note     string           `json:"note,omitempty"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var (
		accounts string
		dbPath   string
		kinds    string
	)

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare the same kind of program across several gyms at once.",
		Long: strings.Trim(`
Aggregate synced catalogs from multiple accounts into one table.

Program names and camp type ids are assigned per tenant, so no upstream call can
span gyms; this joins the local copies instead. Entities are bucketed by program
name where the portal supplies one and by entity kind otherwise, because two
gyms rarely name the same offering identically.

Accounts with no local data are reported by name rather than silently omitted,
so a missing sync never looks like a gym with nothing to offer.`, "\n"),
		Example:     "  iclasspro-pp-cli compare --accounts scottsdalegymnastics,oasisgymnastics,tigar --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--accounts=scottsdalegymnastics,scaq"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "compare")
			}
			list := make([]string, 0)
			for _, a := range strings.Split(accounts, ",") {
				if a = strings.TrimSpace(a); a != "" {
					list = append(list, a)
				}
			}
			list = append(list, args...)
			if len(list) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--accounts needs at least two portal slugs, e.g. --accounts a,b"))
			}
			wantKinds := icpParseKinds(kinds)
			if wantKinds == nil {
				return usageErr(fmt.Errorf("--kinds accepts class, camp, or both"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			path := icpDBPath(dbPath)
			report := compareReport{Accounts: list, Rows: make([]icp.CompareRow, 0), Missing: make([]string, 0)}
			s, ok, err := icpOpenStoreForRead(ctx, path)
			if err != nil {
				return err
			}
			if !ok {
				return icpNoMirror(cmd.OutOrStdout(), cmd.ErrOrStderr(), flags, path, strings.Join(list, " "), report)
			}
			defer func() { _ = s.Close() }()

			all := make([]icp.Entity, 0)
			for _, acct := range list {
				icpStaleHint(ctx, cmd.ErrOrStderr(), s, flags, acct)
				raw, aerr := icpLatestEntities(ctx, s, acct)
				if aerr != nil {
					return aerr
				}
				ents := icpFilterKinds(raw, wantKinds)
				if len(ents) == 0 {
					report.Missing = append(report.Missing, acct)
					continue
				}
				all = append(all, ents...)
			}
			report.Rows = icp.Compare(all)
			if len(report.Missing) > 0 {
				report.Note = fmt.Sprintf("no local data for: %s — run 'iclasspro-pp-cli sync <account>' for each",
					strings.Join(report.Missing, ", "))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if report.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning:", report.Note)
			}
			if len(report.Rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No synced entities for any of the requested accounts.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ACCOUNT\tBUCKET\tENTITIES\tOPEN\tFULL\tAVG OPENINGS\tAGES")
			for _, r := range report.Rows {
				ages := "-"
				if r.MinAge > 0 || r.MaxAge > 0 {
					ages = fmt.Sprintf("%d-%d", r.MinAge, r.MaxAge)
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
					r.Account, truncate(r.Bucket, 28), r.Entities, r.WithOpenings, r.Full,
					strconv.FormatFloat(r.AvgOpenings, 'f', -1, 64), ages)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&accounts, "accounts", "", "Comma-separated portal slugs to compare")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().StringVar(&kinds, "kinds", "both", "Which entities to compare: class, camp, or both")
	return cmd
}
