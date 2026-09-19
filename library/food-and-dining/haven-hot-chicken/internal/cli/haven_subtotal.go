// pp:data-source local
// Hand-authored Haven command; shared implementation lives in haven_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelHavenSubtotalCmd(flags *rootFlags) *cobra.Command {
	o := &havenOptions{}
	cmd := &cobra.Command{Use: "subtotal", Short: "Estimate base item prices using one saved location menu", Example: "  haven-hot-chicken-pp-cli haven subtotal --location 14208 --item 9656289=2 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--location=14208;--item=9656289=2"}}
	cmd.Flags().StringVar(&o.db, "db", "", "Path to the local SQLite observation database")
	cmd.Flags().IntVar(&o.limit, "limit", 50, "Maximum output rows, between 1 and 1000")
	cmd.Flags().Int64Var(&o.location, "location", 0, "Location ID from locations list, for example 14208")
	cmd.Flags().StringArrayVar(&o.quantities, "item", nil, "Item ID and quantity as ID=quantity; repeat for each item")
	return configureHavenCommand(flags, "subtotal", cmd, o)
}

