// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelRoutesShowCmd(flags *rootFlags) *cobra.Command {
	var latest bool
	cmd := &cobra.Command{
		Use:   "show [ROUTE_ID]",
		Short: "Expand a stored route snapshot without another HTTP request.",
		Long:  "Read a stored route ID or --latest without another website request. Returns dated legs, fare groups and original freshness; fetch routes search first if no snapshot exists. The literal latest is an alias for --latest; both return route:null with a note when the snapshot cache is empty.",
		Example: strings.Trim(`
  navitime-pp-cli routes show latest
  navitime-pp-cli routes show --latest --select route.legs,route.fare_groups,meta.fetched_at
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "route_id=latest"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return navitimeDryRun(cmd, flags, "routes show")
			}
			if navitimeHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("routes show reads stored snapshots only; use --data-source auto or local"))
			}
			if len(args) > 1 || (latest && len(args) != 0) || (!latest && len(args) != 1) {
				return usageErr(fmt.Errorf("routes show requires one ROUTE_ID returned by routes search, or --latest"))
			}
			id := "latest"
			if !latest {
				id = strings.TrimSpace(args[0])
				if id == "" {
					return usageErr(fmt.Errorf("ROUTE_ID must not be empty; use a routes search snapshot ID"))
				}
			}
			return runNavitime(cmd, flags, false, func(ctx context.Context, c navitimeService) (any, error) {
				// pp:client-call
				result, err := c.Show(ctx, id)
				return result, err
			})
		},
	}
	cmd.Flags().BoolVar(&latest, "latest", false, "Read the latest stored route; an empty snapshot cache returns route:null")
	return cmd
}
