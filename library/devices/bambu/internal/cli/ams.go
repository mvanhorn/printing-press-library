// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Bambu AMS command family.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/spf13/cobra"
)

func newNovelAmsCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "ams",
		Short:       "ams subcommands: runway",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelAmsRunwayCmd(flags))
	cmd.AddCommand(newBambuDerivedCmd(flags, "ams", "status", "Show AMS units trays materials colors RFID and environment", func(snapshot bambu.Snapshot) any { return amsView(snapshot.Raw) }))
	cmd.AddCommand(newBambuDerivedCmd(flags, "ams", "active", "Resolve the active AMS tray or external spool", func(snapshot bambu.Snapshot) any { return activeAMSView(snapshot.Raw) }))
	cmd.AddCommand(newBambuDerivedCmd(flags, "ams", "services", "Show read-only AMS drying and filament-backup state", func(snapshot bambu.Snapshot) any { return amsServiceView(snapshot.Raw) }))
	return cmd
}
