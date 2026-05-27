package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/squeezeos/internal"
)

var futuresCmd = &cobra.Command{
	Use:   "futures",
	Short: "Signal prediction market — stake RLUSD on the next council verdict",
}

var futuresBrowseCmd = &cobra.Command{
	Use:   "browse",
	Short: "Browse open futures positions",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/futures")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/api/futures")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var futuresLeaderCmd = &cobra.Command{
	Use:   "leaderboard",
	Short: "Top predictors by win rate and volume",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/futures/leaderboard")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/api/futures/leaderboard")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var (
	futureSymbol    string
	futureVerdict   string
	futureStake     float64
	futureWallet    string
)

var futuresCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Stake RLUSD on the next council verdict for a symbol",
	Example: `  squeezeos futures create --symbol IWM --verdict BUY --stake 0.10 --wallet rXXX`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if futureSymbol == "" || futureVerdict == "" || futureWallet == "" {
			return fmt.Errorf("--symbol, --verdict, and --wallet are required")
		}
		body := map[string]any{
			"symbol":       futureSymbol,
			"predicted":    futureVerdict,
			"stake_rlusd":  futureStake,
			"agent_wallet": futureWallet,
		}
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "POST /api/futures/create %+v\n", body)
			return nil
		}
		c := internal.NewClient()
		res, err := c.Post("/api/futures/create", body)
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var futuresWalletCmd = &cobra.Command{
	Use:   "wallet <address>",
	Short: "View all futures positions for a wallet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/api/futures/wallet/" + args[0]
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

func init() {
	futuresCreateCmd.Flags().StringVar(&futureSymbol, "symbol", "", "ticker symbol")
	futuresCreateCmd.Flags().StringVar(&futureVerdict, "verdict", "", "predicted verdict: BUY, SELL, HOLD")
	futuresCreateCmd.Flags().Float64Var(&futureStake, "stake", 0.10, "stake amount in RLUSD")
	futuresCreateCmd.Flags().StringVar(&futureWallet, "wallet", "", "your XRPL wallet address")

	futuresCmd.AddCommand(futuresBrowseCmd, futuresLeaderCmd, futuresCreateCmd, futuresWalletCmd)
}
