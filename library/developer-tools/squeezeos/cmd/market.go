package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/squeezeos/internal"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Free IWM council verdict — no payment required",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/demo/council")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/api/demo/council")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var previewCmd = &cobra.Command{
	Use:   "preview <symbol>",
	Short: "Bias + regime preview for any symbol (free, 15-min cache)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "GET /api/preview/%s\n", args[0])
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/api/preview/" + args[0])
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var councilCmd = &cobra.Command{
	Use:   "council <symbol>",
	Short: "Multi-engine AI verdict for any symbol — 0.10 RLUSD (requires SQUEEZEOS_TOKEN)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "POST /api/council {symbol: %q}\n", args[0])
			return nil
		}
		c := internal.NewClient()
		if c.Token == "" {
			return fmt.Errorf("SQUEEZEOS_TOKEN is required — get a token at https://four02proof.onrender.com")
		}
		res, err := c.Post("/api/council", map[string]string{"symbol": args[0]})
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Full $1-$50 squeeze scanner — 0.05 RLUSD (requires SQUEEZEOS_TOKEN)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/scan")
			return nil
		}
		c := internal.NewClient()
		if c.Token == "" {
			return fmt.Errorf("SQUEEZEOS_TOKEN is required — get a token at https://four02proof.onrender.com")
		}
		res, err := c.Get("/api/scan")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var optionsCmd = &cobra.Command{
	Use:   "options",
	Short: "Institutional options flow — 0.05 RLUSD (requires SQUEEZEOS_TOKEN)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/options")
			return nil
		}
		c := internal.NewClient()
		if c.Token == "" {
			return fmt.Errorf("SQUEEZEOS_TOKEN is required — get a token at https://four02proof.onrender.com")
		}
		res, err := c.Get("/api/options")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var iwmCmd = &cobra.Command{
	Use:   "iwm",
	Short: "IWM 0DTE contract scorer — 0.03 RLUSD (requires SQUEEZEOS_TOKEN)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/iwm")
			return nil
		}
		c := internal.NewClient()
		if c.Token == "" {
			return fmt.Errorf("SQUEEZEOS_TOKEN is required — get a token at https://four02proof.onrender.com")
		}
		res, err := c.Get("/api/iwm")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}

var historyCmd = &cobra.Command{
	Use:  "history [symbol]",
	Short: "Signal history — all recent signals or per-symbol (free)",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/api/history"
		if len(args) == 1 {
			path = "/api/history/" + args[0]
		}
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

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "System health and uptime",
	RunE: func(cmd *cobra.Command, args []string) error {
		if dryRun {
			fmt.Fprintln(cmd.OutOrStdout(), "GET /api/status")
			return nil
		}
		c := internal.NewClient()
		res, err := c.Get("/api/status")
		if err != nil {
			return err
		}
		return internal.Print(cmd.OutOrStdout(), res, compact)
	},
}
