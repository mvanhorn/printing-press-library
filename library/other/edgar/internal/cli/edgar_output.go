// Copyright 2026 magoo242. Licensed under Apache-2.0. See LICENSE.

// Shared output pipeline for hand-built EDGAR commands. Routes typed Go
// values through the same --json/--compact/--select machinery as the
// generated commands.

package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// emitJSON is the standard exit point for hand-built EDGAR commands. It
// honors --json (default true when piped), --compact, --select, --csv,
// --quiet, and --plain by delegating to printOutputWithFlags.
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	// Default to JSON shape for hand-built commands; they have nothing useful
	// as a table without specific projection.
	asJSON := flags.asJSON
	if !asJSON && !isTerminal(cmd.OutOrStdout()) {
		asJSON = true
	}
	if !asJSON && !flags.csv && !flags.quiet && !flags.plain && !flags.compact && flags.selectFields == "" {
		// human terminal path — pretty JSON for readability
		asJSON = true
	}
	tmp := *flags
	tmp.asJSON = asJSON
	return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage(raw), &tmp)
}

// edgarDBPath returns the standard SQLite path for the EDGAR store.
func edgarDBPath() string {
	return defaultDBPath("edgar-pp-cli")
}
