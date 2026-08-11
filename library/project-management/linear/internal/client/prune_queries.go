package client

import (
	"encoding/json"
	"fmt"
)

// Slim id-only enumerations used by sync's reconcile pass.
//
// A reconcile pass may only delete local rows once it holds the COMPLETE set of
// live upstream ids for a resource. The fat TeamsQuery, UsersQuery and
// WorkflowStatesQuery documents ask for no pageInfo and no cursor, so they can
// never prove they reached the last page. These documents can, and they cost
// almost nothing because they select a single field per node.
//
// Resources whose sync document already pages with pageInfo (issues, projects,
// cycles, labels) do not need a second enumeration: their own fetch reports
// completeness through PaginatedQueryComplete.

const TeamIDsQuery = `query($first: Int!, $after: String) {
  teams(first: $first, after: $after) {
    nodes { id }
    pageInfo { hasNextPage endCursor }
  }
}`

const UserIDsQuery = `query($first: Int!, $after: String) {
  users(first: $first, after: $after) {
    nodes { id }
    pageInfo { hasNextPage endCursor }
  }
}`

const WorkflowStateIDsQuery = `query($first: Int!, $after: String) {
  workflowStates(first: $first, after: $after) {
    nodes { id }
    pageInfo { hasNextPage endCursor }
  }
}`

// PaginatedQueryComplete is PaginatedQueryMax plus the two facts a reconcile
// pass needs: how many pages were actually fetched, and whether the crawl ran
// to the last page. complete is false when maxPages cut the crawl short, when
// the transport is in dry-run mode, or when a page failed midway. Callers MUST
// treat complete == false as "this live set is a subset, do not prune".
func (c *Client) PaginatedQueryComplete(query string, variables map[string]any, fieldPath string, pageSize int, maxPages int) ([]json.RawMessage, bool, int, error) {
	if variables == nil {
		variables = map[string]any{}
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	variables["first"] = pageSize

	var all []json.RawMessage
	pages := 0
	for {
		data, err := c.Query(query, variables)
		if err != nil {
			return all, false, pages, err
		}
		if len(data) == 0 {
			// Dry-run transport, or an empty body. Nothing was enumerated, so
			// this is never a complete crawl.
			return all, false, pages, nil
		}

		var root map[string]json.RawMessage
		if err := json.Unmarshal(data, &root); err != nil {
			return all, false, pages, fmt.Errorf("parsing paginated root: %w", err)
		}
		connRaw, ok := root[fieldPath]
		if !ok {
			return all, false, pages, fmt.Errorf("field %q not found in response", fieldPath)
		}
		var conn struct {
			Nodes    []json.RawMessage `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		}
		if err := json.Unmarshal(connRaw, &conn); err != nil {
			return all, false, pages, fmt.Errorf("parsing connection %q: %w", fieldPath, err)
		}

		all = append(all, conn.Nodes...)
		pages++
		if !conn.PageInfo.HasNextPage {
			return all, true, pages, nil
		}
		if maxPages > 0 && pages >= maxPages {
			return all, false, pages, nil
		}
		variables["after"] = conn.PageInfo.EndCursor
	}
}

// LiveIDs runs a slim id-only enumeration and returns the ids it saw, whether
// the crawl was complete, and how many pages it took.
func (c *Client) LiveIDs(query string, variables map[string]any, fieldPath string, pageSize int, maxPages int) ([]string, bool, int, error) {
	nodes, complete, pages, err := c.PaginatedQueryComplete(query, variables, fieldPath, pageSize, maxPages)
	return NodeIDs(nodes), complete, pages, err
}

// NodeIDs pulls the id field out of raw connection nodes, skipping any node
// that carries no id.
func NodeIDs(nodes []json.RawMessage) []string {
	ids := make([]string, 0, len(nodes))
	for _, node := range nodes {
		var n struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(node, &n); err != nil || n.ID == "" {
			continue
		}
		ids = append(ids, n.ID)
	}
	return ids
}
