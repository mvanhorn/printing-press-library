// pp:data-source live
package cli

import (
	"github.com/spf13/cobra"
)

func newNovelStayCmd(f *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "stay", Short: "Find Japanese hotels and ryokan and inspect exact room-plan conditions", Example: "  ikyu-pp-cli stay destinations tokyo --limit 5\n  ikyu-pp-cli stay search --destination tokyo --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --limit 3", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}, RunE: parentNoSubcommandRunE(f)}
	cmd.AddCommand(newStayDestinationsCmd(f), newStayPropertyCmd(f), newNovelStaySearchCmd(f), newNovelStayRoomsCmd(f), newNovelStayOfferCmd(f), newNovelStayCompareCmd(f), newNovelStayDatesCmd(f))
	return cmd
}
