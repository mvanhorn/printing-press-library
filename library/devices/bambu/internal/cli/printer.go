// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Bambu printer command family.

package cli

import (
	"github.com/mvanhorn/printing-press-library/library/devices/bambu/internal/bambu"
	"github.com/spf13/cobra"
)

func newNovelPrinterCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "printer",
		Short:       "printer subcommands: field-diff",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelPrinterFieldDiffCmd(flags))
	cmd.AddCommand(newBambuPrinterStatusCmd(flags))
	cmd.AddCommand(newBambuPrinterRawCmd(flags))
	cmd.AddCommand(newBambuPrinterWatchCmd(flags))
	cmd.AddCommand(newBambuDerivedCmd(flags, "printer", "temperatures", "Show current and target printer temperatures", func(snapshot bambu.Snapshot) any { return temperatureView(snapshot.Raw) }))
	cmd.AddCommand(newBambuDerivedCmd(flags, "printer", "fans", "Show available printer fan and airflow state", func(snapshot bambu.Snapshot) any { return fanView(snapshot.Raw) }))
	cmd.AddCommand(newBambuDerivedCmd(flags, "printer", "capabilities", "Show model-aware printer capabilities", func(snapshot bambu.Snapshot) any { return capabilityView(snapshot.Raw) }))
	cmd.AddCommand(newBambuPrinterHealthCmd(flags))
	cmd.AddCommand(newBambuDerivedCmd(flags, "printer", "services", "Show queue upload upgrade camera and timelapse flags", func(snapshot bambu.Snapshot) any { return serviceView(snapshot.Raw) }))
	return cmd
}
