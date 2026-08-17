// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func newSnapshotsPromotedCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "snapshots",
		Short:       "Saved config snapshots (local store)",
		Long:        "List config snapshots saved by 'snapshot save', read from the local SQLite store.",
		Example:     "  gl-inet-pp-cli snapshots",
		Annotations: map[string]string{"pp:endpoint": "snapshots.list", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openSnapshotStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			snaps, err := st.ListSnapshots()
			if err != nil {
				return apiErr(err)
			}
			rows := snapshotListRows(snaps)
			raw, _ := json.Marshal(rows)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}

	return cmd
}
