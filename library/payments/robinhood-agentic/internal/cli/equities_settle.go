// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: equities settle. Resolves an order to verified terminal truth
// (actual fill price and state) by polling get_equity_orders, riding out the
// cancel {accepted} race and the null-until-backfilled market price.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// equitySettleReport is the settled truth for one order.
type equitySettleReport struct {
	OrderID         string `json:"order_id"`
	Symbol          string `json:"symbol"`
	State           string `json:"state"`
	Terminal        bool   `json:"terminal"`
	FillPrice       string `json:"fill_price"`
	Quantity        string `json:"quantity"`
	ExecutionsCount int    `json:"executions_count"`
	// TimedOut is true when --wait exhausted its polling window without the
	// order reaching a terminal state (or a filled order's price staying null).
	// It distinguishes "not settled within the window" from "settled".
	TimedOut bool `json:"timed_out,omitempty"`
}

// isTerminalOrderState reports whether an equity order state can no longer
// change. Everything else (new, queued, confirmed, unconfirmed,
// partially_filled, ...) is still in flight.
func isTerminalOrderState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	// Robinhood documents the British "cancelled" but the American "canceled"
	// appears in some responses; accept both so a canceled order is never
	// reported as still in flight.
	case "filled", "cancelled", "canceled", "rejected", "failed", "voided":
		return true
	}
	return false
}

// settlePollDone reports whether polling can stop: the state is terminal and,
// for filled orders, the market price has been backfilled. A filled order with
// a still-null price keeps polling (bounded by the attempt cap) so the report
// carries the actual fill price, not a placeholder.
func settlePollDone(state, fillPrice string) bool {
	if !isTerminalOrderState(state) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(state), "filled") && fillPrice == "" {
		return false
	}
	return true
}

// settleFieldString reads a JSON-decoded field defensively: strings pass
// through trimmed, numbers are formatted without exponent noise, null/absent
// and anything else collapse to "".
func settleFieldString(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

// extractSettleOrder pulls a single order object out of a get_equity_orders
// {data, guide} envelope. Accepts data.orders[0], a bare data array, or a
// single-order data object.
func extractSettleOrder(raw json.RawMessage) (map[string]any, error) {
	payload := raw
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err == nil {
		if data, ok := top["data"]; ok {
			if len(data) == 0 || string(data) == "null" {
				return nil, fmt.Errorf("order not found")
			}
			payload = data
		}
	}
	var wrapper struct {
		Orders []map[string]any `json:"orders"`
	}
	if err := json.Unmarshal(payload, &wrapper); err == nil && len(wrapper.Orders) > 0 {
		return wrapper.Orders[0], nil
	}
	var list []map[string]any
	if err := json.Unmarshal(payload, &list); err == nil {
		if len(list) == 0 {
			return nil, fmt.Errorf("order not found")
		}
		return list[0], nil
	}
	var single map[string]any
	if err := json.Unmarshal(payload, &single); err == nil && len(single) > 0 {
		if _, hasOrders := single["orders"]; hasOrders {
			return nil, fmt.Errorf("order not found")
		}
		return single, nil
	}
	return nil, fmt.Errorf("order not found")
}

// buildSettleReport reads order fields defensively and derives the settled
// view. Fill price prefers average_price, then price, then the first
// execution's price (the backfill often lands on executions first).
func buildSettleReport(orderID string, order map[string]any) equitySettleReport {
	state := settleFieldString(order, "state")
	price := settleFieldString(order, "average_price")
	if price == "" {
		price = settleFieldString(order, "price")
	}
	executions, _ := order["executions"].([]any)
	if price == "" && len(executions) > 0 {
		if exec, ok := executions[0].(map[string]any); ok {
			price = settleFieldString(exec, "price")
		}
	}
	return equitySettleReport{
		OrderID:         orderID,
		Symbol:          settleFieldString(order, "symbol"),
		State:           state,
		Terminal:        isTerminalOrderState(state),
		FillPrice:       price,
		Quantity:        settleFieldString(order, "quantity"),
		ExecutionsCount: len(executions),
	}
}

func newNovelEquitiesSettleCmd(flags *rootFlags) *cobra.Command {
	var flagWait bool
	var flagAccount string

	cmd := &cobra.Command{
		Use:         "settle <order-id>",
		Short:       "Resolve an order to verified terminal truth — actual fill price and state — instead of trusting the placement echo.",
		Example:     "  robinhood-agentic-pp-cli equities settle 1a2b3c4d-5678-90ab-cdef-1234567890ab --account RH123456 --wait",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validation lives in RunE (not cobra Args/MarkFlagRequired) so the
			// verify pipeline's bare --dry-run probe still reaches the dry-run
			// guard and exits 0.
			if !flags.dryRun {
				if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
					return usageErr(fmt.Errorf("missing <order-id>; usage: %s <order-id> --account <account-number> [--wait]", cmd.CommandPath()))
				}
				if strings.TrimSpace(flagAccount) == "" {
					return usageErr(fmt.Errorf("--account is required; usage: %s <order-id> --account <account-number> [--wait]", cmd.CommandPath()))
				}
			}
			if dryRunOK(flags) {
				return nil
			}
			orderID := strings.TrimSpace(args[0])
			account := strings.TrimSpace(flagAccount)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"account_number": account,
				"order_id":       orderID,
			}

			const maxAttempts = 10
			var report equitySettleReport
			haveReport := false
			for attempt := 1; ; attempt++ {
				if ctxErr := cmd.Context().Err(); ctxErr != nil {
					return ctxErr
				}
				raw, getErr := c.GetNoCache(cmd.Context(), "/tools/get_equity_orders", params) // pp:client-call
				if getErr != nil {
					return classifyAPIError(getErr, flags)
				}
				order, extractErr := extractSettleOrder(raw)
				if extractErr != nil {
					// During the cancel {accepted} race the order can be
					// briefly unreadable; keep polling if we're waiting.
					if !flagWait || attempt >= maxAttempts {
						return fmt.Errorf("order %s: %w", orderID, extractErr)
					}
				} else {
					report = buildSettleReport(orderID, order)
					haveReport = true
					if !flagWait || settlePollDone(report.State, report.FillPrice) {
						break
					}
					if attempt >= maxAttempts {
						break
					}
				}
				select {
				case <-cmd.Context().Done():
					return cmd.Context().Err()
				case <-time.After(2 * time.Second):
				}
			}

			// If --wait gave up before the order reached settled truth, say so
			// and exit non-zero: a caller that asked to wait must not read a
			// success exit as "settled" when it timed out.
			timedOut := flagWait && haveReport && !settlePollDone(report.State, report.FillPrice)
			report.TimedOut = timedOut

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if perr := printJSONFiltered(cmd.OutOrStdout(), report, flags); perr != nil {
					return perr
				}
				if timedOut {
					return fmt.Errorf("order %s did not settle within the wait window (state %q); still in flight", report.OrderID, report.State)
				}
				return nil
			}
			out := cmd.OutOrStdout()
			settled := "no (still in flight)"
			if report.Terminal {
				settled = "yes"
			}
			display := func(s string) string {
				if s == "" {
					return "-"
				}
				return s
			}
			fmt.Fprintf(out, "Order:       %s\n", report.OrderID)
			fmt.Fprintf(out, "Symbol:      %s\n", display(report.Symbol))
			fmt.Fprintf(out, "State:       %s\n", display(report.State))
			fmt.Fprintf(out, "Terminal:    %s\n", settled)
			fmt.Fprintf(out, "Fill price:  %s\n", display(report.FillPrice))
			fmt.Fprintf(out, "Quantity:    %s\n", display(report.Quantity))
			fmt.Fprintf(out, "Executions:  %d\n", report.ExecutionsCount)
			if timedOut {
				fmt.Fprintf(out, "\nTimed out waiting for settlement (state %q); still in flight.\n", report.State)
				return fmt.Errorf("order %s did not settle within the wait window", report.OrderID)
			}
			if !report.Terminal && !flagWait {
				fmt.Fprintf(out, "\nState is non-terminal; re-run with --wait to poll to settlement.\n")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagWait, "wait", false, "Poll until the order reaches a terminal state (filled/cancelled/rejected/failed/voided) with a backfilled fill price")
	cmd.Flags().StringVar(&flagAccount, "account", "", "Account number the order belongs to (required)")
	return cmd
}
