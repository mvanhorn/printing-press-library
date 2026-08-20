package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/costco/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/costco/internal/store"

	"github.com/spf13/cobra"
)

// receiptKey builds a dedup key for upserts. When TransactionBarcode is
// present it is globally unique; when absent (gas/carwash receipts) we fall
// back to dateTime + warehouse to avoid silent overwrites.
func receiptKey(r costcoReceipt) string {
	if r.TransactionBarcode != "" {
		return r.MembershipNumber.String() + "|" + r.TransactionBarcode
	}
	ts := r.TransactionDateTime
	if ts == "" {
		ts = r.TransactionDate
	}
	return r.MembershipNumber.String() + "|" + ts + "|" + r.WarehouseNumber.String()
}

// toStoreRow flattens a CLI receipt into the store's storage shape.
func toStoreRow(r costcoReceipt) store.ReceiptRow {
	raw, _ := json.Marshal(r)
	items := make([]store.ItemRow, 0, len(r.ItemArray))
	for _, it := range r.ItemArray {
		items = append(items, store.ItemRow{
			ItemNumber:  it.ItemNumber.String(),
			UPC:         it.UPC.String(),
			Description: strings.TrimSpace(it.Description + " " + it.Description2),
			Department:  it.DepartmentNumber.String(),
			UnitPrice:   it.UnitPriceAmount.float(),
			Quantity:    it.Unit.float(),
			Amount:      it.Amount.float(),
		})
	}
	return store.ReceiptRow{
		ID:               receiptKey(r),
		MembershipNumber: r.MembershipNumber.String(),
		TransactionDate:  r.TransactionDate,
		Channel:          r.channel(),
		WarehouseName:    r.WarehouseName,
		WarehouseNumber:  r.WarehouseNumber.String(),
		ItemCount:        int(r.TotalItemCount.float()),
		SubTotal:         r.SubTotal.float(),
		Taxes:            r.Taxes.float(),
		InstantSavings:   r.InstantSavings.float(),
		Total:            r.Total.float(),
		Barcode:          r.TransactionBarcode,
		Raw:              raw,
		Items:            items,
	}
}

func costcoDBPath(db string) string {
	if strings.TrimSpace(db) != "" {
		return db
	}
	dir, err := cliutil.DataDir()
	if err != nil || dir == "" {
		return "costco-archive.db"
	}
	return filepath.Join(dir, "archive.db")
}

// missingStore emits the standard "no local mirror" guidance and returns true
// when the archive file does not exist yet.
func missingStore(cmd *cobra.Command, flags *rootFlags, dbPath string) bool {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local archive at %s\nrun: costco-pp-cli sync --years 3 --db %s\n", dbPath, dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return true
	}
	return false
}

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var since, until, db string
	var years int
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch receipts and store them in a local SQLite archive (idempotent)",
		Long: strings.Trim(`
Fetch receipts for a date range and upsert them into a local SQLite archive,
deduped by membership number + transaction barcode. Re-running is safe: existing
receipts are updated, new ones added. The archive powers 'sql' and 'search' and
lets your history compound past any single token session.`, "\n"),
		Example:     "  costco-pp-cli sync --years 3",
		Annotations: map[string]string{}, // mutates the local store: not read-only
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would sync receipts into the local archive")
				return nil
			}
			start, end, err := resolveRange(since, until, years)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			receipts, err := fetchReceipts(ctx, flags, start, end)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			dbPath := costcoDBPath(db)
			if dir := filepath.Dir(dbPath); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					return fmt.Errorf("creating archive directory: %w", err)
				}
			}
			st, err := store.Open(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			rows := make([]store.ReceiptRow, 0, len(receipts))
			for _, r := range receipts {
				rows = append(rows, toStoreRow(r))
			}
			res, err := st.Upsert(ctx, rows)
			if err != nil {
				return fmt.Errorf("writing archive: %w", err)
			}
			total, _ := st.Count(ctx)
			view := map[string]any{
				"range":          map[string]string{"start": start, "end": end},
				"fetched":        len(receipts),
				"added":          res.Added,
				"updated":        res.Updated,
				"total_in_store": total,
				"db":             dbPath,
			}
			b, _ := json.Marshal(view)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Start of range (YYYY-MM-DD or duration)")
	cmd.Flags().StringVar(&until, "until", "", "End of range (YYYY-MM-DD; default today)")
	cmd.Flags().IntVar(&years, "years", 2, "Lookback in years when --since is not set")
	cmd.Flags().StringVar(&db, "db", "", "Archive path (default: per-user data dir)")
	return cmd
}

func newSQLCmd(flags *rootFlags) *cobra.Command {
	var db string
	var limit int
	cmd := &cobra.Command{
		Use:   "sql <query>",
		Short: "Run a read-only SQL SELECT against the local receipt archive",
		Long: strings.Trim(`
Run a read-only SQL query (SELECT/WITH only) against the local archive built by
'sync'. Tables: receipts(id, membership_number, transaction_date, channel,
warehouse_name, warehouse_number, item_count, subtotal, taxes, instant_savings,
total, barcode, data) and items(receipt_id, transaction_date, warehouse_name,
item_number, upc, description, department, unit_price, quantity, amount).`, "\n"),
		Example:     "  costco-pp-cli sql \"SELECT channel, COUNT(*) c, SUM(total) spend FROM receipts GROUP BY channel\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would run a read-only SQL query against the archive")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a SQL SELECT query is required"))
			}
			dbPath := costcoDBPath(db)
			if missingStore(cmd, flags, dbPath) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			st, err := store.OpenReadOnly(ctx, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			out, err := st.QueryJSON(ctx, strings.Join(args, " "), limit)
			if err != nil {
				return usageErr(err)
			}
			b, _ := json.Marshal(out)
			return printOutputWithFlags(cmd.OutOrStdout(), b, flags)
		},
	}
	cmd.Flags().StringVar(&db, "db", "", "Archive path (default: per-user data dir)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to return (0 = all)")
	return cmd
}
