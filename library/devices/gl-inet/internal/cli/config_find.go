// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelConfigFindCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "find <term>",
		Short:       "Find any config option anywhere across the whole router config tree.",
		Long:        "Search the live uci config (and any saved snapshots) for a term in either the key or the value, case-insensitively.",
		Example:     "  gl-inet-pp-cli config find country",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			term := strings.TrimSpace(args[0])
			if term == "" {
				return usageErr(fmt.Errorf("search term must not be empty"))
			}
			out := cmd.OutOrStdout()
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			result := map[string]any{"term": term}

			// Always search saved snapshots (no network needed).
			snapMatches := map[string][]string{}
			if st, err := openSnapshotStore(ctx); err == nil {
				if snaps, lerr := st.ListSnapshots(); lerr == nil {
					for _, s := range snaps {
						if m := grepUCILines(s.UCIShow, term); len(m) > 0 {
							snapMatches[s.Name] = m
						}
					}
				}
				st.Close()
			}
			result["snapshots"] = snapMatches

			if cliutil.IsVerifyEnv() {
				result["live"] = []string{}
				result["note"] = "live search skipped under verify"
				raw, _ := json.Marshal(result)
				return printOutputWithFlags(out, raw, flags)
			}
			if dryRunOK(flags) {
				fmt.Fprintf(out, "dry-run: would SSH 'uci show' and grep for %q\n", term)
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
			live := grepUCILines(show, term)
			if live == nil {
				live = []string{}
			}
			result["live"] = live
			result["live_count"] = len(live)
			raw, _ := json.Marshal(result)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	return cmd
}
