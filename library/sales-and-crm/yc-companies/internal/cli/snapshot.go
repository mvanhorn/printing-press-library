package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/yc-companies/internal/yclocal"
)

func newSnapshotCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Freeze the current companies table into history for later diffing.",
		Long:        "Capture a point-in-time snapshot of the synced YC companies table. Snapshots power 'watch diff', 'companies new', and 'companies changes'. Run after each 'sync' to keep history.",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newSnapshotCreateCmd(flags))
	cmd.AddCommand(newSnapshotListCmd(flags))
	return cmd
}

func newSnapshotCreateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "create",
		Short:   "Capture a snapshot of the current companies table.",
		Example: "  yc-companies-pp-cli snapshot create",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			id, n, err := yclocal.CaptureSnapshot(cmd.Context(), st.DB())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"snapshot_id": id,
					"rows":        n,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Snapshot %s captured (%d rows).\n", id, n)
			return nil
		},
	}
}

func newSnapshotListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List snapshot IDs newest first.",
		Example:     "  yc-companies-pp-cli snapshot list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store.OpenWithContext(cmd.Context(), defaultDBPath("yc-companies-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			ids, err := yclocal.ListSnapshots(cmd.Context(), st.DB())
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"snapshots": ids})
			}
			if len(ids) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "(no snapshots yet — run 'yc-companies-pp-cli snapshot create' after 'sync')")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(ids, "\n"))
			return nil
		},
	}
}
