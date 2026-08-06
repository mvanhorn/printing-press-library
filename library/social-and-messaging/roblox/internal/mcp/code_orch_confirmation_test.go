// Copyright 2026 Kieran Maynard and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestCodeOrchExecuteAdvertisesConfirmationAndDestructiveBehavior(t *testing.T) {
	s := server.NewMCPServer("roblox", "test")
	RegisterCodeOrchestrationTools(s)
	registered, ok := s.ListTools()["roblox_execute"]
	if !ok {
		t.Fatal("roblox_execute was not registered")
	}
	tool := registered.Tool
	if _, ok := tool.InputSchema.Properties["confirm"]; !ok {
		t.Fatal("roblox_execute schema does not advertise confirm")
	}
	if tool.Annotations.ReadOnlyHint == nil || *tool.Annotations.ReadOnlyHint {
		t.Fatalf("roblox_execute readOnlyHint = %v, want false", tool.Annotations.ReadOnlyHint)
	}
	if tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
		t.Fatalf("roblox_execute destructiveHint = %v, want true", tool.Annotations.DestructiveHint)
	}
}

func TestCodeOrchMethodMutates(t *testing.T) {
	for _, method := range []string{"POST", "put", " PATCH ", "DELETE"} {
		if !codeOrchMethodMutates(method) {
			t.Errorf("codeOrchMethodMutates(%q) = false, want true", method)
		}
	}
	for _, method := range []string{"GET", "HEAD", "OPTIONS", ""} {
		if codeOrchMethodMutates(method) {
			t.Errorf("codeOrchMethodMutates(%q) = true, want false", method)
		}
	}
}

func TestCodeOrchExecuteRequiresConfirmationBeforeMutation(t *testing.T) {
	req := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{
		"endpoint_id": "avatar.create",
		"params":      map[string]any{},
	}}}
	result, err := handleCodeOrchExecute(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCodeOrchExecute returned transport error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("unconfirmed mutation IsError = %v, want true", result != nil && result.IsError)
	}
	message := mcpTextContent(t, result)
	for _, want := range []string{"POST", "avatar.create", "confirm=true"} {
		if !strings.Contains(message, want) {
			t.Fatalf("confirmation error %q missing %q", message, want)
		}
	}
}
