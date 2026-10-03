// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/napcamp"
	"github.com/spf13/cobra"
)

func newNovelPlannerFitCmd(flags *rootFlags) *cobra.Command {
	var r napcamp.Requirements
	cmd := &cobra.Command{Use: "fit <campsite-id> <plan-id>", Short: "Check specific-pitch requirements with verified, contradictory and unknown source evidence.", Example: "  " + "nap-camp-pp-cli planner fit 11007 20005062 --people 2 --length-m 6 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "campsite=11007;plan=20005062;--people=2;--length-m=6"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		if e := napIDs(args, 2); e != nil {
			return e
		}
		if e := napRequirementError(r); e != nil {
			return e
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		s, e := a.Snapshot(cmd.Context(), args[0], args[1], "")
		if e != nil {
			return classifyAPIErrorOnly(e)
		}
		return flags.printJSON(cmd, napcamp.Fit(s.Campsite, s.Pitch, r))
	})
	cmd.Long = "Use this command to check requirements for one selected pitch. For choosing among several pitches, use 'planner compare'. Vehicle clearance, unreported rules, season confirmation and full dated quote stay unknown."
	napRequirements(cmd, &r)
	return cmd
}
