// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source auto
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelHostelsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "hostels",
		Short:       "Work with hostels",
		Example:     "  hostelworld-pp-cli hostels compare 67481 15725 --check-in 2026-11-10 --check-out 2026-11-13 --guests 2 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelHostelsCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHostelsDatesCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHostelsOffersCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHostelsSavedCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHostelsInspectCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelHostelsSearchCmd(flags))
	return cmd
}
