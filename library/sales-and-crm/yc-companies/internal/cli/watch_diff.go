package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/yclocal"
)

func newWatchDiffCmd(flags *rootFlags) *cobra.Command {
	var (
		since string
		field string
	)
	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Show team_size / status / isHiring deltas on watched companies between snapshots.",
		Long:        "Diff two snapshots filtered to your watch list. Defaults to status, team_size, and is_hiring across the oldest and latest snapshots. Use --since <date> to compare against the snapshot at or before that date.",
		Example:     "  yc-companies-pp-cli watch diff\n  yc-companies-pp-cli watch diff --since 2026-04-01 --json\n  yc-companies-pp-cli watch diff --field is_hiring",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			db := st.DB()

			slugs, err := yclocal.WatchedSlugs(cmd.Context(), db)
			if err != nil {
				return err
			}
			if len(slugs) == 0 {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{"changes": []any{}, "watched": 0})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "(no companies watched — try 'yc-companies-pp-cli watch add <slug>')")
				return nil
			}

			fromSnap := ""
			toSnap := ""
			if since != "" {
				t, err := parseSinceFlag(since)
				if err != nil {
					return err
				}
				fromSnap, err = yclocal.SnapshotAtOrBefore(cmd.Context(), db, t)
				if err != nil {
					return err
				}
				if fromSnap == "" {
					return fmt.Errorf("no snapshot at or before %s (run 'snapshot create' regularly to build history)", since)
				}
			}

			fields := []string{"status", "team_size", "is_hiring"}
			if field != "" {
				fields = []string{field}
			}
			var all []yclocal.ChangeRow
			for _, f := range fields {
				rows, err := yclocal.Changes(cmd.Context(), db, yclocal.ChangesQuery{
					Field:    f,
					FromSnap: fromSnap,
					ToSnap:   toSnap,
					Slugs:    slugs,
				})
				if err != nil {
					return err
				}
				all = append(all, rows...)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"changes": all,
					"watched": len(slugs),
				})
			}
			if len(all) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No changes detected across %d watched companies. Need at least two snapshots — run 'snapshot create' after each sync.\n", len(slugs))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-25s %-12s %-15s -> %s\n", "SLUG", "NAME", "FIELD", "FROM", "TO")
			for _, r := range all {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-25s %-12s %-15v -> %v\n", trunc(r.Slug, 25), trunc(r.Name, 25), r.Field, r.From, r.To)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Compare against the snapshot at or before this date (YYYY-MM-DD or RFC3339).")
	cmd.Flags().StringVar(&field, "field", "", "Restrict to one field: status, team_size, is_hiring, top_company.")
	return cmd
}

func parseSinceFlag(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02T15:04:05Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("--since: cannot parse %q (use YYYY-MM-DD or RFC3339)", s)
}
