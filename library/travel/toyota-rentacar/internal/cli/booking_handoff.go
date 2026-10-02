// pp:data-source computed
package cli

import (
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/toyota"
	"github.com/spf13/cobra"
)

func newNovelBookingHandoffCmd(flags *rootFlags) *cobra.Command {
	var rental toyotaRentalFlags
	var class string
	cmd := &cobra.Command{Use: "handoff", Short: "Print a canonical booking link and dated checklist",
		Long:        "Compute a Toyota pickup-shop link and preserve dates,return shop,class and options in a checklist. The URL only prefills the pickup shop; dates,return shop and options must be re-entered on Toyota. No browser is opened and no inventory,personal data,terms or payment is submitted.",
		Example:     "  toyota-rentacar-pp-cli booking handoff --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --class C1 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "computed", "pp:happy-args": "--pickup-shop=63601:01V;--pickup=2026-10-20T09:00;--dropoff=2026-10-21T09:00"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "booking handoff")
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("booking handoff accepts flags only; see --help"))
			}
			period, o, err := rental.parse()
			if err != nil {
				return toyotaError(err)
			}
			out, err := toyota.BookingHandoff(rental.pickupID, rental.dropoffID, period, o, class)
			if err != nil {
				return toyotaError(err)
			}
			return toyotaPrint(cmd, flags, out)
		}}
	cmd.Flags().StringVar(&rental.pickupID, "pickup-shop", "", "Exact pickup shop ID,e.g. 63601:01V")
	cmd.Flags().StringVar(&rental.dropoffID, "dropoff-shop", "", "Return shop ID; defaults to the pickup shop")
	cmd.Flags().StringVar(&rental.pickup, "pickup", "", "Pickup YYYY-MM-DDTHH:MM in JST,or RFC3339")
	cmd.Flags().StringVar(&rental.dropoff, "dropoff", "", "Return YYYY-MM-DDTHH:MM in JST,or RFC3339")
	cmd.Flags().StringVar(&rental.transmission, "transmission", "AT", "Requested transmission: AT or MT")
	cmd.Flags().BoolVar(&rental.fourWD, "four-wd", false, "Request 4WD; optional fees or stock restrictions may apply")
	cmd.Flags().BoolVar(&rental.winter, "winter-tires", false, "Request winter tires; seasonal rates and stock may apply")
	cmd.Flags().StringVar(&rental.seats, "child-seats", "", "Comma-separated child,infant,booster seat kinds; maximum four")
	cmd.Flags().StringVar(&class, "class", "", "Requested Toyota class code,e.g. C1; inventory is not checked")
	toyotaRequireFlags(cmd, flags, "pickup-shop", "pickup", "dropoff")
	return cmd
}
