// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
	"testing"
)

func TestMCPContextOutputCarriesPlatformMetadata(t *testing.T) {
	session := &platform.Session{}
	ctx := platform.ContextWithSession(context.Background(), session)
	result, err := handleContext(server.NewMCPServer("context", "test"))(ctx, mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{Name: "context", Arguments: map[string]any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("context result = %#v", result)
	}
	content, ok := mcplib.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("context content = %#v", result.Content[0])
	}
	var envelope struct {
		Meta json.RawMessage `json:"meta"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(content.Text), &envelope); err != nil {
		t.Fatalf("context output is not JSON: %v", err)
	}
	if len(envelope.Meta) == 0 || len(envelope.Data) == 0 {
		t.Fatalf("context output bypassed the platform envelope: %s", content.Text)
	}
}
