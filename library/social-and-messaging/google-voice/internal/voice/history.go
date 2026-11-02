package voice

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func scanRecords(rows *sql.Rows, account string) ([]Record, error) {
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		var b string
		v := Record{AccountID: account}
		if e := rows.Scan(&b, &v.Sequence, &v.FirstRun, &v.FirstSeen); e != nil {
			return nil, e
		}
		if e := json.Unmarshal([]byte(b), &v.Message); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (m *Mirror) Context(ctx context.Context, id string, before, after int) (map[string]any, error) {
	if id == "" || before < 0 || before > 100 || after < 0 || after > 100 {
		return nil, fmt.Errorf("--message-id is required; --before and --after must be 0 through 100")
	}
	var thread string
	var ts int64
	if e := m.DB.QueryRowContext(ctx, "SELECT thread_id,ts FROM voice_messages WHERE id=?", id).Scan(&thread, &ts); e != nil {
		return nil, e
	}
	rows, e := m.DB.QueryContext(ctx, `SELECT data,sequence,first_run,first_seen FROM voice_messages WHERE thread_id=? AND (ts<? OR (ts=? AND id<=?)) ORDER BY ts DESC,id DESC LIMIT ?`, thread, ts, ts, id, before+2)
	if e != nil {
		return nil, e
	}
	left, e := scanRecords(rows, m.AccountID)
	if e != nil {
		return nil, e
	}
	leftClipped := len(left) > before+1
	if leftClipped {
		left = left[:before+1]
	}
	for i, j := 0, len(left)-1; i < j; i, j = i+1, j-1 {
		left[i], left[j] = left[j], left[i]
	}
	rows, e = m.DB.QueryContext(ctx, `SELECT data,sequence,first_run,first_seen FROM voice_messages WHERE thread_id=? AND (ts>? OR (ts=? AND id>?)) ORDER BY ts,id LIMIT ?`, thread, ts, ts, id, after+1)
	if e != nil {
		return nil, e
	}
	right, e := scanRecords(rows, m.AccountID)
	if e != nil {
		return nil, e
	}
	rightClipped := len(right) > after
	if rightClipped {
		right = right[:after]
	}
	messages := append(left, right...)
	var cursor, observed, lastRefresh, gap string
	var exhausted, needsRefresh bool
	if e = m.DB.QueryRowContext(ctx, "SELECT older_cursor,history_complete,history_observed_at,last_refresh,needs_refresh,refresh_cursor FROM voice_threads WHERE id=?", thread).Scan(&cursor, &exhausted, &observed, &lastRefresh, &needsRefresh, &gap); e != nil {
		return nil, e
	}
	return map[string]any{"account_id": m.AccountID, "thread_id": thread, "anchor_id": id, "messages": messages, "clipped_before": leftClipped, "clipped_after": rightClipped, "collection": map[string]any{"older_cursor_pending": cursor != "", "older_history_exhausted_at_observation": exhausted, "history_observed_at": observed, "last_refresh": lastRefresh, "needs_refresh": needsRefresh, "pending_refresh_gap": gap != "", "note": "This is a window over locally observed records, not guaranteed complete source history."}}, nil
}

type arrivalCursor struct {
	Account       string `json:"account"`
	AfterSequence int64  `json:"after_sequence"`
	AfterRun      int64  `json:"after_run"`
}

func (m *Mirror) Arrivals(ctx context.Context, afterRun int64, cursor string, limit int) (map[string]any, error) {
	if limit < 1 || limit > 10000 || afterRun < 0 {
		return nil, fmt.Errorf("--limit must be 1 through 10000 and --after-run must be nonnegative")
	}
	seek := arrivalCursor{Account: m.AccountID, AfterRun: afterRun}
	if cursor != "" {
		b, e := base64.RawURLEncoding.DecodeString(cursor)
		if e != nil || json.Unmarshal(b, &seek) != nil || seek.Account != m.AccountID || seek.AfterSequence < 0 || seek.AfterRun < 0 || (afterRun != 0 && afterRun != seek.AfterRun) {
			return nil, fmt.Errorf("--cursor is invalid for this account or --after-run")
		}
		afterRun = seek.AfterRun
	}
	if afterRun > 0 {
		var status string
		e := m.DB.QueryRowContext(ctx, "SELECT status FROM voice_runs WHERE id=?", afterRun).Scan(&status)
		if errors.Is(e, sql.ErrNoRows) {
			return nil, fmt.Errorf("--after-run must identify a saved terminal run in this profile")
		}
		if e != nil {
			return nil, e
		}
		if status == "running" {
			return nil, fmt.Errorf("--after-run is still running; select a terminal run")
		}
	}
	rows, e := m.DB.QueryContext(ctx, `SELECT data,sequence,first_run,first_seen FROM voice_messages WHERE sequence>? AND first_run>? ORDER BY sequence LIMIT ?`, seek.AfterSequence, afterRun, limit)
	if e != nil {
		return nil, e
	}
	messages, e := scanRecords(rows, m.AccountID)
	if e != nil {
		return nil, e
	}
	next := ""
	if len(messages) > 0 {
		last := messages[len(messages)-1].Sequence
		var remaining bool
		if e = m.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM voice_messages WHERE sequence>? AND first_run>?)", last, afterRun).Scan(&remaining); e != nil {
			return nil, e
		}
		if remaining {
			b, _ := json.Marshal(arrivalCursor{m.AccountID, last, afterRun})
			next = base64.RawURLEncoding.EncodeToString(b)
		}
	}
	return map[string]any{"account_id": m.AccountID, "messages": messages, "after_run": afterRun, "next_cursor": next, "note": "Arrivals are newly mirrored source IDs, including historical backfill. Source timestamps describe communication time; first_seen describes collection time."}, nil
}

type Activity struct {
	ThreadID         string    `json:"thread_id"`
	Participants     []Contact `json:"participants"`
	Inbound          int64     `json:"inbound"`
	Outbound         int64     `json:"outbound"`
	UnknownDirection int64     `json:"unknown_direction"`
	LatestInbound    *string   `json:"latest_inbound"`
	LatestOutbound   *string   `json:"latest_outbound"`
	LastRefresh      string    `json:"last_refresh"`
	HistoryExhausted bool      `json:"older_history_exhausted_at_observation"`
}

func (m *Mirror) Activity(ctx context.Context, after, before time.Time, limit int) ([]Activity, error) {
	if !after.Before(before) || limit < 1 || limit > 10000 {
		return nil, fmt.Errorf("activity requires an increasing time window and --limit from 1 through 10000")
	}
	rows, e := m.DB.QueryContext(ctx, `SELECT t.id,t.data,COALESCE(SUM(m.direction='inbound'),0),COALESCE(SUM(m.direction='outbound'),0),COALESCE(SUM(m.direction='unknown'),0),MAX(CASE WHEN m.direction='inbound' THEN m.ts END),MAX(CASE WHEN m.direction='outbound' THEN m.ts END),t.last_refresh,t.history_complete FROM voice_threads t LEFT JOIN voice_messages m ON m.thread_id=t.id AND m.kind IN('text','mms') AND m.ts>=? AND m.ts<? GROUP BY t.id ORDER BY MAX(m.ts) DESC,t.id LIMIT ?`, ceilMillis(after), ceilMillis(before), limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Activity{}
	for rows.Next() {
		v := Activity{Participants: []Contact{}}
		var raw string
		var in, outbound sql.NullInt64
		if e = rows.Scan(&v.ThreadID, &raw, &v.Inbound, &v.Outbound, &v.UnknownDirection, &in, &outbound, &v.LastRefresh, &v.HistoryExhausted); e != nil {
			return nil, e
		}
		var thread Thread
		if e = json.Unmarshal([]byte(raw), &thread); e != nil {
			return nil, e
		}
		v.Participants = thread.Contacts
		if in.Valid {
			s := time.UnixMilli(in.Int64).UTC().Format(time.RFC3339Nano)
			v.LatestInbound = &s
		}
		if outbound.Valid {
			s := time.UnixMilli(outbound.Int64).UTC().Format(time.RFC3339Nano)
			v.LatestOutbound = &s
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
