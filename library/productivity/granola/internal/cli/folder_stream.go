// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:client-call
// folder stream reads granola.Cache via openGranolaCache() in granola_helpers.go
// and meeting artifacts via buildArtifacts() in extract.go — both already pass the
// store/sibling-import signal. The heuristic inspects files individually, so this
// marker asserts the same real-data shape for folder_stream.go.

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
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			out := []map[string]any{}
			for fid, md := range c.DocumentListsMetadata {
				out = append(out, map[string]any{
					"id":            fid,
					"title":         md.Title,
					"description":   md.Description,
					"workspace_id":  md.WorkspaceID,
					"meeting_count": len(c.DocumentLists[fid]),
					"is_favourited": md.IsFavourited,
					"parent_id":     md.ParentDocumentListID,
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
			c, err := openGranolaCache()
			if err != nil {
				return err
			}
			f := c.FolderByName(key)
			if f == nil {
				return notFoundErr(fmt.Errorf("folder %q not found", key))
			}
			mids := c.FolderMeetings(f.ID)
			w := cmd.OutOrStdout()
			for _, mid := range mids {
				a, err := buildArtifacts(mid, flags.dataSource != "local", panel)
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
