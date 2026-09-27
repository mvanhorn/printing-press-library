// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"github.com/spf13/cobra"
)

func newNovelStayOfferCmd(f *rootFlags) *cobra.Command {
	s := &stayFlags{}
	cmd := &cobra.Command{Use: "offer <property-id> <room-id> <plan-id>", Short: "Inspect exact room-plan prices, full cancellation rules and verified nightly date echoes", Example: "  ikyu-pp-cli stay offer 00002889 10193741 11055986 --check-in 2026-10-18 --check-out 2026-10-19 --adults 2 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "property=00002889;room=10193741;plan=11055986;--check-in=2026-10-18;--check-out=2026-10-19;--adults=2"}}
	bindStayFlags(cmd, s, true, false, false)
	addFieldsAlias(cmd, f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		request := ikyu.OfferRequest{Stay: s.stay}
		if len(args) == 3 {
			request.PropertyID = args[0]
			request.RoomID = args[1]
			request.PlanID = args[2]
		}
		if dryRunOK(f) {
			return stayDryRun(cmd, f, request)
		}
		if e := stayArg(args, 3, "use stay offer <property-id> <room-id> <plan-id> --check-in YYYY-MM-DD --check-out YYYY-MM-DD"); e != nil {
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
		result, e := c.Offer(ctx, request)
		if e != nil {
			return staySourceError(e)
		}
		stats := c.Stats()
		return stayEmit(cmd, f, result, &stats)
	}
	return cmd
}
