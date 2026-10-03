// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/napcamp"
	"github.com/spf13/cobra"
	"strings"
)

func newNovelPlannerCompareCmd(flags *rootFlags) *cobra.Command {
	var r napcamp.Requirements
	cmd := &cobra.Command{Use: "compare <pair> [<pair>...]", Short: "Compare a bounded shortlist of specific pitches with consistent evidence checks.", Example: "  " + "nap-camp-pp-cli planner compare 11007:20005062 11007:20005063 --people 2 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "pair=11007:20005062;pair=11007:20005063;--people=2"}}
	napBind(flags, cmd, "live", func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 || len(args) > 5 {
			return usageErr(fmt.Errorf("compare requires 1..5 campsite:plan pairs"))
		}
		if e := napRequirementError(r); e != nil {
			return e
		}
		seen := map[string]bool{}
		for _, p := range args {
			xs := strings.Split(p, ":")
			if len(xs) != 2 {
				return usageErr(fmt.Errorf("pair %q must be campsite:plan", p))
			}
			if e := napIDs(xs, 2); e != nil {
				return e
			}
			if seen[p] {
				return usageErr(fmt.Errorf("duplicate pair %q", p))
			}
			seen[p] = true
		}
		a, e := napAPI(flags)
		if e != nil {
			return e
		}
		values, failures := cliutil.FanoutRun(cmd.Context(), args, func(p string) string { return p }, func(ctx context.Context, p string) (napcamp.Object, error) {
			xs := strings.Split(p, ":")
			s, e := a.Snapshot(ctx, xs[0], xs[1], "")
			if e != nil {
				return nil, e
			}
			return napcamp.Fit(s.Campsite, s.Pitch, r), nil
		}, cliutil.WithConcurrency(2))
		rows := make([]napcamp.Object, 0)
		errs := make([]napcamp.Object, 0)
		for _, v := range values {
			rows = append(rows, napcamp.Object{"pair": v.Source, "evidence": v.Value})
		}
		for _, v := range failures {
			errs = append(errs, napcamp.Object{"pair": v.Source, "error": v.Err.Error()})
		}
		if e := flags.printJSON(cmd, napcamp.Object{"results": rows, "errors": errs, "observed_at": napcamp.Now(), "requested_pairs": len(args), "complete": len(failures) == 0, "qualification": "Consistent evidence comparison; no subjective ranking, clearance certification or availability guarantee."}); e != nil {
			return e
		}
		if len(failures) > 0 {
			return classifyAPIErrorOnly(failures[0].Err)
		}
		return nil
	})
	cmd.Long = "Use this command to compare source evidence for 1..5 selected pitches. For consecutive dated acceptance candidates, use 'planner windows'. Each pair has explicit provenance and unknowns. Partial fetch failures are emitted and exit nonzero."
	napRequirements(cmd, &r)
	return cmd
}
