package mcp

import (
	"context"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestTypedMarkersRejectInvalidWindowBeforeClient(t *testing.T) {
	s := server.NewMCPServer("repark", "test")
	RegisterTools(s)
	handler := s.ListTools()["site_markers"].Handler
	for _, window := range []any{nil, 34.0, "bad", "C34,135N40W120S20E150", "C19,135N19.001W134.999S18.999E135.001"} {
		req := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"range": window}}}
		result, err := handler(context.Background(), req)
		if err != nil || result == nil || !result.IsError {
			t.Fatalf("range %#v: result = %#v, error = %v", window, result, err)
		}
		text := mcpTextContent(t, result)
		if !strings.Contains(text, "range") && !strings.Contains(text, "coordinates") {
			t.Fatalf("range %#v reached client instead of window validation: %s", window, text)
		}
	}
}

func TestTypedMarkersValidWindowReachesHandler(t *testing.T) {
	called := false
	handler := boundedReparkMarkers(func(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		called = true
		return mcplib.NewToolResultText(req.GetArguments()["range"].(string)), nil
	})
	window := "C34.663534,135.516310N34.664W135.515S34.663E135.517"
	result, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"range": window}}})
	if err != nil || result == nil || result.IsError || !called || mcpTextContent(t, result) != window {
		t.Fatalf("valid window was not passed unchanged: result = %#v, error = %v", result, err)
	}
}
