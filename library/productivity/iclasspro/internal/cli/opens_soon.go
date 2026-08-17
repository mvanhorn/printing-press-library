// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"

	"github.com/spf13/cobra"
)

// opensSoonReport wraps the findings so every response — including the empty
// one a fresh install produces — still names the account and horizon it
// answered for. A bare array leaves an agent unable to tell which account an
// empty result belongs to.
type opensSoonReport struct {
	Account  string       `json:"account"`
	Days     int          `json:"days"`
	Scanned  int          `json:"entities_scanned"`
	Findings []icp.Window `json:"findings"`
	Note     string       `json:"note,omitempty"`
}

func newNovelOpensSoonCmd(flags *rootFlags) *cobra.Command {
	var (
		days   int
		dbPath string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "opens-soon [account]",
		Short: "Find registration that has not opened yet or is about to close, across every synced class and camp.",
		Long: strings.Trim(`
Scan the local mirror for registration windows moving inside the next N days.

Use this command to see registration that has not opened yet or is about to
close. Do NOT use it to watch for a spot in something already open; use 'watch'
instead.

Every record carries registrationStartDate and registrationEndDate, but the
portal API offers no filter on either and the portal UI surfaces neither, so a
camp that opens next Tuesday is invisible until it is already competitive.`, "\n"),
		Example:     "  iclasspro-pp-cli opens-soon scaq --days 14 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<account>=scaq;--days=30"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "opens-soon")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if days < 0 {
				return usageErr(fmt.Errorf("--days must not be negative"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			path := icpDBPath(dbPath)
			report := opensSoonReport{Account: account, Days: days, Findings: make([]icp.Window, 0)}
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
			windows := icp.OpensSoon(ents, icpNow(), days)
			if limit > 0 && len(windows) > limit {
				windows = windows[:limit]
			}
			if len(ents) == 0 {
				report.Note = fmt.Sprintf("no synced entities for %s", account)
				return icpNoLocalData(cmd, flags, report, account)
			}
			report.Scanned = len(ents)
			report.Findings = windows
			if len(windows) == 0 {
				report.Note = fmt.Sprintf("no registration windows opening or closing within %d days across %d synced entities", days, len(ents))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			if len(windows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(),
					"No registration windows opening or closing within %d days for %s (scanned %d synced entities).\n",
					days, account, len(ents))
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "WHEN\tSTATE\tKIND\tNAME\tDATE")
			for _, w := range windows {
				when := fmt.Sprintf("%dd", w.DaysAway)
				if w.DaysAway == 0 {
					when = "today"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", when, w.State, w.Entity.Kind, truncate(w.Entity.Name, 52), w.Date)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().IntVar(&days, "days", 14, "Horizon in days for windows opening or closing")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum findings to return (0 = all)")
	return cmd
}
