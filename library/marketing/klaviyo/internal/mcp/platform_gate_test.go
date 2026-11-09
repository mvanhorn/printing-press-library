// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestMCPEveryRegisteredToolHasFreshTenantGate(t *testing.T) {
	s := server.NewMCPServer("tenant-gate-conformance", "test")
	RegisterTools(s)
	registered := s.ListTools()
	if len(registered) == 0 {
		t.Fatal("RegisterTools registered no MCP tools")
	}

	originalVerify := verifyFreshMCPInvocation
	t.Cleanup(func() { verifyFreshMCPInvocation = originalVerify })
	sentinel := errors.New("fresh tenant gate sentinel")
	gateAttempts := 0
	verifyFreshMCPInvocation = func(context.Context) (*platform.Session, error) {
		gateAttempts++
		return nil, sentinel
	}

	names := make([]string, 0, len(registered))
	for name := range registered {
		names = append(names, name)
	}
	sort.Strings(names)

	direct, mirrored := 0, 0
	for _, name := range names {
		entry := registered[name]
		owner := ""
		if entry.Tool.Meta != nil && entry.Tool.Meta.AdditionalFields != nil {
			owner, _ = entry.Tool.Meta.AdditionalFields[mcpTenantGateOwnerKey].(string)
		}
		switch owner {
		case "":
			direct++
			before := gateAttempts
			result, err := requireFreshTenantGate(s, entry.Handler)(context.Background(), mcplib.CallToolRequest{
				Params: mcplib.CallToolParams{Name: name, Arguments: map[string]any{}},
			})
			if err != nil {
				t.Fatalf("%s: wrapper returned protocol error: %v", name, err)
			}
			if gateAttempts != before+1 {
				t.Fatalf("%s: fresh tenant gate attempts = %d, want %d", name, gateAttempts, before+1)
			}
			if result == nil || !result.IsError {
				t.Fatalf("%s: handler continued after the tenant gate failed", name)
			}
		case "child-cli":
			// Cobra mirror tools execute the companion CLI. Its root performs
			// the one live gate, so the parent MCP process must not double-gate.
			mirrored++
		default:
			t.Fatalf("%s: registered without a tenant-gate owner", name)
		}
	}
	if direct == 0 {
		t.Fatal("no in-process MCP tools exercised the fresh tenant-gate wrapper")
	}
	t.Logf("verified %d in-process and %d child-CLI MCP tools", direct, mirrored)
}

func TestMCPTypedInvocationUsesSingleFreshTenantGate(t *testing.T) {
	t.Setenv("PRINTING_PRESS_MCP_BOUND_PROFILE", "fixture-tenant")
	originalVerify := verifyFreshMCPInvocation
	t.Cleanup(func() { verifyFreshMCPInvocation = originalVerify })

	gateAttempts := 0
	session := &platform.Session{
		ProfileName: "fixture-tenant",
		Source:      "klaviyo",
		SourceProfile: platform.SourceProfile{
			ExpectedAccountID:   "123",
			ExpectedOrgName:     "Fixture",
			ExpectedStoreID:     "123",
			ExpectedStoreDomain: "fixture.myshopify.com",
			ExpectedPropertyID:  "123",
			ExpectedBaseURL:     "https://api.example.com",
		},
		Credentials: platform.ResolvedCredentials{
			"credential":      []byte("fixture"),
			"username":        []byte("fixture"),
			"access_token":    []byte("fixture"),
			"api_client_id":   []byte("fixture"),
			"developer_token": []byte("fixture"),
			"app_key":         []byte("fixture"),
			"app_secret":      []byte("fixture"),
			"shop_cipher":     []byte("fixture"),
		},
		CredentialFingerprint: "fixture-fingerprint",
		ObservedIdentity:      map[string]string{"account_id": "123"},
		Paths: platform.Paths{
			CacheDir:   t.TempDir(),
			ConfigFile: t.TempDir() + "/missing.toml",
		},
		GateOutcome: platform.GateVerified,
	}
	verifyFreshMCPInvocation = func(context.Context) (*platform.Session, error) {
		gateAttempts++
		return session, nil
	}

	handlerCalls := 0
	s := server.NewMCPServer("tenant-gate-conformance", "test")
	result, err := requireFreshTenantGate(s, func(ctx context.Context, _ mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		handlerCalls++
		boundClient, rebound, bindErr := newMCPClient(ctx)
		if bindErr != nil {
			return mcplib.NewToolResultError(bindErr.Error()), nil
		}
		if boundClient == nil || rebound != session {
			return mcplib.NewToolResultError("typed client did not reuse the verified session"), nil
		}
		return mcplib.NewToolResultText("ok"), nil
	})(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{Name: "typed-conformance", Arguments: map[string]any{}},
	})
	if err != nil {
		t.Fatalf("typed wrapper returned protocol error: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("typed handler failed after a successful tenant gate: %#v", result)
	}
	content, ok := mcplib.AsTextContent(result.Content[0])
	if !ok {
		t.Fatalf("typed content = %#v", result.Content[0])
	}
	var envelope struct {
		Meta json.RawMessage `json:"meta"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(content.Text), &envelope); err != nil || len(envelope.Meta) == 0 || len(envelope.Data) == 0 {
		t.Fatalf("direct MCP output bypassed the platform envelope: %q (error %v)", content.Text, err)
	}
	if gateAttempts != 1 {
		t.Fatalf("fresh tenant gate attempts = %d, want exactly 1", gateAttempts)
	}
	if handlerCalls != 1 {
		t.Fatalf("typed handler calls = %d, want 1", handlerCalls)
	}
}

func TestMCPContextOutputCarriesPlatformMetadata(t *testing.T) {
	session := &platform.Session{}
	ctx := platform.ContextWithSession(context.Background(), session)
	result, err := handleContext(ctx, mcplib.CallToolRequest{
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
