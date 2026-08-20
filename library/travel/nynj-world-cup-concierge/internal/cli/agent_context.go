// Copyright 2026 USER and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type agentContext struct {
	SchemaVersion              string                `json:"schema_version"`
	CLI                        agentContextCLI       `json:"cli"`
	Auth                       agentContextAuth      `json:"auth"`
	Commands                   []agentContextCommand `json:"commands"`
	AvailableProfiles          []string              `json:"available_profiles"`
	FeedbackEndpointConfigured bool                  `json:"feedback_endpoint_configured"`
}

type agentContextCLI struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type agentContextAuth struct {
	Mode    string `json:"mode"`
	EnvVars []any  `json:"env_vars"`
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
			ctx := agentContext{
				SchemaVersion: "3",
				CLI: agentContextCLI{
					Name:        "nynj-world-cup-concierge-pp-cli",
					Description: "Read-only official NYNJ World Cup Concierge extraction CLI.",
					Version:     rootCmd.Version,
				},
				Auth: agentContextAuth{
					Mode:    "none",
					EnvVars: []any{},
				},
				Commands:                   collectAgentCommands(rootCmd),
				AvailableProfiles:          []string{},
				FeedbackEndpointConfigured: false,
			}
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
		if len(sub.Annotations) > 0 {
			entry.Annotations = make(map[string]string, len(sub.Annotations))
			for key, value := range sub.Annotations {
				entry.Annotations[key] = value
			}
		}
		sub.Flags().VisitAll(func(flag *pflag.Flag) {
			entry.Flags = append(entry.Flags, agentContextFlag{
				Name:    flag.Name,
				Type:    flag.Value.Type(),
				Usage:   flag.Usage,
				Default: flag.DefValue,
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
