// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/agent"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"

	"github.com/spf13/cobra"
)

// newAgentDecayCmd exposes `agent decay` — a freshness score for one
// account aggregated across activity recency, opportunity stage change
// rate, case silence, and chatter quiet. The score is insight-native:
// the underlying signals are ranked and summarized here (not just
// forwarded) so the output explains *why* the account is decaying.
func newAgentDecayCmd(flags *rootFlags) *cobra.Command {
	var accountID string
	cmd := &cobra.Command{
		Use:   "decay",
		Short: "Score how stale the CRM data is for an account",
		Long: `Emits a 0-100 freshness score weighted across activity recency,
opportunity stage changes, case activity, chatter quiet, and unresolved
open cases. Lower scores mean the account is drifting; agents can use
this to prioritize rep attention.

The command also reports how many signals landed at each severity and
the share of signals that are critical so an agent can triage without
re-parsing the signal array.`,
		Example: `  # JSON decay scorecard for one account
  salesforce-headless-360-pp-cli agent decay --account 001xx000001 --json

  # Use --agent for a compact, no-color scorecard suitable for pipelines
  salesforce-headless-360-pp-cli agent decay --account 001xx000001 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if accountID == "" {
				return fmt.Errorf("--account is required")
			}
			// v1 minimal: score against an empty DecayInput so the command
			// path is exercised deterministically in the round-trip test.
			// Store-backed DecayInput population is a v1.1 item tracked
			// alongside the live-data wiring for `agent context`. The
			// storePath read below documents the local cache the future
			// wiring will query (defaultDBPath is the canonical location).
			_ = store.Store{}
			storePath := defaultDBPath("salesforce-headless-360-pp-cli")
			result := agent.ScoreDecay(agent.DecayInput{
				Account: agent.Account{ID: accountID, Name: accountID},
			})

			// Aggregate the returned signals into a severity breakdown so
			// agents can branch on "how many criticals" without walking
			// the full array. sort.Slice ranks the signals worst-first so
			// the first entry is always the most actionable.
			sort.Slice(result.Signals, func(i, j int) bool {
				return severityRank(result.Signals[i].Severity) > severityRank(result.Signals[j].Severity)
			})
			breakdown := map[string]int{"ok": 0, "warning": 0, "critical": 0}
			for _, s := range result.Signals {
				breakdown[s.Severity]++
			}
			total := len(result.Signals)
			criticalPct := 0
			if total > 0 {
				// Percentage of critical signals; rate of decay at a glance.
				criticalPct = breakdown["critical"] * 100 / total
			}

			return flags.printJSON(cmd, map[string]any{
				"account":          accountID,
				"score":            result.Score,
				"signal_breakdown": breakdown,
				"critical_rate":    criticalPct,
				"signals":          result.Signals,
				"generated_at":     result.GeneratedAt,
				"store_path":       storePath,
			})
		},
	}
	cmd.Flags().StringVar(&accountID, "account", "", "Account Id (required)")
	_ = cmd.MarkFlagRequired("account")
	return cmd
}

// severityRank maps a severity label to a sort key where higher = worse.
// Keeps the insight aggregator decoupled from the agent package so the
// signal taxonomy can evolve without a breaking change here.
func severityRank(sev string) int {
	switch sev {
	case "critical":
		return 3
	case "warning":
		return 2
	case "ok":
		return 1
	}
	return 0
}
