// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/cmuxclient"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/cmux/internal/snapshotstore"

	"github.com/spf13/cobra"
)

// newCmuxSearchCmd is the cmux-shaped search that searches workspace titles,
// surface titles, notification bodies, AND sampled pane content together,
// and supports --switch to focus the matching surface.
func newCmuxSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var doSwitch bool
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search across workspace titles, surface titles, notifications, and sampled pane content",
		Long: `cmux-pp-cli search returns rich rows: workspace, surface, source kind,
and a snippet — not just the workspace match.

Sources searched in order: workspace titles, surface titles (live, no
sync needed), notification bodies (synced), pane content samples (FTS5,
populated by ` + "`cmux-pp-cli panes sample`" + `).

Pass --switch to focus the top-ranked surface match via cmux's
tab-action — useful when you remember a phrase but not which tab. Without
matches, --switch is a no-op.`,
		Example: `  cmux-pp-cli search "WAF cookie" --json
  cmux-pp-cli search "tonight" --switch
  cmux-pp-cli search rate-limit --limit 5 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := args[0]
			ctx := cmd.Context()
			results, err := runCmuxSearch(ctx, query, limit)
			if err != nil {
				return err
			}
			if doSwitch {
				if len(results) == 0 {
					if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
						return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
							"query":    query,
							"results":  results,
							"switched": false,
							"reason":   "no matches",
						})
					}
					fmt.Fprintln(cmd.OutOrStdout(), "no matches; nothing to switch to")
					return nil
				}
				if isVerifyOrDogfood() {
					fmt.Fprintf(cmd.OutOrStdout(), "would switch to %s/%s\n", results[0].WorkspaceRef, results[0].SurfaceRef)
				} else {
					if err := cmuxclient.FocusSurface(ctx, results[0].WorkspaceRef, results[0].SurfaceRef); err != nil {
						return err
					}
				}
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
						"query":    query,
						"results":  results,
						"switched": true,
						"target":   results[0],
					})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "switched to %s/%s (%s)\n", results[0].WorkspaceRef, results[0].SurfaceRef, results[0].Source)
				return nil
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"query":   query,
					"results": results,
				})
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no matches")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "SOURCE\tWORKSPACE\tSURFACE\tSNIPPET")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", r.Source, r.WorkspaceRef, r.SurfaceRef, r.Snippet)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "max results per source bucket")
	cmd.Flags().BoolVar(&doSwitch, "switch", false, "focus the top-ranked surface match via cmux tab-action (write-side; opt-in)")
	return cmd
}

// SearchHit is one returned row.
type SearchHit struct {
	Source        string  `json:"source"` // workspace_title / surface_title / notification / pane_sample
	WorkspaceRef  string  `json:"workspace_ref,omitempty"`
	SurfaceRef    string  `json:"surface_ref,omitempty"`
	Title         string  `json:"title,omitempty"`
	Snippet       string  `json:"snippet"`
	SampledAtUnix float64 `json:"sampled_at_unix,omitempty"`
}

// runCmuxSearch searches all four sources and returns flat hits.
func runCmuxSearch(ctx context.Context, query string, limit int) ([]SearchHit, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, errors.New("empty query")
	}
	if limit <= 0 {
		limit = 20
	}
	lo := strings.ToLower(q)

	hits := make([]SearchHit, 0)
	wss, err := cmuxclient.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	// Workspace titles
	for _, w := range wss {
		if strings.Contains(strings.ToLower(w.Title), lo) {
			hits = append(hits, SearchHit{
				Source:       "workspace_title",
				WorkspaceRef: w.Ref,
				Title:        w.Title,
				Snippet:      w.Title,
			})
		}
	}
	// Surface titles
	for _, w := range wss {
		surfaces, err := cmuxclient.ListSurfaces(ctx, w.Ref)
		if err != nil {
			continue
		}
		for _, s := range surfaces {
			if strings.Contains(strings.ToLower(s.Title), lo) {
				hits = append(hits, SearchHit{
					Source:       "surface_title",
					WorkspaceRef: w.Ref,
					SurfaceRef:   s.Ref,
					Title:        s.Title,
					Snippet:      s.Title,
				})
			}
		}
	}
	// Notifications (live; titles + body)
	notes, _ := cmuxclient.ListNotifications(ctx)
	for _, n := range notes {
		t := strings.ToLower(n.Title)
		b := strings.ToLower(n.Body)
		if !strings.Contains(t, lo) && !strings.Contains(b, lo) {
			continue
		}
		surfaceRef := ""
		// We don't have a notification.workspace_id -> workspace.ref map.
		// Notifications still surface as hits without surface ref.
		hits = append(hits, SearchHit{
			Source:     "notification",
			SurfaceRef: surfaceRef,
			Title:      n.Title,
			Snippet:    truncate(n.Body, 240),
		})
	}
	// Pane content samples via FTS5
	ss, err := snapshotstore.Open(ctx, "")
	if err == nil {
		defer ss.Close()
		paneHits, ferr := ss.SearchPaneContent(ctx, q, limit)
		if ferr == nil {
			for _, p := range paneHits {
				hits = append(hits, SearchHit{
					Source:        "pane_sample",
					WorkspaceRef:  p.WorkspaceRef,
					SurfaceRef:    p.SurfaceRef,
					Snippet:       p.Snippet,
					SampledAtUnix: p.SampledAtUnix,
				})
			}
		}
	}
	// Cap output total
	if len(hits) > limit*4 {
		hits = hits[:limit*4]
	}
	return hits, nil
}
