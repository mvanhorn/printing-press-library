// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// The `credits` command: gratitude to the people behind disaster sheltering and
// the plain unofficial-tool / safety disclaimer. The same two constants are
// reused as the `brief --markdown` footer.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// sheltersGratitude thanks the people who staff and run shelters. Wording per
// the user request and build-decisions.md.
const sheltersGratitude = "Thank you to all first responders, emergency management practitioners, and " +
	"relief nonprofit organizations for the work you do in communities when disaster strikes."

// sheltersDisclaimer is the unofficial-tool + safety disclaimer that accompanies
// the gratitude note everywhere it appears.
const sheltersDisclaimer = "This is an unofficial tool. FEMA's National Shelter System and your local " +
	"emergency management are the authoritative sources. In a life-threatening emergency call 911 and " +
	"follow official guidance and evacuation orders. Shelter status updates roughly twice a day and may lag reality."

func newNovelCreditsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "credits",
		Short:       "Thank the people behind disaster sheltering, and state that this is an unofficial tool",
		Long:        "Thanks first responders, emergency management practitioners, and relief nonprofits, and states plainly that this is an unofficial tool and FEMA / local emergency management is authoritative.",
		Example:     "  shelters-pp-cli credits",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Prose only for an interactive terminal; piped / --json / --agent /
			// --quiet consumers get the machine path (pipe-default contract).
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return emitData(cmd, flags, map[string]any{
					"gratitude":  sheltersGratitude,
					"disclaimer": sheltersDisclaimer,
				})
			}
			var b strings.Builder
			fmt.Fprintln(&b, sheltersGratitude)
			fmt.Fprintln(&b)
			fmt.Fprintln(&b, sheltersDisclaimer)
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
	return cmd
}
