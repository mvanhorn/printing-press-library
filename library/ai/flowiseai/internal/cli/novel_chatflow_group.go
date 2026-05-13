// Copyright 2026 daniel-larson. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newChatflowGroupCmd is the visible top-level "chatflow" parent for the
// novel features that compound on top of the hidden generated "chatflows"
// resource. Keeping it separate from the spec-generated parent makes the
// command tree easier to read and prevents regenerations from touching
// hand-built code.
func newChatflowGroupCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chatflow",
		Short: "Compound chatflow workflows: dependency graph, staleness audit",
		Long: `Compound chatflow workflows that operate on the locally-synced chatflows table.

` + "`chatflow deps`" + ` parses the cached flowData JSON of a chatflow and emits the
tools, assistants, variables, and document stores it references — without
needing a server round-trip. ` + "`chatflow stale`" + ` lists chatflows not updated
in N days against the local store.

Run ` + "`flowiseai-pp-cli sync`" + ` first to ensure the local cache is fresh.`,
	}
	cmd.AddCommand(newChatflowDepsCmd(flags))
	cmd.AddCommand(newChatflowStaleCmd(flags))
	return cmd
}
