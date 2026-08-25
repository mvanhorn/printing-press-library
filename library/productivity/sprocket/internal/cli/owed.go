// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newNovelOwedCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "owed",
		Short: "Total money owed across all your players: registration balances plus overdue invoices, with one number.",
		Long: "Combine outstanding registration balances with overdue invoices into a single total owed across all " +
			"your players.\n\n" +
			"Use this command for total money owed across all players. Do NOT use it for registration history " +
			"detail (use 'registrations completed') or the raw invoice list (use 'payments overdue-invoices').",
		Example:     "  sprocket-pp-cli owed\n  sprocket-pp-cli owed --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			regData, err := c.Get(ctx, "/api/club-users/completed-registrations", nil)
			if err != nil {
				return fmt.Errorf("fetching registrations: %w", err)
			}
			invData, err := c.Get(ctx, "/api/club-users/overdue-invoice-payments", nil)
			if err != nil {
				return fmt.Errorf("fetching overdue invoices: %w", err)
			}

			report, err := buildOwedReport(regData, invData)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.printJSON(cmd, report)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Outstanding balance: $%.2f\n", report.TotalOwed)
			fmt.Fprintf(w, "  Registration balances: $%.2f (%d registration(s))\n", report.RegistrationBalance, report.RegistrationsWithBalance)
			fmt.Fprintf(w, "  Overdue invoices:      $%.2f (%d invoice(s))\n", report.OverdueInvoices, report.OverdueInvoiceCount)
			return nil
		},
	}
	return cmd
}

type owedReport struct {
	TotalOwed                float64 `json:"totalOwed"`
	RegistrationBalance      float64 `json:"registrationBalance"`
	OverdueInvoices          float64 `json:"overdueInvoices"`
	RegistrationsWithBalance int     `json:"registrationsWithBalance"`
	OverdueInvoiceCount      int     `json:"overdueInvoiceCount"`
	GeneratedAt              string  `json:"generatedAt"`
}

// buildOwedReport sums outstanding registration balances and overdue invoice
// amounts from the two raw API responses. Defensive about field names. Returns
// an error when a non-empty response cannot be interpreted, so the flagship
// "one number" never silently reports $0 on an unexpected shape. Pure and
// unit-tested.
func buildOwedReport(regData, invData json.RawMessage) (owedReport, error) {
	rep := owedReport{GeneratedAt: time.Now().UTC().Format(time.RFC3339)}

	regs, ok := asObjects(regData)
	if !ok {
		return rep, fmt.Errorf("could not interpret registrations response (unexpected shape)")
	}
	for _, m := range regs {
		bal := firstFloat(m, "remainingBalance", "amountDue", "balanceDue", "balance")
		if bal > 0 {
			rep.RegistrationBalance += bal
			rep.RegistrationsWithBalance++
		}
	}
	invs, ok := asObjects(invData)
	if !ok {
		return rep, fmt.Errorf("could not interpret overdue-invoices response (unexpected shape)")
	}
	for _, m := range invs {
		amt := firstFloat(m, "remainingBalance", "amountDue", "balanceDue", "balance", "amount", "total", "amountOwed")
		if amt > 0 {
			rep.OverdueInvoices += amt
			rep.OverdueInvoiceCount++
		}
	}
	rep.TotalOwed = rep.RegistrationBalance + rep.OverdueInvoices
	return rep, nil
}

// asObjects unmarshals a JSON array, an object that IS one record, or any
// single-array-valued wrapper ({"data":[...]}, {"results":[...]}, etc.) into a
// slice of generic objects. ok is false when the body is non-empty but matches
// none of these shapes, so callers can distinguish "no records" from "couldn't
// interpret the response" instead of silently reporting zero.
func asObjects(data json.RawMessage) (objs []map[string]any, ok bool) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		return nil, true
	}
	var arr []map[string]any
	if json.Unmarshal(data, &arr) == nil {
		return arr, true
	}
	var generic map[string]json.RawMessage
	if json.Unmarshal(data, &generic) == nil {
		// Pick the array-valued field deterministically: prefer common wrapper
		// keys, then fall back to alphabetical order. Go map iteration is
		// randomized, so an undefined order would make field selection
		// non-deterministic across runs for multi-array responses.
		keys := make([]string, 0, len(generic))
		for k := range generic {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		ordered := append([]string{"data", "results", "items", "records", "value"}, keys...)
		seen := map[string]bool{}
		for _, k := range ordered {
			if seen[k] {
				continue
			}
			seen[k] = true
			raw, exists := generic[k]
			if !exists {
				continue
			}
			var inner []map[string]any
			if json.Unmarshal(raw, &inner) == nil {
				return inner, true
			}
		}
		// A bare object that is itself one record.
		var single map[string]any
		if json.Unmarshal(data, &single) == nil {
			return []map[string]any{single}, true
		}
	}
	return nil, false
}

// firstFloat returns the first key whose value coerces to a float64, accepting
// JSON numbers and numeric strings (some finance APIs encode money as strings).
func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return n
		case json.Number:
			f, _ := n.Float64()
			return f
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(n), 64); err == nil {
				return f
			}
		}
	}
	return 0
}
