// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: IMPLEMENTED. The body lives in the hand-authored
// internal/cli/nepra_disco.go (+ nepra_disco_panel.go), which also registers
// itself through registerNovelCommand — so if `generate --force` ever refreshes
// THIS file back into a TODO scaffold, addNovelCommandIfAbsent still prefers the
// implementation over the scaffold and the command keeps working.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. `live` is deliberate:
// PER PDFs are never synced into the SQLite store, so there is no local panel
// to serve and --data-source local is refused rather than answered emptily.

package cli

import "github.com/spf13/cobra"

// newNovelDiscoCmd is the name internal/cli/root.go wires, kept so that the
// existing registration needs no edit.
func newNovelDiscoCmd(flags *rootFlags) *cobra.Command {
	return newNepraDiscoCmd(flags)
}
