// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored (was a generator stub). Cross-checks synced credit charges
// against locally recorded jobs to surface credits spent with no matching job.
//
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelReconcileCmd(flags *rootFlags) *cobra.Command {
	var db string
	var limit int

	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "Find credit charges with no matching local job",
		Long: strings.Trim(`
Join the locally synced credit-spend events against the jobs this CLI recorded.
SculptOK's API-Draw credit remarks embed the promptId, so a local join surfaces
credits spent outside this CLI (or jobs that were never recorded). Offline; run
'sync' and some 'generate' runs first. For a grouped spend breakdown use
'analytics'.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli reconcile
  sculptok-pp-cli reconcile --db ./sculptok.db --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would reconcile credits against jobs")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := resolveDBPath(db)
			st, ok, err := openReadStore(ctx, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				return emptyMirror(cmd, flags, dbPath)
			}
			defer st.Close()

			rows, err := st.Reconcile(ctx, limit)
			if err != nil {
				return err
			}
			view := map[string]any{
				"unmatchedCharges": rows,
				"count":            len(rows),
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			if len(rows) == 0 {
				fmt.Fprintln(w, "no unmatched credit charges — every recorded spend maps to a local job (within synced history)")
				return nil
			}
			fmt.Fprintf(w, "%d credit charge(s) with no matching local job:\n", len(rows))
			for _, r := range rows {
				fmt.Fprintf(w, "  %s  %d credits  %s  %s\n", r.CreateDate, r.ChangeNum, r.EventID, r.Remarks)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&db, "db", "", "Local store path (default ~/.config/sculptok-pp-cli/sculptok.db)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum credit events to scan")
	return cmd
}
