// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/autotempest"
	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/store"

	"github.com/spf13/cobra"
)

// slugColExpr is a SQL expression that normalizes a make/model column to the
// same slug shape as autotempest.NormalizeSlug for the realistic catalog
// punctuation (hyphen, space, dot): F-150 -> f150, "Civic Sedan" -> civicsedan.
// It is NOT a perfect mirror of NormalizeSlug (which strips ALL non-alphanumeric
// characters) but covers every separator AutoTempest uses in make/model slugs,
// so `spread "f-150"` matches stored "F-150" without breaking already-matching
// names. col must be a trusted literal column name (never user input).
func slugColExpr(col string) string {
	return fmt.Sprintf(`LOWER(REPLACE(REPLACE(REPLACE(%s,'-',''),' ',''),'.',''))`, col)
}

// appendMakeModelTermFilter appends slug-normalized make/model/term predicates
// to a WHERE clause and returns the extended (clauses, args). Binding rules:
//
//   - mk (--make):    matches the MAKE column ONLY
//   - model (--model): matches the MODEL column ONLY
//   - term (positional): AMBIGUOUS — the user typed e.g. "f-150" without saying
//     which column, so it OR-matches BOTH make and model
//
// This fixes the prior bug where --make bound the same value to both columns
// (an OR), wrongly including a listing whose MODEL literally equaled the make.
// Only the positional term is allowed to OR across columns.
//
// Column names come from slugColExpr (trusted literals); every user value is
// bound via a ? placeholder appended to args.
func appendMakeModelTermFilter(where []string, args []any, mk, model, term string) ([]string, []any) {
	makeExpr := slugColExpr("make")
	modelExpr := slugColExpr("model")
	if mk != "" {
		where = append(where, makeExpr+" = ?")
		args = append(args, autotempest.NormalizeSlug(mk))
	}
	if model != "" {
		where = append(where, modelExpr+" = ?")
		args = append(args, autotempest.NormalizeSlug(model))
	}
	if term != "" {
		where = append(where, "("+makeExpr+" = ? OR "+modelExpr+" = ?)")
		n := autotempest.NormalizeSlug(term)
		args = append(args, n, n)
	}
	return where, args
}

// rejectLiveDataSource returns an error when --data-source live is requested on
// a local-only command. These novel commands read the LOCAL store exclusively;
// there is no live equivalent.
func rejectLiveDataSource(flags *rootFlags) error {
	if flags != nil && flags.dataSource == "live" {
		return usageErr(fmt.Errorf("no live equivalent for this command; it reads the local store only (drop --data-source live or use --data-source local/auto)"))
	}
	return nil
}

// resolveNovelDBPath returns the explicit --db value or the default location.
func resolveNovelDBPath(dbFlag string) string {
	if dbFlag != "" {
		return dbFlag
	}
	return defaultDBPath("autotempest-pp-cli")
}

// openLocalForNovel opens the store at path (creating tables) for a read-only
// novel command. Returns (nil, nil) when the DB file is absent so callers can
// emit the missing-mirror hint.
func openLocalForNovel(ctx context.Context, path string) (*store.Store, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	db, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if err := store.EnsureAutoTempestTables(db.DB()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// atListingsCount returns the row count of at_listings (0 if the table is
// missing).
func atListingsCount(ctx context.Context, sqlDB *sql.DB) int {
	var n int
	_ = sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM at_listings`).Scan(&n)
	return n
}

// missingMirrorHint prints the "run find first" hint to stderr and, in
// machine-output mode, an empty array to stdout. Returns nil so the command
// exits 0 (a fresh store is not an error). make/model/zip seed the example.
func missingMirrorHint(cmd *cobra.Command, flags *rootFlags, mk, model, zip string) error {
	if mk == "" {
		mk = "honda"
	}
	if model == "" {
		model = "civic"
	}
	if zip == "" {
		zip = "33701"
	}
	fmt.Fprintf(cmd.ErrOrStderr(),
		"no local listings; run: autotempest-pp-cli find %q --zip %s\n",
		mk+" "+model, zip)
	if flags != nil && (flags.asJSON || flags.agent) {
		fmt.Fprintln(cmd.OutOrStdout(), "[]")
	}
	return nil
}

// guardLocalNovel performs the standard prelude for a local-only novel command:
// reject --data-source live, open the store, and apply the missing-mirror
// guard. Returns (db, true, nil) when the command should proceed; (nil, false,
// nil) when the guard fired and the command should return nil; or an error.
func guardLocalNovel(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbFlag, mk, model, zip string) (*store.Store, bool, error) {
	if err := rejectLiveDataSource(flags); err != nil {
		return nil, false, err
	}
	path := resolveNovelDBPath(dbFlag)
	db, err := openLocalForNovel(ctx, path)
	if err != nil {
		return nil, false, err
	}
	if db == nil {
		return nil, false, missingMirrorHint(cmd, flags, mk, model, zip)
	}
	if atListingsCount(ctx, db.DB()) == 0 {
		_ = db.Close()
		return nil, false, missingMirrorHint(cmd, flags, mk, model, zip)
	}
	return db, true, nil
}

// emitNovel renders a typed slice through the standard output pipeline. Default
// (terminal, no --json) prints an auto table; otherwise honors --json/--select/etc.
func emitNovel(cmd *cobra.Command, flags *rootFlags, rows []map[string]any) error {
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		if len(rows) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "no rows")
			return nil
		}
		return printAutoTable(cmd.OutOrStdout(), rows)
	}
	return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
}
