// Copyright 2026 John Fiedler and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/foxnews/internal/foxnews"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const agentContextSchemaVersion = "1"

type agentContext struct {
	SchemaVersion string                 `json:"schema_version"`
	CLI           agentContextCLI        `json:"cli"`
	Auth          agentContextAuth       `json:"auth"`
	DefaultSection string                `json:"default_section"`
	Sections      []foxnews.Section      `json:"sections"`
	Commands      []agentContextCommand  `json:"commands"`
	NotAvailable  []string               `json:"not_available,omitempty"`
}

type agentContextCLI struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type agentContextAuth struct {
	Mode    string `json:"mode"`
	EnvVars []string `json:"env_vars"`
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
		Long: `Outputs a machine-readable description of commands, RSS sections, and defaults
so agents can introspect this CLI without parsing --help.`,
		Example: `  foxnews-pp-cli agent-context --pretty
  foxnews-pp-cli agent-context | jq .default_section`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := buildAgentContext(rootCmd)
			enc := json.NewEncoder(cmd.OutOrStdout())
			if pretty {
				enc.SetIndent("", "  ")
			}
			return enc.Encode(ctx)
		},
	}
	cmd.Flags().BoolVar(&pretty, "pretty", false, "Indent JSON output for human reading")
	return cmd
}

func buildAgentContext(rootCmd *cobra.Command) agentContext {
	sections := make([]foxnews.Section, len(foxnews.Sections))
	copy(sections, foxnews.Sections)
	return agentContext{
		SchemaVersion: agentContextSchemaVersion,
		CLI: agentContextCLI{
			Name:        "foxnews-pp-cli",
			Description: "Fox News headlines from Google Publisher RSS feeds (moxie.foxnews.com).",
			Version:     rootCmd.Version,
		},
		Auth: agentContextAuth{
			Mode:    "none",
			EnvVars: []string{"FOX_NEWS_FEED_BASE"},
		},
		DefaultSection: "latest",
		Sections:       sections,
		Commands:       collectAgentCommands(rootCmd),
		NotAvailable: []string{
			"which", "sync", "splash", "breaking", "tail", "tenure", "sources", "on-date", "bent", "story", "digest",
		},
	}
}

func collectAgentCommands(c *cobra.Command) []agentContextCommand {
	children := c.Commands()
	sort.Slice(children, func(i, j int) bool { return children[i].Name() < children[j].Name() })

	out := make([]agentContextCommand, 0, len(children))
	for _, sub := range children {
		if sub.Hidden || sub.Name() == "agent-context" {
			continue
		}
		entry := agentContextCommand{
			Name:  sub.Name(),
			Use:   sub.Use,
			Short: sub.Short,
		}
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
