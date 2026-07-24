// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func asFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case json.Number:
		result, err := v.Float64()
		return result, err == nil
	default:
		return 0, false
	}
}

func rewardMatches(candidate map[string]any, wanted string) bool {
	for _, key := range []string{"_id", "id", "text", "key"} {
		if value, ok := candidate[key].(string); ok && strings.EqualFold(strings.TrimSpace(value), wanted) {
			return true
		}
	}
	return false
}

func newNovelRewardAffordCmd(flags *rootFlags) *cobra.Command {
	var flagReserveGP string
	cmd := &cobra.Command{
		Use:         "afford <reward-or-item>",
		Short:       "Check whether a reward fits current gold while preserving a chosen reserve for later goals.",
		Example:     "  habitica-pp-cli reward afford 'Weekend movie' --reserve-gp 20 --agent",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			reserve, err := strconv.ParseFloat(flagReserveGP, 64)
			if err != nil || reserve < 0 {
				return usageErr(errors.New("--reserve-gp must be a non-negative number"))
			}
			if dryRunOK(flags) {
				return flags.printJSON(cmd, map[string]any{"reward": args[0], "reserve_gp": reserve, "affordable": false, "action": "would compare current gold with catalog cost"})
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			headers, err := habiticaHeaders()
			if err != nil {
				return err
			}
			userRaw, err := c.GetWithHeaders(ctx, "/user", nil, headers)
			if err != nil {
				return fmt.Errorf("fetching gold balance: %w", err)
			}
			rewardsRaw, err := c.GetWithHeaders(ctx, "/tasks/user", map[string]string{"type": "rewards"}, headers)
			if err != nil {
				return fmt.Errorf("fetching custom rewards: %w", err)
			}
			userData, err := habiticaData(userRaw)
			if err != nil {
				return err
			}
			rewardsData, err := habiticaData(rewardsRaw)
			if err != nil {
				return err
			}
			var user map[string]any
			var rewards []map[string]any
			if err := json.Unmarshal(userData, &user); err != nil {
				return fmt.Errorf("decoding gold balance: %w", err)
			}
			if err := json.Unmarshal(rewardsData, &rewards); err != nil {
				return fmt.Errorf("decoding custom rewards: %w", err)
			}
			stats, _ := user["stats"].(map[string]any)
			gp, ok := asFloat(stats["gp"])
			if !ok {
				return fmt.Errorf("Habitica user response did not include stats.gp")
			}
			var selected map[string]any
			for _, reward := range rewards {
				if rewardMatches(reward, args[0]) {
					selected = reward
					break
				}
			}
			if selected == nil {
				return fmt.Errorf("no custom reward matches %q; built-in shop items are not price-stable in this command", args[0])
			}
			cost, ok := asFloat(selected["value"])
			if !ok {
				return fmt.Errorf("matched reward %q did not include a numeric value", args[0])
			}
			return flags.printJSON(cmd, map[string]any{"reward": selected, "gp": gp, "reserve_gp": reserve, "cost": cost, "remaining_gp": gp - cost, "affordable": gp-cost >= reserve})
		},
	}
	cmd.Flags().StringVar(&flagReserveGP, "reserve-gp", "0", "Gold to keep after buying the reward")
	return cmd
}
