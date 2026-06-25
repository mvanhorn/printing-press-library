// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
// Hand-written: populate payment config from a real captured checkout so the
// Stripe customer + saved-card ids don't have to be transcribed from DevTools.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/capture"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/ordertogo/internal/config"
)

func newConfigBootstrapCmd(flags *rootFlags) *cobra.Command {
	var capturePath string
	cmd := &cobra.Command{
		Use:   "bootstrap-from-capture",
		Short: "Populate payment config from a captured order (HAR or postmicmeshorder body)",
		Long: `Read a real captured checkout and set the Stripe customer id, saved-card
key, and customer name/phone needed by 'orders place'. The capture may be a
browser/proxy HAR containing a POST /m/api/postmicmeshorder, the captured-order
JSON artifact, or a file that is the order request body itself.`,
		Example: `  ordertogo-pp-cli config bootstrap-from-capture --capture order.har
  ordertogo-pp-cli config bootstrap-from-capture --capture .captured-real-order.json --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if capturePath == "" {
				return usageErr(fmt.Errorf("--capture <path> is required"))
			}
			pc, err := capture.LoadPaymentConfig(capturePath)
			if err != nil {
				return usageErr(err)
			}

			set := map[string]string{
				"stripe_customer_id":  pc.StripeCustomerID,
				"stripe_default_card": pc.StripeDefaultCard,
				"customer_firstname":  pc.CustomerFirstName,
				"customer_lastname":   pc.CustomerLastName,
				"customer_phone":      pc.CustomerPhone,
				"device_id":           pc.DeviceID,
				"mobile_id":           pc.MobileID,
				"order_context_json":  pc.OrderContextJSON,
			}

			summary := map[string]any{
				"stripe_customer_id":  maskID(pc.StripeCustomerID),
				"stripe_default_card": maskCard(pc.StripeDefaultCard),
				"customer_firstname":  pc.CustomerFirstName,
				"customer_lastname":   pc.CustomerLastName,
				"customer_phone":      maskPhone(pc.CustomerPhone),
				"device_id":           pc.DeviceID,
				"mobile_id":           pc.MobileID,
				"order_context_json":  fmt.Sprintf("<%d bytes>", len(pc.OrderContextJSON)),
			}

			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "would_set": summary}, flags)
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			for key, val := range set {
				if val == "" {
					continue
				}
				if err := cfg.SetKey(key, val); err != nil {
					return usageErr(err)
				}
			}
			if err := cfg.Save(); err != nil {
				return configErr(err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"set": summary, "path": cfg.Path}, flags)
		},
	}
	cmd.Flags().StringVar(&capturePath, "capture", "", "Path to a HAR, captured-order JSON, or order request body")
	return cmd
}

func maskID(s string) string {
	if len(s) <= 7 {
		if s == "" {
			return ""
		}
		return "***"
	}
	return s[:7] + "***"
}

func maskCard(s string) string {
	if s == "" {
		return ""
	}
	return "***"
}

func maskPhone(s string) string {
	if len(s) < 4 {
		if s == "" {
			return ""
		}
		return "***"
	}
	return "***" + s[len(s)-4:]
}
