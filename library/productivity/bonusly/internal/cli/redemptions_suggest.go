// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/bonusly/internal/cliutil"
	"github.com/spf13/cobra"
)

// redemptionHistoryRow is one redemption from the authenticated account.
type redemptionHistoryRow struct {
	RewardName string `json:"reward_name"`
	State      string `json:"state"`
	CreatedAt  string `json:"created_at"`
}

// redemptionSuggestion is one ranked entry in `redemptions suggest` output:
// a reward name pulled from the authenticated user's redemption history,
// ranked by how often and how recently they redeemed it.
//
// Bonusly exposes no live rewards-catalog endpoint with a price (see
// README.md "Known Gaps" #1 -- the awards/incentives catalog was live-probed
// across ~20 path variants and found unreachable, and the Redemption type
// itself has no cost field), so this cannot filter by what the user can
// literally afford. It resurfaces what they've actually redeemed before as
// a memory jog, shown next to their current balance.
type redemptionSuggestion struct {
	RewardName     string `json:"reward_name"`
	TimesRedeemed  int    `json:"times_redeemed"`
	LastRedeemedAt string `json:"last_redeemed_at"`
	LastState      string `json:"last_state"`
}

// nonSuccessfulRedemptionStates is a conservative denylist of state values
// that unambiguously mean a redemption did not complete. Bonusly's real
// state enum is unconfirmed anywhere in this CLI's spec/docs (bonusly-spec.yaml
// discloses every field as inferred-or-unconfirmed; the Redemption type
// carries no documented enum), so this intentionally excludes only
// clear-cut negative-outcome words rather than guessing a full allowlist --
// an unrecognized or empty state is treated as evidence of a real
// redemption rather than silently dropped.
var nonSuccessfulRedemptionStates = []string{
	"pending", "denied", "rejected", "failed", "cancelled", "canceled", "expired",
}

// looksLikeCompletedRedemption reports whether state does not match any
// entry in nonSuccessfulRedemptionStates. Used to keep a handful of pending
// or failed claim attempts from outranking rewards the user has actually
// received (see PR review discussion: a reward retried several times while
// pending would otherwise inflate times_redeemed above rewards that
// completed on the first try).
func looksLikeCompletedRedemption(state string) bool {
	lower := strings.ToLower(strings.TrimSpace(state))
	for _, bad := range nonSuccessfulRedemptionStates {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return true
}

// rankRedemptionSuggestions aggregates raw redemption rows by reward
// name and ranks them most-frequently-redeemed first, breaking ties by most
// recent redemption. Rows whose state clearly indicates the redemption
// never completed (see looksLikeCompletedRedemption) are excluded entirely
// -- a reward with only pending/denied/failed attempts has no evidence of
// a successful past redemption, so it should not be suggested "again".
// Pure function (no I/O) so the ranking logic is independently unit-testable
// without a live client or local database.
func rankRedemptionSuggestions(rows []redemptionHistoryRow) []redemptionSuggestion {
	byName := map[string]*redemptionSuggestion{}
	var order []string
	for _, r := range rows {
		if !looksLikeCompletedRedemption(r.State) {
			continue
		}
		name := r.RewardName
		if name == "" {
			name = "(unnamed reward)"
		}
		s, ok := byName[name]
		if !ok {
			s = &redemptionSuggestion{RewardName: name}
			byName[name] = s
			order = append(order, name)
		}
		s.TimesRedeemed++
		if r.CreatedAt >= s.LastRedeemedAt {
			s.LastRedeemedAt = r.CreatedAt
			s.LastState = r.State
		}
	}

	out := make([]redemptionSuggestion, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TimesRedeemed != out[j].TimesRedeemed {
			return out[i].TimesRedeemed > out[j].TimesRedeemed
		}
		return out[i].LastRedeemedAt > out[j].LastRedeemedAt
	})
	return out
}

// noAffordabilityNote is surfaced on every non-empty response so agent
// callers never mistake "ranked by frequency" for "ranked by what you can
// afford" -- see the redemptionSuggestion doc comment for why no price data
// is available.
const noAffordabilityNote = "Bonusly exposes no live rewards-catalog endpoint with prices, so this can't tell you what you can literally afford -- these are your own past redemptions, ranked by how often and how recently you redeemed them, shown next to your current balance as a memory jog."

const noHistoryNote = "no redemption history returned for the authenticated account"

func newNovelRedemptionsSuggestCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "suggest",
		Short: "Suggest rewards to redeem again from your own history, next to your current point balance.",
		Long: `Suggest rewards to redeem again from your own history, next to your current point balance.

Bonusly's API exposes no live rewards-catalog endpoint with prices (see README.md "Known Gaps" -- the awards/incentives catalog was live-probed across ~20 path variants and found unreachable), so this cannot tell you what you can literally afford. This command requires live access to the selected authenticated account; shared local history and cached balances are not used. Instead it shows your current redeemable balance next to the reward names you've redeemed before, ranked by how often and how recently, as a memory jog for your next redemption.`,
		Example:     "  bonusly-pp-cli redemptions suggest --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagLimit < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be zero or greater (0 means no limit); got %d", flagLimit))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read the authenticated account and its redemption history without using shared local caches")
				return nil
			}

			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			balanceRaw, history, err := fetchRedemptionSuggestionInputs(cmd.Context(), c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			earningBalance, givingBalance, balanceOK := parseBonuslyBalanceFields(balanceRaw)
			balanceUnavailable := ""
			if !balanceOK {
				balanceUnavailable = "authenticated account response did not include point balances"
			}

			suggestions := rankRedemptionSuggestions(history)
			if flagLimit > 0 && len(suggestions) > flagLimit {
				suggestions = suggestions[:flagLimit]
			}

			var note string
			switch {
			case len(history) == 0:
				note = noHistoryNote
			case len(suggestions) == 0:
				note = "no completed redemptions found in the authenticated account history (entries present are pending/denied/failed/expired) -- see: bonusly-pp-cli redemptions list-mine"
			default:
				note = noAffordabilityNote
			}

			if flags.asJSON || flags.agent {
				res := map[string]any{
					"earning_balance":     earningBalance,
					"giving_balance":      givingBalance,
					"balance_unavailable": balanceUnavailable,
					"suggestions":         suggestions,
					"note":                note,
				}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			tw := newTabWriter(cmd.OutOrStdout())
			earning := "unknown"
			if earningBalance != nil {
				earning = fmt.Sprintf("%d", *earningBalance)
			}
			fmt.Fprintf(tw, "CURRENT REDEEMABLE BALANCE\t%s\n", earning)
			if balanceUnavailable != "" {
				fmt.Fprintf(tw, "BALANCE NOTE\t%s\n", cliutil.ScrubTerminal(balanceUnavailable))
			}
			fmt.Fprintln(tw)
			if len(suggestions) == 0 {
				fmt.Fprintf(tw, "NOTE\t%s\n", note)
			} else {
				fmt.Fprintf(tw, "REWARD\tTIMES REDEEMED\tLAST REDEEMED\tLAST STATE\n")
				for _, s := range suggestions {
					// Reward name and state are remote-controlled strings
					// (Bonusly-side reward catalog / redemption state);
					// scrub before writing to a terminal so an org-controlled
					// value containing tabs/newlines/ANSI-OSC bytes cannot
					// inject table rows or trigger escape-sequence behavior.
					fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", cliutil.ScrubTerminal(s.RewardName), s.TimesRedeemed, s.LastRedeemedAt, cliutil.ScrubTerminal(s.LastState))
				}
			}
			_ = tw.Flush()
			if len(suggestions) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "\n%s\n", note)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 5, "Max number of suggestions to show (0 = no limit)")

	return cmd
}

// parseBonuslyBalanceFields extracts earning_balance/giving_balance from a
// /users/me response, tolerating both the documented {"result": {...}}
// envelope and a bare unwrapped object -- balance's response shape has
// drifted from the spec's assumptions before (pp:hand-edit
// bonusly-endpoint-fix in promoted_balance.go). ok is false only when
// neither shape yields either field, so the caller can tell "parsed
// successfully, fields absent" (ok=true, both nil) apart from "could not
// find the fields at all" (ok=false).
func parseBonuslyBalanceFields(raw json.RawMessage) (earningBalance, givingBalance *int64, ok bool) {
	var envelope struct {
		Result struct {
			EarningBalance *int64 `json:"earning_balance"`
			GivingBalance  *int64 `json:"giving_balance"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil &&
		(envelope.Result.EarningBalance != nil || envelope.Result.GivingBalance != nil) {
		return envelope.Result.EarningBalance, envelope.Result.GivingBalance, true
	}

	var bare struct {
		EarningBalance *int64 `json:"earning_balance"`
		GivingBalance  *int64 `json:"giving_balance"`
	}
	if err := json.Unmarshal(raw, &bare); err == nil &&
		(bare.EarningBalance != nil || bare.GivingBalance != nil) {
		return bare.EarningBalance, bare.GivingBalance, true
	}
	return nil, nil, false
}
