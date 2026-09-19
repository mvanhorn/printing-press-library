// pp:data-source live
// Hand-authored Haven command; shared implementation lives in haven_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelHavenRefreshCmd(flags *rootFlags) *cobra.Command {
	o := &havenOptions{limit: 50}
	cmd := &cobra.Command{Use: "refresh", Short: "Fetch complete public menus and save timestamped observations", Example: "  haven-hot-chicken-pp-cli haven refresh --locations 14208,14205 --agent", Annotations: map[string]string{"mcp:read-only": "false", "mcp:local-write": "true", "mcp:destructive": "false", "pp:data-source": "live", "pp:happy-args": "--locations=14208,14205"}}
	cmd.Flags().StringVar(&o.db, "db", "", "Path to the local SQLite observation database")
	cmd.Flags().StringVar(&o.locations, "locations", "14208,14205", "Comma-separated location IDs; at most ten per request")
	return configureHavenCommand(flags, "refresh", cmd, o)
}
