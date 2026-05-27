package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/squeezeos/internal"
)

var settlementCmd = &cobra.Command{
	Use:   "settlement",
	Short: "Conditional escrow contracts — zero custody, on-chain proof",
}

var settlementBrowseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse open settlement contracts",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/settlement")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/api/settlement")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var settlementGetCmd = &cobra.Command{
	Use:   "get <contract_id>",
	Short: "Get a specific settlement contract",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/api/settlement/" + args[0]
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "GET %s\n", path)
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get(path)
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var settlementWalletCmd = &cobra.Command{
	Use:   "wallet <address>",
	Short: "View all contracts for a wallet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/api/settlement/wallet/" + args[0]
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "GET %s\n", path)
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get(path)
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var (
	sCondition   string
	sThreshold   float64
	sSymbol      string
	sBuyer       string
	sSeller      string
	sAmount      float64
)

var settlementCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a conditional escrow contract",
	Example: `  squeezeos settlement create --symbol IWM --condition confidence_above --threshold 80 --buyer rXXX --seller rYYY --amount 1.0`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if sSymbol == "" || sCondition == "" || sBuyer == "" || sSeller == "" {
			return fmt.Errorf("--symbol, --condition, --buyer, and --seller are required")
		}
		body := map[string]any{
			"symbol":        sSymbol,
			"condition":     sCondition,
			"threshold":     sThreshold,
			"buyer_wallet":  sBuyer,
			"seller_wallet": sSeller,
			"amount_rlusd":  sAmount,
		}
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "POST /api/settlement/create %+v\n", body)
			return nil
		}
		c := internal.NewClient()
		res, err := c.Post("/api/settlement/create", body)
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

func init() {
	settlementCreateCmd.Flags().StringVar(&sSymbol, "symbol", "", "ticker symbol")
	settlementCreateCmd.Flags().StringVar(&sCondition, "condition", "", "confidence_above|bias_match|price_above|price_below|time_elapsed")
	settlementCreateCmd.Flags().Float64Var(&sThreshold, "threshold", 0, "threshold value for the condition")
	settlementCreateCmd.Flags().StringVar(&sBuyer, "buyer", "", "buyer XRPL wallet")
	settlementCreateCmd.Flags().StringVar(&sSeller, "seller", "", "seller XRPL wallet")
	settlementCreateCmd.Flags().Float64Var(&sAmount, "amount", 1.0, "escrow amount in RLUSD")

	settlementCmd.AddCommand(settlementBrowseCmd, settlementGetCmd, settlementWalletCmd, settlementCreateCmd)
}
