// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

// This file is hand-built (NOT generator-emitted). It defines the real
// Slack mirror schema and CRUD/query surface that the slack-pp-cli v1.1
// novel verbs depend on. The generator-emitted store.go carries
// spec-endpoint resource tables (resources, conversations_history,
// auth_test, ...) which are response snapshots, not mirror entities.
//
// Schema creation is lazy: EnsureMirrorSchema runs CREATE TABLE IF NOT
// EXISTS / CREATE VIRTUAL TABLE IF NOT EXISTS for every mirror table and
// is called from the mirror sync engine and from every mirror query
// method, so callers never see an un-migrated DB. The generated
// migrate() and StoreSchemaVersion are left untouched.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// mirrorDDL is the full mirror schema. Every statement is idempotent so
// the slice can be replayed on every process start without harm.
var mirrorDDL = []string{
	`CREATE TABLE IF NOT EXISTS m_channels (
		id            TEXT PRIMARY KEY,
		name          TEXT,
		is_archived   INTEGER DEFAULT 0,
		is_member     INTEGER DEFAULT 0,
		is_im         INTEGER DEFAULT 0,
		is_mpim       INTEGER DEFAULT 0,
		is_private    INTEGER DEFAULT 0,
		num_members   INTEGER DEFAULT 0,
		topic         TEXT,
		purpose       TEXT,
		synced_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_m_channels_name ON m_channels(name)`,

	`CREATE TABLE IF NOT EXISTS m_users (
		id            TEXT PRIMARY KEY,
		name          TEXT,
		real_name     TEXT,
		display_name  TEXT,
		email         TEXT,
		is_bot        INTEGER DEFAULT 0,
		deleted       INTEGER DEFAULT 0,
		tz            TEXT,
		synced_at     DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_m_users_name ON m_users(name)`,

	`CREATE TABLE IF NOT EXISTS m_messages (
		channel_id   TEXT NOT NULL,
		ts           TEXT NOT NULL,
		thread_ts    TEXT,
		user_id      TEXT,
		text         TEXT,
		subtype      TEXT,
		reply_count  INTEGER DEFAULT 0,
		reply_users  TEXT,
		reactions    TEXT,
		files        TEXT,
		permalink    TEXT,
		synced_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (channel_id, ts)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_m_messages_thread ON m_messages(channel_id, thread_ts)`,
	`CREATE INDEX IF NOT EXISTS idx_m_messages_user ON m_messages(user_id)`,
	`CREATE INDEX IF NOT EXISTS idx_m_messages_ts ON m_messages(ts)`,

	`CREATE TABLE IF NOT EXISTS m_threads (
		channel_id    TEXT NOT NULL,
		parent_ts     TEXT NOT NULL,
		last_reply_ts TEXT,
		reply_count   INTEGER DEFAULT 0,
		synced_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (channel_id, parent_ts)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_m_threads_last ON m_threads(last_reply_ts)`,

	`CREATE TABLE IF NOT EXISTS m_usergroups (
		id          TEXT PRIMARY KEY,
		handle      TEXT,
		name        TEXT,
		user_ids    TEXT,
		synced_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE TABLE IF NOT EXISTS m_reactions (
		message_channel_id TEXT NOT NULL,
		message_ts         TEXT NOT NULL,
		emoji_name         TEXT NOT NULL,
		user_ids           TEXT,
		count              INTEGER DEFAULT 0,
		synced_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (message_channel_id, message_ts, emoji_name)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_m_reactions_chan ON m_reactions(message_channel_id)`,

	`CREATE TABLE IF NOT EXISTS m_files (
		id          TEXT PRIMARY KEY,
		name        TEXT,
		mimetype    TEXT,
		url_private TEXT,
		permalink   TEXT,
		channel_id  TEXT,
		created     INTEGER DEFAULT 0,
		synced_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_m_files_chan ON m_files(channel_id)`,

	`CREATE TABLE IF NOT EXISTS m_audit_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ts          DATETIME DEFAULT CURRENT_TIMESTAMP,
		caller      TEXT,
		verb        TEXT,
		channel_id  TEXT,
		detail      TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_m_audit_ts ON m_audit_log(ts)`,

	// FTS5 mirror over message text + resolved user name. Kept in sync by
	// explicit upserts in UpsertMessages (the same explicit-upsert approach
	// resources_fts uses in store.go — no triggers, modernc.org/sqlite's
	// FTS5 trigger support is unreliable). content_rowid would couple the
	// FTS table to an INTEGER PK m_messages does not have, so this is a
	// standalone (non-external-content) FTS table keyed by an explicit
	// derived rowid.
	`CREATE VIRTUAL TABLE IF NOT EXISTS m_messages_fts USING fts5(
		channel_id, ts, user_name, text, tokenize='porter unicode61'
	)`,
}

// EnsureMirrorSchema creates the mirror tables and FTS index if absent.
// Every statement is IF NOT EXISTS, so the call is idempotent and cheap
// to repeat — it is run lazily by the mirror sync engine and by every
// mirror query method, so a caller can never hit an un-migrated DB. The
// write lock serialises it against concurrent mirror writes.
//
// A per-process sync.Once is deliberately NOT used: it would bind the
// "already created" decision to the first *Store opened in the process,
// which is wrong for tests (and any caller) that open several distinct
// databases. Replaying ~13 IF NOT EXISTS DDL statements is negligible.
func (s *Store) EnsureMirrorSchema(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, stmt := range mirrorDDL {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating mirror schema: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------
// Row types — the typed shapes the P1/P2 verbs consume.
// ---------------------------------------------------------------------

// Channel is one row of m_channels.
type Channel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsArchived bool   `json:"is_archived"`
	IsMember   bool   `json:"is_member"`
	IsIM       bool   `json:"is_im"`
	IsMPIM     bool   `json:"is_mpim"`
	IsPrivate  bool   `json:"is_private"`
	NumMembers int    `json:"num_members"`
	Topic      string `json:"topic"`
	Purpose    string `json:"purpose"`
}

// User is one row of m_users.
type User struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	RealName    string `json:"real_name"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	IsBot       bool   `json:"is_bot"`
	Deleted     bool   `json:"deleted"`
	TZ          string `json:"tz"`
}

// Message is one row of m_messages. ReplyUsers / Reactions / Files hold
// raw JSON so callers decide how deeply to parse.
type Message struct {
	ChannelID  string          `json:"channel_id"`
	TS         string          `json:"ts"`
	ThreadTS   string          `json:"thread_ts"`
	UserID     string          `json:"user_id"`
	Text       string          `json:"text"`
	Subtype    string          `json:"subtype"`
	ReplyCount int             `json:"reply_count"`
	ReplyUsers json.RawMessage `json:"reply_users,omitempty"`
	Reactions  json.RawMessage `json:"reactions,omitempty"`
	Files      json.RawMessage `json:"files,omitempty"`
	Permalink  string          `json:"permalink"`
}

// Thread is one row of m_threads — a derived index for the drift verb.
type Thread struct {
	ChannelID   string `json:"channel_id"`
	ParentTS    string `json:"parent_ts"`
	LastReplyTS string `json:"last_reply_ts"`
	ReplyCount  int    `json:"reply_count"`
}

// Usergroup is one row of m_usergroups.
type Usergroup struct {
	ID      string   `json:"id"`
	Handle  string   `json:"handle"`
	Name    string   `json:"name"`
	UserIDs []string `json:"user_ids"`
}

// Reaction is one row of m_reactions.
type Reaction struct {
	MessageChannelID string   `json:"message_channel_id"`
	MessageTS        string   `json:"message_ts"`
	EmojiName        string   `json:"emoji_name"`
	UserIDs          []string `json:"user_ids"`
	Count            int      `json:"count"`
}

// File is one row of m_files.
type File struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Mimetype   string `json:"mimetype"`
	URLPrivate string `json:"url_private"`
	Permalink  string `json:"permalink"`
	ChannelID  string `json:"channel_id"`
	Created    int64  `json:"created"`
}

// AuditEntry is one row of m_audit_log.
type AuditEntry struct {
	ID        int64     `json:"id"`
	TS        time.Time `json:"ts"`
	Caller    string    `json:"caller"`
	Verb      string    `json:"verb"`
	ChannelID string    `json:"channel_id"`
	Detail    string    `json:"detail"`
}

// ---------------------------------------------------------------------
// Upsert methods.
// ---------------------------------------------------------------------

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// jsonOrEmpty marshals v to a JSON string, returning "" on a nil slice so
// empty columns stay NULL-ish rather than literal "null".
func jsonOrEmpty(v json.RawMessage) string {
	if len(v) == 0 {
		return ""
	}
	return string(v)
}

// UpsertChannel inserts or replaces one channel row.
func (s *Store) UpsertChannel(ctx context.Context, ch Channel) error {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO m_channels
		 (id, name, is_archived, is_member, is_im, is_mpim, is_private, num_members, topic, purpose, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name=excluded.name, is_archived=excluded.is_archived, is_member=excluded.is_member,
		   is_im=excluded.is_im, is_mpim=excluded.is_mpim, is_private=excluded.is_private,
		   num_members=excluded.num_members, topic=excluded.topic, purpose=excluded.purpose,
		   synced_at=excluded.synced_at`,
		ch.ID, ch.Name, boolToInt(ch.IsArchived), boolToInt(ch.IsMember),
		boolToInt(ch.IsIM), boolToInt(ch.IsMPIM), boolToInt(ch.IsPrivate),
		ch.NumMembers, ch.Topic, ch.Purpose, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("upsert channel %s: %w", ch.ID, err)
	}
	return nil
}

// UpsertUser inserts or replaces one user row.
func (s *Store) UpsertUser(ctx context.Context, u User) error {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO m_users
		 (id, name, real_name, display_name, email, is_bot, deleted, tz, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name=excluded.name, real_name=excluded.real_name, display_name=excluded.display_name,
		   email=excluded.email, is_bot=excluded.is_bot, deleted=excluded.deleted,
		   tz=excluded.tz, synced_at=excluded.synced_at`,
		u.ID, u.Name, u.RealName, u.DisplayName, u.Email,
		boolToInt(u.IsBot), boolToInt(u.Deleted), u.TZ, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("upsert user %s: %w", u.ID, err)
	}
	return nil
}

// UpsertMessages batch-inserts messages in a single transaction and keeps
// m_messages_fts in sync. The FTS user_name column is resolved from
// m_users at upsert time so SearchMessages can match on author name
// without a runtime JOIN. A message with no resolvable user falls back to
// the bare user_id in the FTS text.
func (s *Store) UpsertMessages(ctx context.Context, msgs []Message) error {
	if len(msgs) == 0 {
		return nil
	}
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin message tx: %w", err)
	}
	defer tx.Rollback()

	for _, m := range msgs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO m_messages
			 (channel_id, ts, thread_ts, user_id, text, subtype, reply_count,
			  reply_users, reactions, files, permalink, synced_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(channel_id, ts) DO UPDATE SET
			   thread_ts=excluded.thread_ts, user_id=excluded.user_id, text=excluded.text,
			   subtype=excluded.subtype, reply_count=excluded.reply_count,
			   reply_users=excluded.reply_users, reactions=excluded.reactions,
			   files=excluded.files, permalink=excluded.permalink, synced_at=excluded.synced_at`,
			m.ChannelID, m.TS, m.ThreadTS, m.UserID, m.Text, m.Subtype, m.ReplyCount,
			jsonOrEmpty(m.ReplyUsers), jsonOrEmpty(m.Reactions), jsonOrEmpty(m.Files),
			m.Permalink, time.Now(),
		); err != nil {
			return fmt.Errorf("upsert message %s/%s: %w", m.ChannelID, m.TS, err)
		}

		// Resolve the author display name for the FTS row. Best-effort —
		// a missing user just leaves the FTS user_name as the raw id.
		userName := m.UserID
		if m.UserID != "" {
			var rn, dn, nm sql.NullString
			err := tx.QueryRowContext(ctx,
				`SELECT real_name, display_name, name FROM m_users WHERE id=?`, m.UserID,
			).Scan(&rn, &dn, &nm)
			if err == nil {
				switch {
				case rn.Valid && rn.String != "":
					userName = rn.String
				case dn.Valid && dn.String != "":
					userName = dn.String
				case nm.Valid && nm.String != "":
					userName = nm.String
				}
			}
		}

		rowid := ftsRowID(m.ChannelID, m.TS)
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM m_messages_fts WHERE rowid=?`, rowid,
		); err != nil {
			return fmt.Errorf("fts cleanup %s/%s: %w", m.ChannelID, m.TS, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO m_messages_fts (rowid, channel_id, ts, user_name, text)
			 VALUES (?, ?, ?, ?, ?)`,
			rowid, m.ChannelID, m.TS, userName, m.Text,
		); err != nil {
			return fmt.Errorf("fts insert %s/%s: %w", m.ChannelID, m.TS, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit message tx: %w", err)
	}
	return nil
}

// UpsertReactions batch-inserts reaction rows in a single transaction.
func (s *Store) UpsertReactions(ctx context.Context, rs []Reaction) error {
	if len(rs) == 0 {
		return nil
	}
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reaction tx: %w", err)
	}
	defer tx.Rollback()

	for _, r := range rs {
		users, _ := json.Marshal(r.UserIDs)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO m_reactions
			 (message_channel_id, message_ts, emoji_name, user_ids, count, synced_at)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(message_channel_id, message_ts, emoji_name) DO UPDATE SET
			   user_ids=excluded.user_ids, count=excluded.count, synced_at=excluded.synced_at`,
			r.MessageChannelID, r.MessageTS, r.EmojiName, string(users), r.Count, time.Now(),
		); err != nil {
			return fmt.Errorf("upsert reaction %s/%s/%s: %w",
				r.MessageChannelID, r.MessageTS, r.EmojiName, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reaction tx: %w", err)
	}
	return nil
}

// UpsertUsergroup inserts or replaces one usergroup row.
func (s *Store) UpsertUsergroup(ctx context.Context, g Usergroup) error {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	users, _ := json.Marshal(g.UserIDs)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO m_usergroups (id, handle, name, user_ids, synced_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   handle=excluded.handle, name=excluded.name,
		   user_ids=excluded.user_ids, synced_at=excluded.synced_at`,
		g.ID, g.Handle, g.Name, string(users), time.Now(),
	)
	if err != nil {
		return fmt.Errorf("upsert usergroup %s: %w", g.ID, err)
	}
	return nil
}

// UpsertFile inserts or replaces one file row.
func (s *Store) UpsertFile(ctx context.Context, f File) error {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO m_files
		 (id, name, mimetype, url_private, permalink, channel_id, created, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name=excluded.name, mimetype=excluded.mimetype, url_private=excluded.url_private,
		   permalink=excluded.permalink, channel_id=excluded.channel_id,
		   created=excluded.created, synced_at=excluded.synced_at`,
		f.ID, f.Name, f.Mimetype, f.URLPrivate, f.Permalink, f.ChannelID, f.Created, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("upsert file %s: %w", f.ID, err)
	}
	return nil
}

// SetThread records (or updates) a thread's last_reply_ts — the derived
// index the drift verb scans. Called by the sync engine after fetching
// conversations.replies for a parent message.
func (s *Store) SetThread(ctx context.Context, t Thread) error {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO m_threads (channel_id, parent_ts, last_reply_ts, reply_count, synced_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(channel_id, parent_ts) DO UPDATE SET
		   last_reply_ts=excluded.last_reply_ts, reply_count=excluded.reply_count,
		   synced_at=excluded.synced_at`,
		t.ChannelID, t.ParentTS, t.LastReplyTS, t.ReplyCount, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("set thread %s/%s: %w", t.ChannelID, t.ParentTS, err)
	}
	return nil
}

// AppendAuditLog appends one row to the append-only m_audit_log. Rows are
// never updated or deleted. Every DM/MPIM read during sync calls this.
func (s *Store) AppendAuditLog(ctx context.Context, caller, verb, channelID, detail string) error {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO m_audit_log (ts, caller, verb, channel_id, detail)
		 VALUES (?, ?, ?, ?, ?)`,
		time.Now(), caller, verb, channelID, detail,
	)
	if err != nil {
		return fmt.Errorf("append audit log: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------
// Per-channel sync cursor (high-water-mark).
// ---------------------------------------------------------------------

// channelCursorKey namespaces the per-channel history high-water-mark
// inside the shared sync_state table so it can't collide with the generic
// spec-resource cursors the generated sync writes.
func channelCursorKey(channelID string) string {
	return "mirror:history:" + channelID
}

// SetChannelCursor stores the latest message ts synced for a channel.
// Reuses the generated sync_state table via a namespaced key.
func (s *Store) SetChannelCursor(ctx context.Context, channelID, latestTS string) error {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return err
	}
	return s.SaveSyncCursor(channelCursorKey(channelID), latestTS)
}

// GetChannelCursor returns the latest message ts previously synced for a
// channel, or "" if the channel has never been synced.
func (s *Store) GetChannelCursor(ctx context.Context, channelID string) (string, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return "", err
	}
	return s.GetSyncCursor(channelCursorKey(channelID)), nil
}

// ---------------------------------------------------------------------
// Query methods — the read surface the P1/P2 verbs call.
// ---------------------------------------------------------------------

// scanChannels reads Channel rows from an already-executed query.
func scanChannels(rows *sql.Rows) ([]Channel, error) {
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		var c Channel
		var arch, mem, im, mpim, priv int
		var name, topic, purpose sql.NullString
		if err := rows.Scan(&c.ID, &name, &arch, &mem, &im, &mpim, &priv,
			&c.NumMembers, &topic, &purpose); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		c.Name, c.Topic, c.Purpose = name.String, topic.String, purpose.String
		c.IsArchived, c.IsMember = arch == 1, mem == 1
		c.IsIM, c.IsMPIM, c.IsPrivate = im == 1, mpim == 1, priv == 1
		out = append(out, c)
	}
	return out, rows.Err()
}

const channelSelectCols = `id, name, is_archived, is_member, is_im, is_mpim,
	is_private, num_members, topic, purpose`

// ListChannels returns all mirrored channels ordered by name. When
// memberOnly is true, only channels the authed user is a member of are
// returned.
func (s *Store) ListChannels(ctx context.Context, memberOnly bool) ([]Channel, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return nil, err
	}
	q := `SELECT ` + channelSelectCols + ` FROM m_channels`
	if memberOnly {
		q += ` WHERE is_member = 1`
	}
	q += ` ORDER BY name`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return scanChannels(rows)
}

// ResolveChannel finds a channel by exact id, exact name, or — when no
// exact hit — a unique case-insensitive substring match on the name. A
// leading '#' on the input is stripped. Returns sql.ErrNoRows when there
// is no match and a descriptive error when the substring match is
// ambiguous, so callers can distinguish the two.
func (s *Store) ResolveChannel(ctx context.Context, input string) (Channel, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return Channel{}, err
	}
	needle := input
	if len(needle) > 0 && needle[0] == '#' {
		needle = needle[1:]
	}

	// Exact id or name.
	var c Channel
	var arch, mem, im, mpim, priv int
	var name, topic, purpose sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT `+channelSelectCols+` FROM m_channels WHERE id = ? OR name = ? LIMIT 1`,
		needle, needle,
	).Scan(&c.ID, &name, &arch, &mem, &im, &mpim, &priv, &c.NumMembers, &topic, &purpose)
	if err == nil {
		c.Name, c.Topic, c.Purpose = name.String, topic.String, purpose.String
		c.IsArchived, c.IsMember = arch == 1, mem == 1
		c.IsIM, c.IsMPIM, c.IsPrivate = im == 1, mpim == 1, priv == 1
		return c, nil
	}
	if err != sql.ErrNoRows {
		return Channel{}, fmt.Errorf("resolve channel %q: %w", input, err)
	}

	// Unique case-insensitive substring fallback.
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+channelSelectCols+` FROM m_channels
		 WHERE name LIKE '%' || ? || '%' COLLATE NOCASE ORDER BY name`,
		needle,
	)
	if err != nil {
		return Channel{}, fmt.Errorf("resolve channel %q: %w", input, err)
	}
	matches, err := scanChannels(rows)
	if err != nil {
		return Channel{}, err
	}
	switch len(matches) {
	case 0:
		return Channel{}, sql.ErrNoRows
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return Channel{}, fmt.Errorf("ambiguous channel %q matches %d channels: %v",
			input, len(matches), names)
	}
}

// scanUser reads a single User from a row scanner.
func scanUserRow(scan func(...any) error) (User, error) {
	var u User
	var bot, del int
	var name, rn, dn, email, tz sql.NullString
	if err := scan(&u.ID, &name, &rn, &dn, &email, &bot, &del, &tz); err != nil {
		return User{}, err
	}
	u.Name, u.RealName, u.DisplayName = name.String, rn.String, dn.String
	u.Email, u.TZ = email.String, tz.String
	u.IsBot, u.Deleted = bot == 1, del == 1
	return u, nil
}

const userSelectCols = `id, name, real_name, display_name, email, is_bot, deleted, tz`

// ResolveUser finds a user by exact id, exact name/email, or — when no
// exact hit — a unique case-insensitive substring match on name,
// real_name, or display_name. A leading '@' is stripped. Returns
// sql.ErrNoRows on no match and a descriptive error on ambiguity.
func (s *Store) ResolveUser(ctx context.Context, input string) (User, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return User{}, err
	}
	needle := input
	if len(needle) > 0 && needle[0] == '@' {
		needle = needle[1:]
	}

	u, err := scanUserRow(s.db.QueryRowContext(ctx,
		`SELECT `+userSelectCols+` FROM m_users
		 WHERE id = ? OR name = ? OR email = ? LIMIT 1`,
		needle, needle, needle,
	).Scan)
	if err == nil {
		return u, nil
	}
	if err != sql.ErrNoRows {
		return User{}, fmt.Errorf("resolve user %q: %w", input, err)
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT `+userSelectCols+` FROM m_users
		 WHERE name LIKE '%' || ? || '%' COLLATE NOCASE
		    OR real_name LIKE '%' || ? || '%' COLLATE NOCASE
		    OR display_name LIKE '%' || ? || '%' COLLATE NOCASE
		 ORDER BY name`,
		needle, needle, needle,
	)
	if err != nil {
		return User{}, fmt.Errorf("resolve user %q: %w", input, err)
	}
	defer rows.Close()
	var matches []User
	for rows.Next() {
		m, err := scanUserRow(rows.Scan)
		if err != nil {
			return User{}, fmt.Errorf("scan user: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return User{}, err
	}
	switch len(matches) {
	case 0:
		return User{}, sql.ErrNoRows
	case 1:
		return matches[0], nil
	default:
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			names = append(names, m.Name)
		}
		return User{}, fmt.Errorf("ambiguous user %q matches %d users: %v",
			input, len(matches), names)
	}
}

// scanMessages reads Message rows from an executed query.
func scanMessages(rows *sql.Rows) ([]Message, error) {
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var threadTS, userID, text, subtype, permalink sql.NullString
		var replyUsers, reactions, files sql.NullString
		if err := rows.Scan(&m.ChannelID, &m.TS, &threadTS, &userID, &text,
			&subtype, &m.ReplyCount, &replyUsers, &reactions, &files, &permalink); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		m.ThreadTS, m.UserID, m.Text = threadTS.String, userID.String, text.String
		m.Subtype, m.Permalink = subtype.String, permalink.String
		if replyUsers.String != "" {
			m.ReplyUsers = json.RawMessage(replyUsers.String)
		}
		if reactions.String != "" {
			m.Reactions = json.RawMessage(reactions.String)
		}
		if files.String != "" {
			m.Files = json.RawMessage(files.String)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const messageSelectCols = `channel_id, ts, thread_ts, user_id, text, subtype,
	reply_count, reply_users, reactions, files, permalink`

// SearchMessages runs an FTS5 MATCH over message text and resolved author
// name, newest match first. channelIDs, when non-empty, restricts results
// to those channels. limit <= 0 defaults to 50.
func (s *Store) SearchMessages(ctx context.Context, query string, channelIDs []string, limit int) ([]Message, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT ` + prefixCols("m", messageSelectCols) + `
		 FROM m_messages m
		 JOIN m_messages_fts f ON f.channel_id = m.channel_id AND f.ts = m.ts
		 WHERE m_messages_fts MATCH ?`
	args := []any{query}
	if len(channelIDs) > 0 {
		q += ` AND m.channel_id IN (` + placeholders(len(channelIDs)) + `)`
		for _, id := range channelIDs {
			args = append(args, id)
		}
	}
	q += ` ORDER BY m.ts DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	return scanMessages(rows)
}

// MessagesInWindow returns messages in the given channels whose ts falls
// in [since, until]. Empty since/until are open-ended bounds. channelIDs
// empty means all channels. Slack ts strings are zero-padded decimal
// seconds, so lexical comparison is also chronological.
func (s *Store) MessagesInWindow(ctx context.Context, channelIDs []string, since, until string) ([]Message, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return nil, err
	}
	q := `SELECT ` + messageSelectCols + ` FROM m_messages WHERE 1=1`
	var args []any
	if len(channelIDs) > 0 {
		q += ` AND channel_id IN (` + placeholders(len(channelIDs)) + `)`
		for _, id := range channelIDs {
			args = append(args, id)
		}
	}
	if since != "" {
		q += ` AND ts >= ?`
		args = append(args, since)
	}
	if until != "" {
		q += ` AND ts <= ?`
		args = append(args, until)
	}
	q += ` ORDER BY ts ASC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("messages in window: %w", err)
	}
	return scanMessages(rows)
}

// ThreadReplies returns every reply message of a thread (parentTS) in a
// channel, oldest first. The thread parent itself is included when it was
// mirrored as a message.
func (s *Store) ThreadReplies(ctx context.Context, channelID, parentTS string) ([]Message, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+messageSelectCols+` FROM m_messages
		 WHERE channel_id = ? AND (thread_ts = ? OR ts = ?)
		 ORDER BY ts ASC`,
		channelID, parentTS, parentTS,
	)
	if err != nil {
		return nil, fmt.Errorf("thread replies %s/%s: %w", channelID, parentTS, err)
	}
	return scanMessages(rows)
}

// ReactionsForChannel returns every reaction row recorded for a channel.
func (s *Store) ReactionsForChannel(ctx context.Context, channelID string) ([]Reaction, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT message_channel_id, message_ts, emoji_name, user_ids, count
		 FROM m_reactions WHERE message_channel_id = ?
		 ORDER BY message_ts DESC, count DESC`,
		channelID,
	)
	if err != nil {
		return nil, fmt.Errorf("reactions for channel %s: %w", channelID, err)
	}
	defer rows.Close()
	var out []Reaction
	for rows.Next() {
		var r Reaction
		var users sql.NullString
		if err := rows.Scan(&r.MessageChannelID, &r.MessageTS, &r.EmojiName,
			&users, &r.Count); err != nil {
			return nil, fmt.Errorf("scan reaction: %w", err)
		}
		if users.String != "" {
			_ = json.Unmarshal([]byte(users.String), &r.UserIDs)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListUsergroups returns every mirrored usergroup ordered by handle.
func (s *Store) ListUsergroups(ctx context.Context) ([]Usergroup, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, handle, name, user_ids FROM m_usergroups ORDER BY handle`,
	)
	if err != nil {
		return nil, fmt.Errorf("list usergroups: %w", err)
	}
	defer rows.Close()
	var out []Usergroup
	for rows.Next() {
		var g Usergroup
		var handle, name, users sql.NullString
		if err := rows.Scan(&g.ID, &handle, &name, &users); err != nil {
			return nil, fmt.Errorf("scan usergroup: %w", err)
		}
		g.Handle, g.Name = handle.String, name.String
		if users.String != "" {
			_ = json.Unmarshal([]byte(users.String), &g.UserIDs)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// StaleThreads returns threads in the given channels whose last_reply_ts
// is older than the cutoff (a Slack ts string). channelIDs empty means
// all channels. Powers the drift verb's stale-thread detection.
func (s *Store) StaleThreads(ctx context.Context, channelIDs []string, cutoffTS string) ([]Thread, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return nil, err
	}
	q := `SELECT channel_id, parent_ts, last_reply_ts, reply_count
		 FROM m_threads WHERE last_reply_ts < ?`
	args := []any{cutoffTS}
	if len(channelIDs) > 0 {
		q += ` AND channel_id IN (` + placeholders(len(channelIDs)) + `)`
		for _, id := range channelIDs {
			args = append(args, id)
		}
	}
	q += ` ORDER BY last_reply_ts ASC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("stale threads: %w", err)
	}
	defer rows.Close()
	var out []Thread
	for rows.Next() {
		var t Thread
		var last sql.NullString
		if err := rows.Scan(&t.ChannelID, &t.ParentTS, &last, &t.ReplyCount); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		t.LastReplyTS = last.String
		out = append(out, t)
	}
	return out, rows.Err()
}

// AuditLog returns the most recent audit-log rows, newest first. limit
// <= 0 defaults to 100.
func (s *Store) AuditLog(ctx context.Context, limit int) ([]AuditEntry, error) {
	if err := s.EnsureMirrorSchema(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, ts, caller, verb, channel_id, detail
		 FROM m_audit_log ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("audit log: %w", err)
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		var caller, verb, chID, detail sql.NullString
		if err := rows.Scan(&e.ID, &e.TS, &caller, &verb, &chID, &detail); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		e.Caller, e.Verb = caller.String, verb.String
		e.ChannelID, e.Detail = chID.String, detail.String
		out = append(out, e)
	}
	return out, rows.Err()
}

// placeholders returns "?, ?, ?" for n bind parameters.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, n*3)
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ',', ' ')
		}
		b = append(b, '?')
	}
	return string(b)
}

// prefixCols rewrites a comma-separated column list to qualify each
// column with a table alias, e.g. prefixCols("m", "a, b") -> "m.a, m.b".
func prefixCols(alias, cols string) string {
	var b []byte
	field := make([]byte, 0, 16)
	flush := func() {
		f := trimSpace(string(field))
		if f == "" {
			return
		}
		if len(b) > 0 {
			b = append(b, ',', ' ')
		}
		b = append(b, alias...)
		b = append(b, '.')
		b = append(b, f...)
		field = field[:0]
	}
	for i := 0; i < len(cols); i++ {
		if cols[i] == ',' {
			flush()
			continue
		}
		field = append(field, cols[i])
	}
	flush()
	return string(b)
}

// trimSpace trims ASCII whitespace (incl. newlines/tabs) from both ends —
// the column-list constants are multi-line, so std strings.TrimSpace is
// what we want; this thin wrapper keeps the import surface obvious.
func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
