// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/mcp/bound"
	"github.com/mvanhorn/printing-press-library/library/productivity/concur/internal/mcp/cobratree"
)

// PATCH(Greptile review, "MCP misses the new commands") -- the itemization
// reads and the --type edit were reachable only from the CLI. The Cobra-tree
// mirror skips pp:endpoint commands on the assumption that a typed tool covers
// them, and an in-process typed tool needs a registered platform source for
// its fresh-tenant gate, which this CLI does not have. These tools instead run
// the companion CLI and, like the mirror's tools, let it own the tenant gate.
// That also routes the type edit through the CLI flow: the API rejects expense
// writes for browser-session auth, so the change has to be made by the CLI's
// browser automation.

// cliRoutedCLIPath resolves the companion CLI on each call; tests replace it.
var cliRoutedCLIPath = cobratree.SiblingCLIPath

// cliRoutedParam is one string parameter of a cliRoutedTool.
type cliRoutedParam struct {
	name     string // MCP parameter name
	flag     string // CLI flag name; empty marks the command's one positional argument
	required bool
	desc     string
}

// cliRoutedTool is an MCP tool that runs `<command> ... --agent` on the
// companion CLI.
type cliRoutedTool struct {
	name     string
	desc     string
	command  []string
	params   []cliRoutedParam
	readOnly bool
}

var cliRoutedTools = []cliRoutedTool{
	{
		name:    "expenses_itemizations",
		desc:    "List the itemization lines of an expense. Itemization lines are child expenses (parentExpenseId set); expenses_get and expenses_list return only a link to them. Pass a line's expenseId as expense-id to expenses_update_type. Required: user_id, report-id, expense-id. Optional: context-type (default: TRAVELER). Returns array of Expense.",
		command: []string{"expenses", "itemizations"},
		params: []cliRoutedParam{
			{name: "user_id", flag: "user-id", required: true, desc: "Concur user ID (from 'account whoami')"},
			{name: "context-type", flag: "context-type", desc: "Access level context for the request: TRAVELER (default), MANAGER, PROCESSOR or PROXY"},
			{name: "report-id", flag: "report-id", required: true, desc: "Parent report ID"},
			{name: "expense-id", required: true, desc: "ID of the expense whose itemization lines to list"},
		},
		readOnly: true,
	},
	{
		name:    "reports_exceptions",
		desc:    "List a report's validation exceptions (what blocks submit). This is the authoritative view of blocking errors: the per-expense hasExceptions/hasBlockingExceptions flags can read false while exceptions exist. Required: user_id, report-id. Optional: context-type (default: TRAVELER). Returns array of Exception.",
		command: []string{"reports", "exceptions"},
		params: []cliRoutedParam{
			{name: "user_id", flag: "user-id", required: true, desc: "Concur user ID (from 'account whoami')"},
			{name: "context-type", flag: "context-type", desc: "Access level context for the request: TRAVELER (default), MANAGER, PROCESSOR or PROXY"},
			{name: "report-id", required: true, desc: "Report ID"},
		},
		readOnly: true,
	},
	{
		name:    "expenses_update_type",
		desc:    "Change the expense type of an itemization line, for example from the placeholder \"Undefined\" to \"Hotel\". The API rejects expense writes for browser-session auth, so this drives the Concur web form through the companion CLI: it needs agent-browser and a signed-in Concur Chrome, and can take a minute. Only itemization lines are supported; find them with expenses_itemizations. If the type is not offered for the line under the user's policy, nothing is saved and the offered types are listed. Required: user_id, report-id, expense-id, type. Optional: context-type (default: TRAVELER). Returns the updated Expense.",
		command: []string{"expenses", "update"},
		params: []cliRoutedParam{
			{name: "user_id", flag: "user-id", required: true, desc: "Concur user ID (from 'account whoami')"},
			{name: "context-type", flag: "context-type", desc: "Access level context for the request: TRAVELER (default), MANAGER, PROCESSOR or PROXY"},
			{name: "report-id", flag: "report-id", required: true, desc: "Parent report ID"},
			{name: "expense-id", required: true, desc: "ID of the itemization line to change"},
			{name: "type", flag: "type", required: true, desc: "Expense type name as shown in Concur's picker, for example \"Hotel\""},
		},
	},
}

// registerCLIRoutedTools replaces any same-named tool already registered.
func registerCLIRoutedTools(s *server.MCPServer) {
	for _, t := range cliRoutedTools {
		s.AddTool(t.mcpTool(), t.handler())
	}
}

func (t cliRoutedTool) mcpTool() mcplib.Tool {
	opts := []mcplib.ToolOption{mcplib.WithDescription(t.desc)}
	for _, p := range t.params {
		propOpts := []mcplib.PropertyOption{mcplib.Description(p.desc)}
		if p.required {
			propOpts = append(propOpts, mcplib.Required())
		}
		opts = append(opts, mcplib.WithString(p.name, propOpts...))
	}
	if t.readOnly {
		opts = append(opts, mcplib.WithReadOnlyHintAnnotation(true), mcplib.WithDestructiveHintAnnotation(false))
	}
	opts = append(opts, mcplib.WithOpenWorldHintAnnotation(true))

	tool := mcplib.NewTool(t.name, opts...)
	if tool.Meta == nil {
		tool.Meta = &mcplib.Meta{}
	}
	if tool.Meta.AdditionalFields == nil {
		tool.Meta.AdditionalFields = map[string]any{}
	}
	tool.Meta.AdditionalFields[mcpTenantGateOwnerKey] = "child-cli"
	return tool
}

func (t cliRoutedTool) handler() server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		argv, err := t.argv(req.GetArguments())
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		cliPath, err := cliRoutedCLIPath()
		if err != nil {
			return mcpToolError(fmt.Sprintf("companion CLI binary not found: %v\nTried sibling lookup, CONCUR_CLI_PATH env var, and PATH.", err)), nil
		}
		out, err := cobratree.RunCLICommand(ctx, cliPath, argv)
		if err != nil {
			return mcpToolError(err.Error()), nil
		}
		return mcplib.NewToolResultText(bound.Text(out)), nil
	}
}

// argv turns MCP arguments into the CLI invocation. Flag values use the
// --flag=value form and the positional comes after "--", so no argument value
// can be parsed as a flag of its own.
func (t cliRoutedTool) argv(in map[string]any) ([]string, error) {
	known := make(map[string]bool, len(t.params))
	for _, p := range t.params {
		known[p.name] = true
	}
	var unknown []string
	for name := range in {
		if !known[name] {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown parameter(s) %s; use the tool schema's named parameters", strings.Join(unknown, ", "))
	}

	argv := append([]string{}, t.command...)
	var positional string
	var missing []string
	for _, p := range t.params {
		value, err := stringArg(in, p.name)
		if err != nil {
			return nil, err
		}
		switch {
		case value == "":
			if p.required {
				missing = append(missing, p.name)
			}
		case p.flag == "":
			positional = value
		default:
			argv = append(argv, "--"+p.flag+"="+value)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required parameter(s): %s", strings.Join(missing, ", "))
	}

	argv = append(argv, "--agent")
	if positional != "" {
		argv = append(argv, "--", positional)
	}
	return argv, nil
}

// stringArg reads an optional string argument; absent and blank both read as "".
func stringArg(in map[string]any, name string) (string, error) {
	raw, ok := in[name]
	if !ok || raw == nil {
		return "", nil
	}
	s, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("parameter %q must be a string", name)
	}
	return strings.TrimSpace(s), nil
}
