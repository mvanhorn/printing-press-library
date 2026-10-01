// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

func newNovelLedgerSqlCmd(flags *rootFlags) *cobra.Command {
	var schema bool
	var limit int

	cmd := &cobra.Command{
		Use:   "sql [query]",
		Short: "Run read-only SQL against your real budget database without Node or a running engine.",
		Long: `Use this command for arbitrary read-only SQL against the raw budget tables. Do NOT use this command for keyword lookup of transactions by text; use 'ledger search' instead.

Only a single SELECT, WITH or PRAGMA table_info statement is accepted, and the
mirror is opened read-only. SELECT/WITH results are capped at --limit rows.
Output is an array of row objects (column -> value). Use --schema to list tables,
or --schema <table> to list a table's columns.

Schema notes: transactions.acct is the account id, transactions.description is
the payee id, imported_description is the bank text, dates are YYYYMMDD
integers, amounts are integer minor units (cents), tombstone = 1 means deleted.`,
		Example: strings.Trim(`
  actual-budget-pp-cli ledger sql 'SELECT name, offbudget FROM accounts WHERE tombstone = 0' --agent
  actual-budget-pp-cli ledger sql 'SELECT date, amount FROM transactions WHERE tombstone = 0 ORDER BY date DESC' --limit 20
  actual-budget-pp-cli ledger sql --schema
  actual-budget-pp-cli ledger sql --schema transactions`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "query=SELECT name FROM accounts WHERE tombstone = 0"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if done, err := mirrorPreflight(cmd, flags, "ledger sql"); done {
				return err
			}
			if err := checkLimit(limit); err != nil {
				return err
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			var stmt string
			var qargs []any
			switch {
			case schema && query == "":
				stmt = "SELECT name, type FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY name"
			case schema:
				stmt = "SELECT cid, name, type, \"notnull\" AS not_null, pk FROM pragma_table_info(?) ORDER BY cid"
				qargs = append(qargs, query)
			case query == "":
				return usageErr(errors.New("a SQL query is required (or pass --schema)"))
			default:
				s, wrap, err := actual.CheckReadOnlySQL(query)
				if err != nil {
					return usageErr(err)
				}
				stmt = s
				if wrap && limit > 0 {
					stmt = "SELECT * FROM (" + s + "\n) LIMIT " + strconv.Itoa(limit)
				}
			}
			return withMirror(cmd, flags, make([]map[string]any, 0), func(ctx context.Context, h *mirrorHandle) error {
				rows, _, err := h.Ledger.QueryRows(ctx, stmt, qargs...)
				if err != nil {
					return usageErr(fmt.Errorf("query failed: %w", err))
				}
				if schema && query != "" && len(rows) == 0 {
					return notFoundErr(fmt.Errorf("no table named %q (list tables with --schema)", query))
				}
				return emitRows(cmd, flags, rows, func() error {
					if len(rows) == 0 {
						fmt.Fprintln(cmd.OutOrStdout(), "(no rows)")
						return nil
					}
					return printAutoTable(cmd.OutOrStdout(), rows)
				})
			})
		},
	}
	cmd.Flags().BoolVar(&schema, "schema", false, "List tables, or the columns of the table given as the argument")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Maximum rows for SELECT/WITH queries (0 = no limit)")
	return cmd
}
