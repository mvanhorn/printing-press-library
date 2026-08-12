package client

import (
	"encoding/json"
	"fmt"
)

// GraphQL documents for the issue-relation surface (IssueRelationType:
// blocks, duplicate, related, similar).
//
// Direction, not a fifth type. Linear stores "A blocks B" as a single
// IssueRelation{type: blocks, issue: A, relatedIssue: B}. It appears in
// A.relations (outgoing) and in B.inverseRelations (incoming). "blocked by"
// is therefore the incoming direction over a `blocks` relation and is never
// a value of IssueRelation.type. Every consumer of these documents must
// render it as a direction.

// IssueRelationsQuery reads both relation connections of one issue in a
// single document. $id is the issue UUID: the top-level issue(id:) argument
// behaves inconsistently with TEAM-NUMBER identifiers across workspaces, so
// callers resolve the identifier first.
//
// Both connections select the same node shape so consumers never have to
// branch on which connection a node came from. IssueRelation exposes both
// `issue` (the relation source) and `relatedIssue` (the relation target),
// verified against the introspected schema.
//
// $includeArchived maps to the includeArchived Boolean argument carried by
// both Issue.relations and Issue.inverseRelations, verified live in
// api-inventory.json. Declaring the variable changes nothing for a caller who
// does not pass it: an unprovided variable makes the argument absent rather
// than null, so the server default (exclude archived) still applies.
//
// archivedAt is selected on the relation itself (IssueRelation.archivedAt:
// DateTime) and on both issue projections (Issue.archivedAt: DateTime), both
// verified live in api-inventory.json. It is null on everything live, so it is
// free on the default path and is the only thing that tells an archived row
// apart once includeArchived is on.
const IssueRelationsQuery = `query($id: String!, $first: Int!, $after: String, $inverseAfter: String, $includeArchived: Boolean) {
  issue(id: $id) {
    id
    identifier
    title
    url
    state { id name type }
    team { id key }
    relations(first: $first, after: $after, includeArchived: $includeArchived) {
      nodes {
        id
        type
        createdAt
        archivedAt
        issue { id identifier title url archivedAt state { id name type } }
        relatedIssue { id identifier title url archivedAt state { id name type } }
      }
      pageInfo { hasNextPage endCursor }
    }
    inverseRelations(first: $first, after: $inverseAfter, includeArchived: $includeArchived) {
      nodes {
        id
        type
        createdAt
        archivedAt
        issue { id identifier title url archivedAt state { id name type } }
        relatedIssue { id identifier title url archivedAt state { id name type } }
      }
      pageInfo { hasNextPage endCursor }
    }
  }
}`

// IssueRelationCreateMutation creates one relation. IssueRelationCreateInput
// requires type, issueId and relatedIssueId.
const IssueRelationCreateMutation = `mutation($input: IssueRelationCreateInput!) {
  issueRelationCreate(input: $input) {
    success
    issueRelation {
      id
      type
      createdAt
      issue { id identifier title url state { id name type } }
      relatedIssue { id identifier title url state { id name type } }
    }
  }
}`

// IssueRelationDeleteMutation deletes one relation by its UUID and returns
// the DeletePayload, whose entityId echoes the deleted relation.
const IssueRelationDeleteMutation = `mutation($id: String!) {
  issueRelationDelete(id: $id) {
    success
    entityId
  }
}`

// UnblockedCandidatesQuery is the single crawl behind `unblocked`.
//
// $filter carries the open-group state predicate and the optional team, and
// deliberately carries nothing about relations. IssueFilter's
// hasBlockedByRelations is a bare RelationExistsComparator, and worse than
// merely coarse: Linear stops matching an issue once all of its blockers
// close, so filtering on it drops exactly the issues `unblocked` reports.
// "Blocked by something" and "are all its blockers closed" are both decided
// client-side over the inverseRelations this document selects inline, which
// is why each relation node carries the blocking issue's own state.
const UnblockedCandidatesQuery = `query($first: Int!, $after: String, $filter: IssueFilter, $relFirst: Int!) {
  issues(first: $first, after: $after, filter: $filter) {
    nodes {
      id
      identifier
      title
      url
      priority
      state { id name type }
      team { id key }
      inverseRelations(first: $relFirst) {
        nodes {
          id
          type
          createdAt
          issue { id identifier title url state { id name type } }
          relatedIssue { id identifier title url state { id name type } }
        }
        pageInfo { hasNextPage endCursor }
      }
    }
    pageInfo { hasNextPage endCursor }
  }
}`

// IssueRelationState is the workflow state carried on either side of a
// relation. Type is one of the seven API-documented WorkflowState.type
// values and is what group predicates are evaluated against.
type IssueRelationState struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// IssueRelationIssue is the issue projection carried on both sides of a
// relation node.
type IssueRelationIssue struct {
	ID         string             `json:"id"`
	Identifier string             `json:"identifier"`
	Title      string             `json:"title"`
	URL        string             `json:"url,omitempty"`
	ArchivedAt string             `json:"archivedAt,omitempty"`
	State      IssueRelationState `json:"state"`
}

// IssueRelationNode is one IssueRelation. Type is the IssueRelationType enum
// value as returned by the API. Issue is the relation source, RelatedIssue is
// the relation target, both always populated by the documents above.
type IssueRelationNode struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	CreatedAt    string             `json:"createdAt,omitempty"`
	ArchivedAt   string             `json:"archivedAt,omitempty"`
	Issue        IssueRelationIssue `json:"issue"`
	RelatedIssue IssueRelationIssue `json:"relatedIssue"`
}

// IssueRelationsResult is the fully paged result of IssueRelationsQuery for
// one issue. Relations are the outgoing side, InverseRelations the incoming
// side. Subject is the issue the two connections hang off.
type IssueRelationsResult struct {
	Subject          IssueRelationIssue  `json:"issue"`
	TeamID           string              `json:"team_id,omitempty"`
	TeamKey          string              `json:"team_key,omitempty"`
	Relations        []IssueRelationNode `json:"relations"`
	InverseRelations []IssueRelationNode `json:"inverse_relations"`
}

// relationConnection is the wire shape of either connection.
type relationConnection struct {
	Nodes    []IssueRelationNode `json:"nodes"`
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
}

// defaultRelationPageSize is the per-connection page size. Relation counts
// per issue are small in practice, so one page usually exhausts both sides.
const defaultRelationPageSize = 50

// maxRelationPages bounds the paging loop so a pathological issue cannot
// spin forever against the rate-limit budget. Hitting the bound is an error,
// never a short read: every consumer of this result treats "absent" as
// "no such relation", so a silently truncated set makes `relations list` drop
// links, makes idempotent creation duplicate an existing relation, and lets
// `unblocked` call an issue actionable while an unseen blocker is still open.
const maxRelationPages = 20

// FetchIssueRelations reads both relation connections of one issue to
// exhaustion, excluding archived relations. issueID must be a UUID.
// pageSize <= 0 uses the default.
func (c *Client) FetchIssueRelations(issueID string, pageSize int) (IssueRelationsResult, error) {
	return c.FetchIssueRelationsWith(issueID, pageSize, false)
}

// FetchIssueRelationsWith is FetchIssueRelations with the includeArchived
// connection argument exposed. includeArchived is only sent when true, so the
// default path emits exactly the variable set it always emitted.
//
// Nodes are deduplicated by relation id, which keeps the loop safe when a
// finished connection is re-requested with its last cursor while the other
// side is still paging.
func (c *Client) FetchIssueRelationsWith(issueID string, pageSize int, includeArchived bool) (IssueRelationsResult, error) {
	var out IssueRelationsResult
	if issueID == "" {
		return out, fmt.Errorf("fetching issue relations: empty issue id")
	}
	if pageSize <= 0 {
		pageSize = defaultRelationPageSize
	}
	seenOutgoing := map[string]bool{}
	seenIncoming := map[string]bool{}
	var after, inverseAfter string
	for range maxRelationPages {
		vars := map[string]any{"id": issueID, "first": pageSize}
		if includeArchived {
			vars["includeArchived"] = true
		}
		if after != "" {
			vars["after"] = after
		}
		if inverseAfter != "" {
			vars["inverseAfter"] = inverseAfter
		}
		var resp struct {
			Issue *struct {
				IssueRelationIssue
				Team struct {
					ID  string `json:"id"`
					Key string `json:"key"`
				} `json:"team"`
				Relations        relationConnection `json:"relations"`
				InverseRelations relationConnection `json:"inverseRelations"`
			} `json:"issue"`
		}
		if err := c.QueryInto(IssueRelationsQuery, vars, &resp); err != nil {
			return out, err
		}
		if resp.Issue == nil || resp.Issue.ID == "" {
			return out, fmt.Errorf("issue %q not found", issueID)
		}
		out.Subject = resp.Issue.IssueRelationIssue
		out.TeamID = resp.Issue.Team.ID
		out.TeamKey = resp.Issue.Team.Key
		for _, n := range resp.Issue.Relations.Nodes {
			if n.ID == "" || seenOutgoing[n.ID] {
				continue
			}
			seenOutgoing[n.ID] = true
			out.Relations = append(out.Relations, n)
		}
		for _, n := range resp.Issue.InverseRelations.Nodes {
			if n.ID == "" || seenIncoming[n.ID] {
				continue
			}
			seenIncoming[n.ID] = true
			out.InverseRelations = append(out.InverseRelations, n)
		}
		moreOutgoing := resp.Issue.Relations.PageInfo.HasNextPage && resp.Issue.Relations.PageInfo.EndCursor != ""
		moreIncoming := resp.Issue.InverseRelations.PageInfo.HasNextPage && resp.Issue.InverseRelations.PageInfo.EndCursor != ""
		if !moreOutgoing && !moreIncoming {
			return out, nil
		}
		if moreOutgoing {
			after = resp.Issue.Relations.PageInfo.EndCursor
		}
		if moreIncoming {
			inverseAfter = resp.Issue.InverseRelations.PageInfo.EndCursor
		}
	}
	return out, fmt.Errorf(
		"issue %s has more relations than %d pages of %d can read: refusing to answer from a partial relation set, retry with a larger page size",
		issueID, maxRelationPages, pageSize,
	)
}

// FetchIssueInverseRelations pages only the incoming side of one issue. Used
// by the unblocked traversal when a candidate's first inline page of
// inverseRelations was truncated, so the "all blockers closed" verdict is
// never computed from a partial blocker set.
func (c *Client) FetchIssueInverseRelations(issueID string, pageSize int) ([]IssueRelationNode, error) {
	res, err := c.FetchIssueRelations(issueID, pageSize)
	if err != nil {
		return nil, err
	}
	return res.InverseRelations, nil
}

// DecodeIssueRelationCreate parses an issueRelationCreate response.
func DecodeIssueRelationCreate(raw json.RawMessage) (IssueRelationNode, bool, error) {
	var parsed struct {
		IssueRelationCreate struct {
			Success       bool              `json:"success"`
			IssueRelation IssueRelationNode `json:"issueRelation"`
		} `json:"issueRelationCreate"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return IssueRelationNode{}, false, fmt.Errorf("parsing issueRelationCreate response: %w", err)
	}
	return parsed.IssueRelationCreate.IssueRelation, parsed.IssueRelationCreate.Success, nil
}

// DecodeIssueRelationDelete parses an issueRelationDelete response and
// returns the echoed entity id.
func DecodeIssueRelationDelete(raw json.RawMessage) (string, bool, error) {
	var parsed struct {
		IssueRelationDelete struct {
			Success  bool   `json:"success"`
			EntityID string `json:"entityId"`
		} `json:"issueRelationDelete"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", false, fmt.Errorf("parsing issueRelationDelete response: %w", err)
	}
	return parsed.IssueRelationDelete.EntityID, parsed.IssueRelationDelete.Success, nil
}
