// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

func writeJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	if !flags.agent && !flags.compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

func clampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
