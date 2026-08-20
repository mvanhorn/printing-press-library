// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"encoding/json"
	"fmt"
)

// AddURLSource adds a web URL source to a notebook.
func (c *Client) AddURLSource(ctx context.Context, notebookID, url string) (Source, error) {
	raw, err := c.Call(ctx, RPCAddSource, notebookPath(notebookID), BuildAddURLSourceParams(notebookID, url))
	if err != nil {
		return Source{}, err
	}
	return parseAddedSource(raw)
}

// DeleteSource removes a source from a notebook.
func (c *Client) DeleteSource(ctx context.Context, notebookID, sourceID string) error {
	_, err := c.Call(ctx, RPCDeleteSource, notebookPath(notebookID), BuildDeleteSourceParams(sourceID))
	return err
}

// ListSources returns sources for a notebook via GET_NOTEBOOK.
func (c *Client) ListSources(ctx context.Context, notebookID string) ([]Source, error) {
	detail, err := c.GetNotebook(ctx, notebookID)
	if err != nil {
		return nil, err
	}
	return detail.Sources, nil
}

func parseAddedSource(raw json.RawMessage) (Source, error) {
	var row any
	if err := json.Unmarshal(raw, &row); err != nil {
		return Source{}, err
	}
	// ADD_SOURCE may return nested shapes; walk for first source row.
	if src, ok := findSourceInValue(row); ok {
		return src, nil
	}
	return Source{}, fmt.Errorf("parse added source: unexpected shape")
}

func findSourceInValue(v any) (Source, bool) {
	if src, ok := parseSourceRow(v); ok {
		return src, true
	}
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			if src, ok := findSourceInValue(item); ok {
				return src, true
			}
		}
	}
	return Source{}, false
}
