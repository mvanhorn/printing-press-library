// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "github.com/spf13/cobra"

// Registration for the hand-authored panel commands.
//
// This lives in its own file, and uses the novel-command hook rather than
// editing root.go, so that `generate --force` preserves it. Editing root.go
// directly is the failure mode that loses command wiring on the next
// regeneration.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newPBSReleasesCmd(flags))
		addNovelCommandIfAbsent(root, newPBSSyncCmd(flags))
	})
}
