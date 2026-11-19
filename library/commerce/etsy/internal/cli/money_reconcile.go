// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
	"github.com/spf13/cobra"
)

type shopxReconcileReceipt struct {
	ReceiptID int64  `json:"receipt_id"`
	Amount    int64  `json:"amount_minor"`
	Currency  string `json:"currency"`
}
type shopxReconcilePayment struct {
	PaymentID int64  `json:"payment_id"`
	ReceiptID int64  `json:"receipt_id"`
	Amount    int64  `json:"amount_minor"`
	Currency  string `json:"currency"`
}
type shopxReconcileReference struct {
	EntryID       int64  `json:"entry_id"`
	ReferenceType string `json:"reference_type"`
	ReferenceID   string `json:"reference_id"`
	Amount        int64  `json:"amount_minor"`
	Currency      string `json:"currency"`
}
type shopxReconcileGap struct {
	EntryID  int64  `json:"entry_id"`
	LedgerID int64  `json:"ledger_id"`
	Sequence int64  `json:"sequence_number"`
	Expected int64  `json:"expected_balance_minor"`
	Actual   int64  `json:"actual_balance_minor"`
	Gap      int64  `json:"gap_minor"`
	Currency string `json:"currency"`
}
type shopxReconcileResult struct {
	Window                 string                    `json:"since"`
	MismatchCount          int                       `json:"mismatch_count"`
	ReceiptsWithoutPayment []shopxReconcileReceipt   `json:"receipts_without_payment"`
	PaymentsWithoutReceipt []shopxReconcilePayment   `json:"payments_without_receipt"`
	LedgerMissingReference []shopxReconcileReference `json:"ledger_missing_reference"`
	BalanceDiscontinuities []shopxReconcileGap       `json:"balance_discontinuities"`
}

func shopxReconcileUnique(rows []map[string]any, keys ...string) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	seen := map[int64]bool{}
	for _, r := range rows {
		id, ok := shopxFirstInt(r, keys...)
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := shopxFirstInt(out[i], keys...)
		b, _ := shopxFirstInt(out[j], keys...)
		return a < b
	})
	return out
}

func shopxReconcile(db *store.Store, shop string, now time.Time, window time.Duration) (shopxReconcileResult, []string) {
	notes := []string{}
	result := shopxReconcileResult{ReceiptsWithoutPayment: make([]shopxReconcileReceipt, 0), PaymentsWithoutReceipt: make([]shopxReconcilePayment, 0), LedgerMissingReference: make([]shopxReconcileReference, 0), BalanceDiscontinuities: make([]shopxReconcileGap, 0)}
	receipts := shopxReconcileUnique(shopxRows(db, shopxReceiptTypes...), "receipt_id")
	payments := shopxReconcileUnique(shopxRows(db, shopxPaymentTypes...), "payment_id", "receipt_id")
	ledger := shopxReconcileUnique(shopxRows(db, shopxLedgerTypes...), "entry_id")
	transactions := shopxRows(db, shopxTransactionTypes...)
	listings, _, dropped := shopxListingsByID(db, shop)
	if shop == "" {
		notes = append(notes, shopxNoteShopUnset)
	}
	if dropped > 0 {
		notes = append(notes, shopxForeignNote(dropped))
	}
	targets := map[string]map[string]bool{}
	for kind, rows := range map[string][]map[string]any{"receipt": receipts, "payment": payments, "transaction": transactions, "listing": listings} {
		ids := map[string]bool{}
		for _, r := range rows {
			if id := shopxStr(r[kind+"_id"]); id != "" {
				ids[id] = true
			}
		}
		targets[kind] = ids
	}
	inWindow := func(r map[string]any, keys ...string) bool {
		ts, ok := shopxFirstTS(r, keys...)
		return ok && !ts.Before(now.Add(-window)) && !ts.After(now)
	}
	paidReceipts := map[int64]bool{}
	for _, p := range payments {
		id, _ := shopxInt(p["receipt_id"])
		paidReceipts[id] = true
	}
	if len(payments) == 0 {
		notes = append(notes, "payments not synced; receipts_without_payment skipped")
	} else {
		for _, r := range receipts {
			paid, present := shopxBool(r["is_paid"])
			if !present {
				paid, _ = shopxBool(r["was_paid"])
			}
			id, _ := shopxInt(r["receipt_id"])
			if paid && inWindow(r, "created_timestamp") && !paidReceipts[id] {
				amount, _, currency, _ := shopxMoney(r["grandtotal"])
				result.ReceiptsWithoutPayment = append(result.ReceiptsWithoutPayment, shopxReconcileReceipt{id, amount, currency})
			}
		}
	}
	if len(receipts) == 0 {
		notes = append(notes, "receipts not synced; payments_without_receipt skipped")
	} else {
		for _, p := range payments {
			rid, _ := shopxInt(p["receipt_id"])
			if inWindow(p, "created_timestamp", "create_timestamp") && !targets["receipt"][shopxStr(p["receipt_id"])] {
				pid, _ := shopxInt(p["payment_id"])
				amount, _, currency, _ := shopxMoney(p["amount_gross"])
				result.PaymentsWithoutReceipt = append(result.PaymentsWithoutReceipt, shopxReconcilePayment{pid, rid, amount, currency})
			}
		}
	}
	noted := map[string]bool{}
	groups := map[int64][]map[string]any{}
	for _, r := range ledger {
		lid, _ := shopxInt(r["ledger_id"])
		groups[lid] = append(groups[lid], r)
		ref := shopxStr(r["reference_id"])
		typ := shopxStr(r["reference_type"])
		if ref == "" || !inWindow(r, "created_timestamp") {
			continue
		}
		for _, kind := range []string{"receipt", "payment", "transaction", "listing"} {
			if !strings.Contains(strings.ToLower(typ), kind) {
				continue
			}
			if len(targets[kind]) == 0 {
				if !noted[kind] {
					notes = append(notes, kind+" not synced; ledger reference check skipped")
					noted[kind] = true
				}
			} else if !targets[kind][ref] {
				id, _ := shopxInt(r["entry_id"])
				amount, _ := shopxInt(r["amount"])
				result.LedgerMissingReference = append(result.LedgerMissingReference, shopxReconcileReference{id, typ, ref, amount, shopxStr(r["currency"])})
			}
			break
		}
	}
	lids := make([]int64, 0, len(groups))
	for id := range groups {
		lids = append(lids, id)
	}
	sort.Slice(lids, func(i, j int) bool { return lids[i] < lids[j] })
	for _, lid := range lids {
		rows := groups[lid]
		sort.SliceStable(rows, func(i, j int) bool {
			a, aok := shopxInt(rows[i]["sequence_number"])
			b, bok := shopxInt(rows[j]["sequence_number"])
			if aok && bok && a != b {
				return a < b
			}
			at, _ := shopxFirstTS(rows[i], "created_timestamp")
			bt, _ := shopxFirstTS(rows[j], "created_timestamp")
			return at.Before(bt)
		})
		for i := 1; i < len(rows); i++ {
			prev, cur := rows[i-1], rows[i]
			pb, pok := shopxInt(prev["balance"])
			_, paok := shopxInt(prev["amount"])
			cb, cok := shopxInt(cur["balance"])
			ca, caok := shopxInt(cur["amount"])
			if !pok || !paok || !cok || !caok || !inWindow(cur, "created_timestamp") || pb+ca == cb {
				continue
			}
			id, _ := shopxInt(cur["entry_id"])
			seq, _ := shopxInt(cur["sequence_number"])
			result.BalanceDiscontinuities = append(result.BalanceDiscontinuities, shopxReconcileGap{id, lid, seq, pb + ca, cb, cb - (pb + ca), shopxStr(cur["currency"])})
		}
	}
	result.MismatchCount = len(result.ReceiptsWithoutPayment) + len(result.PaymentsWithoutReceipt) + len(result.LedgerMissingReference) + len(result.BalanceDiscontinuities)
	return result, notes
}

func newNovelMoneyReconcileCmd(flags *rootFlags) *cobra.Command {
	var flagSince, dbPath string
	cmd := &cobra.Command{
		Use:         "reconcile",
		Short:       "Find receipts, payments and ledger entries that don't line up, by id and amount.",
		Long:        "Use this command to find mismatches between receipts, payments and ledger. Do NOT use it for fee totals; use 'money fees' instead.",
		Example:     "  etsy-pp-cli money reconcile --since 30d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "money reconcile")
			}
			window, err := shopxParseWindow("since", flagSince)
			if err != nil {
				return err
			}
			db, err := shopxOpenDB(cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			familyNotes := shopxCommandNotes(db, "receipts", "ledger_entries")
			result, notes := shopxReconcile(db, shopxShopID(cmd), shopxNow(), window)
			result.Window = flagSince
			notes = append(familyNotes, notes...)
			return shopxEmit(cmd, flags, result, notes)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "30d", "Reconcile rows created within this duration")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
