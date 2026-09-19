// pp:data-source local
// Hand-authored Haven command; shared implementation lives in haven_commands.go.
package cli

import "github.com/spf13/cobra"

func newNovelHavenNearbyCmd(flags *rootFlags) *cobra.Command {
	o := &havenOptions{}
	cmd := &cobra.Command{Use: "nearby", Short: "Sort saved Haven locations by straight-line distance", Example: "  haven-hot-chicken-pp-cli haven nearby --lat 41.3 --lon -72.9 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--lat=41.3;--lon=-72.9"}}
	cmd.Flags().StringVar(&o.db, "db", "", "Path to the local SQLite observation database")
	cmd.Flags().IntVar(&o.limit, "limit", 50, "Maximum output rows, between 1 and 1000")
	cmd.Flags().Float64Var(&o.lat, "lat", 0, "Latitude in degrees, between -90 and 90")
	cmd.Flags().Float64Var(&o.lon, "lon", 0, "Longitude in degrees, between -180 and 180")
	return configureHavenCommand(flags, "nearby", cmd, o)
}

