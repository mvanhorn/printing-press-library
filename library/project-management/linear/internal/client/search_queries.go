package client

import "fmt"

// Server-side issue search.
//
// The pre-existing IssueSearchQuery constant targets `issueSearch`, whose own
// schema description says it is deprecated and that `searchIssues` replaces
// it. These documents target `searchIssues`, which is the supported surface,
// and they select the fields the local sync cannot supply: url (IssuesQuery
// has no url field, so a locally stored issue never carries one) plus the
// state object, description and timestamps needed to score a live-only
// candidate the same way a local one is scored.
//
// searchIssues is rate-limited to 30 requests per minute, so callers keep it
// behind an explicit opt-in flag.

// SearchIssuesQuery runs Linear's full-text plus vector issue search.
//
// term is required. teamId, filter and snippetSize are optional and may be
// passed as nil. includeArchived is pinned false: an archived issue is not a
// live duplicate, and the local index already retains archived rows forever
// because sync never prunes.
const SearchIssuesQuery = `query($term: String!, $first: Int, $teamId: String, $filter: IssueFilter, $snippetSize: Float) {
  searchIssues(term: $term, first: $first, teamId: $teamId, filter: $filter, snippetSize: $snippetSize, includeArchived: false) {
    totalCount
    nodes {
      id
      identifier
      title
      description
      url
      createdAt
      updatedAt
      state { id name type }
      team { id key name }
      project { id name }
    }
  }
}`

// SemanticSearchIssuesQuery runs Linear's vector search restricted to issues.
//
// SemanticSearchResult carries no score field, so a hit is a boolean signal
// and nothing more. The payload's `enabled` field is the only way to learn
// whether the workspace has the feature switched on: when it is false the
// signal is absent, not negative.
const SemanticSearchIssuesQuery = `query($query: String!, $maxResults: Int) {
  semanticSearch(query: $query, types: [issue], maxResults: $maxResults, includeArchived: false) {
    enabled
    results {
      id
      type
      issue {
        id
        identifier
        title
        description
        url
        createdAt
        updatedAt
        state { id name type }
        team { id key name }
        project { id name }
      }
    }
  }
}`

// SearchIssueNode is one issue returned by either search leg. The two
// documents select the same shape so a caller can merge them without
// branching.
type SearchIssueNode struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	State       struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
	Team struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"team"`
	Project *struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
}

// SearchIssuesResult is the decoded searchIssues payload.
type SearchIssuesResult struct {
	Nodes      []SearchIssueNode
	TotalCount int
}

// SemanticSearchIssuesResult is the decoded semanticSearch payload.
// Enabled reports whether the workspace has semantic search switched on. When
// it is false, Nodes is empty and that emptiness carries no information.
type SemanticSearchIssuesResult struct {
	Enabled bool
	Nodes   []SearchIssueNode
}

// SearchIssuesOptions are the optional arguments of SearchIssues. Zero values
// mean "omit the argument" rather than "send zero", so the server default
// applies.
type SearchIssuesOptions struct {
	Term        string
	First       int
	TeamID      string
	Filter      map[string]any
	SnippetSize int
}

// SearchIssues executes SearchIssuesQuery and decodes the payload.
func (c *Client) SearchIssues(opts SearchIssuesOptions) (SearchIssuesResult, error) {
	var out SearchIssuesResult
	if opts.Term == "" {
		return out, fmt.Errorf("searchIssues: term is required")
	}
	vars := map[string]any{"term": opts.Term}
	if opts.First > 0 {
		vars["first"] = opts.First
	}
	if opts.TeamID != "" {
		vars["teamId"] = opts.TeamID
	}
	if len(opts.Filter) > 0 {
		vars["filter"] = opts.Filter
	}
	if opts.SnippetSize > 0 {
		vars["snippetSize"] = opts.SnippetSize
	}
	var resp struct {
		SearchIssues struct {
			TotalCount int               `json:"totalCount"`
			Nodes      []SearchIssueNode `json:"nodes"`
		} `json:"searchIssues"`
	}
	if err := c.QueryInto(SearchIssuesQuery, vars, &resp); err != nil {
		return out, err
	}
	out.Nodes = resp.SearchIssues.Nodes
	out.TotalCount = resp.SearchIssues.TotalCount
	return out, nil
}

// SemanticSearchIssues executes SemanticSearchIssuesQuery and flattens the
// result list down to the issues it carried. Results whose issue is null are
// dropped: the type filter should prevent them, but a null would otherwise
// become a zero-valued candidate.
func (c *Client) SemanticSearchIssues(query string, maxResults int) (SemanticSearchIssuesResult, error) {
	var out SemanticSearchIssuesResult
	if query == "" {
		return out, fmt.Errorf("semanticSearch: query is required")
	}
	vars := map[string]any{"query": query}
	if maxResults > 0 {
		vars["maxResults"] = maxResults
	}
	var resp struct {
		SemanticSearch struct {
			Enabled bool `json:"enabled"`
			Results []struct {
				ID    string           `json:"id"`
				Type  string           `json:"type"`
				Issue *SearchIssueNode `json:"issue"`
			} `json:"results"`
		} `json:"semanticSearch"`
	}
	if err := c.QueryInto(SemanticSearchIssuesQuery, vars, &resp); err != nil {
		return out, err
	}
	out.Enabled = resp.SemanticSearch.Enabled
	for _, r := range resp.SemanticSearch.Results {
		if r.Issue == nil || r.Issue.ID == "" {
			continue
		}
		out.Nodes = append(out.Nodes, *r.Issue)
	}
	return out, nil
}
