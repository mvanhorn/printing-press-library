package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/squeezeos/internal"
)

var marketplaceCmd = &cobra.Command{
	Use:   "marketplace",
	Short: "Peer signal marketplace — browse listings, list a signal, read full thesis",
}

var marketplaceBrowseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse all marketplace signal listings (free)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/marketplace")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/api/marketplace")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var (
	listSymbol string
	listBias   string
	listThesis string
	listPrice  float64
	listWallet string
)

var marketplaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List a signal for sale on the marketplace",
	Example: `  squeezeos marketplace list --symbol NVDA --bias LONG --thesis "Gamma squeeze setup" --price 0.05 --wallet rXXX`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if listSymbol == "" || listBias == "" || listThesis == "" || listWallet == "" {
			return fmt.Errorf("--symbol, --bias, --thesis, and --wallet are required")
		}
		body := map[string]any{
			"symbol":        listSymbol,
			"bias":          listBias,
			"thesis":        listThesis,
			"price_rlusd":   listPrice,
			"seller_wallet": listWallet,
		}
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "POST /api/marketplace/list %+v\n", body)
			return nil
		}
		c := internal.NewClient()
		res, err := c.Post("/api/marketplace/list", body)
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var marketplaceReadCmd = &cobra.Command{
	Use:   "read <listing_id>",
	Short: "Read full signal thesis — 0.02 RLUSD (requires SQUEEZEOS_TOKEN)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "POST /api/marketplace/read {listing_id: %q}\n", args[0])
			return nil
		}
		c := internal.NewClient()
		if c.Token == "" {
			return fmt.Errorf("SQUEEZEOS_TOKEN is required to read full thesis")
		}
		res, err := c.Post("/api/marketplace/read", map[string]string{"listing_id": args[0]})
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

func init() {
	marketplaceListCmd.Flags().StringVar(&listSymbol, "symbol", "", "ticker symbol")
	marketplaceListCmd.Flags().StringVar(&listBias, "bias", "", "LONG or SHORT")
	marketplaceListCmd.Flags().StringVar(&listThesis, "thesis", "", "signal thesis text")
	marketplaceListCmd.Flags().Float64Var(&listPrice, "price", 0.02, "price in RLUSD")
	marketplaceListCmd.Flags().StringVar(&listWallet, "wallet", "", "your XRPL wallet address")

	marketplaceCmd.AddCommand(marketplaceBrowseCmd, marketplaceListCmd, marketplaceReadCmd)
}
