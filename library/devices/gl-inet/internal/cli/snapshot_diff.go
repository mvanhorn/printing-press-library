// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelSnapshotDiffCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "diff <name> [<other>]",
		Short:       "See exactly which settings a venue changed vs your standard config.",
		Long:        "Diff a saved snapshot (the baseline) against the CURRENT live config (one arg) or against another saved snapshot (two args). Added/removed/changed are reported relative to the baseline.",
		Example:     "  gl-inet-pp-cli snapshot diff home",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := openSnapshotStore(ctx)
			if err != nil {
				return configErr(err)
			}
			defer st.Close()

			base, err := st.GetSnapshot(args[0])
			if err != nil {
				return notFoundErr(fmt.Errorf("snapshot %q not found: %w", args[0], err))
			}
			fromMap := parseUCIShow(base.UCIShow)

			var toMap map[string]string
			targetLabel := "live"
			if len(args) >= 2 {
				other, err := st.GetSnapshot(args[1])
				if err != nil {
					return notFoundErr(fmt.Errorf("snapshot %q not found: %w", args[1], err))
				}
				toMap = parseUCIShow(other.UCIShow)
				targetLabel = args[1]
			} else {
				// Compare against the current live config over SSH.
				if cliutil.IsVerifyEnv() {
					fmt.Fprintf(cmd.OutOrStdout(), "would diff snapshot %q against the live router config\n", args[0])
					return nil
				}
				if dryRunOK(flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "dry-run: would SSH 'uci show' and diff against snapshot %q\n", args[0])
					return nil
				}
				c, err := glClient(flags)
				if err != nil {
					return err
				}
				cfg, err := glSSHConfig(c)
				if err != nil {
					return classifyGLError(err, flags)
				}
				show, err := glssh.UCIShow(ctx, cfg, "")
				if err != nil {
					return classifyGLError(err, flags)
				}
				toMap = parseUCIShow(show)
			}

			diff := computeUCIDiff(fromMap, toMap)
			result := map[string]any{
				"base":    args[0],
				"target":  targetLabel,
				"added":   diff.Added,
				"removed": diff.Removed,
				"changed": diff.Changed,
				"summary": map[string]int{
					"added":   len(diff.Added),
					"removed": len(diff.Removed),
					"changed": len(diff.Changed),
				},
			}
			raw, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
		},
	}
	return cmd
}
