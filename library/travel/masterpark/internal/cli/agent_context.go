package cli

import "github.com/spf13/cobra"

type agentContext struct {
	SchemaVersion string                `json:"schema_version"`
	CLI           agentContextCLI       `json:"cli"`
	Commands      []agentContextCommand `json:"commands"`
}

type agentContextCLI struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type agentContextCommand struct {
	Name        string                `json:"name"`
	Description string                `json:"description,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	Subcommands []agentContextCommand `json:"subcommands,omitempty"`
}

func newAgentContextCmd(g *globalOpts) *cobra.Command {
	return &cobra.Command{
		Use:    "agent-context",
		Short:  "Print structured context for agents",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return printJSON(agentContext{
				SchemaVersion: "3",
				CLI: agentContextCLI{
					Name:        "masterpark-pp-cli",
					Description: "Unofficial CLI for MasterPark airport parking reservations using the netParkV2 AJAX API.",
				},
				Commands: []agentContextCommand{
					{
						Name:        "locations",
						Description: "List MasterPark parking locations.",
						Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": ""},
					},
					{
						Name:        "quote",
						Description: "Get live parking price quotes for a date range.",
						Annotations: map[string]string{
							"mcp:read-only": "true",
							"pp:happy-args": "--lot=B;--dropoff=2030-06-11 07:00;--pickup=2030-06-13 18:30",
						},
					},
				},
			})
		},
	}
}
