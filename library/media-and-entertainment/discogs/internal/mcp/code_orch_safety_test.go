// Copyright 2026 ganes-j and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestCodeOrchExecuteAdvertisesDestructiveBehavior(t *testing.T) {
	s := server.NewMCPServer("discogs", "test")
	RegisterCodeOrchestrationTools(s)

	execute := s.GetTool("discogs_execute")
	if execute == nil {
		t.Fatal("discogs_execute was not registered")
	}
	if execute.Tool.Annotations.ReadOnlyHint == nil || *execute.Tool.Annotations.ReadOnlyHint {
		t.Fatal("discogs_execute must advertise readOnlyHint=false")
	}
	if execute.Tool.Annotations.DestructiveHint == nil || !*execute.Tool.Annotations.DestructiveHint {
		t.Fatal("discogs_execute must advertise destructiveHint=true")
	}
}

func TestCodeOrchExecuteRequiresConfirmationForMutations(t *testing.T) {
	result, err := handleCodeOrchExecute(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
		Arguments: map[string]any{
			"endpoint_id": "collection.delete_folder",
			"params": map[string]any{
				"username":  "example",
				"folder_id": 42,
			},
		},
	}})
	if err != nil {
		t.Fatalf("handleCodeOrchExecute returned transport error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("unconfirmed mutation IsError = %v, want true", result != nil && result.IsError)
	}
	if text := mcpTextContent(t, result); !strings.Contains(text, "confirm=true") || !strings.Contains(text, "DELETE") {
		t.Fatalf("unconfirmed mutation error is not actionable: %q", text)
	}
}

func TestCodeOrchConfirmationClassification(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		if codeOrchRequiresConfirmation(method) {
			t.Errorf("%s unexpectedly requires confirmation", method)
		}
	}
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		if !codeOrchRequiresConfirmation(method) {
			t.Errorf("%s unexpectedly bypasses confirmation", method)
		}
	}
}
