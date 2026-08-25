// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"os"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/cliutil"
	"github.com/spf13/cobra"
)

const agentContextSchemaVersion = "1"

type agentContext struct {
	SchemaVersion string                `json:"schema_version"`
	CLI           agentContextCLI       `json:"cli"`
	Auth          agentContextAuth      `json:"auth"`
	Paths         agentContextPaths     `json:"paths"`
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

type agentContextPaths struct {
	ConfigDir string `json:"config_dir"`
	DataDir   string `json:"data_dir"`
	CacheDir  string `json:"cache_dir"`
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
		Use:   "agent-context",
		Short: "Emit structured JSON describing this CLI for agents",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		Example: `  notebooklm-pp-cli agent-context --pretty`,
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
	cfgDir, _ := cliutil.ConfigDir()
	dataDir, _ := cliutil.DataDir()
	cacheDir, _ := cliutil.CacheDir()
	return agentContext{
		SchemaVersion: agentContextSchemaVersion,
		CLI: agentContextCLI{
			Name:        "notebooklm-pp-cli",
			Description: rootCmd.Short,
			Version:     version,
		},
		Auth: agentContextAuth{
			Mode: "cookie",
			EnvVars: []agentContextAuthEnvVar{
				{Name: "NOTEBOOKLM_COOKIE", Kind: "cookie", Required: false, Sensitive: true, Description: "Google session cookie header override"},
				{Name: "NOTEBOOKLM_NO_AUTO_REFRESH", Kind: "flag", Required: false, Sensitive: false, Description: "Set to 1 to skip auto-refresh before local reads"},
			},
		},
		Paths: agentContextPaths{
			ConfigDir: cfgDir,
			DataDir:   dataDir,
			CacheDir:  cacheDir,
		},
		Commands: walkAgentCommands(rootCmd),
	}
}

func walkAgentCommands(cmd *cobra.Command) []agentContextCommand {
	if cmd == nil {
		return nil
	}
	var out []agentContextCommand
	for _, sub := range cmd.Commands() {
		if sub.Hidden || sub.Name() == "agent-context" {
			continue
		}
		entry := agentContextCommand{
			Name:        sub.Name(),
			Use:         sub.Use,
			Short:       sub.Short,
			Annotations: sub.Annotations,
		}
		if sub.HasSubCommands() {
			entry.Subcommands = walkAgentCommands(sub)
		}
		out = append(out, entry)
	}
	return out
}
