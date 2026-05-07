package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GraphQLRequest is a standard GraphQL POST body.
type GraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// GraphQLResponse wraps the standard GraphQL response envelope.
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error.
type GraphQLError struct {
	Message    string `json:"message"`
	Extensions struct {
		Code            string `json:"code"`
		UserPresentable bool   `json:"userPresentableMessage"`
	} `json:"extensions"`
}

// Query executes a GraphQL query and returns the raw data payload.
// In dry-run mode the underlying transport prints the request and returns
// a synthetic envelope; callers receive an empty payload so dry-run never
// fails downstream JSON decoding.
func (c *Client) Query(query string, variables map[string]any) (json.RawMessage, error) {
	req := GraphQLRequest{Query: query, Variables: variables}
	raw, _, err := c.Post("/graphql", req)
	if err != nil {
		return nil, err
	}
	if c.DryRun {
		return nil, nil
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("decoding graphql response: %w", err)
	}
	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		for i, e := range resp.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("graphql: %s", strings.Join(msgs, "; "))
	}
	return resp.Data, nil
}

// QueryInto executes a GraphQL query and unmarshals the data into dest.
// In dry-run mode it returns nil without touching dest, so callers can
// continue without seeing a JSON-decode error on the dry-run envelope.
func (c *Client) QueryInto(query string, variables map[string]any, dest any) error {
	data, err := c.Query(query, variables)
	if err != nil {
		return err
	}
	if c.DryRun || len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, dest)
}

// Mutate executes a GraphQL mutation and returns the raw data payload.
func (c *Client) Mutate(query string, variables map[string]any) (json.RawMessage, error) {
	return c.Query(query, variables)
}

// PaginatedQuery fetches all pages of a connection field.
// queryFn returns the query string with $after variable; fieldPath is the dot-path to the connection
// (e.g., "issues"). Returns all collected nodes.
func (c *Client) PaginatedQuery(query string, variables map[string]any, fieldPath string, pageSize int) ([]json.RawMessage, error) {
	return c.PaginatedQueryMax(query, variables, fieldPath, pageSize, 0)
}

// PaginatedQueryMax is like PaginatedQuery but stops after maxPages pages (0 = unlimited).
func (c *Client) PaginatedQueryMax(query string, variables map[string]any, fieldPath string, pageSize int, maxPages int) ([]json.RawMessage, error) {
	if variables == nil {
		variables = map[string]any{}
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	variables["first"] = pageSize

	var all []json.RawMessage
	hasNext := true
	pagesFetched := 0
	for hasNext {
		data, err := c.Query(query, variables)
		if err != nil {
			return all, err
		}

		// Navigate to the connection field
		var root map[string]json.RawMessage
		if err := json.Unmarshal(data, &root); err != nil {
			return all, fmt.Errorf("parsing paginated root: %w", err)
		}

		connRaw, ok := root[fieldPath]
		if !ok {
			return all, fmt.Errorf("field %q not found in response", fieldPath)
		}

		var conn struct {
			Nodes    []json.RawMessage `json:"nodes"`
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
		}
		if err := json.Unmarshal(connRaw, &conn); err != nil {
			return all, fmt.Errorf("parsing connection %q: %w", fieldPath, err)
		}

		all = append(all, conn.Nodes...)
		hasNext = conn.PageInfo.HasNextPage
		pagesFetched++
		if maxPages > 0 && pagesFetched >= maxPages {
			break
		}
		if hasNext {
			variables["after"] = conn.PageInfo.EndCursor
		}
	}
	return all, nil
}
