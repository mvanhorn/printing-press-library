// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"github.com/spf13/cobra"
)

func newNovelStayRoomsCmd(f *rootFlags) *cobra.Command {
	s := &stayFlags{}
	var planLimit, planOffset int
	cmd := &cobra.Command{Use: "rooms [property-id]", Short: "Page rooms and plan summaries; preserve source bath evidence and missing nightly dates", Example: "  ikyu-pp-cli stay rooms 00002889 --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --limit 3 --plan-limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "property=00002889;--check-in=2026-11-17;--check-out=2026-11-18;--adults=2;--limit=3"}}
	bindStayFlags(cmd, s, true, true, true)
	cmd.Flags().IntVar(&planLimit, "plan-limit", 5, "Maximum plan summaries per room, between 1 and 50")
	cmd.Flags().IntVar(&planOffset, "plan-offset", 0, "Source plan offset within each returned room")
	addFieldsAlias(cmd, f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		request := ikyu.RoomsRequest{Stay: s.stay, Limit: s.limit, Offset: s.offset, PlanLimit: planLimit, PlanOffset: planOffset, Preferences: s.preferences(cmd)}
		if len(args) == 1 {
			request.PropertyID = args[0]
		}
		if dryRunOK(f) {
			return stayDryRun(cmd, f, request)
		}
		if e := stayArg(args, 1, "use stay rooms <property-id> --check-in YYYY-MM-DD --check-out YYYY-MM-DD"); e != nil {
			return e
		}
		if e := stayValidate(s.stay); e != nil {
			return e
		}
		ctx, cancel := stayContext(cmd, f)
		defer cancel()
		c, e := stayClient(cmd, f, s)
		if e != nil {
			return e
		}
		result, e := c.Rooms(ctx, request)
		if e != nil {
			return staySourceError(e)
		}
		stats := c.Stats()
		return stayEmit(cmd, f, result, &stats)
	}
	return cmd
}
