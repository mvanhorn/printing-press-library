// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/store"
)

func TestTripSQLRejectsActiveWALThenReadsCheckpointedFacts(t *testing.T) {
	resetMCPPathEnv(t)
	path, err := mcpDBPath()
	if err != nil {
		t.Fatal(err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Upsert("spots", "8220", json.RawMessage(`{"name":"Synthetic checkpoint baseline"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	writer, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if err := writer.Upsert("spots", "8220", json.RawMessage(`{"name":"Synthetic committed WAL facts"}`)); err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	RegisterTripSurface(s)
	tool := s.GetTool("sql")
	request := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"query": "SELECT json_extract(data, '$.name') AS name FROM resources WHERE id='8220'"}}}
	result, err := tool.Handler(context.Background(), request)
	if err != nil || result == nil || !result.IsError || !strings.Contains(mcpTextContent(t, result), "cache") {
		t.Fatalf("SQL emitted stale active-writer facts: %+v %v", result, err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	result, err = tool.Handler(context.Background(), request)
	if err != nil || result == nil || result.IsError {
		t.Fatalf("checkpointed SQL failed: %+v %v", result, err)
	}
	var envelope struct {
		Count int              `json:"count"`
		Rows  []map[string]any `json:"rows"`
	}
	if err := json.Unmarshal([]byte(mcpTextContent(t, result)), &envelope); err != nil || envelope.Count != 1 || envelope.Rows[0]["name"] != "Synthetic committed WAL facts" {
		t.Fatalf("SQL did not read current checkpoint: %+v %v", envelope, err)
	}
}

func TestTripSQLKeepsGeneratedReadOnlyGate(t *testing.T) {
	for _, query := range []string{"SELECT 1; VACUUM INTO 'outside.db'", "/* comment */ ATTACH DATABASE 'outside.db' AS x", "PRAGMA journal_mode=WAL"} {
		request := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"query": query}}}
		result, err := handleTripSQL(context.Background(), request)
		if err != nil || result == nil || !result.IsError {
			t.Fatalf("source SQL lost read-only gate: %s %+v %v", query, result, err)
		}
	}
}
