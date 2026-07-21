// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// guard: client-side trade policy (caps, allow/denylists, kill switch).
// Robinhood's OAuth scope is all-or-nothing and the platform ships no native
// limits, so this local policy is the only enforceable limit layer — it is
// checked before any order leaves the machine.

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"
)

func newNovelGuardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guard",
		Short: "Set per-trade caps, daily caps, symbol allow/denylists",
		Long: "Manage the client-side trade guard policy: per-order and daily notional caps,\n" +
			"symbol allow/denylists, and a kill switch. Enforced locally before any order\n" +
			"leaves the machine — the only enforceable limit layer, since Robinhood's OAuth\n" +
			"scope is all-or-nothing.",
		Example:     "  guard set --max-order 500 --daily-cap 2000\n  guard set --allow AAPL,MSFT\n  guard set --kill\n  guard status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newGuardSetCmd(flags))
	cmd.AddCommand(newGuardStatusCmd(flags))
	return cmd
}

func newGuardSetCmd(flags *rootFlags) *cobra.Command {
	var (
		flagMaxOrder float64
		flagDailyCap float64
		flagAllow    []string
		flagDeny     []string
		flagKill     bool
		flagDisarm   bool
	)

	cmd := &cobra.Command{
		Use:     "set",
		Short:   "Update the guard policy (only the flags you pass change)",
		Example: "  guard set --max-order 500 --daily-cap 2000\n  guard set --allow AAPL,MSFT --deny GME\n  guard set --kill\n  guard set --disarm",
		RunE: func(cmd *cobra.Command, args []string) error {
			changed := map[string]bool{}
			for _, name := range []string{"max-order", "daily-cap", "allow", "deny", "kill", "disarm"} {
				changed[name] = cmd.Flags().Changed(name)
			}
			anyChanged := false
			for _, c := range changed {
				if c {
					anyChanged = true
					break
				}
			}
			if !anyChanged {
				return usageErr(fmt.Errorf("nothing to set: pass at least one of --max-order, --daily-cap, --allow, --deny, --kill, --disarm (usage: guard set --max-order 500)"))
			}
			if changed["kill"] && changed["disarm"] && flagKill && flagDisarm {
				return usageErr(fmt.Errorf("--kill and --disarm conflict: pass one or the other"))
			}
			if flagMaxOrder < 0 || flagDailyCap < 0 {
				return usageErr(fmt.Errorf("--max-order and --daily-cap must be >= 0 (0 clears the cap)"))
			}
			if dryRunOK(flags) {
				return nil
			}

			st, err := store.Open(defaultDBPath("robinhood-agentic-pp-cli"))
			if err != nil {
				return fmt.Errorf("open local store: %w", err)
			}
			defer st.Close()

			existing, err := st.GetGuardPolicy()
			if err != nil {
				return fmt.Errorf("load guard policy: %w", err)
			}
			updated := applyGuardFlags(existing, changed, flagMaxOrder, flagDailyCap, flagAllow, flagDeny, flagKill, flagDisarm)
			if err := st.SetGuardPolicy(updated); err != nil {
				return fmt.Errorf("save guard policy: %w", err)
			}

			out := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(out) {
				return printJSONFiltered(out, updated, flags)
			}
			fmt.Fprintln(out, "Guard policy updated:")
			printGuardPolicy(out, updated)
			return nil
		},
	}
	cmd.Flags().Float64Var(&flagMaxOrder, "max-order", 0, "Per-order notional cap in dollars (0 = no cap)")
	cmd.Flags().Float64Var(&flagDailyCap, "daily-cap", 0, "Daily total notional cap in dollars (0 = no cap)")
	cmd.Flags().StringSliceVar(&flagAllow, "allow", nil, "Symbols allowlist (replaces the existing list; empty clears it)")
	cmd.Flags().StringSliceVar(&flagDeny, "deny", nil, "Symbols denylist (replaces the existing list; empty clears it)")
	cmd.Flags().BoolVar(&flagKill, "kill", false, "Engage the kill switch (block all orders)")
	cmd.Flags().BoolVar(&flagDisarm, "disarm", false, "Clear the kill switch")
	return cmd
}

func newGuardStatusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "status",
		Short:       "Show the current guard policy and what it enforces",
		Example:     "  guard status",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			// Read-only open: status only reads the stored policy. A nil store
			// (no local DB yet) means an unset policy — nothing enforced.
			st, err := openStoreForRead(cmd.Context(), "robinhood-agentic-pp-cli")
			if err != nil {
				return fmt.Errorf("open local store: %w", err)
			}
			var policy store.GuardPolicy
			if st != nil {
				defer st.Close()
				policy, err = st.GetGuardPolicy()
				if err != nil {
					return fmt.Errorf("load guard policy: %w", err)
				}
			}
			empty := guardPolicyIsEmpty(policy)
			view := struct {
				Policy   store.GuardPolicy `json:"policy"`
				Enforced []string          `json:"enforced"`
				Empty    bool              `json:"empty"`
			}{Policy: policy, Enforced: guardEnforcementSummary(policy), Empty: empty}

			out := cmd.OutOrStdout()
			if flags.asJSON || !isTerminal(out) {
				return printJSONFiltered(out, view, flags)
			}
			if empty {
				fmt.Fprintln(out, "No guard policy is set.")
				fmt.Fprintln(out, "Nothing is enforced client-side: any order the CLI reviews will pass the guard.")
				fmt.Fprintln(out, "This local policy is the only enforceable limit layer (Robinhood's OAuth scope is all-or-nothing).")
				fmt.Fprintln(out, "Set one with: guard set --max-order 500 --daily-cap 2000")
				return nil
			}
			fmt.Fprintln(out, "Guard policy (enforced locally before any order leaves this machine):")
			printGuardPolicy(out, policy)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Enforced:")
			for _, line := range view.Enforced {
				fmt.Fprintf(out, "  - %s\n", line)
			}
			return nil
		},
	}
	return cmd
}

// applyGuardFlags merges the flags the user actually changed into the existing
// policy. changed keys use the flag names: "max-order", "daily-cap", "allow",
// "deny", "kill", "disarm". Allow/deny lists replace wholesale (normalized to
// trimmed uppercase, empties dropped). --kill sets the kill switch to the
// flag's value (so --kill=false also disarms); --disarm, applied last, wins
// and clears it. Pure function: never mutates its inputs.
func applyGuardFlags(existing store.GuardPolicy, changed map[string]bool, maxOrder, dailyCap float64, allow, deny []string, kill, disarm bool) store.GuardPolicy {
	p := existing
	// Copy slices so the returned policy never aliases the caller's memory.
	p.Allowlist = append([]string(nil), existing.Allowlist...)
	p.Denylist = append([]string(nil), existing.Denylist...)

	if changed["max-order"] {
		p.MaxOrderNotional = maxOrder
	}
	if changed["daily-cap"] {
		p.DailyCapNotional = dailyCap
	}
	if changed["allow"] {
		p.Allowlist = normalizeGuardSymbols(allow)
	}
	if changed["deny"] {
		p.Denylist = normalizeGuardSymbols(deny)
	}
	if changed["kill"] {
		p.KillSwitch = kill
	}
	if changed["disarm"] && disarm {
		p.KillSwitch = false
	}
	return p
}

// normalizeGuardSymbols trims, uppercases, and drops empty entries. Returns
// nil for an all-empty input so a cleared list serializes as absent.
func normalizeGuardSymbols(symbols []string) []string {
	var out []string
	for _, s := range symbols {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// guardPolicyIsEmpty reports whether the policy is the zero value (nothing
// persisted or everything cleared).
func guardPolicyIsEmpty(p store.GuardPolicy) bool {
	return p.MaxOrderNotional == 0 && p.DailyCapNotional == 0 &&
		len(p.Allowlist) == 0 && len(p.Denylist) == 0 && !p.KillSwitch
}

// guardEnforcementSummary renders human-readable lines describing what the
// policy actually enforces. Pure function.
func guardEnforcementSummary(p store.GuardPolicy) []string {
	var lines []string
	if p.KillSwitch {
		lines = append(lines, "KILL SWITCH ENGAGED: every order is blocked (guard set --disarm to clear)")
	}
	if p.MaxOrderNotional > 0 {
		lines = append(lines, fmt.Sprintf("per-order notional capped at $%.2f", p.MaxOrderNotional))
	}
	if p.DailyCapNotional > 0 {
		lines = append(lines, fmt.Sprintf("total daily notional capped at $%.2f", p.DailyCapNotional))
	}
	if len(p.Allowlist) > 0 {
		lines = append(lines, fmt.Sprintf("only these symbols may trade: %s", strings.Join(p.Allowlist, ", ")))
	}
	if len(p.Denylist) > 0 {
		lines = append(lines, fmt.Sprintf("these symbols are blocked: %s", strings.Join(p.Denylist, ", ")))
	}
	if len(lines) == 0 {
		lines = append(lines, "nothing — the policy is empty, all orders pass the guard")
	}
	return lines
}

// printGuardPolicy writes the labeled human-readable policy lines.
func printGuardPolicy(out io.Writer, p store.GuardPolicy) {
	capStr := func(v float64) string {
		if v <= 0 {
			return "unset"
		}
		return fmt.Sprintf("$%.2f", v)
	}
	list := func(v []string) string {
		if len(v) == 0 {
			return "(none)"
		}
		return strings.Join(v, ", ")
	}
	kill := "off"
	if p.KillSwitch {
		kill = "ENGAGED — all orders blocked"
	}
	fmt.Fprintf(out, "  Max order notional: %s\n", capStr(p.MaxOrderNotional))
	fmt.Fprintf(out, "  Daily cap notional: %s\n", capStr(p.DailyCapNotional))
	fmt.Fprintf(out, "  Allowlist:          %s\n", list(p.Allowlist))
	fmt.Fprintf(out, "  Denylist:           %s\n", list(p.Denylist))
	fmt.Fprintf(out, "  Kill switch:        %s\n", kill)
}
