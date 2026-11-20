// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

type shopxFeeGroup struct {
	Group      string `json:"group"`
	Currency   string `json:"currency"`
	Count      int    `json:"count"`
	TotalMinor int64  `json:"total_minor"`
	Total      string `json:"total"`
	GroupBy    string `json:"group_by"`
}

// Fee classification is an assumption: the Etsy spec defines no fee enum.
var shopxFeeTokens = []string{"fee", "listing", "renew", "processing", "vat", "tax", "ads", "label", "bill", "subscription", "transaction"}
var shopxNonFeeTokens = []string{"disburs", "refund", "reversal", "recoup", "chargeback", "dispute", "sale"}

func shopxLedgerMatches(text string, tokens []string) bool {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) })
	for _, word := range words {
		for _, token := range tokens {
			switch token {
			case "ads", "tax", "vat", "fee":
				if word == token {
					return true
				}
			default:
				if strings.HasPrefix(word, token) {
					return true
				}
			}
		}
	}
	return false
}

// shopxLedgerIsFee: an entry is an Etsy fee only when it is a DEBIT (amount < 0),
// no exclusion token appears, and a fee token appears in its type, reference or description.
func shopxLedgerIsFee(ledgerType, referenceType, description string, amount int64) bool {
	text := ledgerType + " " + referenceType + " " + description
	return amount < 0 && !shopxLedgerMatches(text, shopxNonFeeTokens) && shopxLedgerMatches(text, shopxFeeTokens)
}

func shopxFeeGroups(db *store.Store, now time.Time, window time.Duration, groupBy string) ([]shopxFeeGroup, []string) {
	excluded, unknown := 0, 0
	out := make([]shopxFeeGroup, 0)
	groups := make(map[[2]string]*shopxFeeGroup)
	seen := make(map[string]bool)
	for _, row := range shopxRows(db, shopxLedgerTypes...) {
		key := ""
		if id := shopxStr(row["entry_id"]); id != "" {
			key = "entry:" + id
		} else if ledger, seq := shopxStr(row["ledger_id"]), shopxStr(row["sequence_number"]); ledger != "" && seq != "" {
			key = fmt.Sprintf("ledger:%s:sequence:%s", ledger, seq)
		}
		if key != "" {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		ts, ok := shopxFirstTS(row, "created_timestamp", "create_date")
		if !ok || ts.Before(now.Add(-window)) || ts.After(now.Add(24*time.Hour)) {
			continue
		}
		amount, _ := shopxInt(row["amount"])
		ledgerType, referenceType, description := shopxStr(row["ledger_type"]), shopxStr(row["reference_type"]), shopxStr(row["description"])
		if !shopxLedgerIsFee(ledgerType, referenceType, description, amount) {
			excluded++
			text := ledgerType + " " + referenceType + " " + description
			if amount < 0 && !shopxLedgerMatches(text, shopxFeeTokens) && !shopxLedgerMatches(text, shopxNonFeeTokens) {
				unknown++
			}
			continue
		}
		group := shopxStr(row["ledger_type"])
		if group == "" {
			group = "unknown"
		}
		if groupBy == "month" {
			group = ts.UTC().Format("2006-01")
		}
		currency := shopxStr(row["currency"])
		pair := [2]string{group, currency}
		g := groups[pair]
		if g == nil {
			g = &shopxFeeGroup{Group: group, Currency: currency, GroupBy: groupBy}
			groups[pair] = g
		}
		g.Count++
		g.TotalMinor += amount
	}
	for _, g := range groups {
		g.Total = shopxFormatMinor(g.TotalMinor, g.Currency)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Group == out[j].Group {
			return out[i].Currency < out[j].Currency
		}
		return out[i].Group < out[j].Group
	})
	notes := []string{}
	if excluded > 0 {
		notes = append(notes, fmt.Sprintf("%d ledger entries in the window were not counted as fees (sales, refunds, payouts, credits or unrecognized types)", excluded))
	}
	if unknown > 0 {
		notes = append(notes, fmt.Sprintf("%d in-window debit entries had an unrecognized ledger type and were excluded; fee classification is an allowlist (fee, listing, renew, processing, vat, tax, ads, label, bill, subscription, transaction)", unknown))
	}
	return out, notes
}

func newNovelMoneyFeesCmd(flags *rootFlags) *cobra.Command {
	var since, groupBy, dbPath string
	cmd := &cobra.Command{
		Use:         "fees",
		Short:       "See exactly what Etsy charged you by fee type for any period.",
		Long:        "Only fee debits are totalled. Allowlist: fee, listing, renew, processing, vat, tax, ads, label, bill, subscription, transaction. Exclusion list: disburs, refund, reversal, recoup, chargeback, dispute, sale. Classification uses word boundaries for short tokens and word prefixes for stems; this is an assumption because the Etsy spec defines no fee enum; unrecognized debits are excluded and reported in notes. Amounts are signed minor units (fees negative). Do NOT use it for payout/receipt matching; use 'money reconcile' instead.",
		Example:     "  etsy-pp-cli money fees --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "money fees")
			}
			window, err := shopxParseWindow("since", since)
			if err != nil {
				return err
			}
			if groupBy != "ledger_type" && groupBy != "month" {
				return usageErr(fmt.Errorf("invalid --group-by %q: use ledger_type or month", groupBy))
			}
			db, err := shopxOpenDB(cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			familyNotes := shopxCommandNotes(db, "ledger_entries")
			groups, notes := shopxFeeGroups(db, shopxNow(), window, groupBy)
			return shopxEmit(cmd, flags, groups, append(familyNotes, notes...))
		},
	}
	cmd.Flags().StringVar(&since, "since", "30d", "Period to include, such as 30d or 12w")
	cmd.Flags().StringVar(&groupBy, "group-by", "ledger_type", "Group by ledger_type or month")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
