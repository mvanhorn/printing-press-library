// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
	"github.com/spf13/cobra"
)

type briefCurrency struct {
	Currency  string  `json:"currency"`
	OwedToYou float64 `json:"owed_to_you"`
	YouOwe    float64 `json:"you_owe"`
}
type briefDebt struct {
	Friend    string  `json:"friend"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
	DaysOpen  *int    `json:"days_open"`
	Direction string  `json:"direction"`
}
type briefOutput struct {
	AsOf *time.Time `json:"as_of"`
	Net  struct {
		OwedToYou  float64         `json:"owed_to_you"`
		YouOwe     float64         `json:"you_owe"`
		Currency   string          `json:"currency"`
		ByCurrency []briefCurrency `json:"by_currency"`
	} `json:"net"`
	StalestDebts []briefDebt `json:"stalest_debts"`
	Changes      struct {
		NewExpenses     int `json:"new_expenses"`
		UpdatedExpenses int `json:"updated_expenses"`
		DeletedExpenses int `json:"deleted_expenses"`
		Notifications   int `json:"notifications"`
	} `json:"recent_changes"`
	Since    string `json:"since"`
	NextStep string `json:"next_step"`
}

func newNovelBriefCmd(flags *rootFlags) *cobra.Command {
	top := 5
	since := "7d"
	dbPath := ""
	cmd := &cobra.Command{
		Use: "brief", Short: "One compact digest: net position, stalest debts, and recent changes",
		Long:        "Use this command for a one-shot compact \"what does the user need to know\" state. Do NOT use it when the question is specifically balances, aging, or recent changes; use 'balances', 'debts --aged', or 'activity' for the full detail.",
		Example:     "  splitwise-pp-cli brief --agent\n  splitwise-pp-cli brief --top 3 --json",
		Annotations: map[string]string{"mcp:read-only": "true"}, Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			if top < 0 {
				return novelErr(cmd, flags, usageErr(fmt.Errorf("--top must be non-negative")))
			}
			dur, err := cliutil.ParseDurationLoose(since)
			if err != nil || dur <= 0 {
				return novelErr(cmd, flags, usageErr(fmt.Errorf("invalid --since %q: must be a positive duration", since)))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "brief")
			}
			path := dbPath
			if path == "" {
				path = defaultDBPath("splitwise-pp-cli")
			}
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "hint: local mirror is missing; run 'splitwise-pp-cli sync --resources get-groups,get-friends,get-expenses'")
				return flags.emitStructured(cmd, emptyBriefSince(time.Now(), dur))
			} else if err != nil {
				return err
			}
			db, err := openSplitwiseStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var mirrorRows, syncRows int
			if err := db.DB().QueryRow(`SELECT (SELECT COUNT(*) FROM resources), (SELECT COUNT(*) FROM sync_state)`).Scan(&mirrorRows, &syncRows); err != nil {
				return err
			}
			if mirrorRows == 0 && syncRows == 0 {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "hint: local mirror is missing; run 'splitwise-pp-cli sync --resources get-groups,get-friends,get-expenses'")
				return flags.emitStructured(cmd, emptyBriefSince(time.Now(), dur))
			}
			if !hintIfUnsynced(cmd, db, "") {
				hintIfStale(cmd, db, "", flags.maxAge)
			}
			out, err := buildBrief(db, top, time.Now(), dur)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return flags.emitStructured(cmd, out)
			}
			return printBriefHuman(cmd, out)
		},
	}
	cmd.Flags().IntVar(&top, "top", 5, "Maximum number of stalest debts to include")
	cmd.Flags().StringVar(&since, "since", "7d", "Recency window for changes (e.g. 24h, 7d, 4w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func emptyBrief() briefOutput {
	var out briefOutput
	out.Net.ByCurrency, out.StalestDebts = make([]briefCurrency, 0), make([]briefDebt, 0)
	out.NextStep = "Nothing owed; nothing to do"
	return out
}

// emptyBriefSince is the never-synced payload with the recency window still
// populated, so `since` matches what activity/forecast report for the same
// empty store; as_of stays null because no sync time exists yet.
func emptyBriefSince(now time.Time, window time.Duration) briefOutput {
	out := emptyBrief()
	out.Since = now.UTC().Add(-window).Format(time.RFC3339)
	return out
}

func buildBrief(db *store.Store, top int, now time.Time, window time.Duration) (briefOutput, error) {
	out := emptyBrief()
	asOf, err := briefSyncTime(db)
	if err != nil {
		return out, err
	}
	if asOf.Valid {
		t := asOf.Time.UTC()
		out.AsOf = &t
	}
	friends, err := loadFriends(db)
	if err != nil {
		return out, err
	}
	expenses, err := loadExpenses(db)
	if err != nil {
		return out, err
	}
	agg, debts := make(map[string]*briefCurrency), make([]briefDebt, 0)
	for _, f := range friends {
		for _, b := range f.Balance {
			amount := parseAmount(b.Amount)
			if amount == 0 {
				continue
			}
			c := agg[b.CurrencyCode]
			if c == nil {
				c = &briefCurrency{Currency: b.CurrencyCode}
				agg[b.CurrencyCode] = c
			}
			if amount > 0 {
				c.OwedToYou += amount
			} else {
				c.YouOwe -= amount
			}
			oldest, _, _, parsed := oldestExpenseForFriend(expenses, f.ID)
			var days *int
			if parsed {
				d := int(now.Sub(oldest).Hours() / 24)
				if d < 0 {
					d = 0
				}
				days = &d
			}
			direction := "you_owe"
			if amount > 0 {
				direction = "owes_you"
			}
			debts = append(debts, briefDebt{friendDisplayName(f), round2(math.Abs(amount)), b.CurrencyCode, days, direction})
		}
	}
	for _, c := range agg {
		c.OwedToYou, c.YouOwe = round2(c.OwedToYou), round2(c.YouOwe)
		out.Net.ByCurrency = append(out.Net.ByCurrency, *c)
	}
	sort.Slice(out.Net.ByCurrency, func(i, j int) bool { return out.Net.ByCurrency[i].Currency < out.Net.ByCurrency[j].Currency })
	if len(out.Net.ByCurrency) > 0 {
		primary := out.Net.ByCurrency[0]
		for _, c := range out.Net.ByCurrency[1:] {
			if c.OwedToYou+c.YouOwe > primary.OwedToYou+primary.YouOwe {
				primary = c
			}
		}
		out.Net.Currency, out.Net.OwedToYou, out.Net.YouOwe = primary.Currency, primary.OwedToYou, primary.YouOwe
	}
	sort.SliceStable(debts, func(i, j int) bool {
		if (debts[i].DaysOpen != nil) != (debts[j].DaysOpen != nil) {
			return debts[i].DaysOpen != nil
		}
		if debts[i].DaysOpen != nil && *debts[i].DaysOpen != *debts[j].DaysOpen {
			return *debts[i].DaysOpen > *debts[j].DaysOpen
		}
		if debts[i].Amount != debts[j].Amount {
			return debts[i].Amount > debts[j].Amount
		}
		return debts[i].Friend < debts[j].Friend
	})
	if len(debts) > top {
		debts = debts[:top]
	}
	out.StalestDebts = debts
	out.Since = now.UTC().Add(-window).Format(time.RFC3339)
	if err := countBriefChanges(db, now.UTC().Add(-window), &out); err != nil {
		return out, err
	}
	if len(debts) > 0 {
		out.NextStep = "Run debts --aged for the full list; run activity --since " + window.String() + " for recent details"
	}
	return out, nil
}

func briefSyncTime(db *store.Store) (sql.NullTime, error) {
	var asOf sql.NullTime
	rows, err := db.DB().Query(`SELECT last_synced_at FROM sync_state WHERE last_synced_at IS NOT NULL ORDER BY last_synced_at DESC`)
	if err != nil {
		return asOf, err
	}
	times := make([]sql.NullTime, 0)
	for rows.Next() {
		var synced sql.NullTime
		if err := rows.Scan(&synced); err != nil {
			_ = rows.Close()
			return asOf, err
		}
		times = append(times, synced)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return asOf, err
	}
	if err := rows.Close(); err != nil {
		return asOf, err
	}
	if len(times) > 0 {
		asOf = times[0]
	}
	return asOf, nil
}

func countBriefChanges(db *store.Store, since time.Time, out *briefOutput) error {
	if since.IsZero() {
		return nil
	}
	rows, err := db.DB().Query(`SELECT resource_type,data FROM resources WHERE resource_type IN ('get-expenses','expenses','get-notifications','notifications')`)
	if err != nil {
		return err
	}
	type item struct {
		rt   string
		data []byte
	}
	items := make([]item, 0)
	for rows.Next() {
		var x item
		if err := rows.Scan(&x.rt, &x.data); err != nil {
			_ = rows.Close()
			return err
		}
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, x := range items {
		var v struct {
			CreatedAt string  `json:"created_at"`
			UpdatedAt string  `json:"updated_at"`
			DeletedAt *string `json:"deleted_at"`
		}
		if json.Unmarshal(x.data, &v) != nil {
			continue
		}
		if strings.Contains(x.rt, "notification") {
			if t, ok := parseSplitwiseDate(v.CreatedAt); ok && t.After(since) {
				out.Changes.Notifications++
			}
			continue
		}
		deleted := false
		if v.DeletedAt != nil {
			if t, ok := parseSplitwiseDate(*v.DeletedAt); ok && t.After(since) {
				out.Changes.DeletedExpenses++
				deleted = true
			}
		}
		if deleted {
			continue
		}
		created, cok := parseSplitwiseDate(v.CreatedAt)
		updated, uok := parseSplitwiseDate(v.UpdatedAt)
		if cok && created.After(since) {
			out.Changes.NewExpenses++
		} else if uok && updated.After(since) {
			out.Changes.UpdatedExpenses++
		}
	}
	return nil
}

func printBriefHuman(cmd *cobra.Command, out briefOutput) error {
	asOf := "never"
	if out.AsOf != nil {
		asOf = out.AsOf.Format(time.RFC3339)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "As of: %s\nNet (%s): %.2f owed to you; %.2f you owe\n", asOf, out.Net.Currency, out.Net.OwedToYou, out.Net.YouOwe)
	limit := len(out.StalestDebts)
	if limit > 5 {
		limit = 5
	}
	for _, d := range out.StalestDebts[:limit] {
		age := "unknown age"
		if d.DaysOpen != nil {
			age = fmt.Sprintf("%d days", *d.DaysOpen)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Debt: %s %s %.2f %s (%s)\n", d.Friend, d.Direction, d.Amount, d.Currency, age)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Changes: %d new, %d updated, %d deleted expenses; %d notifications\nNext: %s\n", out.Changes.NewExpenses, out.Changes.UpdatedExpenses, out.Changes.DeletedExpenses, out.Changes.Notifications, out.NextStep)
	return nil
}
