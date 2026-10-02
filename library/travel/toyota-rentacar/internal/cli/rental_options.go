// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/toyota"
	"github.com/spf13/cobra"
)

func newNovelRentalOptionsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "options", Short: "Read live option,child-seat and waiver/NOC fees",
		Long:        "Fetch Toyota's public option and insurance policies. Fees carry tax,basis and class/season/stock conditions. These are policy rates,not a final booking total or an equipment-stock promise.",
		Example:     "  toyota-rentacar-pp-cli rental options --agent --select options.code,options.fee_jpy,options.basis,notes",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": ""},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stop, err := toyotaGuard(cmd, args, flags, "rental options"); stop {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			out, err := toyota.NewClient(flags.rateLimit).Options(ctx)
			if err != nil {
				return toyotaError(err)
			}
			return toyotaPrint(cmd, flags, out)
		}}
	return cmd
}
