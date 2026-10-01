// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual

// Change history from Actual's CRDT message log (messages_crdt). Every edit
// made on any device is a message: timestamp "ISO-COUNTER-NODE", the table
// (dataset), row id, column and a serialized value ("S:str", "N:num", "0:" =
// null).

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CRDTMessage is one decoded change message.
type CRDTMessage struct {
	Timestamp string
	Time      time.Time
	Node      string
	Dataset   string
	Row       string
	Column    string
	Value     any
}

// DecodeCRDTValue decodes Actual's serialized value: "S:text" -> string,
// "N:12.5" -> number (int64 when integral), "0:" -> nil. Unknown prefixes are
// returned verbatim.
func DecodeCRDTValue(v string) any {
	switch {
	case strings.HasPrefix(v, "S:"):
		return v[2:]
	case strings.HasPrefix(v, "N:"):
		s := v[2:]
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return s
	case strings.HasPrefix(v, "0:"):
		return nil
	}
	return v
}

// ParseCRDTTimestamp splits "2026-09-15T10:00:00.000Z-0000-aaaaaaaaaaaaaaaa"
// into its wall time and node id (the last 16 hex characters).
func ParseCRDTTimestamp(ts string) (time.Time, string, error) {
	parts := strings.Split(ts, "-")
	if len(parts) < 5 {
		return time.Time{}, "", fmt.Errorf("invalid CRDT timestamp %q", ts)
	}
	node := parts[len(parts)-1]
	iso := strings.Join(parts[:len(parts)-2], "-")
	t, err := time.Parse(time.RFC3339Nano, iso)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("invalid CRDT timestamp %q: %w", ts, err)
	}
	return t, node, nil
}

// crdtKey renders a time in the lexicographically comparable prefix form
// used by message timestamps.
func crdtKey(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }

// CRDTMessages returns messages with since <= time < until (zero until = no
// upper bound), optionally limited to one dataset, oldest first.
func (l *Ledger) CRDTMessages(ctx context.Context, since, until time.Time, dataset string) ([]CRDTMessage, error) {
	out := make([]CRDTMessage, 0)
	// HasTable reports false on a dead context; surface the real error.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !l.HasTable(ctx, "messages_crdt") {
		return out, nil
	}
	q := "SELECT timestamp, dataset, row, column, value FROM messages_crdt WHERE timestamp >= ?"
	args := []any{crdtKey(since)}
	if !until.IsZero() {
		q += " AND timestamp < ?"
		args = append(args, crdtKey(until))
	}
	if dataset != "" {
		q += " AND dataset = ?"
		args = append(args, dataset)
	}
	q += " ORDER BY timestamp"
	rows, err := l.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("querying messages_crdt: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m CRDTMessage
		var raw []byte
		if err := rows.Scan(&m.Timestamp, &m.Dataset, &m.Row, &m.Column, &raw); err != nil {
			return nil, err
		}
		m.Value = DecodeCRDTValue(string(raw))
		m.Time, m.Node, _ = ParseCRDTTimestamp(m.Timestamp)
		out = append(out, m)
	}
	return out, rows.Err()
}

// RowChange summarizes all messages for one (dataset, row) in a window.
type RowChange struct {
	Dataset      string         `json:"dataset"`
	Row          string         `json:"row"`
	Action       string         `json:"action"` // "edited" or "deleted" (or "restored")
	Entity       string         `json:"entity,omitempty"`
	Changes      map[string]any `json:"changes"`
	FirstChange  string         `json:"first_change"`
	LastChange   string         `json:"last_change"`
	Device       string         `json:"device"`
	Devices      []string       `json:"devices"`
	MessageCount int            `json:"message_count"`
}

// GroupChanges folds messages (oldest first) into one RowChange per row,
// keeping each column's latest value. A tombstone=1 message marks the row
// "deleted", tombstone=0 "restored"; anything else is "edited" (the log
// cannot reliably distinguish creation from editing). Newest change first.
func GroupChanges(msgs []CRDTMessage) []RowChange {
	idx := map[string]int{}
	out := make([]RowChange, 0)
	for _, m := range msgs {
		key := m.Dataset + "\x00" + m.Row
		i, ok := idx[key]
		if !ok {
			i = len(out)
			idx[key] = i
			out = append(out, RowChange{Dataset: m.Dataset, Row: m.Row, Action: "edited", Changes: map[string]any{}, FirstChange: m.Timestamp, Devices: []string{}})
		}
		rc := &out[i]
		rc.Changes[m.Column] = m.Value
		rc.LastChange = m.Timestamp
		rc.Device = m.Node
		rc.MessageCount++
		if !slices.Contains(rc.Devices, m.Node) {
			rc.Devices = append(rc.Devices, m.Node)
		}
		if m.Column == "tombstone" {
			switch m.Value {
			case int64(1):
				rc.Action = "deleted"
			case int64(0):
				rc.Action = "restored"
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].LastChange > out[j].LastChange })
	return out
}

// DescribeEntity returns a human label for a row from the current tables,
// including tombstoned rows: transactions as "payee amount date",
// categories/payees/accounts/groups by name, budget cells as
// "category YYYY-MM". Unknown rows return "".
func (l *Ledger) DescribeEntity(ctx context.Context, dataset, row string) string {
	switch dataset {
	case "transactions":
		var payee, imported sql.NullString
		var amount, dateInt sql.NullInt64
		err := l.DB.QueryRowContext(ctx, `SELECT p.name, t.amount, t.date, t.imported_description FROM transactions t
			LEFT JOIN payee_mapping pm ON pm.id = t.description
			LEFT JOIN payees p ON p.id = COALESCE(pm.targetId, t.description)
			WHERE t.id = ?`, row).Scan(&payee, &amount, &dateInt, &imported)
		if err != nil {
			return ""
		}
		name := payee.String
		if name == "" {
			name = imported.String
		}
		parts := []string{}
		if name != "" {
			parts = append(parts, name)
		}
		parts = append(parts, FormatAmount(amount.Int64))
		if dateInt.Valid {
			parts = append(parts, FormatDate(int(dateInt.Int64)))
		}
		return strings.Join(parts, " ")
	case "categories", "payees", "accounts", "category_groups", "schedules":
		var name sql.NullString
		if err := l.DB.QueryRowContext(ctx, "SELECT name FROM "+dataset+" WHERE id = ?", row).Scan(&name); err != nil {
			return ""
		}
		return name.String
	case "zero_budgets", "reflect_budgets":
		var name sql.NullString
		var month sql.NullInt64
		if err := l.DB.QueryRowContext(ctx, "SELECT c.name, b.month FROM "+dataset+" b LEFT JOIN categories c ON c.id = b.category WHERE b.id = ?", row).Scan(&name, &month); err != nil {
			return ""
		}
		if !month.Valid {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("%s %04d-%02d", name.String, month.Int64/100, month.Int64%100))
	}
	return ""
}

// changeRefs maps (dataset, column) to the table whose name a foreign-key
// value refers to.
var changeRefs = map[string]map[string]string{
	"transactions": {"description": "payees", "acct": "accounts", "category": "categories"},
	"categories":   {"cat_group": "category_groups"},
	"payees":       {"transfer_acct": "accounts"},
}

// changeDateCols are integer YYYYMMDD columns shown as ISO dates.
var changeDateCols = map[string]bool{"date": true}

// changeDropCols are bank-sync payloads too bulky and raw for a change log.
var changeDropCols = map[string]bool{"raw_synced_data": true}

// NameResolver turns payee, account, category and category-group ids into
// display names for HumanizeChanges. Each table is read once, on first use,
// including tombstoned rows; transfer payees (which have no name) resolve to
// the account they move money to.
type NameResolver struct {
	l     *Ledger
	names map[string]map[string]string // table -> id -> non-empty name
}

// NewNameResolver returns a resolver over l's current tables.
func (l *Ledger) NewNameResolver() *NameResolver {
	return &NameResolver{l: l, names: map[string]map[string]string{}}
}

// name returns the non-empty display name of id in table.
func (r *NameResolver) name(ctx context.Context, table, id string) (string, bool) {
	m, loaded := r.names[table]
	if !loaded {
		m = r.load(ctx, table)
		r.names[table] = m
	}
	n, ok := m[id]
	return n, ok
}

func (r *NameResolver) load(ctx context.Context, table string) map[string]string {
	m := map[string]string{}
	if checkTable(table) != nil {
		return m
	}
	q := "SELECT id, name FROM " + table // #nosec G202 -- table passed checkTable allowlist
	if table == "payees" {
		q = "SELECT p.id, COALESCE(NULLIF(p.name, ''), a.name) FROM payees p LEFT JOIN accounts a ON a.id = p.transfer_acct"
	}
	rows, err := r.l.DB.QueryContext(ctx, q)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var id, name sql.NullString
		if rows.Scan(&id, &name) != nil {
			continue
		}
		if !id.Valid || !name.Valid || name.String == "" {
			continue
		}
		if _, dup := m[id.String]; !dup {
			m[id.String] = name.String
		}
	}
	return m
}

// HumanizeChanges rewrites rc.Changes for reading: foreign-key ids become
// names (the id moves to "<column>_id"), YYYYMMDD integers become ISO dates,
// and raw bank-sync payloads are dropped.
func (r *NameResolver) HumanizeChanges(ctx context.Context, rc *RowChange) {
	refs := changeRefs[rc.Dataset]
	for col, v := range rc.Changes {
		switch {
		case changeDropCols[col]:
			delete(rc.Changes, col)
		case changeDateCols[col]:
			if n, ok := v.(int64); ok && n > 19000000 {
				rc.Changes[col] = FormatDate(int(n))
			}
		case refs[col] != "":
			id, ok := v.(string)
			if !ok || id == "" {
				continue
			}
			name, ok := r.name(ctx, refs[col], id)
			if !ok {
				continue
			}
			rc.Changes[col+"_id"] = id
			rc.Changes[col] = name
		}
	}
}
