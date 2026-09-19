// pp:data-source local
// Hand-authored Haven command; shared implementation lives in haven_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelHavenCommonCmd(flags *rootFlags) *cobra.Command {
	o := &havenOptions{}
	cmd := &cobra.Command{Use: "common", Short: "Find confirmed available items shared by every selected location", Example: "  haven-hot-chicken-pp-cli haven common --locations 14208,14205 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--locations=14208,14205"}}
	cmd.Flags().StringVar(&o.db, "db", "", "Path to the local SQLite observation database")
	cmd.Flags().IntVar(&o.limit, "limit", 50, "Maximum output rows, between 1 and 1000")
	cmd.Flags().StringVar(&o.locations, "locations", "14208,14205", "Comma-separated location IDs; at most ten per request")
	return configureHavenCommand(flags, "common", cmd, o)
}

