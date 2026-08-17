// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored (was a generator stub). Offline search over the local mirror of
// jobs / credit events / drawings. No API call.
//
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var typ, db string
	var limit int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search your local mirror of draws, credit events, and drawings (offline)",
		Long: strings.Trim(`
Search the local SQLite mirror, no API call. Default --type is jobs (your
generate runs and the settings that produced them). Run 'sync' first to populate
credit events and drawings. The query matches job kind/status/params/ids,
credit-event remarks, and drawing ids/urls; omit it to list recent rows.`, "\n"),
		Example: strings.Trim(`
  sculptok-pp-cli search "stl" --type jobs --limit 20
  sculptok-pp-cli search "pro" --type jobs --agent
  sculptok-pp-cli search "png" --type drawings
  sculptok-pp-cli search --type drawings`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search local mirror")
				return nil
			}
			term := ""
			if len(args) > 0 {
				term = args[0]
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

			switch typ {
			case "", "jobs":
				jobs, err := st.SearchJobs(ctx, term, limit)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), jobs, flags)
			case "credits", "credit_events":
				// Filter in SQL so --limit caps matched rows (like the jobs
				// path), not just the newest-N window we then scan in memory —
				// otherwise an older matching event past the limit is missed.
				events, err := st.SearchCreditEvents(ctx, term, limit)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), events, flags)
			case "drawings":
				rows, err := st.SearchDrawings(ctx, term, limit)
				if err != nil {
					return err
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			default:
				return usageErr(fmt.Errorf("unknown --type %q (use jobs, credits, or drawings)", typ))
			}
		},
	}
	cmd.Flags().StringVar(&typ, "type", "jobs", "What to search: jobs|credits|drawings")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum rows to return")
	cmd.Flags().StringVar(&db, "db", "", "Local store path (default ~/.config/sculptok-pp-cli/sculptok.db)")
	return cmd
}
