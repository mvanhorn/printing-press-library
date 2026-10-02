// pp:data-source live
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/toyota"
	"github.com/spf13/cobra"
)

func newNovelOnewayQuoteCmd(flags *rootFlags) *cobra.Command {
	var pickupID, dropoffID, family string
	cmd := &cobra.Command{Use: "quote", Short: "Run Toyota's date-independent one-way surcharge calculator",
		Long:        "Calculate the source surcharge for an exact shop pair and vehicle family. This is separate from cars quote and has no dated inventory guarantee. standard covers Standard/Special/Premium excluding LXC/LXP; wagon covers Wagon/SUV/Premium LXC/LXP. Buses and island-crossing restrictions apply.",
		Example:     "  toyota-rentacar-pp-cli oneway quote --pickup-shop 63601:01V --dropoff-shop 63601:095 --family standard --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--pickup-shop=63601:01V;--dropoff-shop=63601:095;--family=standard"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stop, err := toyotaGuard(cmd, args, flags, "oneway quote"); stop {
				return err
			}
			if pickupID == "" || dropoffID == "" {
				return usageErr(fmt.Errorf("--pickup-shop and --dropoff-shop are required; resolve IDs with shops search"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			out, err := toyota.NewClient(flags.rateLimit).OneWay(ctx, pickupID, dropoffID, family)
			if err != nil {
				return toyotaError(err)
			}
			return toyotaPrint(cmd, flags, out)
		}}
	cmd.Flags().StringVar(&pickupID, "pickup-shop", "", "Exact pickup company:branch ID,e.g. 63601:01V")
	cmd.Flags().StringVar(&dropoffID, "dropoff-shop", "", "Exact return company:branch ID,e.g. 63601:095")
	cmd.Flags().StringVar(&family, "family", "standard", "Fee family: standard or wagon (includes SUV/LXC/LXP)")
	toyotaRequireFlags(cmd, flags, "pickup-shop", "dropoff-shop")
	return cmd
}
