// pp:data-source live
package cli

import (
	"github.com/mvanhorn/printing-press-library/library/travel/toyota-rentacar/internal/toyota"
	"github.com/spf13/cobra"
)

func newNovelCarsQuoteCmd(flags *rootFlags) *cobra.Command {
	var rental toyotaRentalFlags
	var category, class string
	var limit int
	var details bool
	cmd := &cobra.Command{Use: "quote", Short: "Read real dated class offers and source price estimates",
		Long:        "Search Toyota with exact JST dates,both shop IDs and vehicle/seat options. Class-page Rental Price includes tax but is an estimate; confirmed_full_total_jpy remains null. A class does not guarantee a representative model. ETC/JAF/waiver/NOC are selected later on Toyota; their inclusion is not inferred.",
		Example:     "  toyota-rentacar-pp-cli cars quote --pickup-shop 63601:01V --pickup 2026-10-20T09:00 --dropoff 2026-10-21T09:00 --category compact --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--pickup-shop=63601:01V;--pickup=2026-10-20T09:00;--dropoff=2026-10-21T09:00;--category=compact"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if stop, err := toyotaGuard(cmd, args, flags, "cars quote"); stop {
				return err
			}
			period, o, err := rental.parse()
			if err != nil {
				return toyotaError(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			out, err := toyota.NewClient(flags.rateLimit).Quote(ctx, rental.pickupID, rental.dropoffID, period, o, category, class, limit, details)
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
	cmd.Flags().StringVar(&category, "category", "compact", "Source category: compact,standard,minivan or suv")
	cmd.Flags().StringVar(&class, "class", "", "Filter one class code before applying --limit,e.g. C1; does not select a model")
	cmd.Flags().IntVar(&limit, "limit", 8, "Maximum class offers returned,1–20")
	cmd.Flags().BoolVar(&details, "details", false, "Include up to twelve representative models per class instead of three")
	toyotaRequireFlags(cmd, flags, "pickup-shop", "pickup", "dropoff")
	return cmd
}
