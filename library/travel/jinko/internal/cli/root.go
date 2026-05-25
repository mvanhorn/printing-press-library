package cli

import (
	"github.com/spf13/cobra"
)

// Version is the CLI version, overridden via -ldflags at build time.
var Version = "0.1.0"

type globalFlags struct {
	apiKey string
	format string
}

var globals globalFlags

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "jinko",
		Short: "Jinko Travel CLI — flights, hotels, multi-product cart booking from one terminal.",
		Long: `Jinko Travel CLI

Search flights and hotels, build a multi-product trip (flight + hotel in one cart),
and check out via a Jinko-hosted Stripe page — all from the terminal.

Auth: pass --api-key, set JINKO_API_KEY, or run "jinko auth login --key jnk_..."
to persist credentials to ~/.jinko/config.yaml.

Token flow (agent-friendly):
  find-flight / find-destination  →  offer_token       (cached search)
  flight-search   --offer-token   →  trip_item_token   (live price + bookable)
  hotel-search    --location ...  →  trip_item_token   (live rate)
  trip            --trip-item-token (flight or hotel)  →  trip_id
  book            --trip-id       →  checkout_url      (Stripe-hosted)
  trip-status     --trip-id       →  PNRs + lifecycle`,
		Version:       Version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.PersistentFlags().StringVar(&globals.apiKey, "api-key", "", "API key (jnk_...) — overrides JINKO_API_KEY env and config file")
	root.PersistentFlags().StringVar(&globals.format, "format", "json", "output format: json | table")

	// Discovery
	root.AddCommand(newFindFlightCmd())
	root.AddCommand(newFindDestinationCmd())
	root.AddCommand(newFlightSearchCmd())
	root.AddCommand(newHotelSearchCmd())
	root.AddCommand(newHotelDetailsCmd())

	// Trip building + checkout
	root.AddCommand(newTripCmd())
	root.AddCommand(newBookCmd())
	root.AddCommand(newTripStatusCmd())

	// Auth
	root.AddCommand(newAuthCmd())

	return root
}

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}
