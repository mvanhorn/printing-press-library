package cli

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

const agentContextSchemaVersion = "4"

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
	Mode    string        `json:"mode"`
	EnvVars []interface{} `json:"env_vars"`
}

type agentContextCommand struct {
	Name        string                `json:"name"`
	Use         string                `json:"use,omitempty"`
	Short       string                `json:"short,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	Subcommands []agentContextCommand `json:"subcommands,omitempty"`
}

func newAgentContextCmd(rootCmd *cobra.Command) *cobra.Command {
	var pretty bool
	cmd := &cobra.Command{
		Use:         "agent-context",
		Short:       "Emit structured JSON describing this CLI for agents",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			enc := json.NewEncoder(os.Stdout)
			if pretty {
				enc.SetIndent("", "  ")
			}
			return enc.Encode(buildAgentContext(rootCmd))
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", false, "indent JSON output for human reading")
	return cmd
}

func buildAgentContext(rootCmd *cobra.Command) agentContext {
	return agentContext{
		SchemaVersion: agentContextSchemaVersion,
		CLI: agentContextCLI{
			Name:        "logodownload-pp-cli",
			Description: "Search public logo entries on logodownload.org and return page URLs plus preview image URLs.",
			Version:     rootCmd.Version,
		},
		Auth: agentContextAuth{
			Mode:    "none",
			EnvVars: []interface{}{},
		},
		Commands: collectAgentCommands(rootCmd),
	}
}

func collectAgentCommands(cmd *cobra.Command) []agentContextCommand {
	children := cmd.Commands()
	commands := make([]agentContextCommand, 0, len(children))
	for _, child := range children {
		if child.Hidden {
			continue
		}
		commands = append(commands, agentContextCommand{
			Name:        child.Name(),
			Use:         child.Use,
			Short:       child.Short,
			Annotations: child.Annotations,
			Subcommands: collectAgentCommands(child),
		})
	}
	return commands
}
