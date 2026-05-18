// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"
)

// newSegmentsCmd is a thin parent for novel segment-management commands.
// Segments themselves live under /lists/{list_id}/segments in the API; this
// parent groups novel CLI verbs that span segments without forcing the user
// to descend into the audience tree.
func newSegmentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "segments",
		Short: "Segment hygiene and analysis (audit empty/stale segments).",
		Long: `Segments are owned by audiences (lists) in the API, but novel segment-management
commands span segments and don't fit cleanly under a single audience. They
live here.

For raw CRUD on a single audience's segments, see:
  mailchimp-pp-cli lists segments-list ...`,
	}
	cmd.AddCommand(newSegmentsAuditCmd(flags))
	return cmd
}
