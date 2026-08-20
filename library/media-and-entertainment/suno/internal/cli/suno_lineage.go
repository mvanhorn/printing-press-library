// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// pp:data-source local
//
// `lineage` — reconstruct a clip's iteration tree (extends / concats / cover /
// remix) from lineage signals stored in the local clip JSON. Reads the local
// SQLite store only; no network and no auth. Read-only.

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
	"github.com/spf13/cobra"
)

func newSunoLineageCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "lineage <clip_id>",
		Short: "Reconstruct a clip's iteration tree from local lineage signals",
		Long: "Render the iteration tree of a track — its extends, concats, cover, and " +
			"remix relationships — reconstructed from lineage references stored in the " +
			"local clip JSON (metadata.history, metadata.concat_history, " +
			"metadata.cover_clip_id, and the is_remix flag).\n\n" +
			"If no lineage data is stored for the clip, a single-node result is returned " +
			"with a note explaining that no ancestry is available.",
		Example:     "  suno-pp-cli lineage 7d869de4-9476-4a4d-a6f2-c0eec968a3e2\n  suno-pp-cli lineage <clip_id> --json",
		Annotations: map[string]string{"pp:data-source": "local", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("a clip id is required: lineage <clip_id>"))
			}
			clipID := args[0]

			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'suno-pp-cli sync' first.", err)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "clips")
			hintIfStale(cmd, db, "clips", flags.maxAge)

			result, err := buildLineage(db, clipID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return notFoundErr(fmt.Errorf("clip %q not found in local store. Run 'suno-pp-cli sync' first", clipID))
				}
				return fmt.Errorf("building lineage: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("suno-pp-cli"), "Path to local SQLite store")
	return cmd
}

// lineageNode is one node in the reconstructed iteration tree.
type lineageNode struct {
	ID       string         `json:"id"`
	Title    string         `json:"title,omitempty"`
	Relation string         `json:"relation,omitempty"` // how this node relates to its parent: root|extend|concat|cover
	IsRemix  bool           `json:"is_remix,omitempty"`
	Parents  []*lineageNode `json:"parents,omitempty"`
}

// lineageResult is the command output.
type lineageResult struct {
	ClipID string       `json:"clip_id"`
	Tree   *lineageNode `json:"tree"`
	Note   string       `json:"note,omitempty"`
}

// buildLineage loads the clip and walks its ancestry references upward,
// guarding against cycles and missing ancestors. The root clip must exist in
// the local store (else sql.ErrNoRows -> not-found); referenced ancestors that
// aren't stored are rendered as honest stubs rather than dropped.
func buildLineage(db *store.Store, clipID string) (lineageResult, error) {
	if _, _, _, present, err := loadClipForLineage(db, clipID); err != nil {
		return lineageResult{}, err
	} else if !present {
		return lineageResult{}, sql.ErrNoRows
	}

	visited := map[string]bool{}
	node, _, err := lineageNodeFor(db, clipID, "root", visited)
	if err != nil {
		return lineageResult{}, err
	}
	res := lineageResult{ClipID: clipID, Tree: node}
	if len(node.Parents) == 0 {
		res.Note = "no ancestry data stored for this clip (no metadata.history, concat_history, or cover reference found); showing single node"
	}
	return res, nil
}

// lineageNodeFor builds a node for clipID and recursively resolves its parent
// references. relation describes how this clip relates to the child that
// referenced it. The visited set breaks reference cycles.
func lineageNodeFor(db *store.Store, clipID, relation string, visited map[string]bool) (*lineageNode, bool, error) {
	if visited[clipID] {
		// Cycle / already-rendered ancestor: emit a stub without recursing.
		return &lineageNode{ID: clipID, Relation: relation}, true, nil
	}
	visited[clipID] = true

	title, data, isRemix, found, err := loadClipForLineage(db, clipID)
	if err != nil {
		return nil, false, err
	}
	if !found {
		// Referenced ancestor that isn't in the local store — keep it as a
		// stub so the tree shows the link honestly rather than dropping it.
		return &lineageNode{ID: clipID, Relation: relation}, true, nil
	}

	node := &lineageNode{ID: clipID, Title: title, Relation: relation, IsRemix: isRemix}
	for _, ref := range lineageParentRefs(data) {
		if ref.id == "" || ref.id == clipID {
			continue
		}
		parent, ok, perr := lineageNodeFor(db, ref.id, ref.relation, visited)
		if perr != nil {
			return nil, false, perr
		}
		if ok {
			node.Parents = append(node.Parents, parent)
		}
	}
	return node, true, nil
}

// loadClipForLineage fetches the title, raw JSON, and is_remix flag for a clip.
func loadClipForLineage(db *store.Store, clipID string) (title string, data json.RawMessage, isRemix, found bool, err error) {
	var t sql.NullString
	var d sql.NullString
	var remix sql.NullInt64
	row := db.DB().QueryRow(`SELECT title, data, is_remix FROM clips WHERE id = ?`, clipID)
	if scanErr := row.Scan(&t, &d, &remix); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return "", nil, false, false, nil
		}
		return "", nil, false, false, scanErr
	}
	return t.String, json.RawMessage(d.String), remix.Int64 == 1, true, nil
}

type lineageRef struct {
	id       string
	relation string
}

// lineageParentRefs extracts ancestor references from a stored clip JSON.
// Covers the Suno-conventional lineage signals:
//   - metadata.history       — array of prior clip refs for extends/edits
//   - metadata.concat_history — array of segment refs for concatenations
//   - metadata.cover_clip_id  — the source clip a cover was derived from
//
// History/concat entries may be bare id strings or objects carrying an "id"
// (or "clip_id"/"source_clip_id") field; both shapes are handled.
func lineageParentRefs(data json.RawMessage) []lineageRef {
	var refs []lineageRef
	if len(data) == 0 {
		return refs
	}
	var clip struct {
		Metadata map[string]json.RawMessage `json:"metadata"`
	}
	if err := json.Unmarshal(data, &clip); err != nil || clip.Metadata == nil {
		return refs
	}

	refs = append(refs, refsFromHistoryArray(clip.Metadata["history"], "extend")...)
	refs = append(refs, refsFromHistoryArray(clip.Metadata["concat_history"], "concat")...)

	if raw, ok := clip.Metadata["cover_clip_id"]; ok {
		if id := jsonStringID(raw); id != "" {
			refs = append(refs, lineageRef{id: id, relation: "cover"})
		}
	}
	return refs
}

// refsFromHistoryArray parses a metadata history-style array into refs.
func refsFromHistoryArray(raw json.RawMessage, relation string) []lineageRef {
	var refs []lineageRef
	if len(raw) == 0 {
		return refs
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return refs
	}
	for _, e := range entries {
		if id := jsonStringID(e); id != "" {
			refs = append(refs, lineageRef{id: id, relation: relation})
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(e, &obj); err != nil {
			continue
		}
		for _, key := range []string{"id", "clip_id", "source_clip_id"} {
			if v, ok := obj[key].(string); ok && v != "" {
				refs = append(refs, lineageRef{id: v, relation: relation})
				break
			}
		}
	}
	return refs
}

// jsonStringID returns the string value of raw if it is a JSON string, else "".
func jsonStringID(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return ""
}
