// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
)

// The generated root reserves one newNovel<Name>Cmd slot per research novel
// feature. The hand-authored commands register themselves in the hook below;
// these slots are marked as scaffolds so preferImplementedNovelCommands drops
// them in favor of the real command with the same name.
func novelSlot(use string) *cobra.Command {
	return &cobra.Command{Use: use, Hidden: true, Annotations: map[string]string{novelScaffoldAnnotation: "true"}}
}

func newNovelRunCmd(*rootFlags) *cobra.Command      { return novelSlot("run") }
func newNovelSchemaCmd(*rootFlags) *cobra.Command   { return novelSlot("schema") }
func newNovelPriceCmd(*rootFlags) *cobra.Command    { return novelSlot("price") }
func newNovelUploadCmd(*rootFlags) *cobra.Command   { return novelSlot("upload") }
func newNovelDownloadCmd(*rootFlags) *cobra.Command { return novelSlot("download") }
func newNovelLastCmd(*rootFlags) *cobra.Command     { return novelSlot("last") }
func newNovelAliasesCmd(*rootFlags) *cobra.Command  { return novelSlot("aliases") }
func newNovelInitCmd(*rootFlags) *cobra.Command     { return novelSlot("init") }
func newNovelPlanCmd(*rootFlags) *cobra.Command     { return novelSlot("plan") }
func newNovelQaCmd(*rootFlags) *cobra.Command       { return novelSlot("qa") }
func newNovelPackCmd(*rootFlags) *cobra.Command     { return novelSlot("pack") }
func newNovelBatchCmd(*rootFlags) *cobra.Command    { return novelSlot("batch") }
func newNovelVariantsCmd(*rootFlags) *cobra.Command { return novelSlot("variants") }
func newNovelComposeCmd(*rootFlags) *cobra.Command  { return novelSlot("compose") }
func newNovelAspectsCmd(*rootFlags) *cobra.Command  { return novelSlot("aspects") }
func newNovelRestyleCmd(*rootFlags) *cobra.Command  { return novelSlot("restyle") }
func newNovelLibraryCmd(*rootFlags) *cobra.Command  { return novelSlot("library") }
func newNovelBrandCmd(*rootFlags) *cobra.Command    { return novelSlot("brand") }

// wavespeedCommandAliases mirrors the official WaveSpeed CLI vocabulary on top
// of the generated endpoint names; existing scripts call both forms.
var wavespeedCommandAliases = map[string][]string{
	"account-balance":      {"balance"},
	"predictions":          {"history"},
	"prediction-results":   {"show"},
	"prediction-deletions": {"delete"},
}

func init() {
	registerNovelCommand(func(rootCmd *cobra.Command, flags *rootFlags) {
		rootCmd.AddCommand(newRunCmd(flags))
		rootCmd.AddCommand(newSchemaCmd(flags))
		rootCmd.AddCommand(newPriceCmd(flags))
		rootCmd.AddCommand(newUploadCmd(flags))
		rootCmd.AddCommand(newDownloadCmd(flags))
		rootCmd.AddCommand(newLastCmd(flags))
		rootCmd.AddCommand(newOpenCmd(flags))
		rootCmd.AddCommand(newAliasesCmd(flags))
		rootCmd.AddCommand(newInitCmd(flags))
		rootCmd.AddCommand(newPlanCmd(flags))
		rootCmd.AddCommand(newQACmd(flags))
		rootCmd.AddCommand(newPackCmd(flags))
		rootCmd.AddCommand(newBatchCmd(flags))
		rootCmd.AddCommand(newVariantsCmd(flags))
		rootCmd.AddCommand(newComposeCmd(flags))
		rootCmd.AddCommand(newAspectsCmd(flags))
		rootCmd.AddCommand(newRestyleCmd(flags))
		rootCmd.AddCommand(newLibraryCmd(flags))
		rootCmd.AddCommand(newBrandCmd(flags))
		for _, child := range rootCmd.Commands() {
			for _, alias := range wavespeedCommandAliases[child.Name()] {
				if !child.HasAlias(alias) {
					child.Aliases = append(child.Aliases, alias)
				}
			}
		}
	})
}

// archiveDBPath is the generated sync store that workflow archive/status and
// novel commands (cached pricing) use. WAVESPEED_ARCHIVE_DB overrides it.
// PATCH(legacy-archive-db): releases before the 4.32.6 reprint archived into
// archive.db beside data.db while sync wrote data.db. Everything now shares
// data.db, the store sync writes. A leftover archive.db is merged into data.db
// through SQLite (so rows still in its WAL are read too), then renamed aside.
func archiveDBPath() string {
	if env := strings.TrimSpace(os.Getenv("WAVESPEED_ARCHIVE_DB")); env != "" {
		return env
	}
	current := defaultDBPath("wavespeed-pp-cli")
	if err := migrateLegacyArchiveDB(context.Background(), current); err != nil {
		fmt.Fprintf(os.Stderr, "warning: archive.db from an earlier release was not merged into %s (it is kept, and the merge retries next run): %v\n", current, err)
	}
	return current
}

// migrateLegacyArchiveDB copies every row of a legacy archive.db into data.db
// with INSERT OR IGNORE, whether or not data.db already exists (learning
// setup can create it first). Rows already in data.db win. The legacy file
// is renamed to archive.db.migrated only after the copy commits, so a failure
// leaves it in place for the next run, and a repeat merge is a no-op.
func migrateLegacyArchiveDB(ctx context.Context, current string) error {
	legacy := filepath.Join(filepath.Dir(current), "archive.db")
	if info, err := os.Stat(legacy); err != nil || !info.Mode().IsRegular() {
		return nil
	}
	s, err := store.OpenWithContext(ctx, current)
	if err != nil {
		return err
	}
	if err := mergeLegacyArchive(ctx, s.DB(), legacy); err != nil {
		_ = s.Close()
		return err
	}
	if err := s.Close(); err != nil {
		return err
	}
	if err := os.Rename(legacy, legacy+".migrated"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("merged, but could not rename %s: %w", legacy, err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(legacy + suffix)
	}
	return nil
}

func mergeLegacyArchive(ctx context.Context, db *sql.DB, legacy string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS legacy`, legacy); err != nil {
		return fmt.Errorf("attach %s: %w", legacy, err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), `DETACH DATABASE legacy`) }()
	tables, err := sqliteTables(ctx, conn, "legacy")
	if err != nil {
		return err
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, table := range tables {
		cols, err := sharedColumns(ctx, tx, table)
		if err != nil {
			return err
		}
		if len(cols) == 0 {
			continue
		}
		list := `"` + strings.Join(cols, `", "`) + `"`
		q := fmt.Sprintf(`INSERT OR IGNORE INTO main."%s" (%s) SELECT %s FROM legacy."%s"`, table, list, list, table)
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("merge %s: %w", table, err)
		}
	}
	return tx.Commit()
}

// sqliteTables lists ordinary tables, skipping SQLite internals and FTS
// shadow tables (main's triggers rebuild those from the merged rows).
func sqliteTables(ctx context.Context, conn *sql.Conn, schema string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM %s.sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%%' AND sql NOT LIKE 'CREATE VIRTUAL%%'`, schema))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if strings.Contains(name, "_fts") || strings.ContainsRune(name, '"') {
			continue
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func sharedColumns(ctx context.Context, tx *sql.Tx, table string) ([]string, error) {
	cols := func(schema string) (map[string]bool, []string, error) {
		rows, err := tx.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM pragma_table_info('%s', '%s')`, table, schema))
		if err != nil {
			return nil, nil, err
		}
		defer rows.Close()
		set := map[string]bool{}
		var order []string
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, nil, err
			}
			set[name] = true
			order = append(order, name)
		}
		return set, order, rows.Err()
	}
	mainCols, _, err := cols("main")
	if err != nil {
		return nil, err
	}
	_, legacyOrder, err := cols("legacy")
	if err != nil {
		return nil, err
	}
	var shared []string
	for _, name := range legacyOrder {
		if mainCols[name] && !strings.ContainsRune(name, '"') {
			shared = append(shared, name)
		}
	}
	return shared, nil
}

// libraryDBFile is the library database file name inside the data dir.
const libraryDBFile = "library" + ".db"

// libraryDBPath is the D2C generation library database. It lives beside the
// generated sync store (data.db) under the CLI's data directory and keeps an
// independent schema version. WAVESPEED_LIBRARY_DB overrides the path.
func libraryDBPath() string {
	if env := strings.TrimSpace(os.Getenv("WAVESPEED_LIBRARY_DB")); env != "" {
		return env
	}
	if dir, err := cliutil.DataDir(); err == nil {
		return filepath.Join(dir, libraryDBFile)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "wavespeed-pp-cli", libraryDBFile)
}

// partialFailureErr marks a completed paid run whose follow-up step (a local
// download) failed. The generation is kept and recorded, so callers get exit 6
// with a partial_failure envelope rather than a generic API error.
func partialFailureErr(err error) error { return &cliError{code: 6, err: err} }

// printRunOutput prints hand-authored run/price/schema/upload/download output
// the way the published CLI always has: the payload itself, not the generated
// {meta, results} envelope. Scripts parse these documents directly (for
// example `price ... --agent | jq .data.unit_price`), so the shape is part of
// the CLI contract. --select, --compact, --quiet, and --csv still apply.
func printRunOutput(w io.Writer, data json.RawMessage, flags *rootFlags) error {
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	if flags.quiet {
		return nil
	}
	if flags.csv {
		return printCSV(w, data)
	}
	return printOutput(w, data, flags.asJSON)
}
