// Hand-written GraphQL helpers for monday.com.
//
// monday.com's GraphQL is non-Relay: list resources accept (limit, page) for
// page-based pagination, items accept (limit, cursor) via items_page, and
// every operation is a POST against /v2 with `{"query","variables"}`.

package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// GraphQLRequest is the body shape monday.com accepts.
type GraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// GraphQLError mirrors a single error entry from monday's GraphQL response.
type GraphQLError struct {
	Message    string         `json:"message"`
	Path       []any          `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Error reports the message in a stable shape.
func (e GraphQLError) Error() string { return e.Message }

// GraphQLResponse is the envelope returned by monday.com.
type GraphQLResponse struct {
	Data       json.RawMessage `json:"data,omitempty"`
	Errors     []GraphQLError  `json:"errors,omitempty"`
	AccountID  any             `json:"account_id,omitempty"`
	Complexity any             `json:"complexity,omitempty"`
}

// GraphQLOptions tweaks a single request.
type GraphQLOptions struct {
	APIVersion string // sets the API-Version header (e.g. "2026-04")
}

// GraphQL POSTs a query/mutation to monday.com's /v2 endpoint and returns the
// `data` field of the response. Errors in the GraphQL `errors` array are
// converted to a Go error with the joined messages; auth failures map to a
// dedicated APIError so the CLI can emit exit code 4.
func (c *Client) GraphQL(query string, variables map[string]any) (json.RawMessage, error) {
	return c.GraphQLWithOptions(query, variables, GraphQLOptions{})
}

// GraphQLWithOptions is the explicit form. Use it to pin an API-Version per
// call (some monday endpoints differ by version).
func (c *Client) GraphQLWithOptions(query string, variables map[string]any, opts GraphQLOptions) (json.RawMessage, error) {
	body := GraphQLRequest{Query: query, Variables: variables}
	headers := map[string]string{}
	if opts.APIVersion != "" {
		headers["API-Version"] = opts.APIVersion
	}

	raw, status, err := c.PostWithHeaders("/v2", body, headers)
	if err != nil {
		return nil, err
	}
	_ = status

	// Dry-run path returns {"dry_run":true}
	if c.DryRun {
		return raw, nil
	}

	var resp GraphQLResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing GraphQL response: %w", err)
	}

	if len(resp.Errors) > 0 {
		// monday.com returns 200 OK with an errors array even for auth failures.
		msgs := make([]string, 0, len(resp.Errors))
		for _, e := range resp.Errors {
			msgs = append(msgs, e.Message)
		}
		joined := strings.Join(msgs, "; ")
		// Promote "Not Authenticated" into an APIError 401 for the classifier.
		lower := strings.ToLower(joined)
		if strings.Contains(lower, "not authenticated") || strings.Contains(lower, "unauthorized") {
			return nil, &APIError{Method: "POST", Path: "/v2", StatusCode: http.StatusUnauthorized, Body: joined}
		}
		// Complexity / rate-limit hints
		if strings.Contains(lower, "complexity") && (strings.Contains(lower, "limit") || strings.Contains(lower, "exceed")) {
			return nil, &APIError{Method: "POST", Path: "/v2", StatusCode: http.StatusTooManyRequests, Body: joined}
		}
		return nil, errors.New(joined)
	}

	return resp.Data, nil
}

// PaginatedList runs a list-style query that uses monday's (limit:Int, page:Int)
// shape and returns the `data.<root>` array merged across pages. Stops when an
// empty page comes back (monday paginates by repeatedly incrementing `page`).
//
// rootKey is the top-level field name in the response (e.g. "boards", "users",
// "docs", "workspaces"). rendered is the GraphQL query string with $limit and
// $page variables.
func (c *Client) PaginatedList(query, rootKey string, extraVars map[string]any, pageSize int) ([]json.RawMessage, error) {
	if pageSize <= 0 {
		pageSize = 100
	}
	var all []json.RawMessage
	for page := 1; ; page++ {
		vars := map[string]any{"limit": pageSize, "page": page}
		for k, v := range extraVars {
			vars[k] = v
		}
		data, err := c.GraphQL(query, vars)
		if err != nil {
			return all, err
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(data, &envelope); err != nil {
			return all, fmt.Errorf("parsing list envelope: %w", err)
		}
		raw, ok := envelope[rootKey]
		if !ok {
			break
		}
		var batch []json.RawMessage
		if err := json.Unmarshal(raw, &batch); err != nil {
			return all, fmt.Errorf("parsing %s array: %w", rootKey, err)
		}
		if len(batch) == 0 {
			break
		}
		all = append(all, batch...)
		if len(batch) < pageSize {
			break
		}
	}
	return all, nil
}
