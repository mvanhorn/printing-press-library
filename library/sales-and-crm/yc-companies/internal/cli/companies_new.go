package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/yclocal"
)

func newCompaniesNewCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		limit int
	)
	cmd := &cobra.Command{
		Use:         "new",
		Short:       "Companies that appeared in the directory after a date or since the last sync.",
		Long:        "Anti-join over snapshot history: companies present in the latest snapshot but absent from the one at or before --since. Needs at least two snapshots — run 'snapshot create' after each sync.",
		Example:     "  yc-companies-pp-cli companies new --since 2026-04-01\n  yc-companies-pp-cli companies new --since-last-sync --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sinceLast, _ := cmd.Flags().GetBool("since-last-sync")
			if since == "" && !sinceLast {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{"error": "either --since <date> or --since-last-sync is required", "usage": cmd.CommandPath() + " --since <date>"})
				}
				return fmt.Errorf("either --since <date> or --since-last-sync is required")
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			sinceSnap := ""
			if sinceLast {
				ids, err := yclocal.ListSnapshots(cmd.Context(), db)
				if err != nil {
					return err
				}
				if len(ids) < 2 {
					return fmt.Errorf("need at least two snapshots to compute 'new since last sync'; got %d. Run 'snapshot create' after each sync", len(ids))
				}
				sinceSnap = ids[1] // second-most-recent
			} else {
				t, err := parseSinceFlag(since)
				if err != nil {
					return err
				}
				sinceSnap, err = yclocal.SnapshotAtOrBefore(cmd.Context(), db, t)
				if err != nil {
					return err
				}
				if sinceSnap == "" {
					return fmt.Errorf("no snapshot at or before %s (try a more recent --since or build snapshot history with 'snapshot create')", since)
				}
			}

			news, err := yclocal.NewSince(cmd.Context(), db, sinceSnap, limit)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, news)
			}
			if len(news) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no new companies in this window)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-25s %-15s %s\n", "SLUG", "NAME", "BATCH", "ONE-LINER")
			for _, n := range news {
				fmt.Fprintf(cmd.OutOrStdout(), "%-30s %-25s %-15s %s\n", trunc(n.Slug, 30), trunc(n.Name, 25), trunc(n.Batch, 15), trunc(n.OneLiner, 80))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Compare against the snapshot at or before this date (YYYY-MM-DD or RFC3339).")
	cmd.Flags().Bool("since-last-sync", false, "Compare the latest snapshot against the snapshot before it.")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = unlimited).")
	return cmd
}
