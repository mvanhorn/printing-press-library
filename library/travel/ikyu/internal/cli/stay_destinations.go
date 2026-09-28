// pp:data-source live
package cli

import (
	"fmt"
	"github.com/spf13/cobra"
)

func newStayDestinationsCmd(f *rootFlags) *cobra.Command {
	s := &stayFlags{}
	cmd := &cobra.Command{Use: "destinations [query]", Short: "Resolve source destination names, IDs and public paths", Example: "  ikyu-pp-cli stay destinations tokyo --limit 5 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "query=tokyo;--limit=5"}}
	bindStayFlags(cmd, s, false, false, true)
	addFieldsAlias(cmd, f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		query := ""
		if len(args) > 1 {
			return usageErr(fmt.Errorf("use stay destinations [query]"))
		}
		if len(args) == 1 {
			query = args[0]
		}
		if dryRunOK(f) {
			return stayDryRun(cmd, f, map[string]any{"operation": "public destination links", "query": query, "limit": s.limit, "offset": s.offset})
		}
		ctx, cancel := stayContext(cmd, f)
		defer cancel()
		c, e := stayClient(cmd, f, s)
		if e != nil {
			return e
		}
		result, e := c.Destinations(ctx, query, s.limit, s.offset)
		if e != nil {
			return staySourceError(e)
		}
		stats := c.Stats()
		return stayEmit(cmd, f, result, &stats)
	}
	return cmd
}
