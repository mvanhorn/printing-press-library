// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

// Hand-written MCP tool registrations for novel CLI commands that are not
// covered by the generator-emitted API operation tools.

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/naver-blog/internal/cli"
)

// RegisterNovelTools adds MCP tools for hand-written CLI commands not covered
// by the generated tools.go. Call after RegisterTools(s).
func RegisterNovelTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("posts_comments",
			mcplib.WithDescription("Fetch comment content for a Naver Blog post (cbox endpoint). Required: blog_id, log_no. Optional: all (default false), tree (default false), limit (default 100)."),
			mcplib.WithString("blog_id", mcplib.Required(), mcplib.Description("Blog ID")),
			mcplib.WithString("log_no", mcplib.Required(), mcplib.Description("Post log number")),
			mcplib.WithBoolean("all", mcplib.Description("Fetch every page until exhausted")),
			mcplib.WithBoolean("tree", mcplib.Description("Return nested replies under their parent comments")),
			mcplib.WithNumber("limit", mcplib.Description("Max comments to return")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		wrapCobraCommand("posts-comments", []string{"blog_id", "log_no"}, []string{"all:bool", "tree:bool", "limit:int"}),
	)
	s.AddTool(
		mcplib.NewTool("posts_diff",
			mcplib.WithDescription("Compute engagement delta for a Naver Blog post using the CLI default look-back window (7d). Required: url. No optional params."),
			mcplib.WithString("url", mcplib.Required(), mcplib.Description("Post URL accepted by posts-diff")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		wrapCobraCommand("posts-diff", []string{"url"}, nil),
	)
	s.AddTool(
		mcplib.NewTool("blogs_info",
			mcplib.WithDescription("Get rich profile metadata for a Naver Blog ID. Required: blog_id. No optional params."),
			mcplib.WithString("blog_id", mcplib.Required(), mcplib.Description("Blog ID, blog homepage URL, or post URL")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		wrapCobraCommand("blogs-info", []string{"blog_id"}, nil),
	)
	s.AddTool(
		mcplib.NewTool("blogs_categories",
			mcplib.WithDescription("Sample a blog's recent post list and group observed categories using the CLI default sample size (30). Required: blog_id. No optional params."),
			mcplib.WithString("blog_id", mcplib.Required(), mcplib.Description("Blog ID")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		wrapCobraCommand("categories", []string{"blog_id"}, nil),
	)
	s.AddTool(
		mcplib.NewTool("search_neighbors",
			mcplib.WithDescription("Return top hashtags that co-occur with a target tag in the local cache. Required: tag. Optional: top (default 20)."),
			mcplib.WithString("tag", mcplib.Required(), mcplib.Description("Target hashtag, with or without leading '#'")),
			mcplib.WithNumber("top", mcplib.Description("Max number of neighbor tags to return")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(false),
		),
		wrapCobraCommand("neighbors", []string{"tag"}, []string{"top:int"}),
	)
	s.AddTool(
		mcplib.NewTool("query",
			mcplib.WithDescription("Search the local Korean-aware FTS cache for posts. Required: text. Optional: limit (default 20)."),
			mcplib.WithString("text", mcplib.Required(), mcplib.Description("FTS query text")),
			mcplib.WithNumber("limit", mcplib.Description("Max results to return")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(false),
		),
		wrapCobraCommand("query", []string{"text"}, []string{"limit:int"}),
	)
	s.AddTool(
		mcplib.NewTool("doctor",
			mcplib.WithDescription("Check CLI health and configuration. No required or optional params."),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(false),
		),
		wrapCobraCommand("doctor", nil, nil),
	)
}

func wrapCobraCommand(subcommand string, posArgs []string, flagSpecs []string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		args, err := buildCLIArgs(subcommand, posArgs, flagSpecs, req)
		if err != nil {
			return mcplib.NewToolResultError(err.Error()), nil
		}
		var stdout, stderr bytes.Buffer
		root := cli.RootCmd()
		root.SetIn(strings.NewReader(""))
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs(args)
		if err := root.ExecuteContext(ctx); err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("%s: %v\nstderr: %s", subcommand, err, stderr.String())), nil
		}
		return mcplib.NewToolResultText(stdout.String()), nil
	}
}

func buildCLIArgs(subcommand string, posArgs []string, flagSpecs []string, req mcplib.CallToolRequest) ([]string, error) {
	args := req.GetArguments()
	out := []string{subcommand}
	for _, pa := range posArgs {
		v, ok := argumentString(args, pa)
		if !ok || strings.TrimSpace(v) == "" {
			return nil, fmt.Errorf("missing required arg %q", pa)
		}
		out = append(out, v)
	}
	for _, spec := range flagSpecs {
		name, typ, ok := strings.Cut(spec, ":")
		if !ok {
			return nil, fmt.Errorf("invalid flag spec %q", spec)
		}
		switch typ {
		case "bool":
			if b, ok := argumentBool(args, name); ok && b {
				out = append(out, "--"+name)
			}
		case "int":
			if n, ok := argumentInt(args, name); ok {
				out = append(out, "--"+name, strconv.Itoa(n))
			}
		case "string":
			if v, ok := argumentString(args, name); ok && strings.TrimSpace(v) != "" {
				out = append(out, "--"+name, v)
			}
		default:
			return nil, fmt.Errorf("unsupported flag type %q for %s", typ, name)
		}
	}
	out = append(out, "--json", "--no-input", "--no-color")
	return out, nil
}

func argumentString(args map[string]any, name string) (string, bool) {
	v, ok := args[name]
	if !ok || v == nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case json.Number:
		return x.String(), true
	default:
		return fmt.Sprintf("%v", x), true
	}
}

func argumentBool(args map[string]any, name string) (bool, bool) {
	v, ok := args[name]
	if !ok || v == nil {
		return false, false
	}
	switch x := v.(type) {
	case bool:
		return x, true
	case string:
		b, err := strconv.ParseBool(x)
		return b, err == nil
	default:
		return false, false
	}
}

func argumentInt(args map[string]any, name string) (int, bool) {
	v, ok := args[name]
	if !ok || v == nil {
		return 0, false
	}
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case json.Number:
		n, err := x.Int64()
		return int(n), err == nil
	case string:
		n, err := strconv.Atoi(x)
		return n, err == nil
	default:
		return 0, false
	}
}
