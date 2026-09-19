// pp:data-source local
// Hand-authored Haven command; shared implementation lives in haven_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelHavenMenuCmd(flags *rootFlags) *cobra.Command {
	o := &havenOptions{}
	cmd := &cobra.Command{Use: "menu", Short: "Search a saved menu with base prices in cents", Example: "  haven-hot-chicken-pp-cli haven menu --location 14208 --query sandwich --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--location=14208"}}
	cmd.Flags().StringVar(&o.db, "db", "", "Path to the local SQLite observation database")
	cmd.Flags().IntVar(&o.limit, "limit", 50, "Maximum output rows, between 1 and 1000")
	cmd.Flags().Int64Var(&o.location, "location", 0, "Location ID from locations list, for example 14208")
	cmd.Flags().StringVar(&o.query, "query", "", "Case-insensitive text to find in saved item names")
	return configureHavenCommand(flags, "menu", cmd, o)
}

