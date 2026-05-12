// Copyright 2026 mazzsterr. Licensed under Apache-2.0. See LICENSE.
// Hand-written transcendence helper. Not generated.

package cli

import (
	"github.com/spf13/cobra"
)

// DefaultModeThreshold is the batch size at or below which a request goes
// Live (synchronous, premium) and above which it goes Standard (queued, ~3.3x
// cheaper). Sourced from DataForSEO public price differential.
const DefaultModeThreshold = 5

// ResolveMode chooses "live" or "standard" execution. override values:
//   - ""        — fall through to threshold logic
//   - "auto"    — same as ""
//   - "live"    — force live, regardless of batch size
//   - "standard" — force standard
//
// Any other override is returned verbatim so future modes (e.g. "sandbox")
// pass through cleanly.
func ResolveMode(batchSize, threshold int, override string) string {
	switch override {
	case "", "auto":
		// fall through
	case "live", "standard":
		return override
	default:
		return override
	}
	if threshold <= 0 {
		threshold = DefaultModeThreshold
	}
	if batchSize <= threshold {
		return "live"
	}
	return "standard"
}

// WireAutoMode adds --auto-mode and --mode-threshold flags to a command so it
// can opt into ResolveMode. Hand-written transcendence commands that need
// these flags call WireAutoMode in their constructor and pass the resulting
// flag pointers to ResolveMode.
//
// Generated endpoint commands do NOT yet call this — wiring them is a
// follow-up that touches ~50 files and lands separately.
func WireAutoMode(cmd *cobra.Command, override *string, threshold *int) {
	if override != nil {
		cmd.Flags().StringVar(override, "auto-mode", "auto", `Mode router: auto|live|standard. "auto" picks live when batch <= --mode-threshold.`)
	}
	if threshold != nil {
		cmd.Flags().IntVar(threshold, "mode-threshold", DefaultModeThreshold, "Batch size at or below which auto-mode picks Live")
	}
}
