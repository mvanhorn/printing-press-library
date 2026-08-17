// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:client-call
// folder stream reads local Granola data via openGranolaRead() in
// granola_helpers.go and meeting artifacts via buildArtifacts() in extract.go —
// both already pass the store/sibling-import signal. The heuristic inspects
// files individually, so this marker asserts the same real-data shape for
// folder_stream.go.

func newFolderCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folder",
		Short: "Stream meetings in a Granola folder with notes + panels inlined",
	}
	cmd.AddCommand(newFolderStreamCmd(flags))
	cmd.AddCommand(newFolderListCmd(flags))
	return cmd
}

func newFolderListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List folders (documentLists) from cache",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// PATCH(dual-path-store-read): store first, cache fallback.
			// description / is_favourited exist only in the cache's
			// documentListsMetadata, so store-sourced folders report them
			// empty rather than dropping the folder entirely.
			v, err := openGranolaRead(cmd.Context())
			if err != nil {
				return err
			}
			defer v.Close()
			out := []map[string]any{}
			for _, f := range v.Folders() {
				out = append(out, map[string]any{
					"id":            f.ID,
					"title":         f.Title,
					"description":   f.Description,
					"workspace_id":  f.WorkspaceID,
					"meeting_count": len(v.FolderMeetings(f.ID)),
					"is_favourited": f.IsFavourited,
					"parent_id":     f.ParentDocumentListID,
				})
			}
			return emitJSON(cmd, flags, out)
		},
	}
	return cmd
}

func newFolderStreamCmd(flags *rootFlags) *cobra.Command {
	var panel string
	cmd := &cobra.Command{
		Use:   "stream <folder-id-or-name>",
		Short: "ndjson stream of meetings in a folder with notes + a named panel inlined",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			key := args[0]
			// PATCH(dual-path-store-read): store first, cache fallback.
			v, err := openGranolaRead(cmd.Context())
			if err != nil {
				return err
			}
			defer v.Close()
			f := v.FolderByName(key)
			if f == nil {
				return notFoundErr(fmt.Errorf("folder %q not found", key))
			}
			mids := v.FolderMeetings(f.ID)
			w := cmd.OutOrStdout()
			for _, mid := range mids {
				a, err := buildArtifacts(cmd.Context(), mid, flags.dataSource != "local", panel)
				if err != nil {
					_ = emitNDJSONLine(w, map[string]any{"id": mid, "error": err.Error()})
					continue
				}
				rec := map[string]any{
					"id":          mid,
					"title":       a.Doc.Title,
					"started_at":  a.Doc.CreatedAt,
					"notes_human": a.NotesHuman,
				}
				if panel != "" {
					rec["panel"] = a.PanelMap[panel]
				} else {
					rec["panel"] = a.PanelSummary
				}
				_ = emitNDJSONLine(w, rec)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&panel, "panel", "", "Panel template slug to inline (default: best available)")
	return cmd
}
