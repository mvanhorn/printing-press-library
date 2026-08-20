// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
//
// Clip↔workspace membership index. After sync fetches the workspace list, this
// walks each synced project, fetches its clip list (GET /api/project/{id}),
// and records clip↔workspace pairs so `clips list`/`top`/`grep` can show a
// workspace column and `analytics --group-by project` can roll up by project.
// Best-effort: a per-project failure warns but never fails the overall sync.

package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/store"
)

// syncWorkspaceMembership rebuilds the clip↔workspace index for every workspace
// currently in the local store. Returns the number of projects indexed.
func syncWorkspaceMembership(ctx context.Context, c *client.Client, db *store.Store) (int, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT "id" FROM "workspace"`)
	if err != nil {
		return 0, fmt.Errorf("listing workspaces: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	indexed := 0
	var firstErr error
	for _, id := range ids {
		if id == "" {
			continue
		}
		path := replacePathParam("/api/project/{project_id}", "project_id", id)
		data, gerr := c.Get(ctx, path, nil)
		if gerr != nil {
			if firstErr == nil {
				firstErr = gerr
			}
			continue
		}
		clipIDs := extractProjectClipIDs(data)
		if err := db.ReplaceWorkspaceMembership(id, clipIDs); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		indexed++
	}
	return indexed, firstErr
}

// extractProjectClipIDs pulls clip IDs out of a GET /api/project/{id} response.
// Suno's project payload shape isn't contractually fixed, so this is tolerant:
// it scans the common array keys and, per element, accepts a nested clip
// object, a clip_id scalar, or a bare id. Returns a de-duplicated slice.
func extractProjectClipIDs(data json.RawMessage) []string {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}

	for _, key := range []string{"project_clips", "clips", "project_clip", "data"} {
		raw, ok := root[key]
		if !ok {
			continue
		}
		var arr []map[string]json.RawMessage
		if err := json.Unmarshal(raw, &arr); err != nil {
			continue
		}
		for _, el := range arr {
			// Nested clip object: {"clip": {"id": ...}}
			if clipRaw, ok := el["clip"]; ok {
				var clip map[string]any
				if json.Unmarshal(clipRaw, &clip) == nil {
					add(stringField(clip, "id"))
					continue
				}
			}
			// Scalar reference: {"clip_id": "..."} or bare {"id": "..."}
			var asAny map[string]any
			b, _ := json.Marshal(el)
			if json.Unmarshal(b, &asAny) == nil {
				if cid := stringField(asAny, "clip_id"); cid != "" {
					add(cid)
					continue
				}
				add(stringField(asAny, "id"))
			}
		}
	}
	return out
}

// stringField returns m[key] as a string when it is one, else "".
func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// enrichClipsWithWorkspace injects a "workspace" field (comma-joined list of
// the workspaces a clip belongs to) into each clip object of a live clips-list
// response, using the local membership index. Tolerant of response shape (a
// bare array or an object wrapping the array under clips/data/results/items)
// and fully best-effort: a missing/unsynced store, an unexpected shape, or an
// empty index returns the input unchanged.
func enrichClipsWithWorkspace(ctx context.Context, data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return data
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("suno-pp-cli"))
	if err != nil {
		return data
	}
	defer db.Close()

	var top any
	if err := json.Unmarshal(data, &top); err != nil {
		return data
	}
	var arr []any
	var container map[string]any
	var arrKey string
	switch v := top.(type) {
	case []any:
		arr = v
	case map[string]any:
		container = v
		for _, k := range []string{"clips", "data", "results", "items"} {
			if a, ok := v[k].([]any); ok {
				arr, arrKey = a, k
				break
			}
		}
	}
	if len(arr) == 0 {
		return data
	}

	ids := make([]string, 0, len(arr))
	for _, el := range arr {
		if m, ok := el.(map[string]any); ok {
			if id, ok := m["id"].(string); ok {
				ids = append(ids, id)
			}
		}
	}
	labels, err := db.WorkspaceLabelsForClips(ids)
	if err != nil || len(labels) == 0 {
		return data
	}
	for _, el := range arr {
		if m, ok := el.(map[string]any); ok {
			if id, ok := m["id"].(string); ok {
				if label, ok := labels[id]; ok {
					m["workspace"] = label
				}
			}
		}
	}

	var out any = arr
	if container != nil && arrKey != "" {
		container[arrKey] = arr
		out = container
	}
	b, err := json.Marshal(out)
	if err != nil {
		return data
	}
	return b
}

// attachWorkspaceColumn fills a per-row workspace label (comma-joined list of
// the workspaces each clip belongs to) from the membership index. Best-effort:
// a lookup error or empty index leaves labels unset rather than failing the
// read command. ids must be index-aligned with the rows the setter writes.
func attachWorkspaceColumn(db *store.Store, ids []string, set func(i int, label string), n int) {
	if db == nil || len(ids) == 0 {
		return
	}
	labels, err := db.WorkspaceLabelsForClips(ids)
	if err != nil || len(labels) == 0 {
		return
	}
	for i := 0; i < n && i < len(ids); i++ {
		if label, ok := labels[ids[i]]; ok {
			set(i, label)
		}
	}
}
