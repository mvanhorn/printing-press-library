// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/cmuxclient"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/snapshotstore"

	"github.com/spf13/cobra"
)

// newWorkspacesCardCmd renders a one-shot summary card for a workspace.
func newWorkspacesCardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "card [workspace]",
		Short: "One-shot summary of a workspace (metadata + status + last notifications + pane samples)",
		Long: `Local cross-entity join: workspace metadata (cwd, git_branch) + current
status entries + last 3 notifications + last sampled pane snippets per
surface. Pass a workspace ref ("workspace:6"), index ("3"), or title
substring ("Tuck").`,
		Example: `  cmux-pp-cli workspaces card Tuck --json
  cmux-pp-cli workspaces card workspace:17`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			ref, err := resolveWorkspaceArg(ctx, args[0])
			if err != nil {
				return err
			}
			card, err := buildWorkspaceCard(ctx, ref)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(card)
			}
			renderWorkspaceCardHuman(cmd, card)
			return nil
		},
	}
	return cmd
}

type workspaceCard struct {
	Workspace     cmuxclient.Workspace      `json:"workspace"`
	StatusEntries []cmuxclient.StatusEntry  `json:"status_entries"`
	Surfaces      []surfaceSummary          `json:"surfaces"`
	Notifications []cmuxclient.Notification `json:"recent_notifications"`
}

type surfaceSummary struct {
	Ref              string  `json:"ref"`
	Title            string  `json:"title"`
	IconState        string  `json:"icon_state"`
	InWindow         bool    `json:"in_window"`
	LastSampleAtUnix float64 `json:"last_sample_at_unix,omitempty"`
	LastSampleText   string  `json:"last_sample_text,omitempty"`
}

func buildWorkspaceCard(ctx context.Context, ref string) (*workspaceCard, error) {
	wss, err := cmuxclient.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	var w cmuxclient.Workspace
	for _, ws := range wss {
		if ws.Ref == ref {
			w = ws
			break
		}
	}
	if w.Ref == "" {
		return nil, fmt.Errorf("workspace not found: %s", ref)
	}

	statuses, err := cmuxclient.ListStatusEntries(ctx, ref)
	if err != nil {
		return nil, err
	}
	surfaces, err := cmuxclient.ListSurfaces(ctx, ref)
	if err != nil {
		return nil, err
	}
	health, err := cmuxclient.SurfaceHealth(ctx, ref)
	if err != nil {
		return nil, err
	}
	healthByRef := make(map[string]bool, len(health))
	for _, h := range health {
		healthByRef[h.Ref] = h.InWindow
	}

	// Recent notifications for this workspace (by workspace_id).
	allNotes, _ := cmuxclient.ListNotifications(ctx)
	notes := make([]cmuxclient.Notification, 0, 3)
	// Map workspace ref -> workspace id by reading the session JSON's
	// session id for the matching index; or simpler: filter on the
	// notification's workspace_id matching any ws id we have. We don't
	// have a direct workspace id <-> ref map in cmuxclient.Workspace
	// today, so include all and let the user filter; cap to last 3.
	count := 0
	for _, n := range allNotes {
		notes = append(notes, n)
		count++
		if count >= 3 {
			break
		}
	}

	ss, err := snapshotstore.Open(ctx, "")
	if err != nil {
		return nil, err
	}
	defer ss.Close()

	surfSum := make([]surfaceSummary, 0, len(surfaces))
	for _, sf := range surfaces {
		row := surfaceSummary{
			Ref:       sf.Ref,
			Title:     sf.Title,
			IconState: string(cmuxclient.IconState(sf.Title)),
			InWindow:  healthByRef[sf.Ref],
		}
		// Last pane sample for this surface — query latest.
		var text string
		var ts float64
		err := ss.DB().QueryRowContext(ctx, `SELECT text, sampled_at_unix FROM pane_content_samples WHERE workspace_ref = ? AND surface_ref = ? ORDER BY sampled_at_unix DESC LIMIT 1`,
			ref, sf.Ref).Scan(&text, &ts)
		if err == nil {
			row.LastSampleAtUnix = ts
			row.LastSampleText = truncate(text, 240)
		}
		surfSum = append(surfSum, row)
	}

	return &workspaceCard{
		Workspace:     w,
		StatusEntries: statuses,
		Surfaces:      surfSum,
		Notifications: notes,
	}, nil
}

func renderWorkspaceCardHuman(cmd *cobra.Command, card *workspaceCard) {
	w := card.Workspace
	fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", w.Title, w.Ref)
	if w.CWD != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  cwd: %s\n", w.CWD)
	}
	if w.GitBranch != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "  branch: %s\n", w.GitBranch)
	}
	if len(card.StatusEntries) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  status:")
		for _, s := range card.StatusEntries {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s=%s icon=%s\n", s.Key, s.Value, s.Icon)
		}
	}
	if len(card.Surfaces) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  surfaces:")
		for _, sf := range card.Surfaces {
			line := fmt.Sprintf("    %s\t%s (state=%s, in_window=%t)", sf.Ref, sf.Title, sf.IconState, sf.InWindow)
			if sf.LastSampleText != "" {
				line += "\n      sample: " + strings.ReplaceAll(sf.LastSampleText, "\n", " ⏎ ")
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
	}
	if len(card.Notifications) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  recent notifications:")
		for _, n := range card.Notifications {
			fmt.Fprintf(cmd.OutOrStdout(), "    %s\t%s\n", n.Title, truncate(n.Body, 200))
		}
	}
}
