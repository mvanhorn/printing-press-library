package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/yclocal"
)

func newCompaniesChangesCmd(flags *rootFlags) *cobra.Command {
	var (
		field   string
		toVal   string
		since   string
		slugs   string
		limit   int
		fromAll bool
	)
	cmd := &cobra.Command{
		Use:         "changes",
		Short:       "Diff status, team_size, isHiring, or top_company across snapshots.",
		Long:        "Snapshot field-diff: report every company whose <field> changed between two snapshots. Combine with --to to filter to a specific target value (e.g. --field is_hiring --to true to find newly-hiring companies). Combine with --slugs a,b,c to scope to a slug list. Needs at least two snapshots.",
		Example:     "  yc-companies-pp-cli companies changes --field is_hiring --to true --since 2026-04-01\n  yc-companies-pp-cli companies changes --field status --to acquired --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if field == "" {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{"error": "--field is required", "valid_fields": []string{"status", "team_size", "is_hiring", "top_company"}})
				}
				return fmt.Errorf("--field is required (status | team_size | is_hiring | top_company)")
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

			fromSnap := ""
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
					return fmt.Errorf("no snapshot at or before %s", since)
				}
			}

			var slugList []string
			if slugs != "" {
				for _, s := range strings.Split(slugs, ",") {
					if s = strings.TrimSpace(s); s != "" {
						slugList = append(slugList, s)
					}
				}
			}

			q := yclocal.ChangesQuery{
				Field:    field,
				FromSnap: fromSnap,
				Slugs:    slugList,
				Limit:    limit,
			}
			if toVal != "" {
				q.ToValueSet = true
				q.ToValue = toVal
			}
			_ = fromAll

			rows, err := yclocal.Changes(cmd.Context(), db, q)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, rows)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no changes in this window — need at least two snapshots; run 'snapshot create' regularly)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-25s %-12s %-15s -> %s\n", "SLUG", "NAME", "FIELD", "FROM", "TO")
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-25s %-12s %-15v -> %v\n", trunc(r.Slug, 25), trunc(r.Name, 25), r.Field, r.From, r.To)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&field, "field", "", "Field to diff: status | team_size | is_hiring | top_company.")
	cmd.Flags().StringVar(&toVal, "to", "", "Optional: only report rows whose new value equals this (e.g. 'true', 'acquired', '50').")
	cmd.Flags().StringVar(&since, "since", "", "Compare against the snapshot at or before this date (YYYY-MM-DD).")
	cmd.Flags().StringVar(&slugs, "slugs", "", "Restrict to these slugs (comma-separated).")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = unlimited).")
	cmd.Flags().BoolVar(&fromAll, "from-all", false, "Reserved for future use.")
	_ = cmd.Flags().MarkHidden("from-all")
	return cmd
}
