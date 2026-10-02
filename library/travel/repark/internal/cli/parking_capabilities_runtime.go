// pp:data-source computed
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/repark"
	"github.com/spf13/cobra"
)

func newReparkCapabilitiesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "capabilities", Short: "Inspect public source support, request bounds and planning unknowns", Example: "  repark-pp-cli parking capabilities --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "parking capabilities")
		}
		if len(args) != 0 {
			return usageErr(fmt.Errorf("capabilities takes no positional arguments"))
		}
		return flags.printJSON(cmd, map[string]any{"provider": "三井のリパーク (Repark)", "source_url": repark.Origin + "/parking_user/time/", "transport": "public HTTP; no runtime browser or credentials", "commands": map[string]any{"search": "provider-resolved named place plus bounded markers", "nearby": "explicit coordinates or source lot anchor", "detail": "canonical REP lot facts", "compare": "2..5 distinct source lots", "quote": "source calculator only when offered; explicit bay and JST interval"}, "bounds": map[string]any{"body_bytes_per_response": repark.MaxBodyBytes, "per_request_timeout_seconds": 20, "whole_command_timeout_seconds": 60, "automatic_retries": 0, "radius_m": []int{50, 2000}, "output_lots": []int{1, 50}, "scan_records": []int{1, 1000}, "request_budgets": map[string]int{"search": 3, "nearby_coordinates": 1, "nearby_lot": 2, "detail": 1, "compare": 5, "quote": 3}}, "unknowns": []string{"No guaranteed coverage or live bay inventory", "Available/crowded may reflect only compact or size-restricted remaining bays", "Published vehicle limits may differ by bay; remaining-bay fit unknown", "Provider import timestamp semantics and timezone unverified", "Tax inclusion unknown unless stated by source; no added tax", "No local pricing arithmetic; source quote excludes discounts and may differ from final charge"}, "location_policy": "Search anchor is supplied explicitly or resolved from the supplied place/lot; driver location is never inferred"})
	}}
	return cmd
}
