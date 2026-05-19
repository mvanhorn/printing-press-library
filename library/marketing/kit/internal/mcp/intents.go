// Copyright 2026 cathrynlavery. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored under printing-press patch kit-first-class-mcp-intents.
// This file is preserved across regen-merge via .printing-press-patches.json.
//
// Intent handlers are higher-level MCP tools that compose multiple endpoint
// calls. Rather than forcing an agent to stitch primitives, a single intent
// tool accepts top-level input, dispatches the matching Cobra workflow
// command in-process, and returns its JSON output. The function name and
// shape mirror the canonical generator template
// (internal/generator/templates/mcp_intents.go.tmpl) so a future
// spec-declared intent regen merges cleanly.

package mcp

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/marketing/kit/internal/cli"
)

// RegisterIntents registers Kit-specific compound workflows as first-class
// MCP intent tools. Each tool delegates in-process to its matching Cobra
// workflow command in internal/cli/channel_workflow.go so orchestration
// logic stays in one place. Names use the intent_ prefix so the cobratree
// runtime mirror's auto-registered workflow_* shell-out tools can coexist
// without name collisions.
func RegisterIntents(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("intent_workflow_creator_snapshot",
			mcplib.WithDescription("Compound read-only operating snapshot for a Kit account: profile, growth stats, subscriber and tag counts, sample sequences/forms/custom fields/webhooks, and broadcast stats in a single response. Use before strategy reviews instead of fanning out across 9+ endpoint tools. Delegates to `kit-pp-cli workflow creator-snapshot`."),
			mcplib.WithString("starting", mcplib.Description("Growth-stat start date in yyyy-mm-dd format. Defaults to 90 days ago.")),
			mcplib.WithString("ending", mcplib.Description("Growth-stat end date in yyyy-mm-dd format. Defaults to today.")),
			mcplib.WithNumber("sample_size", mcplib.Description("Sample rows to include per collection (1-25, default 5).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		runWorkflowIntent("creator-snapshot", []intentFlag{
			{name: "starting", flag: "--starting"},
			{name: "ending", flag: "--ending"},
			{name: "sample_size", flag: "--sample-size"},
		}),
	)

	s.AddTool(
		mcplib.NewTool("intent_workflow_audience_health",
			mcplib.WithDescription("Compound read-only audience report: subscriber status counts (active/inactive/bounced/complained/cancelled), recent growth stats, and largest tags by subscriber count. Use before list cleaning, segmentation, or campaign planning. Delegates to `kit-pp-cli workflow audience-health`."),
			mcplib.WithString("starting", mcplib.Description("Growth-stat start date in yyyy-mm-dd format.")),
			mcplib.WithString("ending", mcplib.Description("Growth-stat end date in yyyy-mm-dd format.")),
			mcplib.WithNumber("top_limit", mcplib.Description("Tags to inspect and rank by subscriber count (1-25, default 8).")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		runWorkflowIntent("audience-health", []intentFlag{
			{name: "starting", flag: "--starting"},
			{name: "ending", flag: "--ending"},
			{name: "top_limit", flag: "--top-limit"},
		}),
	)

	s.AddTool(
		mcplib.NewTool("intent_workflow_content_inventory",
			mcplib.WithDescription("Compound read-only inventory of sequences, sequence emails, snippets, forms, email templates, and recent broadcast stats. Use for content audits and planning. Delegates to `kit-pp-cli workflow content-inventory`."),
			mcplib.WithNumber("item_limit", mcplib.Description("Rows per content collection (1-50, default 10).")),
			mcplib.WithNumber("sequence_limit", mcplib.Description("Sequences whose emails should be inspected (1-25, default 5).")),
			mcplib.WithBoolean("include_content", mcplib.Description("Include sequence email body content when Kit returns it. Default false.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		runWorkflowIntent("content-inventory", []intentFlag{
			{name: "item_limit", flag: "--item-limit"},
			{name: "sequence_limit", flag: "--sequence-limit"},
			{name: "include_content", flag: "--include-content", boolFlag: true},
		}),
	)

	s.AddTool(
		mcplib.NewTool("intent_workflow_subscriber_lookup",
			mcplib.WithDescription("Compound read-only subscriber dossier by email or Kit subscriber id: profile, custom fields, tags, attribution, and email engagement stats. Provide one of --email or --id. Use before support replies, segmentation checks, or personalization debugging. Delegates to `kit-pp-cli workflow subscriber-lookup`."),
			mcplib.WithString("email", mcplib.Description("Subscriber email address to look up. One of email or id is required.")),
			mcplib.WithString("id", mcplib.Description("Kit subscriber id to look up. One of email or id is required.")),
			mcplib.WithString("email_sent_after", mcplib.Description("Only include subscriber email stats after this yyyy-mm-dd date.")),
			mcplib.WithString("email_sent_before", mcplib.Description("Only include subscriber email stats before this yyyy-mm-dd date.")),
			mcplib.WithReadOnlyHintAnnotation(true),
			mcplib.WithDestructiveHintAnnotation(false),
			mcplib.WithOpenWorldHintAnnotation(true),
		),
		runWorkflowIntent("subscriber-lookup", []intentFlag{
			{name: "email", flag: "--email"},
			{name: "id", flag: "--id"},
			{name: "email_sent_after", flag: "--email-sent-after"},
			{name: "email_sent_before", flag: "--email-sent-before"},
		}),
	)
}

type intentFlag struct {
	name     string
	flag     string
	boolFlag bool
}

// runWorkflowIntent builds an MCP tool handler that translates MCP input
// arguments into Cobra flag arguments and invokes the named `workflow`
// subcommand through a fresh cli.RootCmd() tree. Stdout and stderr are
// captured into the MCP response so the intent surface returns the same
// JSON payload the CLI prints with --agent.
func runWorkflowIntent(subcommand string, flagMap []intentFlag) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		args := req.GetArguments()
		cmdArgs := []string{"workflow", subcommand, "--agent"}
		for _, f := range flagMap {
			v, ok := args[f.name]
			if !ok {
				continue
			}
			if f.boolFlag {
				// Cobra's --bool-flag form accepts --flag=true or --flag.
				// Use the explicit value form so false propagates correctly.
				cmdArgs = append(cmdArgs, fmt.Sprintf("%s=%v", f.flag, v))
				continue
			}
			cmdArgs = append(cmdArgs, f.flag, fmt.Sprintf("%v", v))
		}

		root := cli.RootCmd()
		var out, errBuf bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&errBuf)
		root.SetArgs(cmdArgs)
		if err := root.ExecuteContext(ctx); err != nil {
			msg := strings.TrimSpace(errBuf.String())
			if msg == "" {
				msg = err.Error()
			}
			return mcplib.NewToolResultError(msg), nil
		}
		return mcplib.NewToolResultText(out.String()), nil
	}
}
