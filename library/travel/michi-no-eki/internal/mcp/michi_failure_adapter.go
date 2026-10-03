package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/mcp/bound"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/mcp/cobratree"
)

// These source workflows emit factual failure accounting before a nonzero exit.
// Override only their companion handlers, retaining the generated schemas,
// annotations and child-CLI tenant gate without modifying reserved cobratree.
func registerMichiFailureAdapters(s *server.MCPServer, lookup func() (string, error)) {
	path, lookupErr := lookup()
	for _, registered := range s.ListTools() {
		if registered.Tool.Meta == nil {
			continue
		}
		command, _ := registered.Tool.Meta.AdditionalFields["pp:cli-command"].(string)
		if command != "compare" && command != "snapshot" && command != "station-notices" {
			continue
		}
		tool := registered.Tool
		s.AddTool(tool, func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
			if lookupErr != nil {
				return mcpToolError(fmt.Sprintf("companion CLI binary not found: %v", lookupErr)), nil
			}
			argv, err := michiMirrorArgs(command, tool.InputSchema.Properties, req.GetArguments())
			if err != nil {
				return mcpToolError(err.Error()), nil
			}
			out, err := cobratree.RunCLICommand(ctx, path, argv)
			if err == nil {
				return cobratree.ToolResultFromCLICommand(out), nil
			}
			return michiFailedCompanionResult(out, err), nil
		})
	}
}

func michiMirrorArgs(command string, properties map[string]any, args map[string]any) ([]string, error) {
	keys := make([]string, 0, len(args))
	for key := range args {
		if _, allowed := properties[key]; !allowed || strings.Contains(key, "=") {
			return nil, fmt.Errorf("unknown MCP parameter %q; use the tool schema's named parameters", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	argv := []string{command}
	positionals := []string{}
	for _, key := range keys {
		value := args[key]
		if value == nil {
			continue
		}
		if key == "id" || key == "args" {
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("MCP parameter %q requires a string", key)
			}
			parts := []string{text}
			if key == "args" {
				parts = cobratree.SplitShellArgs(text)
			}
			for _, part := range parts {
				if part == "" {
					continue
				}
				// Source positionals are numeric station IDs, never flags or paths.
				if strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
					return nil, fmt.Errorf("MCP positional station ID must contain only digits")
				}
				positionals = append(positionals, part)
			}
			continue
		}
		var text string
		switch v := value.(type) {
		case bool:
			if !v {
				continue
			}
			argv = append(argv, "--"+key)
			continue
		case string:
			if v == "" {
				continue
			}
			text = v
		case float64:
			text = strconv.FormatFloat(v, 'f', -1, 64)
		case []any:
			items := make([]string, len(v))
			for i, item := range v {
				items[i] = fmt.Sprint(item)
			}
			if len(items) == 0 {
				continue
			}
			text = strings.Join(items, ",")
		default:
			return nil, fmt.Errorf("unsupported MCP parameter type for %q", key)
		}
		// Joining values prevents a flag-shaped string from becoming a new flag.
		argv = append(argv, "--"+key+"="+text)
	}
	return append(argv, positionals...), nil
}

func michiFailedCompanionResult(out cobratree.CLICommandResult, commandErr error) *mcplib.CallToolResult {
	// Oversized/truncated captures are not complete JSON evidence. Retain the
	// existing bounded diagnostic fallback instead of implying completeness.
	diagnostic := strings.ToValidUTF8(commandErr.Error(), "�")
	if len(out.Stdout) > bound.MaxBytes || !utf8.ValidString(out.Stdout) || !json.Valid([]byte(out.Stdout)) {
		return mcpToolError(diagnostic)
	}
	result := mcplib.NewToolResultText(out.Stdout)
	result.IsError = true
	// Keep one complete factual JSON block and a separate failure diagnostic;
	// their aggregate text budget still cannot exceed the MCP byte limit.
	remaining := bound.MaxBytes - len(out.Stdout)
	if remaining > 0 {
		if len(diagnostic) > remaining {
			diagnostic = diagnostic[:remaining]
			for !utf8.ValidString(diagnostic) {
				diagnostic = diagnostic[:len(diagnostic)-1]
			}
		}
		result.Content = append(result.Content, mcplib.NewTextContent(diagnostic))
	}
	return result
}
