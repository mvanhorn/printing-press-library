// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package main

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServerRecoversToolPanicAndContinues(t *testing.T) {
	s := newServer()
	s.AddTool(mcp.NewTool("panic-test"), func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) { panic("boom") })
	s.AddTool(mcp.NewTool("after-test"), func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ok"), nil
	})
	panicResponse := s.HandleMessage(t.Context(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"panic-test"}}`))
	if _, ok := panicResponse.(mcp.JSONRPCError); !ok {
		t.Fatalf("panic response=%T %#v", panicResponse, panicResponse)
	}
	afterResponse := s.HandleMessage(t.Context(), []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"after-test"}}`))
	if _, ok := afterResponse.(mcp.JSONRPCResponse); !ok {
		t.Fatalf("second response=%T %#v", afterResponse, afterResponse)
	}
}
