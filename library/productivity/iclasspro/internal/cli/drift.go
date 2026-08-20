// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"

	"github.com/spf13/cobra"
)

type driftReport struct {
	Account   string       `json:"account"`
	FromRun   int64        `json:"from_run"`
	ToRun     int64        `json:"to_run"`
	FromTime  string       `json:"from_time"`
	ToTime    string       `json:"to_time"`
	Changes   []icp.Change `json:"changes"`
	Compared  int          `json:"entities_compared"`
	Note      string       `json:"note,omitempty"`
	RunsFound int          `json:"runs_found"`
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var (
		since  string
		dbPath string
		kinds  string
	)

	cmd := &cobra.Command{
		Use:   "drift [account]",
		Short: "See what changed between syncs: classes and camps added, removed, retimed, or newly marked deleted.",
		Long: strings.Trim(`
Compare the two most relevant local snapshots and report every catalog change.

Use this command for what CHANGED between syncs. Do NOT use it to judge whether
the catalog is well-formed right now; use 'lint' instead.

The portal API has no updated-at cursor, no history, and no deletion feed, so
this is computed entirely from observations recorded by 'sync'. It needs at
least two syncs before it can say anything.

A snapshot that recorded zero entities is never treated as the current state, so
a failed or gated sync cannot be misread as the whole catalog being deleted.`, "\n"),
		Example:     "  iclasspro-pp-cli drift scaq --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "<account>=scaq;--since=30d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "drift")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			var cutoff time.Time
			if strings.TrimSpace(since) != "" {
				d, perr := cliutil.ParseDurationLoose(since)
				if perr != nil {
					return usageErr(fmt.Errorf("--since %q is not a duration (try 7d, 24h, 1w): %w", since, perr))
				}
				cutoff = icpNow().Add(-d)
			}

			wantKinds := icpParseKinds(kinds)
			if wantKinds == nil {
				return usageErr(fmt.Errorf("--kinds accepts class, camp, or both"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			path := icpDBPath(dbPath)
			report := driftReport{Account: account, Changes: make([]icp.Change, 0)}
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

			runs, err := s.ICPRuns(ctx, account, 200)
			if err != nil {
				return err
			}
			report.RunsFound = len(runs)
			if len(runs) == 0 {
				report.Note = fmt.Sprintf("no synced snapshots for %s", account)
				return icpNoLocalData(cmd, flags, report, account)
			}
			if len(runs) < 2 {
				report.Note = fmt.Sprintf(
					"drift needs at least two non-empty syncs of %s to compare; %d recorded so far. Run 'iclasspro-pp-cli sync %s' again later.",
					account, len(runs), account)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), report, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), report.Note)
				return nil
			}

			// runs[0] is newest. The baseline is the newest run at or before the
			// cutoff, falling back to the immediately-previous run.
			cur := runs[0]
			base := runs[1]
			if !cutoff.IsZero() {
				for _, r := range runs[1:] {
					if r.StartedAt.Before(cutoff) || r.StartedAt.Equal(cutoff) {
						base = r
						break
					}
				}
			}

			prevRaw, err := s.ICPSnapshot(ctx, base.ID)
			if err != nil {
				return err
			}
			curRaw, err := s.ICPSnapshot(ctx, cur.ID)
			if err != nil {
				return err
			}
			prev := icpFilterKinds(icpEntitiesFromSnapshot(prevRaw), wantKinds)
			now := icpFilterKinds(icpEntitiesFromSnapshot(curRaw), wantKinds)

			report.FromRun, report.ToRun = base.ID, cur.ID
			report.FromTime = base.StartedAt.Format(time.RFC3339)
			report.ToTime = cur.StartedAt.Format(time.RFC3339)
			report.Compared = len(now)
			report.Changes = icp.Diff(prev, now)

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), report, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s: run %d (%s) -> run %d (%s)\n\n",
				account, base.ID, base.StartedAt.Format("2006-01-02 15:04"),
				cur.ID, cur.StartedAt.Format("2006-01-02 15:04"))
			if len(report.Changes) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No changes across %d entities.\n", report.Compared)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "CHANGE\tKIND\tNAME\tFROM\tTO")
			for _, c := range report.Changes {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					c.Kind, c.Entity, truncate(c.Name, 46), truncate(c.From, 24), truncate(c.To, 24))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Compare against the newest sync at least this old (e.g. 7d, 24h, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().StringVar(&kinds, "kinds", "both", "Which entities to compare: class, camp, or both")
	return cmd
}
