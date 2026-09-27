// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"github.com/spf13/cobra"
)

func newNovelStaySearchCmd(f *rootFlags) *cobra.Command {
	s := &stayFlags{}
	var destination string
	cmd := &cobra.Command{Use: "search [destination]", Short: "Search one source page for a dated per-room party with explicit filter coverage", Example: "  ikyu-pp-cli stay search tokyo --check-in 2026-11-17 --check-out 2026-11-18 --adults 2 --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "destination=tokyo;--check-in=2026-11-17;--check-out=2026-11-18;--adults=2;--limit=3"}}
	bindStayFlags(cmd, s, true, true, true)
	cmd.Flags().StringVar(&destination, "destination", "", "Exact source destination name, ID/path or supported alias")
	cmd.Flags().IntVar(&s.scanPages, "max-scan-pages", 1, "Source scan-page cap (v1 reads exactly one bounded page)")
	addFieldsAlias(cmd, f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		request := ikyu.SearchRequest{Stay: s.stay, Limit: s.limit, Offset: s.offset, Preferences: s.preferences(cmd)}
		request.Destination = destination
		if len(args) > 1 || len(args) == 1 && cmd.Flags().Changed("destination") {
			return usageErr(fmt.Errorf("supply exactly one positional destination or --destination"))
		}
		if len(args) == 1 {
			request.Destination = args[0]
		}
		if dryRunOK(f) {
			return stayDryRun(cmd, f, request)
		}
		if request.Destination == "" {
			return usageErr(fmt.Errorf("use stay search --destination tokyo --check-in YYYY-MM-DD --check-out YYYY-MM-DD"))
		}
		if s.scanPages != 1 {
			return usageErr(fmt.Errorf("v1 supports --max-scan-pages=1; use --offset for subsequent source pages"))
		}
		if e := stayValidate(s.stay); e != nil {
			return e
		}
		ctx, cancel := stayContext(cmd, f)
		defer cancel()
		c, e := stayClient(cmd, f, s)
		if e != nil {
			return e
		}
		result, e := c.Search(ctx, request)
		if e != nil {
			return staySourceError(e)
		}
		stats := c.Stats()
		return stayEmit(cmd, f, result, &stats)
	}
	return cmd
}
