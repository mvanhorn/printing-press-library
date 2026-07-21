// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// surface capture / surface diff — track Robinhood's beta MCP tool surface.
// Robinhood grows and reshapes the agentic tool surface without notice (49→50
// tools mid-July 2026; earlier counts of 22 and 12) and publishes no
// request/response docs, so integrators discover breakage in production. These
// commands snapshot tools/list into the local store and diff consecutive
// snapshots — a change log the platform does not provide.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"

	"github.com/spf13/cobra"
)

// mcpTool is the minimal shape of a tools/list entry needed to diff surfaces.
type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

func newNovelSurfaceCaptureCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "capture",
		Short:       "Snapshot the live MCP tool surface into the local store for later diffing",
		Example:     "  robinhood-agentic-pp-cli surface capture",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tools, err := c.MCPListTools(cmd.Context()) // pp:client-call
			if err != nil {
				return classifyAPIError(err, flags)
			}
			st, err := store.Open(defaultDBPath("robinhood-agentic-pp-cli"))
			if err != nil {
				return err
			}
			defer st.Close()
			if err := st.RecordToolSurface(tools); err != nil {
				return err
			}
			var arr []json.RawMessage
			_ = json.Unmarshal(tools, &arr)
			result := map[string]any{"captured": true, "tool_count": len(arr)}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Captured %d tools into the local surface log.\n", len(arr))
			return nil
		},
	}
	return cmd
}

func newNovelSurfaceDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Show what changed between the two most recent tool-surface snapshots — with dates.",
		Example:     "  robinhood-agentic-pp-cli surface diff",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Read-only open: diff only reads stored surface snapshots.
			st, err := openStoreForRead(cmd.Context(), "robinhood-agentic-pp-cli")
			if err != nil {
				return err
			}
			var snaps []store.ToolSurfaceSnapshot
			if st != nil {
				defer st.Close()
				snaps, err = st.ToolSurfaceSnapshots(2)
				if err != nil {
					return err
				}
			}
			if len(snaps) < 2 {
				note := "need at least two snapshots to diff — run 'surface capture' now and again later"
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"snapshots": len(snaps), "note": note}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), note)
				return nil
			}
			newer := parseToolList(snaps[0].Tools)
			older := parseToolList(snaps[1].Tools)
			d := diffToolSurfaces(older, newer)
			result := map[string]any{
				"from":    snaps[1].CapturedAt,
				"to":      snaps[0].CapturedAt,
				"added":   d.Added,
				"removed": d.Removed,
				"changed": d.Changed,
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Surface diff %s → %s\n", snaps[1].CapturedAt.Format("2006-01-02 15:04"), snaps[0].CapturedAt.Format("2006-01-02 15:04"))
			if len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  no changes")
				return nil
			}
			for _, n := range d.Added {
				fmt.Fprintf(cmd.OutOrStdout(), "  + %s (added)\n", n)
			}
			for _, n := range d.Removed {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s (removed)\n", n)
			}
			for _, n := range d.Changed {
				fmt.Fprintf(cmd.OutOrStdout(), "  ~ %s (schema changed)\n", n)
			}
			return nil
		},
	}
	return cmd
}

func parseToolList(raw json.RawMessage) []mcpTool {
	var tools []mcpTool
	_ = json.Unmarshal(raw, &tools)
	return tools
}

// surfaceDiff is the result of comparing two tool surfaces.
type surfaceDiff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// diffToolSurfaces compares two tool lists by name and input schema. Pure
// function — the diff logic is testable without a store or network.
func diffToolSurfaces(older, newer []mcpTool) surfaceDiff {
	oldByName := map[string]mcpTool{}
	for _, t := range older {
		oldByName[t.Name] = t
	}
	newByName := map[string]mcpTool{}
	for _, t := range newer {
		newByName[t.Name] = t
	}
	var d surfaceDiff
	for name, nt := range newByName {
		ot, existed := oldByName[name]
		if !existed {
			d.Added = append(d.Added, name)
			continue
		}
		if string(ot.InputSchema) != string(nt.InputSchema) {
			d.Changed = append(d.Changed, name)
		}
	}
	for name := range oldByName {
		if _, still := newByName[name]; !still {
			d.Removed = append(d.Removed, name)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	sort.Strings(d.Changed)
	return d
}
