// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"encoding/json"
	"fmt"
)

// CreateNotebook creates a new notebook and returns its summary.
func (c *Client) CreateNotebook(ctx context.Context, title string) (Notebook, error) {
	raw, err := c.Call(ctx, RPCCreateNotebook, "/", BuildCreateNotebookParams(title))
	if err != nil {
		return Notebook{}, err
	}
	// CREATE_NOTEBOOK returns a flat row, not a list envelope.
	if nb, ok, err := parseNotebookRow(raw); err != nil {
		return Notebook{}, err
	} else if ok {
		return nb, nil
	}
	// Some responses wrap the row in a single-element envelope.
	var envelope []json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope) > 0 {
		if nb, ok, err := parseNotebookRow(envelope[0]); err != nil {
			return Notebook{}, err
		} else if ok {
			return nb, nil
		}
	}
	return Notebook{}, fmt.Errorf("create notebook: unexpected response shape")
}

// GetNotebook loads a notebook with its sources.
func (c *Client) GetNotebook(ctx context.Context, notebookID string) (NotebookDetail, error) {
	raw, err := c.Call(ctx, RPCGetNotebook, notebookPath(notebookID), BuildGetNotebookParams(notebookID))
	if err != nil {
		return NotebookDetail{}, err
	}
	return parseNotebookDetail(raw)
}

// RenameNotebook changes a notebook title.
func (c *Client) RenameNotebook(ctx context.Context, notebookID, title string) error {
	_, err := c.Call(ctx, RPCRenameNotebook, "/", BuildRenameNotebookParams(notebookID, title))
	return err
}

// DeleteNotebook removes a notebook.
func (c *Client) DeleteNotebook(ctx context.Context, notebookID string) error {
	_, err := c.Call(ctx, RPCDeleteNotebook, "/", BuildDeleteNotebookParams(notebookID))
	return err
}

func notebookPath(id string) string {
	return "/notebook/" + id
}

func parseNotebookDetail(raw json.RawMessage) (NotebookDetail, error) {
	var outer []json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return NotebookDetail{}, fmt.Errorf("parse notebook detail: %w", err)
	}
	if len(outer) == 0 {
		return NotebookDetail{}, fmt.Errorf("empty notebook response")
	}
	nb, ok, err := parseNotebookRow(outer[0])
	if err != nil {
		return NotebookDetail{}, err
	}
	if !ok {
		return NotebookDetail{}, fmt.Errorf("notebook row missing")
	}
	detail := NotebookDetail{Notebook: nb}
	if len(outer) > 0 {
		var row []any
		if err := json.Unmarshal(outer[0], &row); err == nil && len(row) > 1 {
			detail.Sources = parseSourceRows(row[1])
		}
	}
	return detail, nil
}

func parseSourceRows(v any) []Source {
	rows, ok := v.([]any)
	if !ok {
		return nil
	}
	var out []Source
	for _, r := range rows {
		src, ok := parseSourceRow(r)
		if ok {
			out = append(out, src)
		}
	}
	return out
}

func parseSourceRow(v any) (Source, bool) {
	row, ok := v.([]any)
	if !ok || len(row) < 2 {
		return Source{}, false
	}
	id := extractSourceID(row[0])
	if id == "" {
		return Source{}, false
	}
	title, _ := row[1].(string)
	src := Source{ID: id, Title: title}
	if len(row) > 2 {
		if meta, ok := row[2].([]any); ok && len(meta) > 0 {
			if urls, ok := meta[0].([]any); ok && len(urls) > 0 {
				if u, ok := urls[0].(string); ok {
					src.URL = u
				}
			}
		}
	}
	return src, true
}

func extractSourceID(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) == 0 {
			return ""
		}
		if s, ok := t[0].(string); ok {
			return s
		}
		if len(t) >= 3 {
			if inner, ok := t[2].([]any); ok && len(inner) > 0 {
				if s, ok := inner[0].(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

// ResolveNotebook finds a notebook by UUID or exact/fuzzy title.
func (c *Client) ResolveNotebook(ctx context.Context, ref string) (Notebook, error) {
	if looksLikeUUID(ref) {
		detail, err := c.GetNotebook(ctx, ref)
		if err != nil {
			return Notebook{}, err
		}
		return detail.Notebook, nil
	}
	nbs, err := c.ListNotebooks(ctx)
	if err != nil {
		return Notebook{}, err
	}
	var matches []Notebook
	lower := stringsToLower(ref)
	for _, nb := range nbs {
		if stringsToLower(nb.Title) == lower || stringsContains(stringsToLower(nb.Title), lower) {
			matches = append(matches, nb)
		}
	}
	switch len(matches) {
	case 0:
		return Notebook{}, fmt.Errorf("notebook not found: %q", ref)
	case 1:
		return matches[0], nil
	default:
		return Notebook{}, fmt.Errorf("ambiguous notebook %q: %d matches", ref, len(matches))
	}
}

func stringsToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func stringsContains(hay, needle string) bool {
	return len(needle) > 0 && len(hay) >= len(needle) && indexFold(hay, needle) >= 0
}

func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if stringsToLower(s[i:i+len(sub)]) == sub {
			return i
		}
	}
	return -1
}
