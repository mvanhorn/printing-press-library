package cli

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func newResolveCmd(flags *rootFlags) *cobra.Command {
	var resolveType string

	cmd := &cobra.Command{
		Use:     "resolve <name>",
		Short:   "Resolve a local name to an id",
		Example: "  splitwise-pp-cli resolve \"Alex Kim\" --agent",
		// no-error-path-probe: a name matching nothing is a valid empty result
		// (exit 0, empty list), not an error, so the generic invalid-argument
		// probe does not apply.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				target := "name"
				if len(args) > 0 {
					target = strings.TrimSpace(args[0])
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "would resolve %s\n", target)
				return nil
			}
			if len(args) == 0 {
				return usageErr(errors.New("name argument is required"))
			}
			name := strings.TrimSpace(args[0])

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			resolveOne := func(kind, input string) (map[string]any, error) {
				var fields []string
				switch kind {
				case "friend":
					fields = []string{"first_name", "last_name"}
					id, err := db.ResolveByName("get-friends", input, fields...)
					if err != nil {
						return nil, err
					}
					return map[string]any{"type": "friend", "id": id, "name_input": input}, nil
				case "group":
					id, err := db.ResolveByName("get-groups", input, "name")
					if err != nil {
						return nil, err
					}
					return map[string]any{"type": "group", "id": id, "name_input": input}, nil
				case "category":
					id, err := db.ResolveByName("get-categories", input, "name")
					if err != nil {
						return nil, err
					}
					return map[string]any{"type": "category", "id": id, "name_input": input}, nil
				default:
					return nil, fmt.Errorf("unsupported type %q", kind)
				}
			}

			if resolveType != "" {
				item, err := resolveOne(resolveType, name)
				if err != nil {
					return err
				}
				if flags.asJSON || flags.agent {
					return flags.printJSON(cmd, item)
				}
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "TYPE\tID\tNAME INPUT")
				_, _ = fmt.Fprintf(tw, "%v\t%v\t%v\n", item["type"], item["id"], item["name_input"])
				return tw.Flush()
			}

			results := make([]map[string]any, 0)
			for _, kind := range []string{"friend", "group", "category"} {
				item, err := resolveOne(kind, name)
				if err != nil {
					continue
				}
				results = append(results, item)
			}
			if flags.asJSON || flags.agent {
				return flags.printJSON(cmd, results)
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "TYPE\tID\tNAME INPUT")
			for _, item := range results {
				_, _ = fmt.Fprintf(tw, "%v\t%v\t%v\n", item["type"], item["id"], item["name_input"])
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&resolveType, "type", "", "Type to resolve: friend|group|category")
	return cmd
}

func newBalancesCmd(flags *rootFlags) *cobra.Command {
	var byCurrency bool
	cmd := &cobra.Command{
		Use:         "balances",
		Short:       "Show net friend balances",
		Example:     "  splitwise-pp-cli balances --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would compute balances")
				return nil
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-friends")
			hintIfStale(cmd, db, "get-friends", flags.maxAge)

			friends, err := loadFriends(db)
			if err != nil {
				return err
			}

			type currencyAgg struct {
				CurrencyCode string  `json:"currency_code"`
				OwedToYou    float64 `json:"owed_to_you"`
				YouOwe       float64 `json:"you_owe"`
				Net          float64 `json:"net"`
			}
			type friendRow struct {
				ID           int     `json:"id"`
				Name         string  `json:"name"`
				CurrencyCode string  `json:"currency_code"`
				Amount       float64 `json:"amount"`
			}

			agg := make(map[string]*currencyAgg)
			friendsOut := make([]friendRow, 0)

			for _, f := range friends {
				name := friendDisplayName(f)
				for _, b := range f.Balance {
					amt := parseAmount(b.Amount)
					if _, ok := agg[b.CurrencyCode]; !ok {
						agg[b.CurrencyCode] = &currencyAgg{CurrencyCode: b.CurrencyCode}
					}
					c := agg[b.CurrencyCode]
					if amt > 0 {
						c.OwedToYou += amt
					} else if amt < 0 {
						c.YouOwe += -amt
					}
					c.Net += amt
					if amt != 0 {
						friendsOut = append(friendsOut, friendRow{ID: f.ID, Name: name, CurrencyCode: b.CurrencyCode, Amount: round2(amt)})
					}
				}
			}

			byCurrencyOut := make([]currencyAgg, 0)
			for _, v := range agg {
				v.OwedToYou = round2(v.OwedToYou)
				v.YouOwe = round2(v.YouOwe)
				v.Net = round2(v.Net)
				byCurrencyOut = append(byCurrencyOut, *v)
			}
			sort.Slice(byCurrencyOut, func(i, j int) bool {
				return byCurrencyOut[i].CurrencyCode < byCurrencyOut[j].CurrencyCode
			})
			sort.Slice(friendsOut, func(i, j int) bool {
				ai := math.Abs(friendsOut[i].Amount)
				aj := math.Abs(friendsOut[j].Amount)
				if ai == aj {
					return friendsOut[i].Name < friendsOut[j].Name
				}
				return ai > aj
			})

			out := map[string]any{
				"by_currency": byCurrencyOut,
			}
			// --by-currency narrows output to the per-currency totals only,
			// omitting the per-friend breakdown (useful when an agent just
			// wants the headline net position).
			if !byCurrency {
				out["friends"] = friendsOut
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "CURRENCY\tOWED TO YOU\tYOU OWE\tNET")
			for _, row := range byCurrencyOut {
				_, _ = fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%.2f\n", row.CurrencyCode, row.OwedToYou, row.YouOwe, row.Net)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if byCurrency {
				return nil
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			tw2 := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw2, "FRIEND\tCURRENCY\tAMOUNT")
			for _, row := range friendsOut {
				_, _ = fmt.Fprintf(tw2, "%s\t%s\t%.2f\n", row.Name, row.CurrencyCode, row.Amount)
			}
			return tw2.Flush()
		},
	}
	cmd.Flags().BoolVar(&byCurrency, "by-currency", false, "Show only the per-currency net totals (omit the per-friend breakdown)")
	return cmd
}

func newDebtsCmd(flags *rootFlags) *cobra.Command {
	var aged bool
	cmd := &cobra.Command{
		Use:         "debts",
		Short:       "Show who owes and debt age",
		Example:     "  splitwise-pp-cli debts --aged --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would compute debts")
				return nil
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-friends")
			hintIfUnsynced(cmd, db, "get-expenses")
			hintIfStale(cmd, db, "get-friends", flags.maxAge)
			hintIfStale(cmd, db, "get-expenses", flags.maxAge)

			friends, err := loadFriends(db)
			if err != nil {
				return err
			}
			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}

			type debtRow struct {
				FriendID          int    `json:"friend_id"`
				Name              string `json:"name"`
				CurrencyCode      string `json:"currency_code"`
				Amount            string `json:"amount"`
				Direction         string `json:"direction"`
				OldestExpenseDate string `json:"oldest_expense_date"`
				// AgeDays is nil when the oldest contributing expense is not
				// in the synced window (sync defaults to one recent page, so
				// older debts have no matching expense). Emitting null rather
				// than a sentinel int keeps agents from reading "-1" as a real
				// age.
				AgeDays *int `json:"age_days"`
			}
			results := make([]debtRow, 0)

			for _, f := range friends {
				name := friendDisplayName(f)
				for _, b := range f.Balance {
					amt := parseAmount(b.Amount)
					if amt == 0 {
						continue
					}
					oldest, oldestRaw, _, parsed := oldestExpenseForFriend(expenses, f.ID)
					var ageDays *int // nil: no matching expense or unparseable date
					if parsed {
						d := int(time.Since(oldest).Hours() / 24)
						if d < 0 {
							d = 0 // clamp future-dated expenses
						}
						ageDays = &d
					}
					direction := "you_owe_them"
					if amt > 0 {
						direction = "they_owe_you"
					}
					results = append(results, debtRow{
						FriendID:          f.ID,
						Name:              name,
						CurrencyCode:      b.CurrencyCode,
						Amount:            fmt.Sprintf("%.2f", round2(amt)),
						Direction:         direction,
						OldestExpenseDate: oldestRaw,
						AgeDays:           ageDays,
					})
				}
			}

			if aged {
				// Two tiers: rows with a known age sort oldest-first; rows
				// with an unknown age (debt older than the synced window)
				// follow, sorted by amount so the largest unresolved balance
				// leads its tier instead of being buried by comparing a
				// sentinel age.
				sort.Slice(results, func(i, j int) bool {
					ki, kj := results[i].AgeDays != nil, results[j].AgeDays != nil
					if ki != kj {
						return ki // known-age rows rank ahead of unknown-age rows
					}
					if ki && kj && *results[i].AgeDays != *results[j].AgeDays {
						return *results[i].AgeDays > *results[j].AgeDays
					}
					return math.Abs(parseAmount(results[i].Amount)) > math.Abs(parseAmount(results[j].Amount))
				})
			} else {
				sort.Slice(results, func(i, j int) bool {
					ai := math.Abs(parseAmount(results[i].Amount))
					aj := math.Abs(parseAmount(results[j].Amount))
					if ai == aj {
						return results[i].Name < results[j].Name
					}
					return ai > aj
				})
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, results)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "FRIEND\tCURRENCY\tAMOUNT\tDIRECTION\tOLDEST EXPENSE\tAGE DAYS")
			for _, row := range results {
				// AgeDays is a *int: nil means the oldest contributing expense
				// is outside the synced window, so the age is genuinely unknown.
				// Render the dereferenced value, and a "-" placeholder for nil —
				// printing the pointer with %d would emit the address for known
				// ages and a misleading 0 for unknown ones.
				ageCell := "-"
				if row.AgeDays != nil {
					ageCell = strconv.Itoa(*row.AgeDays)
				}
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", row.Name, row.CurrencyCode, row.Amount, row.Direction, row.OldestExpenseDate, ageCell)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&aged, "aged", false, "Sort debts by age descending")
	return cmd
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// oldestExpenseForFriend finds the oldest non-payment, non-deleted expense
// involving friendID. It returns: oldest (the parsed date, valid only when
// parsed), raw (the original date string for display), found (a matching
// expense existed at all), and parsed (the oldest date actually parsed).
// Callers must not derive an age from oldest unless parsed is true — an
// unparseable date otherwise yields a zero-time age of ~2000 years that would
// corrupt age-sorted output.
func oldestExpenseForFriend(expenses []Expense, friendID int) (oldest time.Time, raw string, found bool, parsed bool) {
	for _, e := range expenses {
		if e.Payment || e.DeletedAt != nil {
			continue
		}
		matched := false
		for _, u := range e.Users {
			if u.UserID == friendID {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		found = true
		t, ok := parseSplitwiseDate(e.Date)
		if ok {
			if !parsed || t.Before(oldest) {
				oldest = t
				raw = e.Date
				parsed = true
			}
			continue
		}
		if !parsed && raw == "" {
			raw = e.Date
		}
	}
	return oldest, raw, found, parsed
}

func parseSplitwiseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
