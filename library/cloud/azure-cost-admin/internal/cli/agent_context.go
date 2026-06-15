package cli

import (
	"encoding/json"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const agentContextSchemaVersion = "3"

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
	Flags       []agentContextFlag    `json:"flags,omitempty"`
	Subcommands []agentContextCommand `json:"subcommands,omitempty"`
}

type agentContextFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Usage   string `json:"usage,omitempty"`
	Default string `json:"default,omitempty"`
}

func newAgentContextCmd(app *app, rootCmd *cobra.Command) *cobra.Command {
	var pretty bool
	cmd := &cobra.Command{
		Use:   "agent-context",
		Short: "Emit structured JSON describing this CLI for agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := buildAgentContext(rootCmd)
			enc := json.NewEncoder(app.out)
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
	return agentContext{
		SchemaVersion: agentContextSchemaVersion,
		CLI: agentContextCLI{
			Name:        "azure-cost-admin-pp-cli",
			Description: "Read-only Azure FinOps CLI for actual spend, tag hygiene, anomaly checks, and price lookup support.",
			Version:     rootCmd.Version,
		},
		Auth: agentContextAuth{
			Mode: "azure-cli",
			EnvVars: []agentContextAuthEnvVar{
				{Name: "AZURE_TENANT_ID", Kind: "environment", Required: false, Sensitive: false},
				{Name: "AZURE_CLIENT_ID", Kind: "environment", Required: false, Sensitive: false},
				{Name: "AZURE_CLIENT_SECRET", Kind: "environment", Required: false, Sensitive: true},
				{Name: "AZURE_SUBSCRIPTION_ID", Kind: "environment", Required: false, Sensitive: false},
			},
		},
		Commands: collectAgentCommands(rootCmd),
	}
}

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
