package cli

import "github.com/spf13/cobra"

// Registers the hand-written NCCPL commands.
//
// Kept in its own file (rather than edited into root.go) so `generate --force`
// preserves the wiring instead of dropping it on the next reprint.

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNCCPLSyncCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLCoverageCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLVerifyCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLPanelCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLUniverseCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLLeverageCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLRiskChangesCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLContractCheckCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLFlowsCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLIngestCmd(flags))
		addNovelCommandIfAbsent(root, newNCCPLCaptureCmd(flags))
	})
}
