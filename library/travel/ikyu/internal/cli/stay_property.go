// pp:data-source live
package cli

import (
	"github.com/spf13/cobra"
)

func newStayPropertyCmd(f *rootFlags) *cobra.Command {
	s := &stayFlags{}
	cmd := &cobra.Command{Use: "property [property-id]", Short: "Read property facilities, source review category scores and tax notes", Example: "  ikyu-pp-cli stay property 00002889 --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "property=00002889"}}
	bindStayFlags(cmd, s, false, false, false)
	addFieldsAlias(cmd, f)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return stayDryRun(cmd, f, map[string]any{"operation": "StayProperty", "property_id": args})
		}
		if e := stayArg(args, 1, "use stay property <eight-digit-property-id>"); e != nil {
			return e
		}
		ctx, cancel := stayContext(cmd, f)
		defer cancel()
		c, e := stayClient(cmd, f, s)
		if e != nil {
			return e
		}
		result, e := c.Property(ctx, args[0])
		if e != nil {
			return staySourceError(e)
		}
		stats := c.Stats()
		return stayEmit(cmd, f, result, &stats)
	}
	return cmd
}
