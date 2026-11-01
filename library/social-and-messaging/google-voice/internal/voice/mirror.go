package voice

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/google-voice/internal/cliutil"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Mirror struct {
	DB        *sql.DB
	AccountID string
	Path      string
}
type Query struct {
	Text, Participant, Direction, After, Before string
	HasAttachment                               bool
	Limit                                       int
	FTS                                         bool
	AfterSequence                               int64
}
type Record struct {
	Message
	AccountID string `json:"account_id"`
	Sequence  int64  `json:"sequence"`
	FirstRun  int64  `json:"first_run"`
	FirstSeen string `json:"first_seen"`
}
type SyncResult struct {
	RunID             int64  `json:"run_id"`
	Pages             int    `json:"pages"`
	NewMessages       int    `json:"new_messages"`
	Status            string `json:"status"`
	PendingThreads    int    `json:"pending_threads"`
	InventoryComplete bool   `json:"inventory_exhausted_at_observation"`
}
type Coverage struct {
	AccountID string           `json:"account_id"`
	Threads   []map[string]any `json:"threads"`
	Inventory []map[string]any `json:"inventory"`
	Runs      []map[string]any `json:"recent_runs"`
	Note      string           `json:"note"`
}

const mirrorSchema = `
 CREATE TABLE IF NOT EXISTS voice_meta(account_id TEXT PRIMARY KEY);
 CREATE UNIQUE INDEX IF NOT EXISTS voice_meta_singleton ON voice_meta((1));
 CREATE TABLE IF NOT EXISTS voice_runs(id INTEGER PRIMARY KEY AUTOINCREMENT,started_at TEXT NOT NULL,finished_at TEXT,status TEXT NOT NULL,pages INTEGER NOT NULL DEFAULT 0,error TEXT NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS voice_inventory(folder TEXT PRIMARY KEY,cursor TEXT NOT NULL,complete INTEGER NOT NULL,observed_at TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS voice_threads(id TEXT PRIMARY KEY,data TEXT NOT NULL,needs_refresh INTEGER NOT NULL DEFAULT 1,older_cursor TEXT NOT NULL DEFAULT '',history_complete INTEGER NOT NULL DEFAULT 0,last_refresh TEXT NOT NULL DEFAULT '',history_observed_at TEXT NOT NULL DEFAULT '',observed_at TEXT NOT NULL,last_work_run INTEGER NOT NULL DEFAULT 0,last_kind TEXT NOT NULL DEFAULT '',refresh_anchor TEXT NOT NULL DEFAULT '',refresh_cursor TEXT NOT NULL DEFAULT '',refresh_stop_id TEXT NOT NULL DEFAULT '');
 CREATE TABLE IF NOT EXISTS voice_participants(thread_id TEXT NOT NULL,phone TEXT NOT NULL,name TEXT NOT NULL,PRIMARY KEY(thread_id,phone));
 CREATE TABLE IF NOT EXISTS voice_messages(sequence INTEGER PRIMARY KEY AUTOINCREMENT,id TEXT NOT NULL UNIQUE,thread_id TEXT NOT NULL,ts INTEGER NOT NULL,direction TEXT NOT NULL,kind TEXT NOT NULL,text TEXT NOT NULL,has_attachment INTEGER NOT NULL,data TEXT NOT NULL,first_run INTEGER NOT NULL,first_seen TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS voice_message_thread_time ON voice_messages(thread_id,ts,id);
 CREATE INDEX IF NOT EXISTS voice_participant_phone ON voice_participants(phone,thread_id);
 CREATE VIRTUAL TABLE IF NOT EXISTS voice_fts USING fts5(id UNINDEXED,text);
 CREATE TABLE IF NOT EXISTS voice_pages(run_id INTEGER NOT NULL,kind TEXT NOT NULL,scope TEXT NOT NULL,cursor_in TEXT NOT NULL,cursor_out TEXT NOT NULL,observed_at TEXT NOT NULL,item_count INTEGER NOT NULL);
`

func OpenMirror(path, account string, write bool) (*Mirror, error) {
	return OpenMirrorContext(context.Background(), path, account, write)
}
func OpenMirrorContext(ctx context.Context, path, account string, write bool) (*Mirror, error) {
	if err := privatePath(path); err != nil {
		return nil, err
	}
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("Voice database must be a regular private file")
		}
		if err = cliutil.VerifyCredsPerms(path); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) || !write {
		return nil, err
	}
	if write {
		if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
			return nil, e
		}
		if e := os.Chmod(filepath.Dir(path), 0700); e != nil { // #nosec G302 -- This is a private directory; owner traversal is required and no group/other access is granted.
			return nil, e
		}
		f, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600) // #nosec G304 -- Operator-selected private mirror path; regular-file, ownership, ACL and trusted-ancestor checks guard access before SQLite reads or writes.
		if e != nil {
			return nil, e
		}
		if e = f.Close(); e != nil {
			return nil, e
		}
		if e = cliutil.VerifyCredsPerms(path); e != nil {
			return nil, e
		}
	}
	if e := verifyPrivateDirectory(filepath.Dir(path)); e != nil {
		return nil, e
	}
	initial, e := os.Lstat(path)
	if e != nil {
		return nil, e
	}
	canonical, e := filepath.EvalSymlinks(path)
	if e != nil {
		return nil, e
	}
	path = canonical
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{"_pragma": {"busy_timeout(3000)", "foreign_keys(1)"}}
	if !write {
		q.Set("mode", "ro")
		q.Add("_pragma", "query_only(1)")
	}
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	m := &Mirror{DB: db, AccountID: account, Path: path}
	fail := func(e error) (*Mirror, error) { return nil, errors.Join(e, db.Close()) }
	if write {
		e = mirrorLock(ctx, path+".schema", func() error {
			if e := verifyPrivateDirectory(filepath.Dir(path)); e != nil {
				return e
			}
			current, e := os.Lstat(path)
			if e != nil {
				return e
			}
			if !current.Mode().IsRegular() || !os.SameFile(initial, current) {
				return fmt.Errorf("Voice database path changed while waiting for the schema lock")
			}
			if e = cliutil.VerifyCredsPerms(path); e != nil {
				return e
			}
			return m.initialize(ctx)
		})
		if e != nil {
			return fail(e)
		}
	}
	current, e := os.Lstat(path)
	if e != nil {
		return fail(e)
	}
	if !current.Mode().IsRegular() || !os.SameFile(initial, current) {
		return fail(fmt.Errorf("Voice database path changed during open"))
	}
	var stored string
	e = db.QueryRowContext(ctx, "SELECT account_id FROM voice_meta LIMIT 1").Scan(&stored)
	if errors.Is(e, sql.ErrNoRows) && write && account != "" {
		if _, e = db.ExecContext(ctx, "INSERT OR IGNORE INTO voice_meta(account_id) VALUES(?)", account); e == nil {
			e = db.QueryRowContext(ctx, "SELECT account_id FROM voice_meta LIMIT 1").Scan(&stored)
		}
	}
	if e != nil {
		return fail(e)
	}
	if account != "" && stored != account {
		return fail(fmt.Errorf("local Voice database belongs to a different account"))
	}
	m.AccountID = stored
	return m, nil
}
func (m *Mirror) initialize(ctx context.Context) error {
	tx, e := m.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback() // #nosec G104 -- Best-effort rollback on failure; after successful Commit, sql.ErrTxDone is expected and must not change success.
	if _, e = tx.ExecContext(ctx, mirrorSchema); e != nil {
		return e
	}
	rows, e := tx.QueryContext(ctx, "PRAGMA table_info(voice_threads)")
	if e != nil {
		return e
	}
	columns := map[string]bool{}
	for rows.Next() {
		var cid, required, pk int
		var name, typ string
		var def sql.NullString
		if e = rows.Scan(&cid, &name, &typ, &required, &def, &pk); e != nil {
			return errors.Join(e, rows.Close())
		}
		columns[name] = true
	}
	if e = rows.Err(); e != nil {
		return errors.Join(e, rows.Close())
	}
	if e = rows.Close(); e != nil {
		return e
	}
	upgraded := false
	for _, name := range []string{"refresh_anchor", "refresh_cursor", "refresh_stop_id"} {
		if !columns[name] {
			if _, e = tx.ExecContext(ctx, "ALTER TABLE voice_threads ADD COLUMN "+name+" TEXT NOT NULL DEFAULT ''"); e != nil {
				return e
			}
			upgraded = true
		}
	}
	// Earlier local builds did not track refresh gaps; restart a bounded verification walk once.
	if upgraded {
		if _, e = tx.ExecContext(ctx, "UPDATE voice_threads SET needs_refresh=1,older_cursor='',history_complete=0,history_observed_at=''"); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func mirrorLock(ctx context.Context, path string, fn func() error) error {
	if fi, e := os.Lstat(path + ".lock"); e == nil {
		if !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Voice lock must be a regular private file")
		}
		if e = cliutil.VerifyCredsPerms(path + ".lock"); e != nil {
			return e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	for {
		if e := ctx.Err(); e != nil {
			return e
		}
		locked, e := cliutil.TryWithFileLock(path, fn)
		if e != nil {
			return e
		}
		if locked {
			return nil
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func (m *Mirror) Close() error { return m.DB.Close() }
func (m *Mirror) commit(ctx context.Context, run int64, kind, scope, input, output string, threads []Thread) (int, error) {
	tx, err := m.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	added := 0
	for _, t := range threads {
		raw, e := json.Marshal(t)
		if e != nil {
			return 0, e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO voice_threads(id,data,observed_at) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET data=excluded.data,observed_at=excluded.observed_at", t.ID, string(raw), now); e != nil {
			return 0, e
		}
		if _, e = tx.ExecContext(ctx, "DELETE FROM voice_participants WHERE thread_id=?", t.ID); e != nil {
			return 0, e
		}
		for _, c := range t.Contacts {
			if _, e = tx.ExecContext(ctx, "INSERT OR REPLACE INTO voice_participants(thread_id,phone,name) VALUES(?,?,?)", t.ID, c.Phone, c.Name); e != nil {
				return 0, e
			}
		}
		for _, msg := range t.Messages {
			b, e := json.Marshal(msg)
			if e != nil {
				return 0, e
			}
			result, e := tx.ExecContext(ctx, `INSERT INTO voice_messages(id,thread_id,ts,direction,kind,text,has_attachment,data,first_run,first_seen) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, msg.ID, t.ID, msg.TimestampMS, msg.Direction, msg.Kind, msg.Text, len(msg.Attachments) > 0, string(b), run, now)
			if e != nil {
				return 0, e
			}
			n, e := result.RowsAffected()
			if e != nil {
				return 0, e
			}
			added += int(n)
			if _, e = tx.ExecContext(ctx, "UPDATE voice_messages SET data=?,text=?,direction=?,kind=?,has_attachment=?,ts=? WHERE id=? AND thread_id=?", string(b), msg.Text, msg.Direction, msg.Kind, len(msg.Attachments) > 0, msg.TimestampMS, msg.ID, t.ID); e != nil {
				return 0, e
			}
			var previousThread string
			if e = tx.QueryRowContext(ctx, "SELECT thread_id FROM voice_messages WHERE id=?", msg.ID).Scan(&previousThread); e != nil {
				return 0, e
			}
			if previousThread != t.ID {
				return 0, fmt.Errorf("source message ID moved to a different thread; refusing page")
			}
			if _, e = tx.ExecContext(ctx, "DELETE FROM voice_fts WHERE id=?", msg.ID); e != nil {
				return 0, e
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO voice_fts(id,text) VALUES(?,?)", msg.ID, msg.Text); e != nil {
				return 0, e
			}
		}
		if kind == "inventory" {
			if _, e = tx.ExecContext(ctx, "UPDATE voice_threads SET needs_refresh=1 WHERE id=?", t.ID); e != nil {
				return 0, e
			}
		} else {
			var anchor, gap, stop, older string
			var complete bool
			if e = tx.QueryRowContext(ctx, "SELECT refresh_anchor,refresh_cursor,refresh_stop_id,older_cursor,history_complete FROM voice_threads WHERE id=?", t.ID).Scan(&anchor, &gap, &stop, &older, &complete); e != nil {
				return 0, e
			}
			historyAt := ""
			fresh := false
			switch kind {
			case "refresh":
				fresh = true
				if t.OlderCursor == "" {
					older = ""
					complete = true
					historyAt = now
					gap = ""
					stop = ""
				} else {
					if anchor != "" && !containsMessage(t.Messages, anchor) {
						gap = t.OlderCursor
						if stop == "" {
							stop = anchor
						}
					}
					if !complete && older == "" {
						older = t.OlderCursor
					}
				}
				if head := latestMessageID(t.Messages); head != "" {
					anchor = head
				}
			case "refresh_gap":
				if t.OlderCursor == "" {
					gap = ""
					stop = ""
					older = ""
					complete = true
					historyAt = now
				} else if stop != "" && containsMessage(t.Messages, stop) {
					gap = ""
					stop = ""
				} else {
					gap = t.OlderCursor
				}
			case "history":
				older = t.OlderCursor
				complete = older == ""
				historyAt = now
			default:
				return 0, fmt.Errorf("unknown Voice page work kind")
			}
			if _, e = tx.ExecContext(ctx, `UPDATE voice_threads SET refresh_anchor=?,refresh_cursor=?,refresh_stop_id=?,older_cursor=?,history_complete=?,last_work_run=?,last_kind=?,needs_refresh=CASE WHEN ? THEN 0 ELSE needs_refresh END,last_refresh=CASE WHEN ? THEN ? ELSE last_refresh END,history_observed_at=CASE WHEN ?<>'' THEN ? ELSE history_observed_at END WHERE id=?`, anchor, gap, stop, older, complete, run, kind, fresh, fresh, now, historyAt, historyAt, t.ID); e != nil {
				return 0, e
			}
		}
	}
	if kind == "inventory" {
		if _, err = tx.ExecContext(ctx, "INSERT INTO voice_inventory(folder,cursor,complete,observed_at) VALUES(?,?,?,?) ON CONFLICT(folder) DO UPDATE SET cursor=excluded.cursor,complete=excluded.complete,observed_at=excluded.observed_at", scope, output, output == "", now); err != nil {
			return 0, err
		}
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO voice_pages VALUES(?,?,?,?,?,?,?)", run, kind, scope, input, output, now, len(threads)); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE voice_runs SET pages=pages+1 WHERE id=?", run); err != nil {
		return 0, err
	}
	return added, tx.Commit()
}

func (m *Mirror) Messages(ctx context.Context, q Query) ([]Record, error) {
	if q.Limit < 1 || q.Limit > 10000 {
		return nil, fmt.Errorf("--limit must be 1 through 10000")
	}
	where := []string{"1=1"}
	args := []any{}
	if q.Text != "" {
		if q.FTS {
			where = append(where, "m.id IN(SELECT id FROM voice_fts WHERE voice_fts MATCH ?)")
			args = append(args, `"`+strings.ReplaceAll(q.Text, `"`, `""`)+`"`)
		} else {
			where = append(where, "instr(lower(m.text),lower(?))>0")
			args = append(args, q.Text)
		}
	}
	if q.Participant != "" {
		where = append(where, "EXISTS(SELECT 1 FROM voice_participants p WHERE p.thread_id=m.thread_id AND p.phone=?)")
		args = append(args, q.Participant)
	}
	if q.Direction != "" {
		if q.Direction != "inbound" && q.Direction != "outbound" && q.Direction != "unknown" {
			return nil, fmt.Errorf("--direction must be inbound, outbound or unknown")
		}
		where = append(where, "m.direction=?")
		args = append(args, q.Direction)
	}
	for _, bound := range []struct{ v, op string }{{q.After, ">="}, {q.Before, "<"}} {
		if bound.v != "" {
			t, e := time.Parse(time.RFC3339Nano, bound.v)
			if e != nil {
				return nil, fmt.Errorf("--after and --before must be RFC3339 timestamps")
			}
			where = append(where, "m.ts "+bound.op+" ?")
			args = append(args, ceilMillis(t))
		}
	}
	if q.HasAttachment {
		where = append(where, "m.has_attachment=1")
	}
	if q.AfterSequence > 0 {
		where = append(where, "m.sequence>?")
		args = append(args, q.AfterSequence)
	}
	order := "m.ts DESC,m.id DESC"
	if q.AfterSequence > 0 {
		order = "m.sequence ASC"
	}
	args = append(args, q.Limit)
	rows, e := m.DB.QueryContext(ctx, "SELECT m.data,m.sequence,m.first_run,m.first_seen FROM voice_messages m WHERE "+strings.Join(where, " AND ")+" ORDER BY "+order+" LIMIT ?", args...) // #nosec G202 -- SQL fragments and order clauses are code-owned constants; every caller-provided filter and limit is bound through placeholders.
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var b string
		var v Record
		v.AccountID = m.AccountID
		if e = rows.Scan(&b, &v.Sequence, &v.FirstRun, &v.FirstSeen); e != nil {
			return nil, e
		}
		if e = json.Unmarshal([]byte(b), &v.Message); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (m *Mirror) Coverage(ctx context.Context, participant string, limit int) (Coverage, error) {
	c := Coverage{AccountID: m.AccountID, Threads: []map[string]any{}, Inventory: []map[string]any{}, Runs: []map[string]any{}, Note: "Only observed history is represented. Timestamp bounds never prove continuous or complete history. Exhausted cursors are observations at their recorded time."}
	if limit < 1 || limit > 10000 {
		return c, fmt.Errorf("--limit must be 1 through 10000")
	}
	queries := []struct {
		sql  string
		args []any
		dest *[]map[string]any
	}{{"SELECT folder,cursor<>'' AS pending_cursor,complete AS exhausted_at_observation,observed_at FROM voice_inventory ORDER BY folder", nil, &c.Inventory}, {"SELECT id,status,pages,started_at,finished_at,error FROM voice_runs ORDER BY id DESC LIMIT 10", nil, &c.Runs}}
	threadSQL := `SELECT t.id,t.needs_refresh,t.older_cursor<>'' AS pending_older_cursor,t.history_complete AS older_history_exhausted,t.refresh_cursor<>'' AS pending_refresh_gap,t.last_refresh,t.history_observed_at,t.observed_at,(SELECT min(ts) FROM voice_messages m WHERE m.thread_id=t.id) AS oldest_message_ms,(SELECT max(ts) FROM voice_messages m WHERE m.thread_id=t.id) AS newest_message_ms FROM voice_threads t`
	args := []any{}
	if participant != "" {
		threadSQL += " WHERE EXISTS(SELECT 1 FROM voice_participants p WHERE p.thread_id=t.id AND p.phone=?)"
		args = append(args, participant)
	}
	threadSQL += " ORDER BY t.id LIMIT ?"
	args = append(args, limit)
	queries = append(queries, struct {
		sql  string
		args []any
		dest *[]map[string]any
	}{threadSQL, args, &c.Threads})
	for _, q := range queries {
		rows, e := m.DB.QueryContext(ctx, q.sql, q.args...)
		if e != nil {
			return c, e
		}
		cols, e := rows.Columns()
		if e != nil {
			return c, errors.Join(e, rows.Close())
		}
		for rows.Next() {
			values := make([]any, len(cols))
			targets := make([]any, len(cols))
			for i := range values {
				targets[i] = &values[i]
			}
			if e = rows.Scan(targets...); e != nil {
				return c, errors.Join(e, rows.Close())
			}
			v := map[string]any{}
			for i, col := range cols {
				v[col] = values[i]
			}
			*q.dest = append(*q.dest, v)
		}
		if e = rows.Err(); e != nil {
			return c, errors.Join(e, rows.Close())
		}
		if e = rows.Close(); e != nil {
			return c, e
		}
	}
	for _, folder := range []string{"texts", "archived"} {
		found := false
		for _, item := range c.Inventory {
			if item["folder"] == folder {
				found = true
			}
		}
		if !found {
			c.Inventory = append(c.Inventory, map[string]any{"folder": folder, "observation": "unknown: not collected in this profile", "exhausted_at_observation": nil, "observed_at": nil})
		}
	}
	return c, nil
}
