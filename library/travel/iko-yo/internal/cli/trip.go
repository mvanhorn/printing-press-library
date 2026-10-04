// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"github.com/spf13/cobra"
)

func newNovelTripCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "trip",
		Short:       "Plan with Iko-yo Trip published family experiences and local events",
		Example:     "  iko-yo-pp-cli trip cached Mooovi --kind spots --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	addNovelCommandIfAbsent(cmd, newNovelTripCachedCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelTripCompareCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelTripDiscoverCmd(flags))
	addNovelCommandIfAbsent(cmd, newNovelTripInspectCmd(flags))
	return cmd
}
