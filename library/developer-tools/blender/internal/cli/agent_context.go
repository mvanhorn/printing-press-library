package cli

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type agentContext struct {
	SchemaVersion string                `json:"schema_version"`
	CLI           agentContextCLI       `json:"cli"`
	Auth          agentContextAuth      `json:"auth"`
	Commands      []agentContextCommand `json:"commands"`
}

type agentContextCLI struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type agentContextAuth struct {
	Mode    string   `json:"mode"`
	EnvVars []string `json:"env_vars"`
}

type agentContextCommand struct {
	Name        string                `json:"name"`
	Use         string                `json:"use,omitempty"`
	Short       string                `json:"short,omitempty"`
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
		Use:   "agent-context",
		Short: "Emit structured JSON describing this CLI for agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			enc := json.NewEncoder(os.Stdout)
			if pretty {
				enc.SetIndent("", "  ")
			}
			return enc.Encode(agentContext{
				SchemaVersion: "1",
				CLI: agentContextCLI{
					Name:        "blender-pp-cli",
					Description: "Agent-native Blender automation for CAD, rendering, animation, and MCP-assisted tandem sessions",
					Version:     rootCmd.Version,
				},
				Auth:     agentContextAuth{Mode: "none", EnvVars: []string{"BLENDER_BIN", "BLENDER_MCP_BIN"}},
				Commands: collectAgentCommands(rootCmd),
			})
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Indent JSON output")
	return cmd
}

func collectAgentCommands(c *cobra.Command) []agentContextCommand {
	children := c.Commands()
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })
	out := make([]agentContextCommand, 0, len(children))
	for _, sub := range children {
		if sub.Name() == "agent-context" {
			continue
		}
		entry := agentContextCommand{Name: sub.Name(), Use: sub.Use, Short: sub.Short}
		sub.Flags().VisitAll(func(f *pflag.Flag) {
			entry.Flags = append(entry.Flags, agentContextFlag{
				Name:    f.Name,
				Type:    f.Value.Type(),
				Usage:   f.Usage,
				Default: f.DefValue,
			})
		})
		sort.Slice(entry.Flags, func(i, j int) bool { return entry.Flags[i].Name < entry.Flags[j].Name })
		if len(sub.Commands()) > 0 {
			entry.Subcommands = collectAgentCommands(sub)
		}
		out = append(out, entry)
	}
	return out
}
