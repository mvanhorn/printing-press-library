// pp:data-source live

package cli

import (
	"context"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/navitime"
	"github.com/spf13/cobra"
)

func newNovelRoutesSearchCmd(flags *rootFlags) *cobra.Command {
	var query routeQueryFlags
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Return dated timetable alternatives with explicit fare assumptions.",
		Long:  "Find bounded route summaries between required --from and --to references with exactly one of --depart-at, --arrive-by, --first-on or --last-on. Use routes show for stored legs and fare groups; --pass preserves published fares and source coverage caveats.",
		Example: strings.Trim(`
  navitime-pp-cli routes search --from station:00006668 --to station:00001756 --depart-at 2026-10-01T09:00
  navitime-pp-cli routes search --from station:00006668 --to station:00001756 --arrive-by 2026-10-01T12:00 --pass japan_rail_pass
  navitime-pp-cli routes search --from station:00006668 --to station:00001756 --first-on 2026-10-01
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "--from=station:00006668;--to=station:00001756;--depart-at=2026-10-01T09:00",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return navitimeDryRun(cmd, flags, "routes search")
			}
			if navitimeHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if err := query.validate(cmd, args, flags); err != nil {
				return err
			}
			return runNavitime(cmd, flags, query.refresh, func(ctx context.Context, c navitimeService) (any, error) {
				// pp:client-call
				result, err := c.Routes(ctx, query.query)
				if err != nil {
					return nil, err
				}
				return navitime.Summaries(result, query.limit), nil
			})
		},
	}
	cmd.Flags().StringVar(&query.query.From, "from", "", "Origin reference from places search, such as station:00006668")
	cmd.Flags().StringVar(&query.query.To, "to", "", "Destination reference from places search, such as station:00001756")
	cmd.Flags().StringVar(&query.query.DepartAt, "depart-at", "", "Departure date and time: YYYY-MM-DDTHH:mm in Asia/Tokyo, or RFC3339")
	cmd.Flags().StringVar(&query.query.ArriveBy, "arrive-by", "", "Arrival deadline with explicit date and time in Asia/Tokyo, or RFC3339")
	cmd.Flags().StringVar(&query.query.FirstOn, "first-on", "", "First scheduled journey on YYYY-MM-DD in Asia/Tokyo")
	cmd.Flags().StringVar(&query.query.LastOn, "last-on", "", "Last scheduled journey on YYYY-MM-DD in Asia/Tokyo")
	cmd.Flags().StringArrayVar(&query.passes, "pass", nil, "One advertised pass ID from passes list; published fares remain source fares")
	cmd.Flags().IntVar(&query.limit, "limit", 3, "Maximum returned alternatives, 1 to 10, bounded by this source response")
	cmd.Flags().BoolVar(&query.refresh, "refresh", false, "Fetch a fresh route response instead of reading its cache")
	return cmd
}
