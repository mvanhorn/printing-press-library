// Agent-family MCP tools. Per D7, MCP exposes read/compute/emit verbs only:
// context, brief, decay, verify, refresh, doctor. Stateful config mutations
// (auth login/switch-org, trust register/rotate/revoke) remain CLI-only.
package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterAgentTools adds the agent-family tools to an MCP server. Called
// alongside RegisterTools by the MCP main entry.
func RegisterAgentTools(s *server.MCPServer) {
	s.AddTool(
		mcplib.NewTool("agent_context",
			mcplib.WithDescription("Assemble a signed agent-context bundle for one Salesforce Account. Returns the bundle JSON with envelope + manifest + signature. Respects FLS and compliance redactions when the local sync has them mapped."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Account Id, name, or domain")),
			mcplib.WithString("since", mcplib.Description("Query window (e.g., P90D, P30D)")),
			mcplib.WithString("org", mcplib.Description("Org alias (default: SF360_ORG env)")),
			mcplib.WithString("run_as_user", mcplib.Description("SF User Id to scope FLS against (required in JWT mode)")),
			mcplib.WithBoolean("dry_run", mcplib.Description("Preview what would be bundled without signing or persisting")),
		),
		agentShellHandler([]string{"agent", "context"}, []agentArg{
			{name: "account", positional: true, required: true},
			{name: "since", flag: "--since"},
			{name: "org", flag: "--org"},
			{name: "run_as_user", flag: "--run-as-user"},
			{name: "dry_run", flag: "--dry-run", boolean: true},
		}),
	)

	s.AddTool(
		mcplib.NewTool("agent_brief",
			mcplib.WithDescription("Render a markdown + JSON brief for one Opportunity, field-gated so redactions don't leak through narrative."),
			mcplib.WithString("opp", mcplib.Required(), mcplib.Description("Opportunity Id")),
			mcplib.WithString("format", mcplib.Description("markdown | json (default: markdown)")),
		),
		agentShellHandler([]string{"agent", "brief"}, []agentArg{
			{name: "opp", flag: "--opp", required: true},
			{name: "format", flag: "--format"},
		}),
	)

	s.AddTool(
		mcplib.NewTool("agent_decay",
			mcplib.WithDescription("Score how stale the CRM data is for an account, 0-100 (higher = fresher). Returns structured signals."),
			mcplib.WithString("account", mcplib.Required(), mcplib.Description("Account Id")),
		),
		agentShellHandler([]string{"agent", "decay"}, []agentArg{
			{name: "account", flag: "--account", required: true},
		}),
	)

	s.AddTool(
		mcplib.NewTool("agent_verify",
			mcplib.WithDescription("Verify a previously-generated bundle signed with an org-registered key. Returns structured pass/fail with warnings."),
			mcplib.WithString("path", mcplib.Required(), mcplib.Description("Path to bundle JSON file")),
			mcplib.WithBoolean("live", mcplib.Description("Re-fetch the org's key collection and check retirement")),
			mcplib.WithBoolean("deep", mcplib.Description("Re-fetch ContentVersion bytes and rehash")),
			mcplib.WithBoolean("strict", mcplib.Description("Combine live + deep and fail on expired exp")),
		),
		agentShellHandler([]string{"agent", "verify"}, []agentArg{
			{name: "path", positional: true, required: true},
			{name: "live", flag: "--live", boolean: true},
			{name: "deep", flag: "--deep", boolean: true},
			{name: "strict", flag: "--strict", boolean: true},
		}),
	)

	s.AddTool(
		mcplib.NewTool("agent_refresh",
			mcplib.WithDescription("Force a fresh sync of the CRM data before the next bundle assembly. Useful when an agent knows the cache is stale."),
			mcplib.WithString("account", mcplib.Description("Account Id (default: all)")),
		),
		agentShellHandler([]string{"agent", "refresh"}, []agentArg{
			{name: "account", flag: "--account"},
		}),
	)

	s.AddTool(
		mcplib.NewTool("agent_doctor",
			mcplib.WithDescription("Machine-readable health status of the CLI's sources (auth, local store, Apex companion, Data Cloud, Slack linkage, competing tools detected). Use this before other tool calls to introspect failures."),
		),
		agentShellHandler([]string{"doctor"}, []agentArg{}),
	)
}

// agentArg describes one argument of an MCP-to-shell mapping.
type agentArg struct {
	name       string
	flag       string // when empty and !positional, arg is dropped
	positional bool
	boolean    bool
	required   bool
}

// agentShellHandler returns an MCP handler that shells out to the installed
// CLI with --json appended. This shares the same code path as the CLI so MCP
// callers get identical FLS, signing, and error-envelope behavior without
// duplicating logic.
func agentShellHandler(subcmd []string, args []agentArg) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		// Look up the CLI binary: prefer PATH, fall back to a sibling of the
		// MCP binary.
		cliName := "salesforce-headless-360-pp-cli"
		binPath, _ := exec.LookPath(cliName)
		if binPath == "" {
			if exe, err := os.Executable(); err == nil {
				binPath = exe + "/../salesforce-headless-360-pp-cli"
			}
		}
		if binPath == "" {
			return mcplib.NewToolResultError(cliName + " binary not found on PATH"), nil
		}

		cmdArgs := append([]string{}, subcmd...)
		raw := req.Params.Arguments
		for _, a := range args {
			val, present := raw[a.name]
			if !present {
				if a.required {
					return mcplib.NewToolResultError("required argument missing: " + a.name), nil
				}
				continue
			}
			if a.boolean {
				if b, _ := val.(bool); b {
					cmdArgs = append(cmdArgs, a.flag)
				}
				continue
			}
			strVal, _ := val.(string)
			if strVal == "" {
				if a.required {
					return mcplib.NewToolResultError("required argument empty: " + a.name), nil
				}
				continue
			}
			if a.positional {
				cmdArgs = append(cmdArgs, strVal)
			} else if a.flag != "" {
				cmdArgs = append(cmdArgs, a.flag, strVal)
			}
		}
		cmdArgs = append(cmdArgs, "--json")

		cmd := exec.CommandContext(ctx, binPath, cmdArgs...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Wrap the stderr in the structured D9-shaped envelope so MCP
			// callers see the CLI's own error shape.
			payload := map[string]any{
				"code":     "SF360.MCP.SUBPROCESS_FAILED",
				"stage":    "mcp.agent_tool",
				"hint":     "Run the command manually: " + binPath + " " + joinArgs(cmdArgs),
				"cli_exit": err.Error(),
				"stderr":   string(out),
			}
			body, _ := json.Marshal(payload)
			return mcplib.NewToolResultError(string(body)), nil
		}
		return mcplib.NewToolResultText(string(out)), nil
	}
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}
