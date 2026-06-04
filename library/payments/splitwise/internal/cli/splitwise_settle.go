package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/cliutil"
	"github.com/spf13/cobra"
)

type settleTransfer struct {
	FromID       int     `json:"from_id"`
	FromName     string  `json:"from_name"`
	ToID         int     `json:"to_id"`
	ToName       string  `json:"to_name"`
	Amount       float64 `json:"amount"`
	CurrencyCode string  `json:"currency_code"`
}

func newSettleUpCmd(flags *rootFlags) *cobra.Command {
	record := false

	cmd := &cobra.Command{
		Use:   "settle-up <group-or-friend>",
		Short: "Compute a settle-up transfer plan and optionally record payment expenses",
		Example: "  splitwise-pp-cli settle-up \"Tahoe Trip\"\n" +
			"  splitwise-pp-cli settle-up \"Tahoe Trip\" --record",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would compute settle-up plan")
				return nil
			}
			if len(args) == 0 {
				return usageErr(errors.New("group name/id or friend name is required"))
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			input := joinNameArgs(args)
			if input == "" {
				return usageErr(errors.New("group name/id or friend name is required"))
			}

			groups, err := loadGroups(db)
			if err != nil {
				return err
			}
			friends, err := loadFriends(db)
			if err != nil {
				return err
			}

			youID := loadCurrentUserID(db)
			targetType := ""
			targetName := ""
			targetGroupID := 0
			plan := make([]settleTransfer, 0)

			groupMatch, hasGroupMatch, groupAmbErr := resolveSettleGroup(input, groups)
			if isAllDigits(input) || hasGroupMatch {
				if !hasGroupMatch {
					return usageErr(fmt.Errorf("no group or friend matches %q; run sync first", input))
				}
				targetType = "group"
				targetName = strings.TrimSpace(groupMatch.Name)
				targetGroupID = groupMatch.ID

				memberNames := make(map[int]string)
				for _, m := range groupMatch.Members {
					name := strings.TrimSpace(strings.TrimSpace(m.FirstName) + " " + strings.TrimSpace(m.LastName))
					if name == "" {
						name = fmt.Sprintf("user %d", m.ID)
					}
					memberNames[m.ID] = name
				}

				for _, d := range groupMatch.SimplifiedDebts {
					amt := parseAmount(d.Amount)
					if amt == 0 {
						continue
					}
					fromName := memberNames[d.From]
					if strings.TrimSpace(fromName) == "" {
						fromName = fmt.Sprintf("user %d", d.From)
					}
					toName := memberNames[d.To]
					if strings.TrimSpace(toName) == "" {
						toName = fmt.Sprintf("user %d", d.To)
					}
					plan = append(plan, settleTransfer{
						FromID:       d.From,
						FromName:     fromName,
						ToID:         d.To,
						ToName:       toName,
						Amount:       amt,
						CurrencyCode: strings.TrimSpace(d.CurrencyCode),
					})
				}
			} else {
				// Not a unique group. Try friend before surfacing any group
				// ambiguity — a uniquely-named friend whose name is also a
				// substring of several group names should still settle.
				friendMatch, ok, friendAmbErr := resolveSettleFriend(input, friends)
				if !ok {
					switch {
					case groupAmbErr != nil:
						return usageErr(groupAmbErr)
					case friendAmbErr != nil:
						return usageErr(friendAmbErr)
					default:
						return usageErr(fmt.Errorf("no group or friend matches %q; run sync first", input))
					}
				}
				targetType = "friend"
				targetName = friendDisplayName(friendMatch)
				if targetName == "" {
					targetName = fmt.Sprintf("friend %d", friendMatch.ID)
				}

				for _, b := range friendMatch.Balance {
					amt := parseAmount(b.Amount)
					if amt == 0 {
						continue
					}
					cc := strings.TrimSpace(b.CurrencyCode)
					if amt > 0 {
						plan = append(plan, settleTransfer{
							FromID:       friendMatch.ID,
							FromName:     targetName,
							ToID:         youID,
							ToName:       "you",
							Amount:       amt,
							CurrencyCode: cc,
						})
					} else {
						plan = append(plan, settleTransfer{
							FromID:       youID,
							FromName:     "you",
							ToID:         friendMatch.ID,
							ToName:       targetName,
							Amount:       -amt,
							CurrencyCode: cc,
						})
					}
				}
			}

			out := map[string]any{
				"target_type": targetType,
				"target_name": targetName,
				"transfers":   plan,
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				if err := flags.emitStructured(cmd, out); err != nil {
					return err
				}
			} else {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
				for _, t := range plan {
					_, _ = fmt.Fprintf(tw, "%s -> %s: %.2f %s\n", settleDisplayName(t.FromName), settleDisplayName(t.ToName), t.Amount, t.CurrencyCode)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}

			if !record {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "plan only — re-run with --record to create %d payment expense(s)\n", len(plan))
				return nil
			}

			if cliutil.IsVerifyEnv() {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "would record %d payment(s) (verify mode)\n", len(plan))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type recordedPayment struct {
				From   string `json:"from"`
				To     string `json:"to"`
				Amount string `json:"amount"`
				Code   int    `json:"status_code"`
			}
			recorded := make([]recordedPayment, 0)

			for _, t := range plan {
				if targetType == "friend" && (t.FromID == 0 || t.ToID == 0) {
					return fmt.Errorf("friend settle-up --record needs both user ids; record this payment in the app or via create-expense")
				}

				cost := fmt.Sprintf("%.2f", t.Amount)
				users := []map[string]any{
					{
						"user_id":    t.FromID,
						"paid_share": cost,
						"owed_share": "0.00",
					},
					{
						"user_id":    t.ToID,
						"paid_share": "0.00",
						"owed_share": cost,
					},
				}
				body := map[string]any{
					"payment":       true,
					"cost":          cost,
					"currency_code": t.CurrencyCode,
					"users":         users,
				}
				if targetType == "group" {
					body["group_id"] = targetGroupID
				}

				// Splitwise has no atomic multi-expense API. If a transfer
				// fails mid-loop, the earlier ones are already posted; surface
				// how many succeeded so the user can reconcile the remainder in
				// the app rather than silently losing the partial-progress count.
				respData, statusCode, postErr := c.Post(cmd.Context(), "/create_expense", body)
				if postErr != nil {
					return fmt.Errorf("recorded %d of %d transfer(s) before %s -> %s failed: %w", len(recorded), len(plan), t.FromName, t.ToName, postErr)
				}
				if statusCode < 200 || statusCode >= 300 {
					return fmt.Errorf("recorded %d of %d transfer(s); transfer %s -> %s %.2f %s failed: status %d", len(recorded), len(plan), t.FromName, t.ToName, t.Amount, t.CurrencyCode, statusCode)
				}
				// Splitwise returns HTTP 200 with a non-empty "errors" body when
				// the create is rejected, so the status check above is not
				// sufficient — inspect the body too.
				if envErr := splitwiseMutationError(respData); envErr != nil {
					return fmt.Errorf("recorded %d of %d transfer(s); transfer %s -> %s rejected: %w", len(recorded), len(plan), t.FromName, t.ToName, envErr)
				}
				recorded = append(recorded, recordedPayment{
					From:   t.FromName,
					To:     t.ToName,
					Amount: fmt.Sprintf("%.2f %s", t.Amount, t.CurrencyCode),
					Code:   statusCode,
				})
			}

			summary := map[string]any{
				"recorded_payments": recorded,
				"count":             len(recorded),
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.emitStructured(cmd, summary)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created %d payment expense(s)\n", len(recorded))
			return nil
		},
	}

	cmd.Flags().BoolVar(&record, "record", false, "Create payment expenses from the computed plan")
	return cmd
}

// joinNameArgs derives a single group/friend name from positional args by joining
// them with spaces, so a multi-word name survives whitespace-splitting (e.g. the
// MCP command-mirror tokenizes `args:"EDCO 2021"` into ["EDCO","2021"]). Joining
// reassembles "EDCO 2021" so the exact-match path resolves it to the one group,
// instead of the bare prefix "EDCO" substring-matching several. Shared by the
// name-positional commands (settle-up, resolve).
func joinNameArgs(args []string) string {
	return strings.TrimSpace(strings.Join(args, " "))
}

// matchGroupsByName returns groups matching input with exact-match preference:
// if any group's name equals input (case-insensitive), only those exact matches
// are returned; otherwise all case-insensitive substring matches are returned.
// Callers decide none/one/ambiguous so a name that matches several groups (e.g.
// "Shy 25" → three trips) errors instead of silently resolving to the first.
func matchGroupsByName(input string, groups []Group) []Group {
	needle := strings.ToLower(strings.TrimSpace(input))
	var exact, substr []Group
	for _, g := range groups {
		name := strings.ToLower(strings.TrimSpace(g.Name))
		switch {
		case name == needle:
			exact = append(exact, g)
		case strings.Contains(name, needle):
			substr = append(substr, g)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return substr
}

// matchFriendsByName mirrors matchGroupsByName for friends, matching on first,
// last, or full name with exact-match preference.
func matchFriendsByName(input string, friends []Friend) []Friend {
	needle := strings.ToLower(strings.TrimSpace(input))
	var exact, substr []Friend
	for _, f := range friends {
		first := strings.ToLower(strings.TrimSpace(f.FirstName))
		last := strings.ToLower(strings.TrimSpace(f.LastName))
		full := strings.TrimSpace(first + " " + last)
		switch {
		case needle == full || needle == first || needle == last:
			exact = append(exact, f)
		case strings.Contains(first, needle) || strings.Contains(last, needle) || strings.Contains(full, needle):
			substr = append(substr, f)
		}
	}
	if len(exact) > 0 {
		return exact
	}
	return substr
}

func ambiguousGroupErr(input string, matches []Group) error {
	const maxShown = 5
	capacity := len(matches)
	if capacity > maxShown {
		capacity = maxShown
	}
	parts := make([]string, 0, capacity)
	for i, g := range matches {
		if i >= maxShown {
			break
		}
		parts = append(parts, fmt.Sprintf("%q (id %d)", strings.TrimSpace(g.Name), g.ID))
	}
	suffix := ""
	if len(matches) > maxShown {
		suffix = fmt.Sprintf("; … and %d more", len(matches)-maxShown)
	}
	return fmt.Errorf("%q is ambiguous — matches %d groups: %s%s. Re-run with a numeric group id or the exact name", strings.TrimSpace(input), len(matches), strings.Join(parts, "; "), suffix)
}

func ambiguousFriendErr(input string, matches []Friend) error {
	const maxShown = 5
	capacity := len(matches)
	if capacity > maxShown {
		capacity = maxShown
	}
	parts := make([]string, 0, capacity)
	for i, f := range matches {
		if i >= maxShown {
			break
		}
		parts = append(parts, fmt.Sprintf("%q (id %d)", friendDisplayName(f), f.ID))
	}
	suffix := ""
	if len(matches) > maxShown {
		suffix = fmt.Sprintf("; … and %d more", len(matches)-maxShown)
	}
	return fmt.Errorf("%q is ambiguous — matches %d friends: %s%s. Re-run with the exact name", strings.TrimSpace(input), len(matches), strings.Join(parts, "; "), suffix)
}

// resolveSettleGroup resolves a group by numeric id or name. The bool reports a
// unique match; a non-nil error means the name was ambiguous (multiple matches)
// and the caller must not silently fall through to another resolution path.
func resolveSettleGroup(input string, groups []Group) (Group, bool, error) {
	trimmed := strings.TrimSpace(input)
	if isAllDigits(trimmed) {
		id, _ := strconv.Atoi(trimmed)
		for _, g := range groups {
			if g.ID == id {
				return g, true, nil
			}
		}
		return Group{}, false, nil
	}

	matches := matchGroupsByName(input, groups)
	switch len(matches) {
	case 0:
		return Group{}, false, nil
	case 1:
		return matches[0], true, nil
	default:
		return Group{}, false, ambiguousGroupErr(input, matches)
	}
}

func resolveSettleFriend(input string, friends []Friend) (Friend, bool, error) {
	matches := matchFriendsByName(input, friends)
	switch len(matches) {
	case 0:
		return Friend{}, false, nil
	case 1:
		return matches[0], true, nil
	default:
		return Friend{}, false, ambiguousFriendErr(input, matches)
	}
}

func settleDisplayName(name string) string {
	if strings.EqualFold(strings.TrimSpace(name), "you") {
		return "You"
	}
	return strings.TrimSpace(name)
}
