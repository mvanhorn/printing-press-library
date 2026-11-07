// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output: secrets are replaced before any row is stored.

package store

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/secretfield"
)

// secretsSealedSchemaVersion is the first schema whose rows hold no secret values.
const secretsSealedSchemaVersion = 15

const saltFileName = "mirror-secret-salt"

// unsaltedPlaceholder is used when no salt is available. It loses change detection for
// that field, which is the right side to lose: the alternative is storing the secret.
const unsaltedPlaceholder = secretfield.DigestPrefix + "unavailable]"

var (
	saltMu       sync.Mutex
	saltByPath   = map[string][]byte{}
	saltWarnOnce sync.Once
)

// mirrorSalt returns this install's salt. It lives in the state directory, not with the
// database, so someone holding a copied database has digests but no way to test guesses
// against them, which matters because WiFi passphrases are often guessable. Cached per
// path, because tests and profiles can change the state directory within one process.
func mirrorSalt() []byte {
	dir, err := cliutil.StateDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(dir, saltFileName)
	saltMu.Lock()
	defer saltMu.Unlock()
	if salt, ok := saltByPath[path]; ok {
		return salt
	}
	salt, err := readOrCreateSalt(path)
	if err != nil {
		saltWarnOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "warning: mirror secret salt unavailable (%v); secrets are still not stored, but drift cannot detect changes to them\n", err)
		})
		return nil
	}
	saltByPath[path] = salt
	return salt
}

func readOrCreateSalt(path string) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	fresh := make([]byte, 32)
	if _, err := rand.Read(fresh); err != nil {
		return nil, err
	}
	// O_EXCL so two processes on a first run cannot each keep a different salt, which
	// would read as every secret having changed.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- state dir plus a constant name; never set by an API response or MCP argument
	if err == nil {
		defer f.Close()
		if _, err := f.WriteString(hex.EncodeToString(fresh)); err != nil {
			_ = os.Remove(path)
			return nil, err
		}
		return fresh, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("salt file is a symlink")
	}
	if info.Mode().Perm() != 0o600 {
		if err := os.Chmod(path, 0o600); err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(path) // #nosec G304 -- state dir plus a constant name; never set by an API response or MCP argument
	if err != nil {
		return nil, err
	}
	salt, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(salt) != 32 {
		return nil, errors.New("salt file is not 32 hex-encoded bytes")
	}
	return salt, nil
}

func sealValue(value any) any {
	if s, ok := value.(string); ok && secretfield.IsPlaceholder(s) {
		return s // already sealed; sealing twice would read as a change to drift
	}
	salt := mirrorSalt()
	if salt == nil {
		return unsaltedPlaceholder
	}
	var material []byte
	if s, ok := value.(string); ok {
		material = []byte(s)
	} else {
		// encoding/json sorts map keys, so equal objects produce equal digests.
		encoded, err := json.Marshal(value)
		if err != nil {
			return unsaltedPlaceholder
		}
		material = encoded
	}
	mac := hmac.New(sha256.New, salt)
	mac.Write(material)
	return secretfield.DigestPrefix + hex.EncodeToString(mac.Sum(nil))[:32] + "]"
}

// sealStoredJSON is the one gate between API data and SQLite. A row with no secret keeps
// its original bytes, so sealing cannot disturb field order or large numbers elsewhere.
func sealStoredJSON(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value any
	if dec.Decode(&value) != nil {
		return data
	}
	sealed, changed := secretfield.Walk(value, sealValue)
	if !changed {
		return data
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(sealed); err != nil {
		// Not storable safely, and an empty object is honest about having lost it.
		return []byte(`{}`)
	}
	return bytes.TrimRight(out.Bytes(), "\n")
}

// sealExistingSecrets rewrites rows stored before sealing existed. It finds columns by
// shape instead of a table list, so a typed table added by a later print is covered.
// Called inside the migration transaction.
func sealExistingSecrets(ctx context.Context, conn *sql.Conn) (int, error) {
	tables, err := sealableTables(ctx, conn)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, table := range tables {
		cols, err := sealableColumns(ctx, conn, table)
		if err != nil {
			return total, err
		}
		for _, col := range cols {
			count, err := sealColumn(ctx, conn, table, col)
			if err != nil {
				return total, fmt.Errorf("sealing %s.%s: %w", table, col, err)
			}
			total += count
		}
	}
	return total, nil
}

func sealableTables(ctx context.Context, conn *sql.Conn) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '%\_fts%' ESCAPE '\' AND sql NOT LIKE 'CREATE VIRTUAL%'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func sealableColumns(ctx context.Context, conn *sql.Conn, table string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`SELECT name FROM pragma_table_info(%q)`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		switch {
		case name == "data", name == "request_body", name == "before_image", name == "after_image":
			out = append(out, name)
		case secretfield.IsSecretField(name) && name != "secrets_scrubbed":
			out = append(out, name)
		}
	}
	return out, rows.Err()
}

func sealColumn(ctx context.Context, conn *sql.Conn, table, col string) (int, error) {
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`SELECT rowid, %q FROM %q WHERE %q IS NOT NULL AND %q != ''`, col, table, col, col))
	if err != nil {
		return 0, err
	}
	type change struct {
		rowid int64
		value string
	}
	var changes []change
	wholeValue := col != "data" && col != "request_body" && col != "before_image" && col != "after_image"
	for rows.Next() {
		var rowid int64
		var raw sql.NullString
		if err := rows.Scan(&rowid, &raw); err != nil {
			_ = rows.Close()
			return 0, err
		}
		next := raw.String
		if wholeValue {
			// A typed column named like a secret is the value itself, not a JSON document.
			if sealed, ok := sealValue(raw.String).(string); ok {
				next = sealed
			}
		} else {
			next = string(sealStoredJSON([]byte(raw.String)))
		}
		if next != raw.String {
			changes = append(changes, change{rowid, next})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return 0, err
	}
	// Closed before writing: the migration holds a single pinned connection.
	if err := rows.Close(); err != nil {
		return 0, err
	}
	for _, c := range changes {
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`UPDATE %q SET %q = ? WHERE rowid = ?`, table, col), c.value, c.rowid); err != nil {
			return 0, err
		}
	}
	return len(changes), nil
}

// removeOrphanFTSEntries drops index text no row accounts for. Rewriting rows cannot reach
// it, and it may have been indexed from a secret.
func removeOrphanFTSEntries(ctx context.Context, conn *sql.Conn) (int64, error) {
	exists, err := tableExists(ctx, conn, "resources_fts")
	if err != nil || !exists {
		return 0, err
	}
	result, err := conn.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid IN (
		SELECT f.rowid FROM resources_fts f LEFT JOIN resources r ON r.id = f.id AND r.resource_type = f.resource_type WHERE r.id IS NULL)`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// compactAfterSealing removes the old values from the file itself. Rewriting a row leaves
// its previous bytes in free pages and the WAL, so without this a passphrase would
// survive the migration in exactly the raw bytes someone copying the file would get.
func compactAfterSealing(ctx context.Context, conn *sql.Conn) {
	if _, err := conn.ExecContext(ctx, `VACUUM`); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not compact the local mirror after removing secrets: %v\n", err)
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not checkpoint the local mirror after removing secrets: %v\n", err)
	}
}

// migrateBeforeReadOnlyOpen exists because read-only opens run no migrations, and they are
// how MCP sql and search reach the store, so an older file would keep serving plaintext
// until something happened to open it for writing. If it cannot be upgraded, refusing is
// right: there is no way to serve that file without its secrets.
func migrateBeforeReadOnlyOpen(ctx context.Context, dbPath string) error {
	if info, err := os.Stat(dbPath); err != nil || info.Size() == 0 {
		return nil
	}
	probe, err := openReadOnlySQLite(ctx, "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil // the real open below reports this with its own context
	}
	var version int
	err = probe.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version)
	_ = probe.Close()
	// Version 0 is left alone: a file this CLI wrote is always stamped, and an unstamped
	// one is an older or foreign file that read-only callers must be able to open unchanged.
	if err != nil || version == 0 || version >= secretsSealedSchemaVersion {
		return nil
	}
	upgraded, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("this store predates secret removal and could not be upgraded, so it was not opened: %w", err)
	}
	return upgraded.Close()
}
