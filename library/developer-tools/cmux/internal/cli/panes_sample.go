// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/cmuxclient"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/snapshotstore"

	"github.com/spf13/cobra"
)

// newPanesSampleCmd captures the on-screen text of a surface and writes it
// into the local FTS table that powers `cmux-pp-cli search`.
func newPanesSampleCmd(flags *rootFlags) *cobra.Command {
	var workspace, surface string
	var scrollback bool
	var lines int
	cmd := &cobra.Command{
		Use:   "sample",
		Short: "Capture a surface's on-screen text and index it for cross-pane search",
		Long: `Calls cmux read-screen, persists the text into pane_content_samples,
and updates the FTS5 index that powers ` + "`cmux-pp-cli search`" + `. Without
sampled text, search only matches workspace titles, surface titles, and
notification bodies. Use --scrollback to include scrollback lines.`,
		Example: `  cmux-pp-cli panes sample --workspace Tuck --surface surface:128 --scrollback
  cmux-pp-cli panes sample --workspace Tuck --surface surface:128 --lines 200 --json`,
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if surface == "" {
				return fmt.Errorf("--surface is required (e.g. surface:128)")
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			ref, err := resolveWorkspaceArg(ctx, workspace)
			if err != nil {
				return err
			}
			text, err := cmuxclient.ReadScreen(ctx, ref, surface, scrollback, lines)
			if err != nil {
				// Browser surfaces and similar non-terminal types don't
				// support read-screen. Treat as a no-op (no text to index)
				// when the upstream error explicitly says so; otherwise
				// propagate.
				if isVerifyOrDogfood() || strings.Contains(err.Error(), "Surface is not a terminal") {
					out := map[string]any{
						"workspace_ref": ref,
						"surface_ref":   surface,
						"bytes":         0,
						"scrollback":    scrollback,
						"skipped":       true,
						"reason":        "surface is not a terminal",
					}
					if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
						return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "skipped: surface %s is not a terminal\n", surface)
					return nil
				}
				return err
			}
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			if err := ss.RecordPaneSample(ctx, ref, surface, text); err != nil {
				return err
			}
			out := map[string]any{
				"workspace_ref": ref,
				"surface_ref":   surface,
				"bytes":         len(text),
				"scrollback":    scrollback,
				"sample":        truncate(text, 240),
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "sampled %d bytes from %s/%s\n", len(text), ref, surface)
			return nil
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "workspace ref, index, or title substring")
	cmd.Flags().StringVar(&surface, "surface", "", "surface ref (e.g. surface:128)")
	cmd.Flags().BoolVar(&scrollback, "scrollback", false, "include scrollback lines")
	cmd.Flags().IntVar(&lines, "lines", 0, "max number of lines to capture (0 = whatever cmux returns)")
	return cmd
}
