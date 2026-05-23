// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/cmuxclient"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/snapshotstore"

	"github.com/spf13/cobra"
)

// addNovelStatusSubcommands attaches the novel `status awaiting`, `status
// timeline`, `status stuck`, `status changes`, and `status icons` commands
// to the existing generator-emitted `status` parent.
func addNovelStatusSubcommands(parent *cobra.Command, flags *rootFlags) {
	parent.AddCommand(newStatusAwaitingCmd(flags))
	parent.AddCommand(newStatusTimelineCmd(flags))
	parent.AddCommand(newStatusStuckCmd(flags))
	parent.AddCommand(newStatusChangesCmd(flags))
	parent.AddCommand(newStatusIconsCmd(flags))
}

// ───── status awaiting ─────

type awaitingRow struct {
	WorkspaceRef   string  `json:"workspace_ref"`
	WorkspaceTitle string  `json:"workspace_title"`
	Key            string  `json:"key"`
	State          string  `json:"state"`
	LatestValue    string  `json:"latest_value"`
	ChangedAtUnix  float64 `json:"changed_at_unix"`
	AgeSeconds     float64 `json:"age_seconds"`
}

func newStatusAwaitingCmd(flags *rootFlags) *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "awaiting",
		Short: "List workspaces whose canonical agent state is `awaiting`",
		Long: `Normalize per-workspace agent state into one column. The canonical state
is ` + "`idle | working | awaiting | stranded`" + `, computed by joining each
workspace's status entry value with its surface title icons and surface
health. By default only awaiting rows are returned; pass --all to see
every workspace's normalized state.`,
		Example:     "  cmux-pp-cli status awaiting --all --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dryRunOK(flags) {
				return nil
			}
			wss, err := cmuxclient.ListWorkspaces(ctx)
			if err != nil {
				return err
			}
			entries, err := cmuxclient.ListStatusEntries(ctx, "")
			if err != nil {
				return err
			}
			// Do NOT take a snapshot here — that walks every workspace's
			// surfaces sequentially and balloons the latency. `snapshot` is
			// the explicit command; readers just read the most-recent rows
			// already in the DB. If the DB is empty, `changed_at_unix` is
			// zero and `--all` still works without a fan-out.
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			latest, err := ss.LatestPerWorkspaceKey(ctx)
			if err != nil {
				return err
			}
			latestByWSKey := make(map[string]snapshotstore.StatusSnapshot)
			for _, l := range latest {
				latestByWSKey[l.WorkspaceRef+"|"+l.Key] = l
			}
			// Light-weight canonical state: derive from the statusEntry
			// value alone. Surface-title icon enrichment is opt-in via the
			// explicit `snapshot` command, which writes canonical into the
			// store.
			wsIdx := indexWorkspacesByRef(wss)
			titles := map[string][]string{}
			stranded := map[string]int{}

			now := snapshotstore.Now()
			rows := make([]awaitingRow, 0)
			// Build one row per (workspace, key). If a workspace has no
			// status entries, also emit an idle row when --all.
			seen := make(map[string]bool)
			for _, se := range entries {
				canonical := string(cmuxclient.CanonicalState(se.Value, titles[se.WorkspaceRef], stranded[se.WorkspaceRef]))
				key := se.WorkspaceRef + "|" + se.Key
				seen[key] = true
				lat := latestByWSKey[key]
				row := awaitingRow{
					WorkspaceRef:   se.WorkspaceRef,
					WorkspaceTitle: wsIdx[se.WorkspaceRef].Title,
					Key:            se.Key,
					State:          canonical,
					LatestValue:    se.Value,
					ChangedAtUnix:  lat.ObservedAtUnix,
					AgeSeconds:     now - lat.ObservedAtUnix,
				}
				if all || canonical == string(cmuxclient.StateAwaiting) {
					rows = append(rows, row)
				}
			}
			if all {
				for _, w := range wss {
					key := w.Ref + "|claude_code"
					if seen[key] {
						continue
					}
					rows = append(rows, awaitingRow{
						WorkspaceRef:   w.Ref,
						WorkspaceTitle: w.Title,
						Key:            "claude_code",
						State:          string(cmuxclient.StateIdle),
					})
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].WorkspaceTitle < rows[j].WorkspaceTitle
			})
			return renderRows(cmd, flags, rows, func() {
				fmt.Fprintln(cmd.OutOrStdout(), "STATE\tWORKSPACE\tVALUE\tAGE")
				for _, r := range rows {
					age := "-"
					if r.ChangedAtUnix > 0 {
						age = humanDuration(r.AgeSeconds)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", r.State, r.WorkspaceTitle, r.LatestValue, age)
				}
			})
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "include workspaces in non-awaiting states (idle/working/stranded)")
	return cmd
}

// ───── status timeline ─────

func newStatusTimelineCmd(flags *rootFlags) *cobra.Command {
	var workspace string
	var since string
	cmd := &cobra.Command{
		Use:   "timeline",
		Short: "Time-series of agent state transitions from the local snapshot store",
		Long: `Reads status_snapshots for one workspace (or all) over a time window.
Defaults to the last 4 hours. The window is anchored to the most recent
snapshot you have, not to wall-clock now — run sync (or snapshot) to
refresh.`,
		Example: `  cmux-pp-cli status timeline --workspace Tuck --since 4h --json
  cmux-pp-cli status timeline --since 24h --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dryRunOK(flags) {
				return nil
			}
			ref, err := resolveWorkspaceArg(ctx, workspace)
			if err != nil {
				return err
			}
			if since == "" {
				since = "4h"
			}
			sinceUnix, err := parseSinceUnix(since)
			if err != nil {
				return err
			}
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			tl, err := ss.Timeline(ctx, ref, sinceUnix)
			if err != nil {
				return err
			}
			return renderRows(cmd, flags, tl, func() {
				fmt.Fprintln(cmd.OutOrStdout(), "TIME\tWORKSPACE\tKEY\tVALUE\tCANONICAL")
				for _, r := range tl {
					ts := time.Unix(int64(r.ObservedAtUnix), 0).Format("15:04:05")
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", ts, r.WorkspaceRef, r.Key, r.Value, r.Canonical)
				}
			})
		},
	}
	cmd.Flags().StringVar(&workspace, "workspace", "", "limit to one workspace (ref, index, or title substring)")
	cmd.Flags().StringVar(&since, "since", "4h", "time window: 30m, 4h, 24h, or a unix timestamp")
	return cmd
}

// ───── status stuck ─────

type stuckRow struct {
	WorkspaceRef   string  `json:"workspace_ref"`
	WorkspaceTitle string  `json:"workspace_title"`
	Key            string  `json:"key"`
	LatestValue    string  `json:"latest_value"`
	Canonical      string  `json:"canonical"`
	ChangedAtUnix  float64 `json:"changed_at_unix"`
	AgeSeconds     float64 `json:"age_seconds"`
}

func newStatusStuckCmd(flags *rootFlags) *cobra.Command {
	var over string
	var key string
	cmd := &cobra.Command{
		Use:   "stuck",
		Short: "List workspaces awaiting input longer than a threshold",
		Long: `Returns every (workspace, key) whose latest persisted snapshot is
canonical state "awaiting" and whose transition timestamp is older than
--over (default 30 minutes).`,
		Example: `  cmux-pp-cli status stuck --over 30m --json
  cmux-pp-cli status stuck --over 2h`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dryRunOK(flags) {
				return nil
			}
			if over == "" {
				over = "30m"
			}
			d, err := time.ParseDuration(over)
			if err != nil {
				return fmt.Errorf("invalid --over duration: %w", err)
			}
			// stuck reads the existing snapshot table; run `snapshot` first
			// (or set up a cron/launchd loop) to keep state fresh.
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			latest, err := ss.LatestPerWorkspaceKey(ctx)
			if err != nil {
				return err
			}
			wss, err := cmuxclient.ListWorkspaces(ctx)
			if err != nil {
				return err
			}
			wsIdx := indexWorkspacesByRef(wss)
			now := snapshotstore.Now()
			rows := make([]stuckRow, 0)
			for _, l := range latest {
				if key != "" && l.Key != key {
					continue
				}
				if l.Canonical != string(cmuxclient.StateAwaiting) {
					continue
				}
				age := now - l.ObservedAtUnix
				if age < d.Seconds() {
					continue
				}
				rows = append(rows, stuckRow{
					WorkspaceRef:   l.WorkspaceRef,
					WorkspaceTitle: wsIdx[l.WorkspaceRef].Title,
					Key:            l.Key,
					LatestValue:    l.Value,
					Canonical:      l.Canonical,
					ChangedAtUnix:  l.ObservedAtUnix,
					AgeSeconds:     age,
				})
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i].AgeSeconds > rows[j].AgeSeconds })
			return renderRows(cmd, flags, rows, func() {
				fmt.Fprintln(cmd.OutOrStdout(), "AGE\tWORKSPACE\tKEY\tVALUE")
				for _, r := range rows {
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", humanDuration(r.AgeSeconds), r.WorkspaceTitle, r.Key, r.LatestValue)
				}
			})
		},
	}
	cmd.Flags().StringVar(&over, "over", "30m", "threshold age (e.g. 15m, 1h, 4h)")
	cmd.Flags().StringVar(&key, "key", "", "limit to one status key (default: any)")
	return cmd
}

// ───── status changes ─────

type changeRow struct {
	WorkspaceRef   string  `json:"workspace_ref"`
	WorkspaceTitle string  `json:"workspace_title"`
	Key            string  `json:"key"`
	PrevValue      string  `json:"prev_value"`
	NewValue       string  `json:"new_value"`
	Canonical      string  `json:"canonical"`
	TransitionedAt float64 `json:"transitioned_at_unix"`
}

func newStatusChangesCmd(flags *rootFlags) *cobra.Command {
	var since string
	cmd := &cobra.Command{
		Use:   "changes",
		Short: "Recently flipped workspaces from the local snapshot store",
		Long: `Returns only transition rows (where the value changed) within the time
window. Takes a fresh snapshot first so the window includes the current
state.`,
		Example:     "  cmux-pp-cli status changes --since 1h --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if dryRunOK(flags) {
				return nil
			}
			if since == "" {
				since = "1h"
			}
			sinceUnix, err := parseSinceUnix(since)
			if err != nil {
				return err
			}
			// changes reads the existing snapshot table; run `snapshot` first
			// (or set up a cron/launchd loop) to keep state fresh.
			ss, err := snapshotstore.Open(ctx, "")
			if err != nil {
				return err
			}
			defer ss.Close()
			changes, err := ss.Changes(ctx, sinceUnix)
			if err != nil {
				return err
			}
			wss, err := cmuxclient.ListWorkspaces(ctx)
			if err != nil {
				return err
			}
			wsIdx := indexWorkspacesByRef(wss)
			rows := make([]changeRow, 0, len(changes))
			for _, c := range changes {
				rows = append(rows, changeRow{
					WorkspaceRef:   c.WorkspaceRef,
					WorkspaceTitle: wsIdx[c.WorkspaceRef].Title,
					Key:            c.Key,
					PrevValue:      c.PrevValue,
					NewValue:       c.Value,
					Canonical:      c.Canonical,
					TransitionedAt: c.ObservedAtUnix,
				})
			}
			return renderRows(cmd, flags, rows, func() {
				fmt.Fprintln(cmd.OutOrStdout(), "TIME\tWORKSPACE\tKEY\tPREV→NEW")
				for _, r := range rows {
					ts := time.Unix(int64(r.TransitionedAt), 0).Format("15:04:05")
					fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s→%s\n", ts, r.WorkspaceTitle, r.Key, r.PrevValue, r.NewValue)
				}
			})
		},
	}
	cmd.Flags().StringVar(&since, "since", "1h", "time window: 30m, 4h, 24h, or unix ts")
	return cmd
}

// ───── status icons ─────

type iconRow struct {
	Title string `json:"title"`
	State string `json:"state"`
}

func newStatusIconsCmd(flags *rootFlags) *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "icons",
		Short: "Decode a cmux surface title icon into a canonical agent state",
		Long: `Pure local function. Pass a surface title (e.g. "✻ Claude Code") with
--title; returns the canonical state (working / awaiting / idle /
unknown). Implements the cookbook's icon-priority rule as a callable.`,
		Example: `  cmux-pp-cli status icons --title "✻ Claude Code"
  cmux-pp-cli status icons --title "⠂ Working" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return cmd.Help()
			}
			row := iconRow{Title: title, State: string(cmuxclient.IconState(title))}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(row)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", row.State)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "surface title text whose icon to decode")
	return cmd
}

// renderRows is a tiny generic helper: JSON when --json or non-TTY,
// otherwise calls the human-printer callback.
func renderRows[T any](cmd *cobra.Command, flags *rootFlags, rows []T, human func()) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}
	if human != nil {
		human()
	}
	return nil
}
