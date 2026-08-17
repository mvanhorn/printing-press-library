// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package nlm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ListArtifacts returns studio artifacts for a notebook.
func (c *Client) ListArtifacts(ctx context.Context, notebookID string) ([]Artifact, error) {
	raw, err := c.Call(ctx, RPCListArtifacts, notebookPath(notebookID), BuildListArtifactsParams(notebookID))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	return parseArtifactList(raw)
}

// GenerateQuiz starts quiz generation for a notebook.
func (c *Client) GenerateQuiz(ctx context.Context, notebookID string, sourceIDs []string, instructions string) (Artifact, error) {
	if len(sourceIDs) == 0 {
		var err error
		sourceIDs, err = c.sourceIDs(ctx, notebookID)
		if err != nil {
			return Artifact{}, err
		}
	}
	raw, err := c.Call(ctx, RPCCreateArtifact, notebookPath(notebookID), BuildQuizArtifactParams(notebookID, sourceIDs, instructions))
	if err != nil {
		return Artifact{}, err
	}
	art, ok := parseArtifactFromValue(raw)
	if ok {
		art.Type = "quiz"
		return art, nil
	}
	return Artifact{Type: "quiz", Status: "pending"}, nil
}

func artifactReady(status string) bool {
	if status == "" {
		return true
	}
	s := strings.ToLower(status)
	return strings.EqualFold(status, "ready") || strings.Contains(s, "complete")
}

func artifactFailed(status string) bool {
	s := strings.ToLower(status)
	return strings.Contains(s, "fail") || strings.Contains(s, "error")
}

// WaitForArtifact polls until an artifact appears or timeout.
func (c *Client) WaitForArtifact(ctx context.Context, notebookID, artifactID string, timeout time.Duration) (Artifact, error) {
	deadline := time.Now().Add(timeout)
	for {
		arts, err := c.ListArtifacts(ctx, notebookID)
		if err != nil {
			return Artifact{}, err
		}
		for _, a := range arts {
			if artifactID != "" && a.ID != artifactID {
				continue
			}
			if artifactFailed(a.Status) {
				return Artifact{}, fmt.Errorf("artifact %s failed: status %q", a.ID, a.Status)
			}
			if artifactReady(a.Status) {
				return a, nil
			}
		}
		if time.Now().After(deadline) {
			return Artifact{}, fmt.Errorf("artifact wait timed out after %s", timeout)
		}
		select {
		case <-ctx.Done():
			return Artifact{}, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func parseArtifactList(raw json.RawMessage) ([]Artifact, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	rows := unwrapArtifactEnvelope(v)
	var out []Artifact
	for _, row := range rows {
		if art, ok := parseArtifactRow(row); ok {
			out = append(out, art)
		}
	}
	return out, nil
}

func unwrapArtifactEnvelope(v any) []any {
	switch t := v.(type) {
	case []any:
		if len(t) == 1 {
			if inner, ok := t[0].([]any); ok && len(inner) > 0 {
				if _, ok := inner[0].([]any); ok {
					return inner
				}
			}
		}
		return t
	}
	return nil
}

func parseArtifactFromValue(raw json.RawMessage) (Artifact, bool) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return Artifact{}, false
	}
	if art, ok := parseArtifactRow(v); ok {
		return art, true
	}
	if row, ok := v.([]any); ok {
		for _, item := range row {
			if art, ok := parseArtifactRow(item); ok {
				return art, true
			}
		}
	}
	return Artifact{}, false
}

func parseArtifactRow(v any) (Artifact, bool) {
	row, ok := v.([]any)
	if !ok || len(row) < 2 {
		return Artifact{}, false
	}
	id := ""
	title := ""
	status := ""
	if s, ok := row[0].(string); ok && looksLikeUUID(s) {
		id = s
	}
	if s, ok := row[1].(string); ok {
		if id == "" && looksLikeUUID(s) {
			id = s
		} else if title == "" {
			title = s
		}
	}
	if id == "" {
		id = extractUUIDFromNested(row)
	}
	if id == "" {
		return Artifact{}, false
	}
	for _, item := range row {
		if s, ok := item.(string); ok && s != id && s != title && len(s) > 2 && len(s) < 200 {
			if title == "" {
				title = s
			}
		}
		if s, ok := item.(string); ok && (strings.Contains(strings.ToLower(s), "status") || strings.Contains(strings.ToLower(s), "ready") || strings.Contains(strings.ToLower(s), "pending")) {
			status = s
		}
	}
	return Artifact{ID: id, Title: title, Status: status}, true
}
