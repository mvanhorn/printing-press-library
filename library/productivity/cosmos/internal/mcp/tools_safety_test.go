// Copyright 2026 Elliott Jacobs and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

func TestRegisterToolsDoesNotExposeRawGraphQLDocuments(t *testing.T) {
	s := server.NewMCPServer("cosmos-safety", "test")
	RegisterTools(s)

	tools := s.ListTools()
	if len(tools) == 0 {
		t.Fatal("RegisterTools registered no tools")
	}
	for name, entry := range tools {
		props := entry.Tool.InputSchema.Properties
		if _, ok := props["operationName"]; ok {
			t.Errorf("tool %q exposes the raw GraphQL transport", name)
		}
		for _, controlFlag := range []string{"audit-dir", "client-profile", "db", "notes-file", "playbook-file", "playbook-notes-file", "rate-limit", "receipt", "receipt-file"} {
			if _, ok := props[controlFlag]; ok {
				t.Errorf("tool %q exposes MCP-blocked root control %q", name, controlFlag)
			}
		}
	}
	if _, ok := tools["identity"]; !ok {
		t.Fatal("agent surface does not expose the authenticated Cosmos identity check")
	}
	if _, ok := tools["import_elements"]; ok {
		t.Fatal("agent surface exposes the generic GraphQL importer")
	}
	if _, ok := tools["import_status"]; !ok {
		t.Fatal("agent surface does not expose the safe Cosmos import status command")
	}
	for _, name := range []string{"client_add", "client_source_set", "client_set_default", "client_delete", "client_migrate", "client_cache_clear"} {
		if _, ok := tools[name]; ok {
			t.Fatalf("agent surface exposes operator-only profile mutation %q", name)
		}
	}
	for _, name := range []string{"export_collection", "export_gallery"} {
		if _, ok := tools[name]; ok {
			t.Fatalf("agent surface exposes caller-selected host filesystem write %q", name)
		}
	}
	syncTool, ok := tools["sync"]
	if !ok {
		t.Fatal("agent surface does not expose sync")
	}
	annotations := syncTool.Tool.Annotations
	if annotations.DestructiveHint == nil || *annotations.DestructiveHint {
		t.Fatal("sync must be marked non-destructive")
	}
	if annotations.OpenWorldHint == nil || !*annotations.OpenWorldHint {
		t.Fatal("sync must disclose that it reads from the live Cosmos service")
	}
}
