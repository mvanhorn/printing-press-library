// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: export the local ledger to plaintextaccounting formats
// (ledger, beancount, qif) plus csv/json. Replaces the generic framework export
// so the SimpleFIN audience (Beancount/Ledger users) gets first-class formats.
//
// pp:data-source local

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/store"
)

func writeExport(w writer, format string, txns []storedTxn, cats map[string]string) error {
	switch format {
	case "csv":
		return exportCSV(w, txns, cats)
	case "json":
		return exportJSON(w, txns, cats)
	case "ledger":
		return exportLedger(w, txns, cats)
	case "beancount":
		return exportBeancount(w, txns, cats)
	case "qif":
		return exportQIF(w, txns, cats)
	}
	return nil
}

func newSimplefinExportCmd(flags *rootFlags) *cobra.Command {
	var format, since, until, account, dbPath string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the local ledger to ledger, beancount, qif, csv, or json",
		Long: "Serialize synced transactions into accounting-tool formats for plaintextaccounting\n" +
			"workflows. Unlike the global --csv flag (a raw table dump), this produces structured\n" +
			"double-entry records.\n\n" +
			"Formats: ledger, beancount, qif, csv, json.",
		Example:     "  simplefin-pp-cli export --format beancount --since 90d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			// The global --json/--agent flag forces JSON output, overriding
			// --format. Lets agents get structured output without knowing the
			// format vocabulary.
			if flags.asJSON || flags.agent {
				format = "json"
			}
			switch format {
			case "ledger", "beancount", "qif", "csv", "json":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be one of ledger, beancount, qif, csv, json"))
			}

			now := time.Now().UTC()
			var minP, maxP int64
			if since != "" {
				t, err := simplefin.ParseDate(since, now)
				if err != nil {
					return usageErr(fmt.Errorf("--since: %w", err))
				}
				minP = t.Unix()
			}
			if until != "" {
				t, err := simplefin.ParseDate(until, now)
				if err != nil {
					return usageErr(fmt.Errorf("--until: %w", err))
				}
				maxP = t.Unix()
			}

			// Soft-open: a missing local store is an empty export, not an error.
			// We still emit a valid (header/comment-only) document below so output
			// is never silently empty.
			var txns []storedTxn
			cats := map[string]string{}
			resolved := dbPath
			if resolved == "" {
				resolved = defaultDBPath("simplefin-pp-cli")
			}
			if _, statErr := os.Stat(resolved); statErr == nil {
				db, derr := store.OpenWithContext(ctx, resolved)
				if derr != nil {
					return fmt.Errorf("opening database: %w", derr)
				}
				defer db.Close()
				if err := db.EnsureSimpleFINSchema(ctx); err != nil {
					return fmt.Errorf("ensuring schema: %w", err)
				}
				// Assign into the outer txns/cats (no shadowing) so the single
				// writeExport below always sees the loaded data.
				loaded, lerr := loadTransactions(ctx, db, account, minP, maxP, false)
				if lerr != nil {
					return lerr
				}
				cm, cerr := loadCategoryMap(ctx, db)
				if cerr != nil {
					return cerr
				}
				sort.Slice(loaded, func(i, j int) bool { return loaded[i].Posted < loaded[j].Posted })
				txns, cats = loaded, cm
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local data at %s — run: simplefin-pp-cli sync\n", resolved)
			}
			return writeExport(cmd.OutOrStdout(), format, txns, cats)
		},
	}
	cmd.Flags().StringVar(&format, "format", "csv", "Export format: ledger, beancount, qif, csv, json")
	cmd.Flags().StringVar(&since, "since", "", "Only transactions on or after this date (unix epoch, relative, or YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Only transactions before this date")
	cmd.Flags().StringVar(&account, "account", "", "Filter by account id or name substring")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func accountLeaf(name string) string {
	// Beancount/Ledger account components must be word-ish; collapse to colons.
	parts := strings.Fields(name)
	for i, p := range parts {
		parts[i] = capitalize(strings.ToLower(p))
	}
	out := strings.Join(parts, "")
	if out == "" {
		out = "Unknown"
	}
	return out
}

func exportCSV(w writer, txns []storedTxn, cats map[string]string) error {
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"date", "account", "amount", "description", "payee", "category"})
	for _, t := range txns {
		amt, _ := simplefin.ParseAmount(t.Amount)
		_ = cw.Write([]string{
			time.Unix(t.Posted, 0).UTC().Format("2006-01-02"),
			t.AccountName,
			fmt.Sprintf("%.2f", amt),
			t.Description, t.Payee, cats[t.ResourceID],
		})
	}
	cw.Flush()
	return cw.Error()
}

func exportJSON(w writer, txns []storedTxn, cats map[string]string) error {
	type rec struct {
		Date        string  `json:"date"`
		Account     string  `json:"account"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
		Payee       string  `json:"payee,omitempty"`
		Category    string  `json:"category,omitempty"`
	}
	out := make([]rec, 0, len(txns))
	for _, t := range txns {
		amt, _ := simplefin.ParseAmount(t.Amount)
		out = append(out, rec{
			Date:    time.Unix(t.Posted, 0).UTC().Format("2006-01-02"),
			Account: t.AccountName, Amount: amt, Description: t.Description, Payee: t.Payee,
			Category: cats[t.ResourceID],
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// categoryAccount builds a valid double-entry counter-account. Expenses for
// outflow, Income for inflow; the category (if any) becomes the leaf.
func categoryAccount(amt float64, category string) string {
	leaf := accountLeaf(category)
	if leaf == "" || leaf == "Unknown" {
		leaf = "Uncategorized"
	}
	if amt >= 0 {
		return "Income:" + leaf
	}
	return "Expenses:" + leaf
}

func exportLedger(w writer, txns []storedTxn, cats map[string]string) error {
	if len(txns) == 0 {
		fmt.Fprintln(w, "; no transactions in the selected range — run: simplefin-pp-cli sync")
		return nil
	}
	for _, t := range txns {
		amt, _ := simplefin.ParseAmount(t.Amount)
		date := time.Unix(t.Posted, 0).UTC().Format("2006/01/02")
		desc := t.Description
		if t.Payee != "" {
			desc = t.Payee
		}
		asset := "Assets:" + accountLeaf(t.AccountName)
		counter := categoryAccount(amt, cats[t.ResourceID])
		// Double-entry: the asset posting carries the amount; the counter
		// posting is left blank so ledger balances it automatically.
		fmt.Fprintf(w, "%s %s\n", date, desc)
		fmt.Fprintf(w, "    %-32s %10.2f USD\n", asset, amt)
		fmt.Fprintf(w, "    %s\n\n", counter)
	}
	return nil
}

func exportBeancount(w writer, txns []storedTxn, cats map[string]string) error {
	if len(txns) == 0 {
		fmt.Fprintln(w, "; no transactions in the selected range — run: simplefin-pp-cli sync")
		return nil
	}
	for _, t := range txns {
		amt, _ := simplefin.ParseAmount(t.Amount)
		date := time.Unix(t.Posted, 0).UTC().Format("2006-01-02")
		asset := "Assets:" + accountLeaf(t.AccountName)
		counter := categoryAccount(amt, cats[t.ResourceID])
		// Balanced double entry: the two postings sum to zero.
		fmt.Fprintf(w, "%s * %q %q\n", date, t.Payee, t.Description)
		fmt.Fprintf(w, "  %-32s %10.2f USD\n", asset, amt)
		fmt.Fprintf(w, "  %-32s %10.2f USD\n\n", counter, -amt)
	}
	return nil
}

func exportQIF(w writer, txns []storedTxn, cats map[string]string) error {
	fmt.Fprintln(w, "!Type:Bank")
	for _, t := range txns {
		amt, _ := simplefin.ParseAmount(t.Amount)
		fmt.Fprintf(w, "D%s\n", time.Unix(t.Posted, 0).UTC().Format("01/02/2006"))
		fmt.Fprintf(w, "T%.2f\n", amt)
		if t.Payee != "" {
			fmt.Fprintf(w, "P%s\n", t.Payee)
		} else {
			fmt.Fprintf(w, "P%s\n", t.Description)
		}
		fmt.Fprintf(w, "M%s\n", t.Description)
		if c := cats[t.ResourceID]; c != "" {
			fmt.Fprintf(w, "L%s\n", c)
		}
		fmt.Fprintln(w, "^")
	}
	return nil
}

// writer is io.Writer; aliased to keep signatures short.
type writer = interface {
	Write([]byte) (int, error)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}
