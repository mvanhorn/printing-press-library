package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/ghost-layer/internal"
)

var (
	bridgeChain     string
	bridgeAmount    string
	bridgeRecipient string
	bridgeSigner    string
	bridgeSig       string
)

var bridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Execute a dual-chain bridge settlement (XRPL RLUSD or Base USDC)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if bridgeChain == "" || bridgeAmount == "" || bridgeRecipient == "" {
			return fmt.Errorf("--chain, --amount, and --recipient are required")
		}
		body := map[string]any{
			"chain":     bridgeChain,
			"amount":    bridgeAmount,
			"recipient": bridgeRecipient,
		}
		if bridgeSigner != "" {
			body["signer"] = bridgeSigner
		}
		if bridgeSig != "" {
			body["signature"] = bridgeSig
		}
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "POST /v1/bridge/execute %+v\n", body)
			return nil
		}
		c := internal.NewClient()
		res, err := c.Post("/v1/bridge/execute", body)
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var x402Cmd = &cobra.Command{
	Use:   "x402",
	Short: "x402 payment catalog — list products, get quote, dispense",
}

var x402CatalogCmd = &cobra.Command{
	Use:   "catalog",
	Short: "List all x402 products available for purchase",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /v1/x402/catalog")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/v1/x402/catalog")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var (
	quoteProduct string
	quoteWallet  string
)

var x402QuoteCmd = &cobra.Command{
	Use:   "quote",
	Short: "Get a payment quote for a product",
	RunE: func(cmd *cobra.Command, args []string) error {
		if quoteProduct == "" || quoteWallet == "" {
			return fmt.Errorf("--product and --wallet are required")
		}
		body := map[string]any{"product_id": quoteProduct, "agent_wallet": quoteWallet}
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "POST /v1/x402/quote %+v\n", body)
			return nil
		}
		c := internal.NewClient()
		res, err := c.Post("/v1/x402/quote", body)
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var x402DispenseCmd = &cobra.Command{
	Use:   "dispense <product_id>",
	Short: "Dispense a product after payment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/v1/x402/dispense/" + args[0]
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

var agentCmd = &cobra.Command{
	Use:   "agent <wallet>",
	Short: "Get agent stats, loyalty tier, and passport info",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/api/agent/" + args[0] + "/stats"
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

var cubeCmd = &cobra.Command{
	Use:   "cube",
	Short: "54-block execution matrix state",
}

var cubeStateCmd = &cobra.Command{
	Use:   "state",
	Short: "Current cube state snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/cube/state")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/api/cube/state")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Ghost Layer health check",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /health")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/health")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

func init() {
	bridgeCmd.Flags().StringVar(&bridgeChain, "chain", "", "XRPL or Base")
	bridgeCmd.Flags().StringVar(&bridgeAmount, "amount", "", "amount in drops (XRPL) or wei (Base)")
	bridgeCmd.Flags().StringVar(&bridgeRecipient, "recipient", "", "recipient wallet address")
	bridgeCmd.Flags().StringVar(&bridgeSigner, "signer", "", "signer wallet address")
	bridgeCmd.Flags().StringVar(&bridgeSig, "sig", "", "XRPL signature or EIP-3009 authorization")

	x402QuoteCmd.Flags().StringVar(&quoteProduct, "product", "", "product ID from catalog")
	x402QuoteCmd.Flags().StringVar(&quoteWallet, "wallet", "", "agent wallet address")

	x402Cmd.AddCommand(x402CatalogCmd, x402QuoteCmd, x402DispenseCmd)
	cubeCmd.AddCommand(cubeStateCmd)
}
