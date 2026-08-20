// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newCustomerCardsCmd intentionally returns only a count. Domino's card
// payloads can contain opaque payment references and masked display data;
// neither belongs in terminal output, logs, templates, or agent context.
func newCustomerCardsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cards [customerID]",
		Short:       "Report whether the signed-in customer has saved cards (metadata is never printed)",
		Annotations: map[string]string{"pp:endpoint": "customer.cards", "pp:method": "GET", "pp:path": "/power/customer/{customerID}/card", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			customerID, err := resolveCustomerID(flags, args)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/power/customer/{customerID}/card", "customerID", customerID)
			data, err := c.GetUncached(path, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":           "customer_cards",
					"dry_run":          true,
					"lookup_performed": false,
					"details_redacted": true,
				}, flags)
			}
			count, err := savedCardCount(data)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"saved_card_count": count,
				"has_saved_cards":  count > 0,
				"details_redacted": true,
			}, flags)
		},
	}
	return cmd
}

func resolveCustomerID(flags *rootFlags, args []string) (string, error) {
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		return args[0], nil
	}
	cfg, err := flags.loadConfig()
	if err != nil {
		return "", configErr(err)
	}
	if cfg.CredentialMarketMismatch() {
		return "", usageErr(fmt.Errorf("stored credentials belong to market %s; run 'dominos-pp-cli auth login --market %s'", cfg.CredentialMarket, cfg.Market))
	}
	if cfg.AuthSource == "env:DOMINOS_TOKEN" {
		return "", usageErr(fmt.Errorf("DOMINOS_TOKEN does not identify a customer; pass <customerID> explicitly"))
	}
	if cfg.DominosCustomerID == "" {
		return "", usageErr(fmt.Errorf("no CustomerID — run 'dominos-pp-cli auth login' or pass <customerID> explicitly"))
	}
	return cfg.DominosCustomerID, nil
}

func savedCardCount(data json.RawMessage) (int, error) {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return 0, fmt.Errorf("parsing saved-card response: %w", err)
	}
	switch value := decoded.(type) {
	case []any:
		return len(value), nil
	case map[string]any:
		for _, key := range []string{"CreditCards", "creditCards", "Cards", "cards"} {
			if rawCards, exists := value[key]; exists {
				cards, ok := rawCards.([]any)
				if !ok {
					return 0, fmt.Errorf("saved-card field %q has an unexpected shape", key)
				}
				return len(cards), nil
			}
		}
		// The Canadian endpoint returns an empty object when no card is saved.
		if len(value) == 0 {
			return 0, nil
		}
		return 0, fmt.Errorf("saved-card response has an unexpected shape")
	default:
		return 0, fmt.Errorf("saved-card response has an unexpected shape")
	}
}
