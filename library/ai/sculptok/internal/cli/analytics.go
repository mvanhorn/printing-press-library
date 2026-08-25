// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored (was a generator stub). Offline aggregation over locally synced
// credit events — where credits went, by action type / remarks / day.
//
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelAnalyticsCmd(flags *rootFlags) *cobra.Command {
	var typ, groupBy, db string
	var limit int

	cmd := &cobra.Command{
		Use:   "analytics",
		Short: "Aggregate locally synced credit events to see where credits went",
		Long: strings.Trim(`
Group the local mirror of credit events and sum the credit change per group, so
you can see where credits went. Offline; run 'sync' first. For a forward
estimate before spending, use 'cost'.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli analytics --type credits --group-by actionType
  sculptok-pp-cli analytics --type credits --group-by day
  sculptok-pp-cli analytics --type credits --group-by remarks --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate local credit events")
				return nil
			}
			if typ != "" && typ != "credits" && typ != "credit_events" {
				return usageErr(fmt.Errorf("unsupported --type %q (only 'credits' is supported)", typ))
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

			groups, err := st.AnalyticsCreditEvents(ctx, groupBy, limit)
			if err != nil {
				return usageErr(err)
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), groups, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-24s %8s %12s\n", "group", "count", "credit_delta")
			for _, g := range groups {
				fmt.Fprintf(w, "%-24s %8d %12d\n", g.Group, g.Count, g.TotalChange)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&typ, "type", "credits", "Resource to aggregate (credits)")
	cmd.Flags().StringVar(&groupBy, "group-by", "actionType", "Group by: actionType|remarks|day")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum groups to return")
	cmd.Flags().StringVar(&db, "db", "", "Local store path (default ~/.config/sculptok-pp-cli/sculptok.db)")
	return cmd
}
