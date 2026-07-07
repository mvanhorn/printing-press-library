// Copyright 2026 matt-van-horn. Licensed under Apache-2.0.
// `expense bulk` — create many new expenses in a single Expense_Create request.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// bulkExpenseInput is one row of bulk input. amount is in cents. `created` is
// canonical; `date` is tolerated as an alias on input.
type bulkExpenseInput struct {
	Merchant     string `json:"merchant"`
	Amount       int    `json:"amount"`
	Created      string `json:"created"`
	Date         string `json:"date"`
	Currency     string `json:"currency"`
	Category     string `json:"category"`
	Tag          string `json:"tag"`
	Comment      string `json:"comment"`
	Reimbursable *bool  `json:"reimbursable"`
}

func newExpenseBulkCmd(flags *rootFlags) *cobra.Command {
	var inputFile, reportID, defCategory, defTag, defCurrency string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Create many expenses in a single request (Expense_Create transactionList)",
		Long: `Create many expenses in ONE API call by posting a transactionList to
Expense_Create. Input is JSONL (one JSON object per line) or a JSON array, read
from a file (--input) or stdin (--input -). Per-row fields:
  merchant, amount (CENTS), created (YYYY-MM-DD), currency, category,
  tag (the DEPARTMENT, e.g. "101 G&A"), comment, reimbursable.
Rows that omit category/tag/currency inherit the --category/--tag/--currency
defaults. --report attaches every created expense to a report.`,
		Example: `  cat rows.jsonl | expensify-pp-cli expense bulk --input - --report 12345
  expensify-pp-cli expense bulk -i rows.jsonl --category "Facilities - Office Supplies & Printing" --tag "101 G&A" --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			items, err := readBulkInput(inputFile)
			if err != nil {
				return usageErr(err)
			}
			if len(items) == 0 {
				return usageErr(fmt.Errorf("no expense rows parsed from input"))
			}

			txns := make([]map[string]any, 0, len(items))
			var totalCents int
			for i, it := range items {
				if it.Merchant == "" || it.Amount == 0 {
					return usageErr(fmt.Errorf("row %d: merchant and amount (cents) are required", i+1))
				}
				created := it.Created
				if created == "" {
					created = it.Date
				}
				currency := firstNonEmptyStr(it.Currency, defCurrency, "USD")
				category := firstNonEmptyStr(it.Category, defCategory)
				tag := firstNonEmptyStr(it.Tag, defTag)
				reimbursable := true
				if it.Reimbursable != nil {
					reimbursable = *it.Reimbursable
				}
				t := map[string]any{
					"merchant":     it.Merchant,
					"amount":       it.Amount,
					"created":      created,
					"currency":     currency,
					"reimbursable": reimbursable,
				}
				if category != "" {
					t["category"] = category
				}
				if tag != "" {
					t["tag"] = tag
				}
				if it.Comment != "" {
					t["comment"] = it.Comment
				}
				if reportID != "" {
					t["reportID"] = reportID
				}
				txns = append(txns, t)
				totalCents += it.Amount
			}

			cur := firstNonEmptyStr(defCurrency, "USD")
			w := cmd.OutOrStdout()
			if dryRun || flags.dryRun {
				fmt.Fprintf(w, "DRY RUN: would create %d expenses, total %s %.2f\n", len(txns), cur, float64(totalCents)/100)
				for i, t := range txns {
					created, _ := t["created"].(string)
					merch, _ := t["merchant"].(string)
					amt, _ := t["amount"].(int)
					dep, _ := t["tag"].(string)
					if dep == "" {
						dep = "-"
					}
					fmt.Fprintf(w, "  %2d  %-10s  %-10s  %8.2f  %s\n", i+1, created, dep, float64(amt)/100, truncate(merch, 30))
				}
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// The client JSON-encodes a slice body field into a single form value,
			// so transactionList travels as one field — no transport change needed.
			body := map[string]any{"transactionList": txns}
			if reportID != "" {
				body["reportID"] = reportID
			}
			data, status, err := c.Post(cmd.Context(), "/Expense_Create", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if status < 200 || status >= 300 {
				return apiErr(fmt.Errorf("Expense_Create returned HTTP %d: %s", status, truncate(string(data), 200)))
			}
			suffix := ""
			if reportID != "" {
				suffix = fmt.Sprintf(" on report %s", reportID)
			}
			fmt.Fprintf(w, "Created %d expenses (total %s %.2f)%s.\n", len(txns), cur, float64(totalCents)/100, suffix)
			return nil
		},
	}
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "JSONL or JSON-array file (use - for stdin)")
	cmd.Flags().StringVar(&reportID, "report", "", "Attach all created expenses to this reportID")
	cmd.Flags().StringVar(&defCategory, "category", "", "Default category for rows that omit one")
	cmd.Flags().StringVar(&defTag, "tag", "", "Default tag/department for rows that omit one")
	cmd.Flags().StringVar(&defCurrency, "currency", "USD", "Default currency for rows that omit one")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview the transactionList without sending")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

// readBulkInput parses bulk rows from a JSON array or JSONL stream.
func readBulkInput(path string) ([]bulkExpenseInput, error) {
	var r io.Reader
	if path == "-" || path == "" {
		r = os.Stdin
	} else {
		f, err := os.Open(path) // #nosec G304 -- path is the user-supplied bulk-import file; opening the caller's own file is the documented purpose of this command.
		if err != nil {
			return nil, fmt.Errorf("opening input: %w", err)
		}
		defer f.Close()
		r = f
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("empty input")
	}
	if trimmed[0] == '[' {
		var arr []bulkExpenseInput
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, fmt.Errorf("parsing JSON array: %w", err)
		}
		return arr, nil
	}
	var items []bulkExpenseInput
	sc := bufio.NewScanner(strings.NewReader(trimmed))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	ln := 0
	for sc.Scan() {
		ln++
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		var it bulkExpenseInput
		if err := json.Unmarshal([]byte(line), &it); err != nil {
			return nil, fmt.Errorf("line %d: invalid JSON: %w", ln, err)
		}
		items = append(items, it)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
