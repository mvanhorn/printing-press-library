package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// agent-context emits the machine-readable command tree (schema_version 3)
// that `cli-printing-press dogfood` and MCP tooling parse to discover the
// CLI surface. This is the framework introspection command; the human-facing
// "when to use" guidance lives in SKILL.md/README.md.

type agentContextDoc struct {
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
	Use         string                `json:"use"`
	Short       string                `json:"short"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	Flags       []agentContextFlag    `json:"flags"`
	Subcommands []agentContextCommand `json:"subcommands"`
}

type agentContextFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Usage   string `json:"usage"`
	Default string `json:"default,omitempty"`
}

func newAgentContextCmd(_ *RootOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "agent-context",
		Short:       "Emit the machine-readable command tree (schema, CLI identity, commands, flags) for agents and tooling",
		Annotations: map[string]string{"pp:typed-exit-codes": "0", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := cmd.Root()
			doc := agentContextDoc{
				SchemaVersion: "3",
				CLI: agentContextCLI{
					Name:        root.Name(),
					Description: root.Short,
					Version:     CLIVersion,
				},
				Auth:     agentContextAuth{Mode: "none", EnvVars: []string{}},
				Commands: walkAgentContextCommands(root),
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(doc)
		},
	}
}

func walkAgentContextCommands(parent *cobra.Command) []agentContextCommand {
	out := []agentContextCommand{}
	for _, c := range parent.Commands() {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		out = append(out, agentContextCommand{
			Name:        c.Name(),
			Use:         c.Use,
			Short:       c.Short,
			Annotations: c.Annotations,
			Flags:       agentContextFlags(c),
			Subcommands: walkAgentContextCommands(c),
		})
	}
	return out
}

func agentContextFlags(c *cobra.Command) []agentContextFlag {
	flags := []agentContextFlag{}
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		flags = append(flags, agentContextFlag{
			Name:    f.Name,
			Type:    f.Value.Type(),
			Usage:   f.Usage,
			Default: f.DefValue,
		})
	})
	return flags
}
