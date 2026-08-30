package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// agentContextSchemaVersion tracks the JSON shape emitted by `agent-context`.
// The Phase 5 live-dogfood harness discovers this CLI's command surface
// through this command rather than by parsing --help, so the shape here is a
// contract with the harness and not merely documentation.
const agentContextSchemaVersion = "4"

type agentContext struct {
	SchemaVersion     string                `json:"schema_version"`
	CLI               agentContextCLI       `json:"cli"`
	Auth              agentContextAuth      `json:"auth"`
	Paths             agentContextPaths     `json:"paths"`
	Commands          []agentContextCommand `json:"commands"`
	AvailableProfiles []string              `json:"available_profiles"`
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
	StateDir  string `json:"state_dir"`
	CacheDir  string `json:"cache_dir"`
}

type agentContextCommand struct {
	Name        string                `json:"name"`
	Use         string                `json:"use,omitempty"`
	Short       string                `json:"short,omitempty"`
	Annotations map[string]string     `json:"annotations,omitempty"`
	Flags       []agentContextFlag    `json:"flags,omitempty"`
	Runnable    bool                  `json:"runnable,omitempty"`
	Subcommands []agentContextCommand `json:"subcommands,omitempty"`
}

type agentContextFlag struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Usage   string `json:"usage,omitempty"`
	Default string `json:"default,omitempty"`
}

// readOnly marks a command as safe for an agent to invoke without confirmation.
func readOnly() map[string]string { return map[string]string{"mcp:read-only": "true"} }

// mutating marks a command that changes server state. set-retries is dry-run
// unless --yes is passed, but it is still advertised as destructive so the
// live-dogfood matrix skips it by default rather than probing a write path
// against a production instance.
func mutating() map[string]string {
	return map[string]string{
		"mcp:destructive":     "true",
		"pp:destructive-auth": "true",
	}
}

func buildAgentContext() agentContext {
	cfg := userDir(os.UserConfigDir)
	cache := userDir(os.UserCacheDir)
	home, _ := os.UserHomeDir()
	return agentContext{
		SchemaVersion: agentContextSchemaVersion,
		CLI: agentContextCLI{
			Name:        "kuma-pp-cli",
			Description: "Uptime Kuma v2 operator CLI over the Socket.IO management protocol.",
			Version:     version,
		},
		Auth: agentContextAuth{
			Mode: "username_password",
			EnvVars: []agentContextAuthEnvVar{
				{Name: "UPTIME_KUMA_URL", Kind: "config", Required: true, Sensitive: false,
					Description: "Base origin of the Uptime Kuma server, e.g. https://kuma.example.com"},
				{Name: "UPTIME_KUMA_USERNAME", Kind: "credential", Required: true, Sensitive: true},
				{Name: "UPTIME_KUMA_PASSWORD", Kind: "credential", Required: true, Sensitive: true,
					Description: "Set to your Uptime Kuma account password."},
			},
		},
		Paths: agentContextPaths{
			ConfigDir: filepath.Join(cfg, "kuma-pp-cli"),
			DataDir:   filepath.Join(home, ".local", "share", "kuma-pp-cli"),
			StateDir:  filepath.Join(home, ".local", "state", "kuma-pp-cli"),
			CacheDir:  filepath.Join(cache, "kuma-pp-cli"),
		},
		Commands:          agentCommands(),
		AvailableProfiles: []string{},
	}
}

func userDir(fn func() (string, error)) string {
	d, err := fn()
	if err != nil {
		return ""
	}
	return d
}

func agentCommands() []agentContextCommand {
	cmds := []agentContextCommand{
		{
			Name: "health", Use: "health", Runnable: true,
			Short:       "Check connectivity and authentication against the configured server",
			Annotations: readOnly(),
		},
		{
			Name: "monitors", Use: "monitors [--query text]", Runnable: true,
			Short:       "List configured monitors",
			Annotations: readOnly(),
			Flags: []agentContextFlag{
				{Name: "query", Type: "string", Usage: "case-insensitive substring filter on monitor name"},
			},
		},
		{
			Name: "heartbeats", Use: "heartbeats [--hours N]", Runnable: true,
			Short:       "Recent heartbeat history across monitors",
			Annotations: readOnly(),
			Flags: []agentContextFlag{
				{Name: "hours", Type: "int", Usage: "lookback window in hours", Default: "3"},
				{Name: "monitor-id", Type: "int", Usage: "restrict output to a single monitor id", Default: "0"},
			},
		},
		{
			Name: "incident-context", Use: "incident-context --monitor <id|name>", Runnable: true,
			Short:       "Composite incident brief for one monitor",
			Annotations: readOnly(),
			Flags: []agentContextFlag{
				{Name: "monitor", Type: "string", Usage: "monitor id or exact name"},
				{Name: "lookback-minutes", Type: "int", Usage: "heartbeat window to summarize", Default: "60"},
			},
		},
		{
			Name: "set-retries", Use: "set-retries --monitor <id> --maxretries <n> [--yes]", Runnable: true,
			Short:       "Change a monitor's retry count (dry-run unless --yes)",
			Annotations: mutating(),
			Flags: []agentContextFlag{
				{Name: "monitor", Type: "int", Usage: "monitor id to edit", Default: "0"},
				{Name: "maxretries", Type: "int", Usage: "new maxretries value", Default: "-1"},
				{Name: "yes", Type: "bool", Usage: "apply the change instead of previewing it", Default: "false"},
			},
		},
		{
			Name: "version", Use: "version", Runnable: true,
			Short:       "Print the CLI version",
			Annotations: readOnly(),
		},
	}
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name < cmds[j].Name })
	return cmds
}

func runAgentContext(stdout io.Writer, args []string) error {
	pretty := false
	for _, a := range args {
		if a == "--pretty" {
			pretty = true
		}
	}
	enc := json.NewEncoder(stdout)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(buildAgentContext())
}
