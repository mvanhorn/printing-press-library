// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"fmt"
	hw "github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/hostelworld"
	"github.com/spf13/cobra"
	"sort"
	"time"
)

func newNovelHostelsCompareCmd(flags *rootFlags) *cobra.Command {
	var o stayOptions
	cmd := &cobra.Command{Use: "compare <id> <id2> [id3] [id4] [id5]", Short: "Compare bounded dated bed and room plans using derived party estimates", Example: "  " + "hostelworld-pp-cli hostels compare 67481 15725 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent", Annotations: map[string]string{"mcp:read-only": "false", "mcp:local-write": "true", "pp:live-happy-path": "true", "pp:data-source": "live", "pp:happy-args": "id=67481;id2=15725;--check-in=2026-10-04;--check-out=2026-10-07;--guests=2"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 || len(args) > 5 {
			return usageErr(fmt.Errorf("compare requires 2–5 distinct numeric property IDs"))
		}
		seen := map[string]bool{}
		for _, id := range args {
			if !hw.ValidID(id) || seen[id] {
				return usageErr(fmt.Errorf("compare requires distinct valid numeric property IDs"))
			}
			seen[id] = true
		}
		q, err := o.query()
		if err != nil {
			return usageErr(err)
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		out := []any{}
		failures := []any{}
		success := 0
		properties := []any{}
		for _, id := range args {
			p, err := propertyObject(ctx, c, id)
			var v map[string]any
			if err == nil {
				v, err = offersObject(ctx, c, id, p, q, o)
			}
			if err != nil {
				if fatalRate(err) {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				failures = append(failures, map[string]any{"property_id": id, "error": err.Error()})
				continue
			}
			success++
			properties = append(properties, map[string]any{"property_id": id, "observed_at": v["observed_at"], "status": v["status"], "booking_url": v["booking_url"], "property_rules": v["property_rules"], "tax_policy": v["tax_policy"], "cancellation_policies": v["cancellation_policies"], "special_event_conditions": v["special_event_conditions"]})
			for _, row := range v["offers"].([]any) {
				r := row.(map[string]any)
				r["property_id"] = id
				r["property_name"] = v["property_name"]
				out = append(out, r)
			}
		}
		if len(failures) > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d of %d properties failed; comparison covers %d successful properties\n", len(failures), len(args), success)
		}
		if success == 0 {
			return fmt.Errorf("all requested properties failed; no comparison available")
		}
		sort.SliceStable(out, func(i, j int) bool { return hw.CompareAmounts(out[i].(map[string]any), out[j].(map[string]any)) })
		scanned := len(out)
		if len(out) > o.limit {
			out = out[:o.limit]
		}
		return outputPlanning(cmd, flags, map[string]any{"query": q, "results": out, "properties": properties, "fetch_failures": failures, "successful_properties": success, "requested_properties": len(args), "scanned_plans": scanned, "truncated": scanned > len(out), "comparison_basis": "derived whole-party stay estimates; currencies grouped separately; unknown/restricted inventory is unranked", "observed_at": time.Now().UTC().Format(time.RFC3339)}, o.save)
	}}
	decoratePlanning(cmd, flags)
	stayFlags(cmd, &o)
	return cmd
}
