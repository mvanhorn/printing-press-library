// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// agentContextSchemaVersion is bumped on any breaking change to the JSON
// shape emitted by `agent-context`. Agents should check this before
// parsing. Shape at v3 adds kind-aware auth env var metadata.
const agentContextSchemaVersion = "4"

// agentContext is the structured description of this CLI consumed by AI
// agents. Inspired by Cloudflare's /cdn-cgi/explorer/api runtime endpoint
// (2026-04-13 Wrangler post): agents can introspect the live CLI without
// parsing --help or reading source.
type agentContext struct {
	SchemaVersion              string                `json:"schema_version"`
	CLI                        agentContextCLI       `json:"cli"`
	Auth                       agentContextAuth      `json:"auth"`
	GlobalFlags                []agentContextFlag    `json:"global_flags"`
	Commands                   []agentContextCommand `json:"commands"`
	MCP                        agentContextMCP       `json:"mcp"`
	AvailableProfiles          []string              `json:"available_profiles"`
	FeedbackEndpointConfigured bool                  `json:"feedback_endpoint_configured"`
}

// agentContextMCP summarizes the companion MCP server's surface so an
// introspecting agent can decide whether to drive the CLI directly or
// reach for the MCP gateway, without spawning gorgias-pp-mcp and
// inspecting its tools/list response.
type agentContextMCP struct {
	Mode          string   `json:"mode"`
	Binary        string   `json:"binary"`
	FrameworkTool []string `json:"framework_tools"`
	GatewayTools  []string `json:"gateway_tools"`
	Note          string   `json:"note"`
}

type agentContextCLI struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type agentContextAuth struct {
	Mode    string                   `json:"mode"`
	EnvVars []agentContextAuthEnvVar `json:"env_vars"`
}

type agentContextAuthEnvVar struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Required    bool   `json:"required"`
	Sensitive   bool   `json:"sensitive"`
	Description string `json:"description,omitempty"`
}

type agentContextCommand struct {
	Name        string                `json:"name"`
	Use         string                `json:"use,omitempty"`
	Short       string                `json:"short,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	Flags       []agentContextFlag    `json:"flags,omitempty"`
	Subcommands []agentContextCommand `json:"subcommands,omitempty"`
}

type agentContextFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Usage   string `json:"usage,omitempty"`
	Default string `json:"default,omitempty"`
}

func newAgentContextCmd(rootCmd *cobra.Command) *cobra.Command {
	var pretty bool
	cmd := &cobra.Command{
		Use:         "agent-context",
		Short:       "Emit structured JSON describing this CLI for agents",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := buildAgentContext(rootCmd)
			enc := json.NewEncoder(os.Stdout)
			if pretty {
				enc.SetIndent("", "  ")
			}
			return enc.Encode(ctx)
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", false, "indent JSON output for human reading")
	return cmd
}

func buildAgentContext(rootCmd *cobra.Command) agentContext {
	envVars := []agentContextAuthEnvVar{
		{
			Name:        "GORGIAS_USERNAME",
			Kind:        "per_call",
			Required:    true,
			Sensitive:   false,
			Description: "Your Gorgias account email; the username half of HTTP Basic auth.",
		},
		{
			Name:        "GORGIAS_API_KEY",
			Kind:        "per_call",
			Required:    true,
			Sensitive:   true,
			Description: "Your Gorgias API key; the password half of HTTP Basic auth (Settings → Account → REST API).",
		},
		{
			Name:        "GORGIAS_BASE_URL",
			Kind:        "per_call",
			Required:    true,
			Sensitive:   false,
			Description: "Your tenant's API URL, e.g. https://<tenant>.gorgias.com/api. Gorgias is multi-tenant — there is no default.",
		},
		{
			Name:        "GORGIAS_CONFIG",
			Kind:        "per_call",
			Required:    false,
			Sensitive:   false,
			Description: "Override path to the TOML config file (default $XDG_CONFIG_HOME/gorgias-pp-cli/config.toml).",
		},
	}
	profiles := ListProfileNames()
	if profiles == nil {
		profiles = []string{}
	}
	return agentContext{
		SchemaVersion: agentContextSchemaVersion,
		CLI: agentContextCLI{
			Name:        "gorgias-pp-cli",
			Description: "Every Gorgias support workflow, agent-native, in one binary.",
			Version:     rootCmd.Version,
		},
		Auth: agentContextAuth{
			Mode:    "api_key",
			EnvVars: envVars,
		},
		GlobalFlags: collectGlobalFlags(rootCmd),
		Commands:    collectAgentCommands(rootCmd),
		MCP: agentContextMCP{
			Mode:          "code-orchestration",
			Binary:        "gorgias-pp-mcp",
			FrameworkTool: []string{"search", "sql", "context"},
			GatewayTools:  []string{"gorgias_search", "gorgias_execute"},
			Note:          "The MCP server also exposes a runtime cobra-mirror of agent-relevant CLI verbs (workflow_*, sync, analytics, pm_*, export, import, tail). Reach the full Gorgias endpoint surface via gorgias_execute. Live tool_count is reported by the MCP `context` tool.",
		},
		AvailableProfiles:          profiles,
		FeedbackEndpointConfigured: FeedbackEndpointConfigured(),
	}
}

// collectGlobalFlags returns the root persistent flags (e.g., --json, --agent,
// --compact, --data-source, --profile, --dry-run, --deliver, --rate-limit).
// An agent introspecting via `agent-context --json` needs these to learn what
// global toggles every subcommand inherits; without this section, the agent
// would have to parse `--help` text or run `gorgias-pp-cli --help` and scrape.
func collectGlobalFlags(rootCmd *cobra.Command) []agentContextFlag {
	var out []agentContextFlag
	rootCmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		out = append(out, agentContextFlag{
			Name:    f.Name,
			Type:    f.Value.Type(),
			Usage:   f.Usage,
			Default: f.DefValue,
		})
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// collectAgentCommands walks the cobra tree from the given command and
// returns its direct children (skipping the agent-context command itself
// to avoid self-reference). Each child is recursed into if it has
// subcommands. Flags are captured via VisitAll. Output is sorted by
// command name for stable diffs across regenerations.
//
// Cobra's Hidden flag suppresses listing in --help but does not gate
// agent discovery. Raw resource parents are Hidden so --help stays
// curated and the `api` browser populates; the agent-context surface
// must still enumerate them and their endpoints so agents can call any
// action a CLI user could.
func collectAgentCommands(c *cobra.Command) []agentContextCommand {
	children := c.Commands()
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })

	out := make([]agentContextCommand, 0, len(children))
	for _, sub := range children {
		if sub.Name() == "agent-context" {
			continue
		}
		entry := agentContextCommand{
			Name:  sub.Name(),
			Use:   sub.Use,
			Short: sub.Short,
		}
		// Surface Cobra annotations (e.g., pp:endpoint, mcp:read-only) so
		// agents and the live-dogfood classifier can detect destructive-at-auth
		// endpoints without parsing source. Empty maps are stripped via
		// omitempty in the struct tag.
		if len(sub.Annotations) > 0 {
			entry.Annotations = make(map[string]string, len(sub.Annotations))
			for k, v := range sub.Annotations {
				entry.Annotations[k] = v
			}
		}
		sub.Flags().VisitAll(func(f *pflag.Flag) {
			entry.Flags = append(entry.Flags, agentContextFlag{
				Name:    f.Name,
				Type:    f.Value.Type(),
				Usage:   f.Usage,
				Default: f.DefValue,
			})
		})
		sort.Slice(entry.Flags, func(i, j int) bool {
			return entry.Flags[i].Name < entry.Flags[j].Name
		})
		if len(sub.Commands()) > 0 {
			entry.Subcommands = collectAgentCommands(sub)
		}
		out = append(out, entry)
	}
	return out
}
