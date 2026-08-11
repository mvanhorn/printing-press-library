package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

// Server-side leg of `issues search --live`.
//
// The local FTS5 index has no synonyms, no fuzzy matching and no
// transposition tolerance, so `auth` and `authentication` stem apart and a
// misspelling matches nothing. Those recall failures are silent, which makes
// an empty local result set weaker evidence than it looks. The live leg is
// the only surface in reach that can catch one, at the cost of an API call
// against an endpoint rate limited to 30 requests per minute.

// searchNodePayload renders a live search node into the same JSON shape the
// sync writes to issues.data, so a live row is indistinguishable from a
// synced one to every downstream reader.
func searchNodePayload(node client.SearchIssueNode) (json.RawMessage, error) {
	payload := map[string]any{
		"id":          node.ID,
		"identifier":  node.Identifier,
		"title":       node.Title,
		"description": node.Description,
		"url":         node.URL,
		"createdAt":   node.CreatedAt,
		"updatedAt":   node.UpdatedAt,
		"state":       map[string]any{"id": node.State.ID, "name": node.State.Name, "type": node.State.Type},
		"team":        map[string]any{"id": node.Team.ID, "key": node.Team.Key, "name": node.Team.Name},
		"teamId":      node.Team.ID,
	}
	if node.Project != nil {
		payload["project"] = map[string]any{"id": node.Project.ID, "name": node.Project.Name}
		payload["projectId"] = node.Project.ID
	}
	return json.Marshal(payload)
}

// mergeLiveIssueSearch appends server-side results that the local index did
// not already return, deduplicating by issue id, and write-throughs every
// live node into the local store the way the live issue list path does.
//
// A live failure is an error rather than a quiet downgrade to local-only
// results: a caller that asked for the live leg asked for the recall it
// provides, and swallowing the failure would turn into a false negative.
func mergeLiveIssueSearch(flags *rootFlags, db *store.Store, local []json.RawMessage, query, teamID string, limit int) ([]json.RawMessage, error) {
	c, err := newPortfolioLookupClient(flags)
	if err != nil {
		return local, err
	}
	res, err := c.SearchIssues(client.SearchIssuesOptions{
		Term:   query,
		First:  limit,
		TeamID: teamID,
	})
	if err != nil {
		return local, classifyAPIError(fmt.Errorf("live searchIssues failed: %w", err), flags)
	}

	seen := make(map[string]struct{}, len(local))
	for _, raw := range local {
		var row struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &row); err == nil && row.ID != "" {
			seen[row.ID] = struct{}{}
		}
	}

	merged := local
	for _, node := range res.Nodes {
		if node.ID == "" {
			continue
		}
		raw, err := searchNodePayload(node)
		if err != nil {
			continue
		}
		if db != nil {
			_ = db.UpsertIssue(node.ID, node.Identifier, node.Title, raw)
		}
		if _, dup := seen[node.ID]; dup {
			continue
		}
		seen[node.ID] = struct{}{}
		merged = append(merged, raw)
	}
	return merged, nil
}
