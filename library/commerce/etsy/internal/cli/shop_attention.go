// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"

	"github.com/spf13/cobra"
)

type shopxAttentionResult struct {
	UnshippedCount      *int              `json:"unshipped_count"`
	Expiring30d         *int              `json:"expiring_30d"`
	AuditFindings       map[string]int    `json:"audit_findings"`
	ReconcileMismatches *int              `json:"reconcile_mismatches"`
	LastSyncAgeSeconds  map[string]*int64 `json:"last_sync_age_seconds"`
}

func shopxUnshippedCount(db *store.Store) int {
	seen := make(map[int64]bool)
	count := 0
	for _, row := range shopxRows(db, shopxReceiptTypes...) {
		id, ok := shopxInt(row["receipt_id"])
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		if shopxReceiptNeedsShipping(row) {
			count++
		}
	}
	return count
}

func shopxAttention(db *store.Store, shop string, now time.Time) (result shopxAttentionResult, notes []string) {
	result.LastSyncAgeSeconds = shopxSyncAges(db, []string{"listings", "listing_images", "listings_inventory", "receipts", "transactions", "shops_receipts_payments", "ledger_entries", "reviews", "economic_operators"})
	if db == nil {
		return result, []string{shopxNoteNoStore}
	}
	shopxWithMemo(func() {
		audit, auditNotes := shopxAuditListings(db, shop, "all")
		reconcile, reconcileNotes := shopxReconcile(db, shop, now, 30*24*time.Hour)
		expiring, _ := shopxExpiringListings(db, shop, now, 30*24*time.Hour)
		if shopxFamilyHasData(db, shopxReceiptTypes) {
			n := shopxUnshippedCount(db)
			result.UnshippedCount = &n
		} else {
			notes = append(notes, "receipts not synced; unshipped_count is unknown, not zero")
		}
		if shop != "" && shopxFamilyHasData(db, shopxListingTypes) {
			n := len(expiring)
			result.Expiring30d = &n
			result.AuditFindings = shopxAuditCounts(audit)
		}
		if !shopxFamilyHasData(db, shopxListingTypes) {
			notes = append(notes, "listings not synced; expiring_30d and audit_findings are unknown, not zero")
		}
		if shopxFamilyHasData(db, shopxReceiptTypes) || shopxFamilyHasData(db, shopxPaymentTypes) || shopxFamilyHasData(db, shopxLedgerTypes) {
			n := reconcile.MismatchCount
			result.ReconcileMismatches = &n
		}
		for _, note := range auditNotes {
			notes = append(notes, shopxAttentionNote("audit: ", note))
		}
		for _, note := range reconcileNotes {
			notes = append(notes, shopxAttentionNote("reconcile: ", note))
		}
		if len(shopxRows(db, shopxListingTypes...)) == 0 && len(shopxRows(db, shopxReceiptTypes...)) == 0 {
			notes = append(notes, "store empty: run 'etsy-pp-cli sync'")
		}
	})
	return result, notes
}

func newNovelShopAttentionCmd(flags *rootFlags) *cobra.Command {

	var dbPath string
	cmd := &cobra.Command{
		Use:         "attention",
		Short:       "One compact summary of what needs attention: unshipped orders, expiring listings, audit findings",
		Example:     "  etsy-pp-cli shop attention --agent",
		Long:        `Use this command for a single cross-domain "what needs attention" summary. Do NOT use it for item-level detail; use 'listings audit', 'listings expiring', 'inventory audit' or 'money reconcile' instead.`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "shop attention")
			}
			db, err := shopxOpenDB(cmd, dbPath)
			if err != nil {
				return err
			}
			if db != nil {
				defer db.Close()
			}
			result, notes := shopxAttention(db, shopxShopID(cmd), shopxNow())
			return shopxEmit(cmd, flags, result, notes)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func shopxAttentionNote(prefix, note string) string {
	if note == shopxNoteShopUnset || strings.Contains(note, "listing(s) from other shops ignored") {
		return note
	}
	return prefix + note
}
