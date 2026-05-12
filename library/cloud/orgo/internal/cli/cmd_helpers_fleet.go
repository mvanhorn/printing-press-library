// Hand-authored: shared fleet helpers for novel commands.
// Fall through to the live API when the local computers store is empty —
// the spec exposes no /computers list endpoint, but /projects returns
// nested `desktops[]` per project, which is the canonical fleet source.
// Created during Phase 3 fixes; survives regen because it lives in its
// own file.
package cli

import (
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"
)

// loadFleetComputers returns every computer the user has access to as
// raw JSON rows, in two attempts:
//  1. db.List("computers") — populated by `computers get` calls and any
//     future sync hook for the computers resource.
//  2. Live fetch via GET /projects (rewritten from the spec's
//     /workspaces in client.do). The response is `{projects: [{id,
//     name, desktops: [...], ...}, ...]}`. Each desktop is annotated
//     with `workspace_id` (= project id) so downstream filtering
//     by workspace works.
//
// Returns the rows plus a string label noting where the data came from
// ("local" or "live") so commands can mention provenance in their
// output. Returns nil rows if both paths produce nothing.
func loadFleetComputers(flags *rootFlags, db *store.Store) ([]json.RawMessage, string, error) {
	rows, err := db.List("computers", 0)
	if err == nil && len(rows) > 0 {
		return rows, "local", nil
	}

	c, err := flags.newClient()
	if err != nil {
		return nil, "", err
	}
	data, err := c.Get("/workspaces", nil)
	if err != nil {
		return nil, "", err
	}

	var resp struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, "", err
	}

	var out []json.RawMessage
	for _, proj := range resp.Projects {
		projID, _ := proj["id"].(string)
		desks, _ := proj["desktops"].([]any)
		for _, d := range desks {
			dm, ok := d.(map[string]any)
			if !ok {
				continue
			}
			// Stamp workspace_id onto each computer for downstream filtering.
			if _, has := dm["workspace_id"]; !has {
				dm["workspace_id"] = projID
			}
			b, err := json.Marshal(dm)
			if err != nil {
				continue
			}
			out = append(out, b)
		}
	}
	return out, "live", nil
}
