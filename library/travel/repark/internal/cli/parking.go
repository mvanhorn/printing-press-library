package cli

import "github.com/spf13/cobra"

func newNovelParkingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "parking", Short: "Discover and compare Repark parking using public source facts", Example: "  repark-pp-cli parking search 東京駅 --limit 3 --agent\n  repark-pp-cli parking detail REP0022209 --agent", RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newReparkSearchCmd(flags), newNovelParkingNearbyCmd(flags), newNovelParkingDetailCmd(flags), newNovelParkingCompareCmd(flags), newReparkQuoteCmd(flags), newNovelParkingCapabilitiesCmd(flags))
	return cmd
}
