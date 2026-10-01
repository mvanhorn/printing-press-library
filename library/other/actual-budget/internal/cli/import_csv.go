// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

const transactionsImportPath = "/budgets/{budgetSyncId}/accounts/{accountId}/transactions/import"

var uuidLike = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type importCSVPlan struct {
	DryRun       bool                 `json:"dry_run,omitempty"`
	Preview      bool                 `json:"preview,omitempty"`
	Method       string               `json:"method"`
	Path         string               `json:"path"`
	AccountID    string               `json:"account_id"`
	Account      string               `json:"account,omitempty"`
	Submitted    int                  `json:"submitted"`
	Transactions []actual.ImportTxn   `json:"transactions"`
	InvalidRows  []actual.CSVRowError `json:"invalid_rows"`
	HTTPStatus   int                  `json:"http_status,omitempty"`
	Response     any                  `json:"response,omitempty"`
}

// resolveImportAccount maps --account (id or name) to an account id using the
// local mirror when present. Without a mirror only UUID-shaped ids are taken.
func resolveImportAccount(ctx context.Context, flags *rootFlags, value string) (string, string, error) {
	value = strings.TrimSpace(value)
	if h := openSelectedMirror(flags); h != nil {
		defer h.Close()
		id, rerr := h.Ledger.ResolveAccountID(ctx, value)
		if errors.Is(rerr, sql.ErrNoRows) && uuidLike.MatchString(value) {
			// Created after the last pull; the sidecar will validate it.
			return value, "", nil
		}
		if rerr != nil {
			return "", "", ledgerResolveErr(rerr, "accounts")
		}
		var name string
		_ = h.Ledger.DB.QueryRowContext(ctx, "SELECT COALESCE(name, '') FROM accounts WHERE id = ?", id).Scan(&name)
		return id, name, nil
	}
	if uuidLike.MatchString(value) {
		return value, "", nil
	}
	return "", "", usageErr(fmt.Errorf("--account %q is not an account id and there is no local mirror to resolve names; run 'mirror pull' or pass the account id", value))
}

// openSelectedMirror quietly opens the selected budget's mirror, or returns
// nil when no budget is selected, nothing is pulled, or it cannot be opened.
func openSelectedMirror(flags *rootFlags) *mirrorHandle {
	syncID := resolveSyncID(flags)
	if syncID == "" {
		return nil
	}
	path, _, found, err := actual.LocateMirror(syncID)
	if err != nil || !found {
		return nil
	}
	h, err := openMirrorAt(path)
	if err != nil {
		return nil
	}
	return h
}

func newImportCSVCmd(flags *rootFlags) *cobra.Command {
	var m actual.CSVMapping
	var flagAccount string
	var flagPreview bool

	cmd := &cobra.Command{
		Use:   "import-csv <file>",
		Short: "Import a bank CSV export into an account through the actual-http-api sidecar, with reconciliation",
		Long: `Parses a bank CSV (with a header row) using the column-mapping flags and posts
the rows to the actual-http-api sidecar's transactions import endpoint
(POST /budgets/{budgetSyncId}/accounts/{accountId}/transactions/import), which
runs Actual's reconciliation: rules are applied and rows already imported
(matched by imported_id or by date/amount/payee) are not duplicated.

Amounts are converted to integer minor units. Use --amount-col for a signed
amount column, or --debit-col/--credit-col for split columns (debits become
negative). Parentheses mean negative; --invert flips every sign (for cards that
export charges as positive). Dates are auto-detected among YYYY-MM-DD and
M/D/YYYY (D/M/YYYY with --day-first), or set --date-format to a Go layout such
as 02.01.2006. Rows that fail to parse are reported in invalid_rows with their
line numbers and skipped.

--account takes an account id, or a name resolved through the local mirror.
--preview prints the parsed rows without contacting the sidecar; --dry-run
prints the parsed rows and the planned request.`,
		Example: strings.Trim(`
  actual-budget-pp-cli import-csv checking-2026-09.csv --account Checking --date-col Date --amount-col Amount --payee-col Description
  actual-budget-pp-cli import-csv card.csv --account "Visa Card" --date-col "Posted Date" --debit-col Debit --credit-col Credit --payee-col Payee --preview
  actual-budget-pp-cli import-csv export.csv --account Checking --date-col Date --amount-col Amount --payee-col Description --id-col Reference --dry-run --json`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live",
			"pp:happy-args": "file=testdata/sample.csv;--account=Checking;--date-col=Date;--amount-col=Amount;--payee-col=Description;--dry-run"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 && !flags.dryRun {
				return cmd.Help()
			}
			parse := func() ([]actual.ImportTxn, []actual.CSVRowError, error) {
				if len(args) == 0 {
					return nil, nil, usageErr(errors.New("missing <file>: the bank CSV to import"))
				}
				switch strings.ToLower(filepath.Ext(args[0])) {
				case ".csv", ".tsv", ".txt":
				default:
					return nil, nil, usageErr(fmt.Errorf("%s: import-csv reads .csv, .tsv or .txt bank exports", args[0]))
				}
				data, err := os.ReadFile(args[0])
				if err != nil {
					return nil, nil, usageErr(fmt.Errorf("reading %s: %w", args[0], err))
				}
				txns, bad, err := actual.ParseBankCSV(bytes.NewReader(data), m)
				if err != nil {
					return nil, nil, usageErr(err)
				}
				return txns, bad, nil
			}
			emitPlan := func(plan importCSVPlan) error {
				return emitRows(cmd, flags, plan, func() error { return printImportPlan(cmd, plan) })
			}
			if dryRunOK(flags) {
				// Best effort: show the parsed plan when the file and mapping are
				// usable, otherwise the generic dry-run line (still exit 0).
				txns, bad, err := parse()
				if err != nil || strings.TrimSpace(flagAccount) == "" {
					return writeDryRun(cmd.OutOrStdout(), flags, "import-csv")
				}
				id, name, aerr := resolveImportAccount(cmd.Context(), flags, flagAccount)
				if aerr != nil {
					id = flagAccount
				}
				plan := importCSVPlan{DryRun: true, Method: "POST", Path: replacePathParam(transactionsImportPath, "accountId", id),
					AccountID: id, Account: name, Submitted: 0, Transactions: txns, InvalidRows: bad}
				return emitPlan(plan)
			}
			if len(args) == 0 {
				return usageErr(errors.New("missing <file>: the bank CSV to import"))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("expected one <file>, got %d", len(args)))
			}
			if strings.TrimSpace(flagAccount) == "" {
				return usageErr(errors.New("--account is required (account id, or name when a local mirror exists)"))
			}
			if err := m.Validate(); err != nil {
				return usageErr(err)
			}
			txns, bad, err := parse()
			if err != nil {
				return err
			}
			if len(bad) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d row(s) could not be parsed and will be skipped (see invalid_rows)\n", len(bad))
			}
			accountID, accountName, err := resolveImportAccount(cmd.Context(), flags, flagAccount)
			if err != nil {
				return err
			}
			plan := importCSVPlan{Preview: flagPreview, Method: "POST", Path: replacePathParam(transactionsImportPath, "accountId", accountID),
				AccountID: accountID, Account: accountName, Transactions: txns, InvalidRows: bad}
			if flagPreview {
				return emitPlan(plan)
			}
			if len(txns) == 0 {
				return usageErr(errors.New("no valid rows to import"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			data, status, err := c.PostWithHeaders(ctx, plan.Path, map[string]any{"transactions": txns}, sidecarHeaders())
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			plan.HTTPStatus, plan.Submitted = status, len(txns)
			if len(data) > 0 {
				var parsed any
				if json.Unmarshal(data, &parsed) == nil {
					plan.Response = parsed
				} else {
					plan.Response = string(data)
				}
			}
			if status < 200 || status >= 300 {
				_ = emitPlan(plan)
				return apiErr(fmt.Errorf("import failed: HTTP %d", status))
			}
			return emitPlan(plan)
		},
	}
	cmd.Flags().StringVar(&flagAccount, "account", "", "Account id, or account name (resolved through the local mirror)")
	cmd.Flags().StringVar(&m.DateCol, "date-col", "", "CSV header of the date column")
	cmd.Flags().StringVar(&m.AmountCol, "amount-col", "", "CSV header of a signed amount column")
	cmd.Flags().StringVar(&m.DebitCol, "debit-col", "", "CSV header of the debit (money out) column, instead of --amount-col")
	cmd.Flags().StringVar(&m.CreditCol, "credit-col", "", "CSV header of the credit (money in) column, instead of --amount-col")
	cmd.Flags().StringVar(&m.PayeeCol, "payee-col", "", "CSV header of the payee/description column")
	cmd.Flags().StringVar(&m.NotesCol, "notes-col", "", "CSV header of a memo/notes column")
	cmd.Flags().StringVar(&m.IDCol, "id-col", "", "CSV header of the bank's unique transaction id (sent as imported_id for de-duplication)")
	cmd.Flags().StringVar(&m.DateFormat, "date-format", "", "Go date layout of the date column, e.g. 02.01.2006 (default: auto-detect)")
	cmd.Flags().BoolVar(&m.DayFirst, "day-first", false, "Auto-detect DD/MM/YYYY dates instead of MM/DD/YYYY")
	cmd.Flags().BoolVar(&m.Invert, "invert", false, "Flip the sign of every amount")
	cmd.Flags().BoolVar(&flagPreview, "preview", false, "Print the parsed rows without contacting the sidecar")
	return cmd
}

func printImportPlan(cmd *cobra.Command, p importCSVPlan) error {
	w := cmd.OutOrStdout()
	switch {
	case p.DryRun:
		fmt.Fprintf(w, "dry-run: would %s %s with %d transaction(s)\n", p.Method, p.Path, len(p.Transactions))
	case p.Preview:
		fmt.Fprintf(w, "preview: %d transaction(s) parsed for account %s\n", len(p.Transactions), p.AccountID)
	default:
		fmt.Fprintf(w, "Imported %d transaction(s) into %s (HTTP %d)\n", p.Submitted, p.AccountID, p.HTTPStatus)
	}
	if p.DryRun || p.Preview {
		table := make([][]string, 0, len(p.Transactions))
		for _, t := range p.Transactions {
			table = append(table, []string{fmt.Sprint(t.Line), t.Date, amt(t.Amount), t.PayeeName, t.ImportedID})
		}
		if err := printOrderedTable(w, []string{"LINE", "DATE", "AMOUNT", "PAYEE", "IMPORTED_ID"}, table); err != nil {
			return err
		}
	}
	for _, b := range p.InvalidRows {
		fmt.Fprintf(w, "  skipped line %d: %s\n", b.Line, b.Error)
	}
	return nil
}
