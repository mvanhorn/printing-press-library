// pp:data-source local
// Hand-authored Haven command; shared implementation lives in haven_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelHavenChangesCmd(flags *rootFlags) *cobra.Command {
	o := &havenOptions{}
	cmd := &cobra.Command{Use: "changes", Short: "Compare the two latest complete observations of one location", Example: "  haven-hot-chicken-pp-cli haven changes --location 14208 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--location=14208"}}
	cmd.Flags().StringVar(&o.db, "db", "", "Path to the local SQLite observation database")
	cmd.Flags().IntVar(&o.limit, "limit", 50, "Maximum output rows, between 1 and 1000")
	cmd.Flags().Int64Var(&o.location, "location", 0, "Location ID from locations list, for example 14208")
	return configureHavenCommand(flags, "changes", cmd, o)
}

