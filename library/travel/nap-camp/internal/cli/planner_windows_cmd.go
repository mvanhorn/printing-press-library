// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/napcamp"
	"github.com/spf13/cobra"
)

func newNovelPlannerWindowsCmd(flags *rootFlags) *cobra.Command {
	var month string
	var nights, limit int
	cmd := &cobra.Command{Use: "windows <campsite-id> <plan-id>", Short: "Find consecutive candidate nights from source acceptance evidence, with vacancy unknown.", Example: "  " + "nap-camp-pp-cli planner windows 11007 20005062 --month 2026-10 --nights 2 --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "campsite=11007;plan=20005062;--month=2026-10;--nights=2;--limit=5"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		if e := napIDs(args, 2); e != nil {
			return e
		}
		if e := napMonth(month); e != nil {
			return e
		}
		if e := napLimit(limit, 50); e != nil {
			return e
		}
		if nights < 1 || nights > 14 {
			return usageErr(napWindowNightsError())
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		c, e := a.Campsite(cmd.Context(), args[0])
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		days, e := a.Calendar(cmd.Context(), args[0], args[1], month)
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		v, e := napcamp.Windows(days, month, nights, limit)
		if e != nil {
			return usageErr(e)
		}
		v["campsite_id"] = args[0]
		v["plan_id"] = args[1]
		v["canonical_url"] = c["canonical_url"].(string) + "/plans/" + args[1]
		v["source_url"] = napcamp.Origin + "/api/campsite/" + args[0] + "/plans/" + args[1] + "/reservation?month=" + month
		return flags.printJSON(cmd, v)
	})
	cmd.Long = "Use this command to find consecutive candidate nights starting in the requested JST month. Statuses 1/2 are source acceptance candidates; checkout is excluded. Following-month nights can be evaluated only when present in the source response. For vehicle and service requirements, use 'planner fit'. Starting-price sums are not complete quotes."
	cmd.Flags().StringVar(&month, "month", napNowMonth(), "JST start month YYYY-MM; source may cover the following month")
	cmd.Flags().IntVar(&nights, "nights", 1, "Required consecutive nights (1..14), excluding checkout")
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum returned candidate windows (1..50)")
	return cmd
}
