// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelRequestsPlanCmd(flags *rootFlags) *cobra.Command {
	var platforms, operation string
	cmd := &cobra.Command{Use: "plan", Short: "Calculate fan-out credit cost and affordability before making paid platform requests.", Example: "  quickcommerce-pp-cli requests plan --platforms blinkit,zepto --operation search --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,2"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "requests plan")
		}
		if strings.TrimSpace(platforms) == "" {
			_ = cmd.Usage()
			return usageErr(fmt.Errorf("--platforms is required; use comma-separated platform names"))
		}
		operation = strings.ToLower(strings.TrimSpace(operation))
		if operation != "search" && operation != "item" && operation != "eta" {
			return usageErr(fmt.Errorf("--operation must be search, item, or eta"))
		}
		seen := map[string]bool{}
		names := make([]string, 0)
		for _, raw := range strings.Split(platforms, ",") {
			name := strings.TrimSpace(raw)
			if name != "" && !seen[strings.ToLower(name)] {
				seen[strings.ToLower(name)] = true
				names = append(names, name)
			}
		}
		if len(names) == 0 {
			return usageErr(fmt.Errorf("--platforms must contain at least one platform"))
		}
		sort.Strings(names)
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		raw, err := c.Get(ctx, "/v1/credits", nil)
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		var credit struct {
			Summary struct {
				TotalAvailable int `json:"total_available"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(raw, &credit); err != nil {
			return apiErr(fmt.Errorf("decoding credit balance: %w", err))
		}
		cost := len(names)
		view := map[string]any{"operation": operation, "platforms": names, "credit_cost": cost, "credits_available": credit.Summary.TotalAvailable, "credits_after": credit.Summary.TotalAvailable - cost, "affordable": credit.Summary.TotalAvailable >= cost, "next": "run the corresponding command only when affordable"}
		if credit.Summary.TotalAvailable < cost {
			view["note"] = fmt.Sprintf("short by %d credit(s); reduce --platforms or add credits", cost-credit.Summary.TotalAvailable)
		}
		return qcPrint(cmd.OutOrStdout(), flags, view, []map[string]any{view})
	}}
	cmd.Flags().StringVar(&platforms, "platforms", "", "Comma-separated platforms to query")
	cmd.Flags().StringVar(&operation, "operation", "", "Paid operation: search, item, or eta")
	return cmd
}
