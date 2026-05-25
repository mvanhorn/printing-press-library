package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func (s *Store) DB() *sql.DB { return s.db }

type SyncMeta struct {
	SyncedAt                    string `json:"synced_at"`
	Profile                     string `json:"profile"`
	URLsCount                   int64  `json:"urls_count"`
	VisitsCount                 int64  `json:"visits_count"`
	TermsCount                  int64  `json:"terms_count"`
	ChromeSchemaVersion         int64  `json:"chrome_schema_version"`
	ChromeLastCompatibleVersion int64  `json:"chrome_last_compatible_version"`
}

func Open(snapshotPath string) (*Store, error) {
	db, err := sql.Open("sqlite", snapshotPath)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
func (s *Store) Close() error { return s.db.Close() }

func BuildSnapshotIndex(snapshotPath, profile string) (SyncMeta, error) {
	return BuildSnapshotIndexWithVersions(snapshotPath, profile, 0, 0)
}

func BuildSnapshotIndexWithVersions(snapshotPath, profile string, schemaVersion, lastCompat int64) (SyncMeta, error) {
	st, err := Open(snapshotPath)
	if err != nil {
		return SyncMeta{}, err
	}
	defer st.Close()
	if err := st.createMeta(); err != nil {
		return SyncMeta{}, err
	}
	if err := st.createFTS(); err != nil {
		return SyncMeta{}, err
	}
	if err := st.populateFTS(); err != nil {
		return SyncMeta{}, err
	}
	meta := SyncMeta{SyncedAt: time.Now().UTC().Format(time.RFC3339), Profile: profile}
	if schemaVersion > 0 {
		meta.ChromeSchemaVersion = schemaVersion
		meta.ChromeLastCompatibleVersion = lastCompat
	} else if err := st.loadChromeVersionMeta(&meta); err != nil {
		return SyncMeta{}, err
	}
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM urls`).Scan(&meta.URLsCount); err != nil {
		return SyncMeta{}, err
	}
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM visits`).Scan(&meta.VisitsCount); err != nil {
		return SyncMeta{}, err
	}
	if tableExists(st.db, "keyword_search_terms") {
		if err := st.db.QueryRow(`SELECT COUNT(*) FROM keyword_search_terms`).Scan(&meta.TermsCount); err != nil {
			return SyncMeta{}, err
		}
	}
	if _, err := st.db.Exec(`INSERT INTO meta_pp(synced_at, profile, urls_count, visits_count, terms_count, chrome_schema_version, chrome_last_compatible_version) VALUES(?,?,?,?,?,?,?)`, meta.SyncedAt, meta.Profile, meta.URLsCount, meta.VisitsCount, meta.TermsCount, meta.ChromeSchemaVersion, meta.ChromeLastCompatibleVersion); err != nil {
		return SyncMeta{}, err
	}
	return meta, nil
}

func tableExists(db *sql.DB, table string) bool {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table).Scan(&name)
	return err == nil && name == table
}

func (s *Store) createMeta() error {
	if _, err := s.db.Exec(`DROP TABLE IF EXISTS meta_pp`); err != nil {
		return err
	}
	_, err := s.db.Exec(`CREATE TABLE meta_pp (
		synced_at TEXT NOT NULL,
		profile TEXT NOT NULL,
		urls_count INTEGER NOT NULL,
		visits_count INTEGER NOT NULL,
		terms_count INTEGER NOT NULL,
		chrome_schema_version INTEGER NOT NULL,
		chrome_last_compatible_version INTEGER NOT NULL
	)`)
	return err
}

func (s *Store) loadChromeVersionMeta(meta *SyncMeta) error {
	if !tableExists(s.db, "meta") {
		return nil
	}
	rows, err := s.db.Query(`SELECT key, value FROM meta WHERE key IN ('version','last_compatible_version')`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		if !k.Valid || !v.Valid {
			continue
		}
		var n int64
		if _, err := fmt.Sscanf(v.String, "%d", &n); err != nil {
			continue
		}
		switch k.String {
		case "version":
			meta.ChromeSchemaVersion = n
		case "last_compatible_version":
			meta.ChromeLastCompatibleVersion = n
		}
	}
	return rows.Err()
}

func (s *Store) createFTS() error {
	if _, err := s.db.Exec(`DROP TABLE IF EXISTS history_fts`); err != nil {
		return err
	}
	_, err := s.db.Exec(`CREATE VIRTUAL TABLE history_fts USING fts5(url, title, search_terms)`)
	return err
}

func (s *Store) populateFTS() error {
	query := `INSERT INTO history_fts(url, title, search_terms)
	SELECT u.url, COALESCE(u.title,''), COALESCE(GROUP_CONCAT(k.term, ' '), '')
	FROM urls u
	LEFT JOIN keyword_search_terms k ON k.url_id = u.id
	GROUP BY u.id`
	if !tableExists(s.db, "keyword_search_terms") {
		query = `INSERT INTO history_fts(url, title, search_terms)
		SELECT u.url, COALESCE(u.title,''), '' FROM urls u`
	}
	_, err := s.db.Exec(query)
	return err
}

func OpenExisting(snapshotPath string) (*Store, error) {
	if _, err := os.Stat(snapshotPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSnapshot
		}
		return nil, err
	}
	return Open(snapshotPath)
}

var ErrNoSnapshot = errors.New("snapshot not found")

func (s *Store) GetSyncMeta() (SyncMeta, error) {
	var m SyncMeta
	var sv, cv sql.NullInt64
	err := s.db.QueryRow(`SELECT synced_at, profile, urls_count, visits_count, terms_count, chrome_schema_version, chrome_last_compatible_version FROM meta_pp LIMIT 1`).
		Scan(&m.SyncedAt, &m.Profile, &m.URLsCount, &m.VisitsCount, &m.TermsCount, &sv, &cv)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return m, nil
		}
		return m, err
	}
	if sv.Valid {
		m.ChromeSchemaVersion = sv.Int64
	}
	if cv.Valid {
		m.ChromeLastCompatibleVersion = cv.Int64
	}
	return m, nil
}

func (s *Store) IsFTSReady() bool { return tableExists(s.db, "history_fts") }

// allowedCountTables bounds the table names RowCount will interpolate into SQL.
// Callers pass only these constants today; the allowlist keeps the string
// concatenation safe even if a future caller is less careful.
var allowedCountTables = map[string]struct{}{
	"urls":        {},
	"visits":      {},
	"history_fts": {},
}

func (s *Store) RowCount(table string) int64 {
	if _, ok := allowedCountTables[table]; !ok {
		return 0
	}
	if !tableExists(s.db, table) {
		return 0
	}
	var n int64
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		return 0
	}
	return n
}

// Compiled once at package load; IsSelectOnly/stripSQLComments run on every
// `sql` command (and every MCP tool call that routes through it), so recompiling
// these per-call is needless work.
var (
	// "replace" is intentionally absent: REPLACE() is a common read-only scalar
	// function. A REPLACE *statement* starts with "replace" and is already
	// rejected by the SELECT/WITH prefix check below.
	reBlockedSQL      = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|attach|alter|vacuum|create)\b`)
	rePragmaQueryOnly = regexp.MustCompile(`(?i)^\s*pragma\s+query_only\b`)
	reBlockComment    = regexp.MustCompile(`(?s)/\*.*?\*/`)
	reLineComment     = regexp.MustCompile(`(?m)--[^\n]*`)
	reStringLiteral   = regexp.MustCompile(`'(?:[^']|'')*'`)
)

func IsSelectOnly(q string) bool {
	n := strings.TrimSpace(q)
	if n == "" {
		return false
	}
	if hasSemicolonOutsideString(n) {
		return false
	}
	n = stripSQLComments(n)
	ln := strings.ToLower(strings.TrimSpace(n))
	if !strings.HasPrefix(ln, "select") && !strings.HasPrefix(ln, "with") {
		return false
	}
	// Blank string-literal contents before keyword matching so a blocked word
	// inside a LIKE pattern (e.g. '%create%') or any other literal is not
	// mistaken for a write statement.
	scan := reStringLiteral.ReplaceAllString(ln, "''")
	if reBlockedSQL.MatchString(scan) {
		return false
	}
	if strings.Contains(scan, "pragma") && !rePragmaQueryOnly.MatchString(scan) {
		return false
	}
	return true
}

func stripSQLComments(s string) string {
	return reLineComment.ReplaceAllString(reBlockComment.ReplaceAllString(s, " "), " ")
}

func hasSemicolonOutsideString(s string) bool {
	inSingle := false
	inDouble := false
	for _, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ';':
			if !inSingle && !inDouble {
				return true
			}
		}
	}
	return false
}

func (s *Store) RunSelect(query string, limit int) ([]map[string]any, error) {
	if !IsSelectOnly(query) {
		return nil, fmt.Errorf("only SELECT statements are allowed")
	}
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	// Authoritative read-only guard: with query_only set, SQLite rejects any
	// write on this connection regardless of the query text — the IsSelectOnly
	// text check is only a fast, friendly pre-filter.
	if _, err := conn.ExecContext(ctx, "PRAGMA query_only=ON"); err != nil {
		return nil, err
	}
	wrapped := fmt.Sprintf("SELECT * FROM (%s) LIMIT %d", query, limit)
	rows, err := conn.QueryContext(ctx, wrapped)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	res := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range ptrs {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		rec := map[string]any{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				rec[c] = string(b)
			} else {
				rec[c] = vals[i]
			}
		}
		res = append(res, rec)
	}
	return res, rows.Err()
}
