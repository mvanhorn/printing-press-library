// Copyright 2026 mikesnowbie and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
// No local mirror exists for this CLI (no sync command was generated for
// this spec), so this command reads live from the YNAB API on every call.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// ynabAccountsEnvelope mirrors YNAB's AccountsResponse: { "data": { "accounts": [...] } }.
type ynabAccountsEnvelope struct {
	Data struct {
		Accounts []ynabAccount `json:"accounts"`
	} `json:"data"`
}

type ynabAccount struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	OnBudget        bool    `json:"on_budget"`
	Closed          bool    `json:"closed"`
	Deleted         bool    `json:"deleted"`
	BalanceCurrency float64 `json:"balance_currency"`
	Balance         int64   `json:"balance"`
}

type exportedBalance struct {
	Name     string  `json:"name"`
	Balance  float64 `json:"balance"`
	Type     string  `json:"type"`
	OnBudget bool    `json:"on_budget"`
	Closed   bool    `json:"closed"`
}

type exportBalancesResult struct {
	Format     string            `json:"format"`
	PlanID     string            `json:"plan_id"`
	ExportedAt string            `json:"exported_at"`
	Accounts   []exportedBalance `json:"accounts"`
}

func newNovelExportBalancesCmd(flags *rootFlags) *cobra.Command {
	var flagFormat string
	var flagPlan string
	var flagIncludeClosed bool

	cmd := &cobra.Command{
		Use:         "balances",
		Short:       "One-shot export of current account balances reshaped into ProjectionLab's expected schema",
		Example:     "  ynab-pp-cli export balances --format projectionlab --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export balances")
			}
			if flagFormat != "" && flagFormat != "projectionlab" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format %q is not supported; only \"projectionlab\" is implemented", flagFormat))
			}
			format := flagFormat
			if format == "" {
				format = "projectionlab"
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/plans/{plan_id}/accounts"
			path = replacePathParam(path, "plan_id", flagPlan)
			raw, err := c.GetWithHeadersNoCache(ctx, path, nil, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var envelope ynabAccountsEnvelope
			if err := json.Unmarshal(raw, &envelope); err != nil {
				return fmt.Errorf("parsing accounts response: %w", err)
			}

			result := exportBalancesResult{
				Format:     format,
				PlanID:     flagPlan,
				ExportedAt: time.Now().UTC().Format(time.RFC3339),
				Accounts:   make([]exportedBalance, 0, len(envelope.Data.Accounts)),
			}
			for _, a := range envelope.Data.Accounts {
				if a.Deleted {
					continue
				}
				if a.Closed && !flagIncludeClosed {
					continue
				}
				result.Accounts = append(result.Accounts, exportedBalance{
					Name:     a.Name,
					Balance:  a.BalanceCurrency,
					Type:     a.Type,
					OnBudget: a.OnBudget,
					Closed:   a.Closed,
				})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			rows := make([]map[string]any, 0, len(result.Accounts))
			for _, a := range result.Accounts {
				rows = append(rows, map[string]any{
					"name":      a.Name,
					"balance":   a.Balance,
					"type":      a.Type,
					"on_budget": a.OnBudget,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}
	cmd.Flags().StringVar(&flagFormat, "format", "projectionlab", "Export schema shape (only \"projectionlab\" is implemented)")
	cmd.Flags().StringVar(&flagPlan, "plan", "last-used", `Plan (budget) ID, or "last-used" for the most recently used plan`)
	cmd.Flags().BoolVar(&flagIncludeClosed, "include-closed", false, "Include closed accounts in the export")
	return cmd
}
