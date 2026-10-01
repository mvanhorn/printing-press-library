// Provider-specific domain parser replaces the generated HTML text extraction.
package cli

import (
	"github.com/spf13/cobra"
)

// pp:data-source auto
func newBathShowCmd(f *rootFlags) *cobra.Command {
	var id string
	cmd := &cobra.Command{Use: "show [id]", Short: "Inspect admission, hours, access and explicit bath/policy evidence", Example: "  nifty-onsen-pp-cli bath show --id=onsen012278 --agent\n  nifty-onsen-pp-cli bath show onsen012278 --select=name,admission,policies --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "--id=onsen012278"}, RunE: func(cmd *cobra.Command, args []string) error {
		if dryRunOK(f) {
			return writeDryRun(cmd.OutOrStdout(), f, "GET public Nifty facility detail")
		}
		id, e := onsenInput(args, id)
		if e != nil {
			return e
		}
		c, e := onsenClient(f)
		if e != nil {
			return e
		}
		ctx, cancel := boundCtx(cmd.Context(), f)
		defer cancel()
		v, p, e := c.Detail(ctx, id)
		if e != nil {
			return e
		}
		return onsenOutput(cmd, f, v, p, map[string]any{"scope": "facility semantic fields", "exhaustive": false, "note": "Raw JPY admission preserves weekdays, holidays, ages, extras and fee basis. Source labels are claims; unknown policies and rentable inventory require direct confirmation. Reviews/FAQ prose are excluded."})
	}}
	cmd.Flags().StringVar(&id, "id", "", "Stable Nifty facility ID, for example onsen012278")
	return cmd
}
