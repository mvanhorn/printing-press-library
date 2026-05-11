package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/yclocal"
)

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "watch",
		Short:       "Track a personal set of YC companies and see what changes between syncs.",
		Long:        "Maintain a local watch list of YC company slugs. Use 'watch diff' to see team_size, status, and hiring deltas on those slugs between snapshots.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchDiffCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "add [slug...]",
		Short:   "Add one or more YC company slugs to your watch list.",
		Example: "  yc-companies-pp-cli watch add stripe airbnb doordash",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			added, skipped, err := yclocal.WatchAdd(cmd.Context(), st.DB(), args)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"added":   added,
					"skipped": skipped,
				})
			}
			if len(added) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Added %d slug(s): %v\n", len(added), added)
			}
			if len(skipped) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Skipped %d slug(s) (already watched or not in store — run 'sync' first): %v\n", len(skipped), skipped)
			}
			if len(added)+len(skipped) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no slugs supplied)")
			}
			return nil
		},
	}
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "remove [slug...]",
		Aliases: []string{"rm"},
		Short:   "Remove one or more slugs from your watch list.",
		Example: "  yc-companies-pp-cli watch remove stripe",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			removed, missed, err := yclocal.WatchRemove(cmd.Context(), st.DB(), args)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"removed": removed,
					"missed":  missed,
				})
			}
			if len(removed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Removed %d slug(s): %v\n", len(removed), removed)
			}
			if len(missed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Not watched: %v\n", missed)
			}
			return nil
		},
	}
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Aliases:     []string{"ls"},
		Short:       "Show your watched companies with their current state.",
		Example:     "  yc-companies-pp-cli watch list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			items, err := yclocal.WatchList(cmd.Context(), st.DB())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, items)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no companies watched yet — try 'yc-companies-pp-cli watch add <slug>')")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-25s %-15s %-12s %5s %s\n", "SLUG", "NAME", "BATCH", "STATUS", "TEAM", "HIRING")
			for _, w := range items {
				hiring := "no"
				if w.IsHiring {
					hiring = "yes"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-25s %-25s %-15s %-12s %5d %s\n", trunc(w.Slug, 25), trunc(w.Name, 25), trunc(w.Batch, 15), trunc(w.Status, 12), w.TeamSize, hiring)
			}
			return nil
		},
	}
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
