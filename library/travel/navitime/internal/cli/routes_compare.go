// pp:data-source computed

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/navitime"
	"github.com/spf13/cobra"
)

func newNovelRoutesCompareCmd(flags *rootFlags) *cobra.Command {
	var query routeQueryFlags
	options := navitime.CompareOptions{MaxDurationMinutes: -1, MaxFareJPY: -1, MaxWalkMeters: -1, MaxTransfers: -1}
	var sort string
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Rank this query's returned alternatives, keeping unknown metrics explicit.",
		Long:  "Rank the alternatives for required --from/--to and one dated time mode using --sort and optional duration, fare, walking or transfer caps. Returns only this query's alternatives; unknown metrics cannot satisfy caps.",
		Example: strings.Trim(`
  navitime-pp-cli routes compare --from station:00006668 --to station:00001756 --arrive-by 2026-10-01T12:00 --sort fare
  navitime-pp-cli routes compare --from station:00006668 --to station:00001756 --depart-at 2026-10-01T09:00 --sort walking --max-transfers 1
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "computed",
			"pp:happy-args":  "--from=station:00006668;--to=station:00001756;--arrive-by=2026-10-01T12:00;--sort=fare",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return navitimeDryRun(cmd, flags, "routes compare")
			}
			if navitimeHelpOnly(cmd, args) {
				return cmd.Help()
			}
			if err := query.validate(cmd, args, flags); err != nil {
				return err
			}
			switch sort {
			case "duration", "fare", "transfers":
				options.Sort = sort
			case "walking":
				options.Sort = "walk"
			default:
				return usageErr(fmt.Errorf("--sort must be duration, fare, walking or transfers"))
			}
			for _, cap := range []struct {
				name  string
				value int
			}{{"max-duration", options.MaxDurationMinutes}, {"max-fare", options.MaxFareJPY}, {"max-walk", options.MaxWalkMeters}, {"max-transfers", options.MaxTransfers}} {
				if cmd.Flags().Changed(cap.name) && cap.value < 0 {
					return usageErr(fmt.Errorf("--%s must be zero or greater", cap.name))
				}
			}
			return runNavitime(cmd, flags, query.refresh, func(ctx context.Context, c navitimeService) (any, error) {
				// pp:client-call
				result, err := c.Routes(ctx, query.query)
				if err != nil {
					return nil, err
				}
				compared, err := navitime.CompareRoutes(result, options)
				if err != nil {
					return nil, err
				}
				// Limit after local filters and ranking, never before them.
				view, ok := compared.(navitime.CompareResult)
				if !ok {
					return nil, fmt.Errorf("route comparison returned an unsupported result shape")
				}
				view.Sort = sort
				if len(view.Routes) > query.limit {
					view.Routes = view.Routes[:query.limit]
				}
				return view, nil
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
	cmd.Flags().StringVar(&sort, "sort", "duration", "Metric to rank ascending: duration, fare, walking or transfers")
	cmd.Flags().IntVar(&options.MaxDurationMinutes, "max-duration", -1, "Local maximum duration in minutes; unknown durations do not match")
	cmd.Flags().IntVar(&options.MaxFareJPY, "max-fare", -1, "Local maximum displayed source fare in JPY; unknown fares do not match")
	cmd.Flags().IntVar(&options.MaxWalkMeters, "max-walk", -1, "Local maximum walking distance in metres; unknown distances do not match")
	cmd.Flags().IntVar(&options.MaxTransfers, "max-transfers", -1, "Local maximum transfer count; unknown counts do not match")
	return cmd
}
