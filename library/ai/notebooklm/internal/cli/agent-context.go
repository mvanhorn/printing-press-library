package cli

import "github.com/spf13/cobra"

func newAgentContextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "agent-context",
		Short: "Emit structured JSON describing this CLI for agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := map[string]any{
				"name":        "notebooklm-pp-cli",
				"version":     version,
				"description": "Query and manage Google NotebookLM notebooks from the terminal",
				"transport":   "subprocess-nlm",
				"requires":    []string{"nlm binary on PATH"},
				"commands": []map[string]string{
					{"name": "notebooks list", "desc": "List all notebooks"},
					{"name": "notebooks get", "desc": "Get notebook details"},
					{"name": "notebooks create", "desc": "Create notebook"},
					{"name": "notebooks describe", "desc": "AI summary + topics"},
					{"name": "query", "desc": "Ask a question against notebook sources"},
					{"name": "cross-query", "desc": "Query across multiple notebooks"},
					{"name": "sources list", "desc": "List sources in a notebook"},
					{"name": "sources add", "desc": "Add source (URL/text/file/drive)"},
					{"name": "sources content", "desc": "Get raw source text"},
					{"name": "sync", "desc": "Cache to SQLite for offline search"},
					{"name": "search", "desc": "Full-text search across synced data"},
					{"name": "studio create", "desc": "Create audio/video/report artifact"},
					{"name": "research start", "desc": "Start fast/deep research"},
					{"name": "doctor", "desc": "Health check"},
				},
				"flags": map[string]string{
					"--agent": "JSON + compact + non-interactive mode",
					"--json":  "JSON output",
					"--yes":   "Skip confirmations",
				},
			}
			return printJSON(ctx)
		},
	}
}
