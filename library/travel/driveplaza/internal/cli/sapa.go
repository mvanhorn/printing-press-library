package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSapaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "sapa", Short: "Show usage for directional SA/PA list, detail and facility-selector subcommands", Long: "Use list for road-specific summaries, detail for facility sections and weekday hours, and facilities for source filter IDs. Up/down are provider direction identities. Availability and weekday hours do not establish that a facility is open now.", Annotations: dpAnnotations("live", "")}
	cmd.Example = "  driveplaza-pp-cli sapa list --road 1040 --query hasuda --direction up --limit 1 --agent\n  driveplaza-pp-cli sapa detail --id 1040/1040021/1 --agent"
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, cmd.CommandPath())
		}
		if len(args) > 0 {
			return usageErr(fmt.Errorf("unknown sapa subcommand; use list, detail or facilities"))
		}
		return cmd.Help()
	}
	cmd.AddCommand(dpStops(flags), dpDetail(flags), dpFacilities(flags))
	return cmd
}
