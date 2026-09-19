// pp:data-source local
// Hand-authored Haven command; shared implementation lives in haven_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelHavenCompareCmd(flags *rootFlags) *cobra.Command {
	o := &havenOptions{}
	cmd := &cobra.Command{Use: "compare", Short: "Compare exact item names across saved location menus", Example: "  haven-hot-chicken-pp-cli haven compare --item 'The Sandwich' --locations 14208,14205 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--item=The Sandwich;--locations=14208,14205"}}
	cmd.Flags().StringVar(&o.db, "db", "", "Path to the local SQLite observation database")
	cmd.Flags().IntVar(&o.limit, "limit", 50, "Maximum output rows, between 1 and 1000")
	cmd.Flags().StringVar(&o.locations, "locations", "14208,14205", "Comma-separated location IDs; at most ten per request")
	cmd.Flags().StringVar(&o.item, "item", "", "Exact item name to compare across selected locations")
	return configureHavenCommand(flags, "compare", cmd, o)
}

