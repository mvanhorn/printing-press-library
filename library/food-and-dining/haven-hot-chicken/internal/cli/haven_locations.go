// pp:data-source local
// Hand-authored Haven command; shared implementation lives in haven_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelHavenLocationsCmd(flags *rootFlags) *cobra.Command {
	o := &havenOptions{}
	cmd := &cobra.Command{Use: "locations", Short: "List saved Haven locations and public coordinates", Example: "  haven-hot-chicken-pp-cli haven locations --limit 10 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--limit=10"}}
	cmd.Flags().StringVar(&o.db, "db", "", "Path to the local SQLite observation database")
	cmd.Flags().IntVar(&o.limit, "limit", 50, "Maximum output rows, between 1 and 1000")
	return configureHavenCommand(flags, "locations", cmd, o)
}

