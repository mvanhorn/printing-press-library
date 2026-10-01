package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/mcp/bound"
	"github.com/mvanhorn/printing-press-library/library/travel/jma/internal/mcp/cobratree"
	"os/exec"
	"sort"
	"strconv"
)

// The generic mirror reports stderr for nonzero exits. Municipality warning
// completeness is itself useful evidence, so retain its bounded JSON envelope
// on an exit-5 semantic failure while signaling MCP isError.
func preserveIncompleteWarnings(s *server.MCPServer) {
	registered := s.ListTools()["warnings_get"]
	if registered == nil {
		return
	}
	tool := registered.Tool
	s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		args, e := focusedCLIArgs(req.GetArguments(), tool.InputSchema.Properties)
		if e != nil {
			return mcplib.NewToolResultError(e.Error()), nil
		}
		path, e := cobratree.SiblingCLIPath()
		if e != nil {
			return mcplib.NewToolResultError(e.Error()), nil
		}
		out, runErr := cobratree.RunCLICommand(ctx, path, append([]string{"warnings", "get", "--agent"}, args...))
		if runErr == nil {
			return cobratree.ToolResultFromCLICommand(out), nil
		}
		var v struct {
			Meta    map[string]any `json:"meta"`
			Results struct {
				State string `json:"state"`
			} `json:"results"`
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) && exitErr.ExitCode() == 5 && json.Unmarshal([]byte(out.Stdout), &v) == nil && v.Meta != nil {
			result := mcplib.NewToolResultText(bound.Text(out.Stdout))
			result.IsError = true
			result.Content = append(result.Content, mcplib.TextContent{Type: "text", Text: "diagnostic: " + bound.Text(runErr.Error())})
			return result, nil
		}
		return mcplib.NewToolResultError(bound.Text(runErr.Error())), nil
	})
}
func focusedCLIArgs(args map[string]any, properties map[string]any) ([]string, error) {
	keys := []string{}
	for key := range args {
		if _, ok := properties[key]; !ok {
			return nil, fmt.Errorf("unknown MCP argument %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []string{}
	for _, key := range keys {
		value := args[key]
		switch v := value.(type) {
		case string:
			out = append(out, "--"+key+"="+v)
		case bool:
			out = append(out, "--"+key+"="+strconv.FormatBool(v))
		case float64:
			out = append(out, "--"+key+"="+strconv.FormatFloat(v, 'f', -1, 64))
		default:
			return nil, fmt.Errorf("MCP argument %s must be a scalar flag value", key)
		}
	}
	return out, nil
}

// Limit simultaneous companion processes. Reject excess calls immediately so
// neither a queue nor HTTP fan-out can grow with MCP host concurrency.
func limitFocusedToolConcurrency(s *server.MCPServer) {
	slots := make(chan struct{}, 2)
	s.Use(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
				return next(ctx, req)
			default:
				return mcplib.NewToolResultError("JMA tool concurrency limit (2) reached; retry later"), nil
			}
		}
	})
}
