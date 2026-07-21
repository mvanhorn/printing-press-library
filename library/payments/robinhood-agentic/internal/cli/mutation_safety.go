// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-written mutation-safety wiring. Installs the transport-level guard and
// journal hooks (see internal/client/mcp_transport.go) at package init so every
// mutating MCP tool call is gated, policy-checked, and journaled from one place
// — no edits to the generated command files.
//
// The safety model has three layers, strongest first:
//  1. Write gate: no mutating tool call reaches the network unless
//     ROBINHOOD_AGENTIC_PP_ALLOW_WRITES=1 is set. This is the hard floor that
//     makes read-only testing safe by construction.
//  2. Guard policy: order placement is additionally checked against the local
//     trade policy (per-order cap, daily cap, allow/denylist, kill switch).
//  3. Journal: every mutation attempt and its outcome is recorded locally for
//     the `audit` command — the accountability trail the server does not keep.

package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"
)

// writeGateEnv is the environment variable that must equal "1" for any mutating
// tool call to leave the machine. Mirrors the library's ROBINHOOD_PP_ALLOW_WRITES
// convention, namespaced to this CLI.
const writeGateEnv = "ROBINHOOD_AGENTIC_PP_ALLOW_WRITES"

func init() {
	client.MutationGuard = guardMutation
	client.MutationJournal = journalMutation
}

// guardMutation enforces the write gate and, for order placement, the trade
// policy. Returns a non-nil error to refuse the call before it dials out.
func guardMutation(ctx context.Context, tool string, args map[string]any) error {
	if os.Getenv(writeGateEnv) != "1" {
		recordJournal(store.WriteJournalEntry{
			Tool: tool, Action: mutationAction(tool), Outcome: "blocked_gate",
			Account: argString(args, "account_number"), Symbol: argString(args, "symbol"),
			Detail: "write gate closed",
		})
		return fmt.Errorf(
			"write blocked: %s is a live mutation and %s is not set to 1.\n"+
				"Reads and `review` are always allowed. To place or cancel for real, review the order first, then set %s=1 and re-run.",
			tool, writeGateEnv, writeGateEnv)
	}

	// Order-placement policy check (kill switch, caps, allow/denylist).
	if tool == "place_equity_order" || tool == "place_option_order" {
		if reason := checkOrderPolicy(tool, args); reason != "" {
			recordJournal(store.WriteJournalEntry{
				Tool: tool, Action: "place", Outcome: "blocked_guard",
				Account: argString(args, "account_number"), Symbol: orderSymbol(args),
				Detail: reason,
			})
			return fmt.Errorf("guard blocked order: %s", reason)
		}
	}
	return nil
}

// checkOrderPolicy loads the guard policy and evaluates the order. It FAILS
// CLOSED: any failure to read the policy, or an order whose notional cannot be
// computed while a cap is configured, returns a denial reason. A guard that
// silently passes on a store error would let an order bypass a kill switch or
// cap the user set, which is the opposite of what a guard is for.
func checkOrderPolicy(tool string, args map[string]any) string {
	st, err := openAgenticStore()
	if err != nil {
		return "guard policy could not be read (local store error); refusing to place rather than risk bypassing a configured kill switch or cap. Fix the store or run `guard status`."
	}
	defer st.Close()
	policy, err := st.GetGuardPolicy()
	if err != nil {
		return "guard policy could not be loaded (local store error); refusing to place. Run `guard status` to inspect."
	}
	notional := orderNotional(tool, args)
	// A market order with only a share quantity has no computable notional. If
	// any notional cap is configured, refuse rather than treat the order as
	// zero exposure and slip past the cap. With no cap set, notional == 0 is
	// fine and the order proceeds.
	if notional == 0 && (policy.MaxOrderNotional > 0 || policy.DailyCapNotional > 0) {
		return "cannot enforce a notional cap on this order: it has no computable price (a market order with only a quantity). Use a limit order or --dollar-amount, or clear the cap with `guard set`."
	}
	daySpent, err := st.SumPlacedNotionalSince(startOfUTCDay())
	if err != nil {
		return "guard daily-cap check failed (journal read error); refusing to place."
	}
	// NOTE (known limitation): daySpent is read and evaluated without a
	// cross-process atomic reservation, so two concurrent placements can both
	// pass a daily cap they jointly exceed (a TOCTOU window). The env write
	// gate is the primary safety floor; the daily cap is a best-effort
	// single-process guard. An atomic reservation is tracked as a follow-up.
	return policy.EvaluateOrder(orderSymbol(args), notional, daySpent)
}

// journalMutation records the outcome of a mutating call that reached the
// network (guard-blocked calls are journaled inside guardMutation).
func journalMutation(ctx context.Context, tool string, args map[string]any, status int, callErr error) {
	outcome := "placed"
	action := mutationAction(tool)
	detail := ""
	switch {
	case callErr != nil:
		outcome = "error"
		detail = callErr.Error()
	case status >= 400:
		outcome = "rejected"
	default:
		if tool == "place_equity_order" || tool == "place_option_order" {
			detail = fmt.Sprintf("notional=%.2f", orderNotional(tool, args))
		}
	}
	recordJournal(store.WriteJournalEntry{
		Tool: tool, Action: action, Outcome: outcome,
		Account: argString(args, "account_number"), Symbol: orderSymbol(args),
		RefID: argString(args, "ref_id"), Detail: detail,
	})
}

func recordJournal(e store.WriteJournalEntry) {
	st, err := openAgenticStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not open the local store to journal a %s (%s); the audit trail and daily-cap tracking may be incomplete: %v\n", e.Action, e.Tool, err)
		return
	}
	defer st.Close()
	if err := st.RecordWrite(e); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to journal a %s (%s); the audit trail and daily-cap tracking may be incomplete: %v\n", e.Action, e.Tool, err)
	}
}

func openAgenticStore() (*store.Store, error) {
	return store.Open(defaultDBPath("robinhood-agentic-pp-cli"))
}

// mutationAction maps a tool to a short journal action verb.
func mutationAction(tool string) string {
	switch {
	case strings.HasPrefix(tool, "place_"):
		return "place"
	case strings.HasPrefix(tool, "cancel_"):
		return "cancel"
	case strings.Contains(tool, "watchlist"):
		return "watchlist"
	case strings.Contains(tool, "scan"):
		return "scan"
	default:
		return "mutate"
	}
}

// orderSymbol resolves the symbol for journaling. Equity orders carry `symbol`;
// option orders carry a leg option_id (no symbol), so fall back to that.
func orderSymbol(args map[string]any) string {
	if s := argString(args, "symbol"); s != "" {
		return s
	}
	if legs, ok := args["legs"].([]any); ok && len(legs) > 0 {
		if leg, ok := legs[0].(map[string]any); ok {
			if oid, ok := leg["option_id"].(string); ok {
				return "option:" + oid
			}
		}
	}
	return ""
}

// orderNotional best-effort estimates an order's dollar notional for the caps.
// Equity: dollar_amount, else quantity×(limit_price|stop_price). Option:
// quantity×price×100 (contract multiplier). A market order with only a share
// quantity has no known price and returns 0 (uncapped — honestly reported).
func orderNotional(tool string, args map[string]any) float64 {
	if tool == "place_option_order" {
		return argFloatVal(args, "quantity") * argFloatVal(args, "price") * 100
	}
	if d := argFloatVal(args, "dollar_amount"); d > 0 {
		return d
	}
	price := argFloatVal(args, "limit_price")
	if price == 0 {
		price = argFloatVal(args, "stop_price")
	}
	return argFloatVal(args, "quantity") * price
}

func startOfUTCDay() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func argString(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func argFloatVal(args map[string]any, key string) float64 {
	v, ok := args[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case int64:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(n), 64)
		return f
	default:
		return 0
	}
}
