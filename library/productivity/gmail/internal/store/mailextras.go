// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written data layer for the unsubscribe engine's ledger
// (mail_unsub_ledger), the delta checkpoints (mail_checkpoints), the score
// snapshots (mail_scores), and the read-side aggregations the novel
// commands (digest, delta, storage report, sort suggest, trash report,
// score, unsub audit/verify) run over mail_meta and the cleanup ledger.
// Schema lives in extras.go; this file owns every read/write so commands
// never hand-roll SQL against these tables.

package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// mail_unsub_ledger
// ---------------------------------------------------------------------------

// MailUnsubAttempt is one row of mail_unsub_ledger. Status vocabulary:
// 'skipped:<reason>' (nothing left this machine), a numeric HTTP status
// string ('200', '302', ...) when a response arrived, or 'unknown' (network
// error after the connection was established; never auto-retried).
type MailUnsubAttempt struct {
	ID        int64  `json:"id"`
	Account   string `json:"account"`
	Sender    string `json:"sender"`
	URL       string `json:"url"`
	PlanSha   string `json:"plan_sha"`
	PostedAt  string `json:"posted_at"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// InsertMailUnsubAttempt records one attempt row and returns its rowid.
// PostedAt/CreatedAt default to now (RFC3339 UTC) when empty.
func (s *Store) InsertMailUnsubAttempt(a MailUnsubAttempt) (int64, error) {
	if a.Account == "" || a.Sender == "" {
		return 0, fmt.Errorf("mail_unsub_ledger insert: account and sender are required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if a.PostedAt == "" {
		a.PostedAt = nowRFC3339()
	}
	if a.CreatedAt == "" {
		a.CreatedAt = nowRFC3339()
	}
	res, err := s.db.Exec(
		`INSERT INTO mail_unsub_ledger (account, sender, url, plan_sha, posted_at, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.Account, a.Sender, a.URL, a.PlanSha, a.PostedAt, a.Status, a.CreatedAt,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetMailUnsubAttemptStatus updates one attempt's status (the post-POST
// stamp: rows are inserted 'unknown' before the request goes out).
func (s *Store) SetMailUnsubAttemptStatus(id int64, status string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE mail_unsub_ledger SET status = ? WHERE id = ?`, status, id)
	return err
}

// ListMailUnsubAttempts returns an account's attempts, oldest first.
func (s *Store) ListMailUnsubAttempts(account string) ([]MailUnsubAttempt, error) {
	rows, err := s.db.Query(
		`SELECT id, account, sender, url, plan_sha, posted_at, status, created_at
		 FROM mail_unsub_ledger WHERE account = ? ORDER BY id ASC`, account,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MailUnsubAttempt
	for rows.Next() {
		var a MailUnsubAttempt
		if err := rows.Scan(&a.ID, &a.Account, &a.Sender, &a.URL, &a.PlanSha, &a.PostedAt, &a.Status, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UnsubViolation is one sender that kept mailing after a 2xx one-click POST.
type UnsubViolation struct {
	Sender        string `json:"sender"`
	URL           string `json:"url"`
	PostedAt      string `json:"posted_at"`
	ArrivalsSince int    `json:"arrivals_since"`
	NewestSubject string `json:"newest_subject"`
	NewestDateMs  int64  `json:"newest_date_ms"`
}

// UnsubViolations joins successful (2xx) unsubscribe attempts against
// mail_meta arrivals landing after posted_at + grace. Only attempts with
// posted_at >= postedSince are considered; per sender the LATEST successful
// post wins. Senders with zero post-grace arrivals are omitted.
func (s *Store) UnsubViolations(account string, postedSince time.Time, grace time.Duration) ([]UnsubViolation, error) {
	attempts, err := s.ListMailUnsubAttempts(account)
	if err != nil {
		return nil, err
	}
	sinceCut := postedSince.UTC()
	latest := map[string]MailUnsubAttempt{}
	var order []string
	for _, a := range attempts {
		if len(a.Status) != 3 || a.Status[0] != '2' {
			continue // only real 2xx responses count as "posted successfully"
		}
		ts, perr := time.Parse(time.RFC3339, a.PostedAt)
		if perr != nil || ts.Before(sinceCut) {
			continue
		}
		if _, seen := latest[a.Sender]; !seen {
			order = append(order, a.Sender)
		}
		latest[a.Sender] = a // list is oldest-first; last write is the latest post
	}
	var out []UnsubViolation
	for _, sender := range order {
		a := latest[sender]
		ts, _ := time.Parse(time.RFC3339, a.PostedAt)
		cutMs := ts.Add(grace).UnixMilli()
		var count int
		var newestMs sql.NullInt64
		err := s.db.QueryRow(
			`SELECT COUNT(*), MAX(internal_date) FROM mail_meta
			 WHERE account = ? AND from_email = ? AND internal_date > ?`,
			account, sender, cutMs,
		).Scan(&count, &newestMs)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			continue
		}
		v := UnsubViolation{Sender: sender, URL: a.URL, PostedAt: a.PostedAt, ArrivalsSince: count, NewestDateMs: newestMs.Int64}
		var subject string
		err = s.db.QueryRow(
			`SELECT subject FROM mail_meta WHERE account = ? AND from_email = ? AND internal_date > ?
			 ORDER BY internal_date DESC LIMIT 1`,
			account, sender, cutMs,
		).Scan(&subject)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		v.NewestSubject = subject
		out = append(out, v)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// mail_checkpoints
// ---------------------------------------------------------------------------

// MailCheckpoint is one (account, kind) watermark row.
type MailCheckpoint struct {
	Account     string `json:"account"`
	Kind        string `json:"kind"`
	WatermarkMs int64  `json:"watermark_ms"`
	MsgCount    int    `json:"msg_count"`
	TakenAt     string `json:"taken_at"`
}

// GetMailCheckpoint returns the checkpoint row and whether one exists.
func (s *Store) GetMailCheckpoint(account, kind string) (MailCheckpoint, bool, error) {
	cp := MailCheckpoint{Account: account, Kind: kind}
	err := s.db.QueryRow(
		`SELECT watermark_ms, msg_count, taken_at FROM mail_checkpoints WHERE account = ? AND kind = ?`,
		account, kind,
	).Scan(&cp.WatermarkMs, &cp.MsgCount, &cp.TakenAt)
	if err == sql.ErrNoRows {
		return cp, false, nil
	}
	if err != nil {
		return cp, false, err
	}
	return cp, true, nil
}

// SaveMailCheckpoint upserts the checkpoint row (TakenAt defaults to now).
func (s *Store) SaveMailCheckpoint(cp MailCheckpoint) error {
	if cp.Account == "" || cp.Kind == "" {
		return fmt.Errorf("mail_checkpoints upsert: account and kind are required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if cp.TakenAt == "" {
		cp.TakenAt = nowRFC3339()
	}
	_, err := s.db.Exec(
		`INSERT INTO mail_checkpoints (account, kind, watermark_ms, msg_count, taken_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(account, kind) DO UPDATE SET
			watermark_ms = excluded.watermark_ms,
			msg_count = excluded.msg_count,
			taken_at = excluded.taken_at`,
		cp.Account, cp.Kind, cp.WatermarkMs, cp.MsgCount, cp.TakenAt,
	)
	return err
}

// MailWatermark reports the store's current high-water mark for one account:
// max internal_date plus total row count (both 0 for an empty account).
func (s *Store) MailWatermark(account string) (int64, int, error) {
	var maxMs sql.NullInt64
	var count int
	err := s.db.QueryRow(
		`SELECT MAX(internal_date), COUNT(*) FROM mail_meta WHERE account = ?`, account,
	).Scan(&maxMs, &count)
	if err != nil {
		return 0, 0, err
	}
	return maxMs.Int64, count, nil
}

// ---------------------------------------------------------------------------
// mail_scores
// ---------------------------------------------------------------------------

// MailScore is one hygiene-metrics snapshot row.
type MailScore struct {
	ID      int64  `json:"id"`
	Account string `json:"account"`
	TakenAt string `json:"taken_at"`
	Metrics string `json:"metrics"` // JSON object
}

// InsertMailScore appends one snapshot (TakenAt defaults to now) and
// returns its rowid.
func (s *Store) InsertMailScore(account, takenAt, metricsJSON string) (int64, error) {
	if account == "" {
		return 0, fmt.Errorf("mail_scores insert: account is required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if takenAt == "" {
		takenAt = nowRFC3339()
	}
	if strings.TrimSpace(metricsJSON) == "" {
		metricsJSON = "{}"
	}
	res, err := s.db.Exec(
		`INSERT INTO mail_scores (account, taken_at, metrics) VALUES (?, ?, ?)`,
		account, takenAt, metricsJSON,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// LatestMailScore returns the newest snapshot and whether one exists.
func (s *Store) LatestMailScore(account string) (MailScore, bool, error) {
	return s.mailScoreAt(account, "DESC")
}

// FirstMailScore returns the oldest snapshot (the baseline) and whether one
// exists.
func (s *Store) FirstMailScore(account string) (MailScore, bool, error) {
	return s.mailScoreAt(account, "ASC")
}

func (s *Store) mailScoreAt(account, dir string) (MailScore, bool, error) {
	var sc MailScore
	// dir comes from the two callers above, never from input.
	err := s.db.QueryRow(fmt.Sprintf(
		`SELECT id, account, taken_at, metrics FROM mail_scores WHERE account = ? ORDER BY id %s LIMIT 1`, dir),
		account,
	).Scan(&sc.ID, &sc.Account, &sc.TakenAt, &sc.Metrics)
	if err == sql.ErrNoRows {
		return sc, false, nil
	}
	if err != nil {
		return sc, false, err
	}
	return sc, true, nil
}

// ---------------------------------------------------------------------------
// digest aggregations
// ---------------------------------------------------------------------------

// CategoryDigestRow is one derived category's aggregate for `digest`.
// OldestUnreadMs is 0 when the category has no unread messages.
type CategoryDigestRow struct {
	Category       string `json:"category"`
	Total          int    `json:"total"`
	Unread         int    `json:"unread"`
	OldestUnreadMs int64  `json:"oldest_unread_ms"`
	TotalSize      int64  `json:"total_size"`
}

// CategoryDigest aggregates mail_meta per derived category for one account.
// sinceMs 0 means no lower time bound. Only categories with rows are
// returned; the digest command fills in the fixed category vocabulary.
func (s *Store) CategoryDigest(account string, sinceMs int64) ([]CategoryDigestRow, error) {
	q := `SELECT category, COUNT(*), COALESCE(SUM(unread), 0),
			COALESCE(MIN(CASE WHEN unread = 1 THEN internal_date END), 0),
			COALESCE(SUM(size_estimate), 0)
		 FROM mail_meta WHERE account = ?`
	args := []any{account}
	if sinceMs > 0 {
		q += ` AND internal_date >= ?`
		args = append(args, sinceMs)
	}
	q += ` GROUP BY category ORDER BY category ASC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CategoryDigestRow
	for rows.Next() {
		var r CategoryDigestRow
		if err := rows.Scan(&r.Category, &r.Total, &r.Unread, &r.OldestUnreadMs, &r.TotalSize); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SenderCount is one (from_email, count) pair.
type SenderCount struct {
	FromEmail string `json:"from_email"`
	Count     int    `json:"count"`
}

// CategoryTopSenders returns the top `per` senders by message count within
// each category (map key = category). sinceMs 0 means no lower time bound.
func (s *Store) CategoryTopSenders(account string, sinceMs int64, per int) (map[string][]SenderCount, error) {
	if per <= 0 {
		per = 3
	}
	q := `SELECT category, from_email, COUNT(*) AS cnt FROM mail_meta WHERE account = ?`
	args := []any{account}
	if sinceMs > 0 {
		q += ` AND internal_date >= ?`
		args = append(args, sinceMs)
	}
	q += ` GROUP BY category, from_email ORDER BY category ASC, cnt DESC, from_email ASC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]SenderCount{}
	for rows.Next() {
		var cat string
		var sc SenderCount
		if err := rows.Scan(&cat, &sc.FromEmail, &sc.Count); err != nil {
			return nil, err
		}
		if len(out[cat]) < per {
			out[cat] = append(out[cat], sc)
		}
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// delta aggregations
// ---------------------------------------------------------------------------

// DeltaCategoryCounts counts messages newer than watermarkMs per category.
func (s *Store) DeltaCategoryCounts(account string, watermarkMs int64) (map[string]int, error) {
	rows, err := s.db.Query(
		`SELECT category, COUNT(*) FROM mail_meta WHERE account = ? AND internal_date > ? GROUP BY category`,
		account, watermarkMs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var cat string
		var n int
		if err := rows.Scan(&cat, &n); err != nil {
			return nil, err
		}
		out[cat] = n
	}
	return out, rows.Err()
}

// DeltaSenderStat is one sender's activity relative to a watermark: how many
// messages arrived after it, how many the store held before it, and the
// sender's first-seen timestamp (for the prior-daily-average spike math).
type DeltaSenderStat struct {
	FromEmail    string `json:"from_email"`
	SinceCount   int    `json:"since_count"`
	PriorCount   int    `json:"prior_count"`
	FirstSeenMs  int64  `json:"first_seen_ms"`
	NewestSeenMs int64  `json:"newest_seen_ms"`
}

// DeltaSenderStats returns per-sender since/prior counts for every sender
// with at least one message newer than watermarkMs, ordered by since-count
// descending then sender ascending.
func (s *Store) DeltaSenderStats(account string, watermarkMs int64) ([]DeltaSenderStat, error) {
	rows, err := s.db.Query(
		`SELECT from_email,
			SUM(CASE WHEN internal_date > ? THEN 1 ELSE 0 END) AS since_cnt,
			SUM(CASE WHEN internal_date <= ? THEN 1 ELSE 0 END) AS prior_cnt,
			MIN(internal_date), MAX(internal_date)
		 FROM mail_meta WHERE account = ?
		 GROUP BY from_email
		 HAVING since_cnt > 0
		 ORDER BY since_cnt DESC, from_email ASC`,
		watermarkMs, watermarkMs, account,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DeltaSenderStat
	for rows.Next() {
		var st DeltaSenderStat
		if err := rows.Scan(&st.FromEmail, &st.SinceCount, &st.PriorCount, &st.FirstSeenMs, &st.NewestSeenMs); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// storage aggregations
// ---------------------------------------------------------------------------

// StorageSenderRow is one sender's storage attribution.
type StorageSenderRow struct {
	FromEmail string `json:"from_email"`
	Count     int    `json:"count"`
	TotalSize int64  `json:"total_size"`
}

// StorageBySender returns the top senders by total size.
func (s *Store) StorageBySender(account string, top int) ([]StorageSenderRow, error) {
	if top <= 0 {
		top = 15
	}
	rows, err := s.db.Query(
		`SELECT from_email, COUNT(*), COALESCE(SUM(size_estimate), 0) AS total
		 FROM mail_meta WHERE account = ?
		 GROUP BY from_email ORDER BY total DESC, from_email ASC LIMIT ?`,
		account, top,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StorageSenderRow
	for rows.Next() {
		var r StorageSenderRow
		if err := rows.Scan(&r.FromEmail, &r.Count, &r.TotalSize); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// StorageCategoryRow is one category's storage attribution.
type StorageCategoryRow struct {
	Category  string `json:"category"`
	Count     int    `json:"count"`
	TotalSize int64  `json:"total_size"`
}

// StorageByCategory attributes storage per derived category.
func (s *Store) StorageByCategory(account string) ([]StorageCategoryRow, error) {
	rows, err := s.db.Query(
		`SELECT category, COUNT(*), COALESCE(SUM(size_estimate), 0) AS total
		 FROM mail_meta WHERE account = ?
		 GROUP BY category ORDER BY total DESC, category ASC`,
		account,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StorageCategoryRow
	for rows.Next() {
		var r StorageCategoryRow
		if err := rows.Scan(&r.Category, &r.Count, &r.TotalSize); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// StorageYearRow is one calendar year's storage attribution (UTC year of
// internal_date).
type StorageYearRow struct {
	Year      int   `json:"year"`
	Count     int   `json:"count"`
	TotalSize int64 `json:"total_size"`
}

// StorageByYear attributes storage per UTC calendar year.
func (s *Store) StorageByYear(account string) ([]StorageYearRow, error) {
	rows, err := s.db.Query(
		`SELECT CAST(strftime('%Y', internal_date / 1000, 'unixepoch') AS INTEGER) AS y,
			COUNT(*), COALESCE(SUM(size_estimate), 0)
		 FROM mail_meta WHERE account = ?
		 GROUP BY y ORDER BY y DESC`,
		account,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StorageYearRow
	for rows.Next() {
		var r StorageYearRow
		if err := rows.Scan(&r.Year, &r.Count, &r.TotalSize); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// StorageLargestRow is one of the largest single messages.
type StorageLargestRow struct {
	ID           string `json:"id"`
	FromEmail    string `json:"from_email"`
	Subject      string `json:"subject"`
	SizeEstimate int64  `json:"size_estimate"`
	InternalDate int64  `json:"internal_date_ms"`
}

// StorageLargest returns the top messages by sizeEstimate.
func (s *Store) StorageLargest(account string, top int) ([]StorageLargestRow, error) {
	if top <= 0 {
		top = 15
	}
	rows, err := s.db.Query(
		`SELECT id, from_email, subject, size_estimate, internal_date
		 FROM mail_meta WHERE account = ?
		 ORDER BY size_estimate DESC, id ASC LIMIT ?`,
		account, top,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StorageLargestRow
	for rows.Next() {
		var r StorageLargestRow
		if err := rows.Scan(&r.ID, &r.FromEmail, &r.Subject, &r.SizeEstimate, &r.InternalDate); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// sort-suggest aggregations
// ---------------------------------------------------------------------------

// SenderLabelStat is one (sender, user-label) pair with the label's count,
// plus the sender's labeled-message and total-message counts. "Labeled"
// means the message carries at least one non-system label.
type SenderLabelStat struct {
	FromEmail    string `json:"from_email"`
	Label        string `json:"label"`
	LabelCount   int    `json:"label_count"`
	LabeledTotal int    `json:"labeled_total"`
	SenderTotal  int    `json:"sender_total"`
}

// sortSystemLabels is the closed system-label set sort-suggest excludes;
// CATEGORY_* is excluded by prefix in SQL.
var sortSystemLabels = []string{
	"INBOX", "UNREAD", "TRASH", "SPAM", "SENT", "DRAFT", "IMPORTANT", "STARRED",
}

// SenderLabelStats aggregates user-label usage per sender: for each
// (sender, user label) pair its message count, and per sender the number of
// messages carrying any user label plus the sender's total message count.
func (s *Store) SenderLabelStats(account string) ([]SenderLabelStat, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(sortSystemLabels)), ",")
	userLabelCond := `je.value NOT IN (` + placeholders + `) AND je.value NOT LIKE 'CATEGORY\_%' ESCAPE '\'`
	sysArgs := make([]any, 0, len(sortSystemLabels))
	for _, l := range sortSystemLabels {
		sysArgs = append(sysArgs, l)
	}

	// Per (sender, label) counts over user labels only.
	q1 := `SELECT m.from_email, je.value, COUNT(*)
		 FROM mail_meta m, json_each(m.label_ids) je
		 WHERE m.account = ? AND ` + userLabelCond + `
		 GROUP BY m.from_email, je.value`
	args1 := append([]any{account}, sysArgs...)
	rows, err := s.db.Query(q1, args1...)
	if err != nil {
		return nil, err
	}
	type pair struct{ sender, label string }
	labelCounts := map[pair]int{}
	var order []pair
	for rows.Next() {
		var p pair
		var n int
		if err := rows.Scan(&p.sender, &p.label, &n); err != nil {
			rows.Close()
			return nil, err
		}
		labelCounts[p] = n
		order = append(order, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Per sender: messages carrying >= 1 user label.
	q2 := `SELECT m.from_email, COUNT(*)
		 FROM mail_meta m
		 WHERE m.account = ? AND EXISTS (
			SELECT 1 FROM json_each(m.label_ids) je WHERE ` + userLabelCond + `)
		 GROUP BY m.from_email`
	labeled := map[string]int{}
	rows, err = s.db.Query(q2, args1...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sender string
		var n int
		if err := rows.Scan(&sender, &n); err != nil {
			rows.Close()
			return nil, err
		}
		labeled[sender] = n
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	// Per sender totals.
	totals := map[string]int{}
	rows, err = s.db.Query(`SELECT from_email, COUNT(*) FROM mail_meta WHERE account = ? GROUP BY from_email`, account)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sender string
		var n int
		if err := rows.Scan(&sender, &n); err != nil {
			rows.Close()
			return nil, err
		}
		totals[sender] = n
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	out := make([]SenderLabelStat, 0, len(order))
	for _, p := range order {
		out = append(out, SenderLabelStat{
			FromEmail:    p.sender,
			Label:        p.label,
			LabelCount:   labelCounts[p],
			LabeledTotal: labeled[p.sender],
			SenderTotal:  totals[p.sender],
		})
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// trash-report aggregations
// ---------------------------------------------------------------------------

// TrashLedgerRow is one applied plan's trash summary from the cleanup ledger.
type TrashLedgerRow struct {
	LedgerID  string `json:"ledger_id"`
	PlanSha   string `json:"plan_sha"`
	Action    string `json:"action"`
	CreatedAt string `json:"created_at"`
	Trashed   int    `json:"trashed"`
	Undone    int    `json:"undone"`
	Conflict  int    `json:"conflict"`
}

// TrashLedgers summarizes every ledger holding trash entries for an account,
// oldest first.
func (s *Store) TrashLedgers(account string) ([]TrashLedgerRow, error) {
	rows, err := s.db.Query(
		`SELECT l.ledger_id, l.plan_sha, l.action, l.created_at,
			COUNT(*),
			COALESCE(SUM(CASE WHEN e.undone = 'undone' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN e.undone = 'conflict' THEN 1 ELSE 0 END), 0)
		 FROM mail_ledger l
		 JOIN mail_ledger_entries e ON e.ledger_id = l.ledger_id AND e.kind = 'trash'
		 WHERE l.account = ?
		 GROUP BY l.ledger_id ORDER BY l.created_at ASC, l.ledger_id ASC`,
		account,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrashLedgerRow
	for rows.Next() {
		var r TrashLedgerRow
		if err := rows.Scan(&r.LedgerID, &r.PlanSha, &r.Action, &r.CreatedAt, &r.Trashed, &r.Undone, &r.Conflict); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TrashOutsideLedgerCount counts currently-TRASH-labeled messages in the
// local store that no trash ledger entry accounts for (trashed outside this
// tool, or synced from another client's action).
func (s *Store) TrashOutsideLedgerCount(account string) (int, error) {
	var n int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM mail_meta m
		 WHERE m.account = ?
		   AND EXISTS (SELECT 1 FROM json_each(m.label_ids) je WHERE je.value = 'TRASH')
		   AND NOT EXISTS (SELECT 1 FROM mail_ledger_entries e WHERE e.kind = 'trash' AND e.id = m.id)`,
		account,
	).Scan(&n)
	return n, err
}

// ---------------------------------------------------------------------------
// score aggregations
// ---------------------------------------------------------------------------

// ScoreAggregates is the raw material for one score snapshot.
type ScoreAggregates struct {
	Total               int   `json:"total"`
	Unread              int   `json:"unread"`
	Promotions          int   `json:"promotions"`
	SubscriptionSenders int   `json:"subscription_senders"`
	TotalSize           int64 `json:"total_size"`
	OldestUnreadMs      int64 `json:"oldest_unread_ms"`
}

// ScoreAggregates computes the hygiene aggregates for one account in a
// single pass over mail_meta.
func (s *Store) ScoreAggregates(account string) (ScoreAggregates, error) {
	var a ScoreAggregates
	err := s.db.QueryRow(
		`SELECT COUNT(*),
			COALESCE(SUM(unread), 0),
			COALESCE(SUM(CASE WHEN category = 'promotions' THEN 1 ELSE 0 END), 0),
			COUNT(DISTINCT CASE WHEN list_unsubscribe != '' THEN from_email END),
			COALESCE(SUM(size_estimate), 0),
			COALESCE(MIN(CASE WHEN unread = 1 THEN internal_date END), 0)
		 FROM mail_meta WHERE account = ?`,
		account,
	).Scan(&a.Total, &a.Unread, &a.Promotions, &a.SubscriptionSenders, &a.TotalSize, &a.OldestUnreadMs)
	return a, err
}

// ---------------------------------------------------------------------------
// unsub-audit aggregations
// ---------------------------------------------------------------------------

// UnsubSenderAgg is one unsubscribe-capable sender's aggregate row.
type UnsubSenderAgg struct {
	FromEmail   string `json:"from_email"`
	Count       int    `json:"count"`
	UnreadCount int    `json:"unread_count"`
	NewestMs    int64  `json:"newest_ms"`
}

// UnsubSenderAggregates returns senders with at least minCount messages in
// the window AND at least one message carrying a List-Unsubscribe header,
// ordered by count descending. sinceMs 0 means no lower time bound.
func (s *Store) UnsubSenderAggregates(account string, sinceMs int64, minCount int) ([]UnsubSenderAgg, error) {
	if minCount <= 0 {
		minCount = 1
	}
	q := `SELECT from_email, COUNT(*) AS cnt, COALESCE(SUM(unread), 0), MAX(internal_date)
		 FROM mail_meta WHERE account = ? AND from_email != ''`
	args := []any{account}
	if sinceMs > 0 {
		q += ` AND internal_date >= ?`
		args = append(args, sinceMs)
	}
	q += ` GROUP BY from_email
		 HAVING cnt >= ? AND MAX(CASE WHEN list_unsubscribe != '' THEN 1 ELSE 0 END) = 1
		 ORDER BY cnt DESC, from_email ASC`
	args = append(args, minCount)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UnsubSenderAgg
	for rows.Next() {
		var r UnsubSenderAgg
		if err := rows.Scan(&r.FromEmail, &r.Count, &r.UnreadCount, &r.NewestMs); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// NewestUnsubMeta returns the sender's newest message that carries a
// List-Unsubscribe header. Propagates sql.ErrNoRows when the sender has
// none.
func (s *Store) NewestUnsubMeta(account, fromEmail string) (MailMeta, error) {
	var m MailMeta
	var labelJSON string
	var unread int
	err := s.db.QueryRow(
		`SELECT account, id, thread_id, from_email, from_name, subject, snippet,
			internal_date, size_estimate, label_ids, category,
			list_unsubscribe, list_unsubscribe_post, unread,
			auth_results, list_unsub_domain
		 FROM mail_meta
		 WHERE account = ? AND from_email = ? AND list_unsubscribe != ''
		 ORDER BY internal_date DESC, id DESC LIMIT 1`,
		account, fromEmail,
	).Scan(&m.Account, &m.ID, &m.ThreadID, &m.FromEmail, &m.FromName, &m.Subject, &m.Snippet,
		&m.InternalDate, &m.SizeEstimate, &labelJSON, &m.Category,
		&m.ListUnsubscribe, &m.ListUnsubscribePost, &unread,
		&m.AuthResults, &m.ListUnsubDomain)
	if err != nil {
		return MailMeta{}, err
	}
	m.LabelIDs = unmarshalLabelIDs(labelJSON)
	m.Unread = unread != 0
	return m, nil
}

// SetMailListUnsubDomain stamps the extracted registrable unsubscribe
// domain on every unsubscribe-bearing row of one sender (the audit
// command's local-store write). Returns the number of rows updated.
func (s *Store) SetMailListUnsubDomain(account, fromEmail, domain string) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.Exec(
		`UPDATE mail_meta SET list_unsub_domain = ?
		 WHERE account = ? AND from_email = ? AND list_unsubscribe != ''`,
		domain, account, fromEmail,
	)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}
