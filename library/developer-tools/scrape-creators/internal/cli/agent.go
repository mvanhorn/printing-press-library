// Copyright 2026 adrian-horning. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/config"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
)

const (
	mcpServerName   = "scrape-creators"
	mcpLocalCommand = "scrape-creators-pp-mcp"
	mcpHostedURL    = "https://api.scrapecreators.com/mcp"
	mcpEnvVarName   = "SCRAPE_CREATORS_API_KEY"
	mcpHostedHeader = "x-api-key"
)

// supportedTargets lists every MCP host the CLI can wire. Order matters for
// the help text — keep alphabetical.
var supportedTargets = []string{"claude-code", "claude-desktop", "codex", "cursor"}

func newAgentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage AI agent integrations (MCP server wiring)",
	}
	cmd.AddCommand(newAgentAddCmd(flags))
	return cmd
}

func newAgentAddCmd(flags *rootFlags) *cobra.Command {
	var hosted bool
	var force bool

	cmd := &cobra.Command{
		Use:   "add <target>",
		Short: "Wire the scrape-creators MCP server into an AI agent's config",
		Long: fmt.Sprintf(`Write the scrape-creators MCP server entry into the target agent's
configuration file. Supported targets: %s.

Default target command is the local '%s' stdio binary. Pass --hosted
to write the hosted MCP endpoint (%s) instead.

Files are created (or chmoded) to mode 0600. Existing entries under
the name %q are refused without --force and a diff is printed.`,
			strings.Join(supportedTargets, ", "),
			mcpLocalCommand,
			mcpHostedURL,
			mcpServerName,
		),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := strings.ToLower(args[0])
			if !contains(supportedTargets, target) {
				return &cliError{code: 2, err: fmt.Errorf("unknown target %q: supported targets are %s", target, strings.Join(supportedTargets, ", "))}
			}

			apiKey, err := optionalAPIKey(flags)
			if err != nil {
				return err
			}

			entry := mcpEntry(hosted, apiKey)

			path, err := configPathFor(target)
			if err != nil {
				return err
			}

			action, err := writeMCPEntry(target, path, entry, force)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%s %s at %s (mode 0600)\n", action, target, path)
			if apiKey == "" {
				if hosted {
					fmt.Fprintf(cmd.OutOrStdout(), "no API key was found, so the hosted MCP entry was written without an %s header.\n", mcpHostedHeader)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "no API key was found, so the MCP entry was written without %s in its env block.\n", mcpEnvVarName)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "set %s before first use, or re-run once credentials are available.\n", mcpEnvVarName)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "restart %s for the change to take effect.\n", target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&hosted, "hosted", false, "Write the hosted MCP URL instead of the local binary")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing scrape-creators MCP entry without prompting")
	return cmd
}

// mcpEntry returns the server-entry payload (shape-agnostic map) for either
// local or hosted mode. The JSON and TOML writers both consume this map.
func mcpEntry(hosted bool, apiKey string) map[string]any {
	if hosted {
		entry := map[string]any{"url": mcpHostedURL}
		if apiKey != "" {
			entry["headers"] = map[string]any{
				mcpHostedHeader: apiKey,
			}
		}
		return entry
	}
	entry := map[string]any{"command": mcpLocalCommand}
	if apiKey != "" {
		entry["env"] = map[string]any{
			mcpEnvVarName: apiKey,
		}
	}
	return entry
}

// configPathFor returns the target's config-file path for the current OS.
func configPathFor(target string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	switch target {
	case "cursor":
		return filepath.Join(home, ".cursor", "mcp.json"), nil
	case "claude-code":
		return filepath.Join(home, ".claude.json"), nil
	case "codex":
		return filepath.Join(home, ".codex", "config.toml"), nil
	case "claude-desktop":
		switch runtime.GOOS {
		case "darwin":
			return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
		case "windows":
			appData := os.Getenv("APPDATA")
			if appData == "" {
				appData = filepath.Join(home, "AppData", "Roaming")
			}
			return filepath.Join(appData, "Claude", "claude_desktop_config.json"), nil
		default:
			return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), nil
		}
	}
	return "", fmt.Errorf("unknown target %q", target)
}

// writeMCPEntry writes the server entry into the target file, enforcing 0600
// mode and the --force overwrite policy. Returns a verb describing the action
// ("wired", "overwrote", "updated").
func writeMCPEntry(target, path string, entry map[string]any, force bool) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", fmt.Errorf("creating parent dir: %w", err)
	}

	if target == "codex" {
		return writeMCPEntryTOML(path, entry, force)
	}
	return writeMCPEntryJSON(path, entry, force)
}

// writeMCPEntryJSON merges the entry into a JSON config under mcpServers.
func writeMCPEntryJSON(path string, entry map[string]any, force bool) (string, error) {
	var config map[string]any
	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, &config); err != nil {
			return "", fmt.Errorf("parsing existing %s: %w", path, err)
		}
	} else {
		config = map[string]any{}
	}

	servers, _ := config["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}

	action := "wired"
	if existing, ok := servers[mcpServerName]; ok {
		if !force {
			diff, _ := renderDiff(existing, entry)
			return "", overwriteRefused(path, diff)
		}
		action = "overwrote"
	}
	servers[mcpServerName] = entry
	config["mcpServers"] = servers

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling config: %w", err)
	}
	if err := writeFile0600(path, out); err != nil {
		return "", err
	}
	return action, nil
}

// writeMCPEntryTOML merges the entry into a TOML config under [mcp_servers.*].
func writeMCPEntryTOML(path string, entry map[string]any, force bool) (string, error) {
	var config map[string]any
	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		if err := toml.Unmarshal(raw, &config); err != nil {
			return "", fmt.Errorf("parsing existing %s: %w", path, err)
		}
	} else {
		config = map[string]any{}
	}

	servers, _ := config["mcp_servers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}

	action := "wired"
	if existing, ok := servers[mcpServerName]; ok {
		if !force {
			diff, _ := renderDiff(existing, entry)
			return "", overwriteRefused(path, diff)
		}
		action = "overwrote"
	}
	servers[mcpServerName] = entry
	config["mcp_servers"] = servers

	out, err := toml.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("marshaling config: %w", err)
	}
	if err := writeFile0600(path, out); err != nil {
		return "", err
	}
	return action, nil
}

func writeFile0600(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

// renderDiff produces a simple two-block view for the overwrite refusal path.
func renderDiff(old, new any) (string, error) {
	oldJSON, err := json.MarshalIndent(old, "  ", "  ")
	if err != nil {
		return "", err
	}
	newJSON, err := json.MarshalIndent(new, "  ", "  ")
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("existing entry:\n  ")
	b.Write(oldJSON)
	b.WriteString("\n\nproposed entry:\n  ")
	b.Write(newJSON)
	b.WriteString("\n")
	return b.String(), nil
}

func overwriteRefused(path, diff string) error {
	return &cliError{code: 2, err: fmt.Errorf(
		"a scrape-creators MCP entry already exists in %s. Re-run with --force to overwrite.\n\n%s",
		path, diff,
	)}
}

// optionalAPIKey pulls the API key from config or env when one exists, but does
// not force the user to have credentials before wiring the MCP server config.
func optionalAPIKey(flags *rootFlags) (string, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return "", configErr(err)
	}
	return cfg.APIKey, nil
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// listSupportedTargets is used by error messages and potential future help.
func listSupportedTargets() string {
	out := append([]string(nil), supportedTargets...)
	sort.Strings(out)
	return strings.Join(out, ", ")
}
