// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelSnapshotDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <name>",
		Short:   "Delete a saved config snapshot from the local store",
		Long:    "Remove a named config snapshot from the local store. This only deletes the local capture; it never touches the router.",
		Example: "  gl-inet-pp-cli snapshot delete preflight",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openSnapshotStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			n, err := st.DeleteSnapshot(name)
			if err != nil {
				return apiErr(err)
			}
			if n == 0 {
				return notFoundErr(fmt.Errorf("no snapshot named %q", name))
			}
			raw, _ := json.Marshal(map[string]any{"name": name, "status": "deleted"})
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
