package yclocal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ChangeRow describes a per-slug delta between two snapshots.
type ChangeRow struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Batch    string `json:"batch"`
	Field    string `json:"field"`
	From     any    `json:"from"`
	To       any    `json:"to"`
	FromSnap string `json:"from_snapshot"`
	ToSnap   string `json:"to_snapshot"`
}

// ChangesQuery filters the change feed.
type ChangesQuery struct {
	Field      string   // one of: status, team_size, is_hiring, top_company
	ToValueSet bool     // true if ToValue is meaningful (filter result rows by .To)
	ToValue    string   // string form ("true"/"false" for bool fields, raw int for team_size, raw status for status)
	FromSnap   string   // older snapshot id; empty = oldest available
	ToSnap     string   // newer snapshot id; empty = latest
	Slugs      []string // optional slug filter (empty = all)
	Limit      int      // 0 = no limit
}

// Changes computes per-field deltas between two snapshots.
// If FromSnap is empty, uses the oldest snapshot (or none if only one exists).
// If ToSnap is empty, uses the latest snapshot.
func Changes(ctx context.Context, db *sql.DB, q ChangesQuery) ([]ChangeRow, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	field := strings.ToLower(strings.TrimSpace(q.Field))
	switch field {
	case "status", "team_size", "is_hiring", "top_company":
	default:
		return nil, fmt.Errorf("unsupported field %q (use status, team_size, is_hiring, top_company)", q.Field)
	}

	if q.ToSnap == "" {
		latest, err := LatestSnapshotID(ctx, db)
		if err != nil {
			return nil, err
		}
		q.ToSnap = latest
	}
	if q.FromSnap == "" {
		var oldest sql.NullString
		err := db.QueryRowContext(ctx, `SELECT snapshot_id FROM companies_history WHERE snapshot_id <> ? ORDER BY snapshot_id ASC LIMIT 1`, q.ToSnap).Scan(&oldest)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}
		q.FromSnap = oldest.String
	}
	if q.ToSnap == "" || q.FromSnap == "" || q.ToSnap == q.FromSnap {
		return nil, nil
	}

	var (
		whereSlug string
		args      = []any{q.FromSnap, q.ToSnap}
	)
	if len(q.Slugs) > 0 {
		placeholders := make([]string, len(q.Slugs))
		for i, s := range q.Slugs {
			placeholders[i] = "?"
			args = append(args, s)
		}
		whereSlug = " AND a.slug IN (" + strings.Join(placeholders, ",") + ")"
	}

	var fieldExpr string
	switch field {
	case "status":
		fieldExpr = "a.status, b.status"
	case "team_size":
		fieldExpr = "a.team_size, b.team_size"
	case "is_hiring":
		fieldExpr = "a.is_hiring, b.is_hiring"
	case "top_company":
		fieldExpr = "a.top_company, b.top_company"
	}

	query := `
SELECT a.slug, COALESCE(c.name, b.name, a.name), COALESCE(c.batch, b.batch, a.batch), ` + fieldExpr + `
FROM companies_history a
JOIN companies_history b ON b.slug = a.slug
LEFT JOIN companies c ON c.slug = a.slug
WHERE a.snapshot_id = ? AND b.snapshot_id = ?` + whereSlug

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ChangeRow
	for rows.Next() {
		var (
			slug, name, batch string
			fromAny, toAny    any
		)
		switch field {
		case "status":
			var f, t sql.NullString
			if err := rows.Scan(&slug, &name, &batch, &f, &t); err != nil {
				return nil, err
			}
			if f.String == t.String {
				continue
			}
			fromAny, toAny = f.String, t.String
		case "team_size":
			var f, t sql.NullInt64
			if err := rows.Scan(&slug, &name, &batch, &f, &t); err != nil {
				return nil, err
			}
			if f.Int64 == t.Int64 {
				continue
			}
			fromAny, toAny = f.Int64, t.Int64
		case "is_hiring", "top_company":
			var f, t sql.NullInt64
			if err := rows.Scan(&slug, &name, &batch, &f, &t); err != nil {
				return nil, err
			}
			if f.Int64 == t.Int64 {
				continue
			}
			fromAny, toAny = f.Int64 != 0, t.Int64 != 0
		}
		if q.ToValueSet {
			if !matchTo(toAny, q.ToValue) {
				continue
			}
		}
		out = append(out, ChangeRow{
			Slug: slug, Name: name, Batch: batch,
			Field: field, From: fromAny, To: toAny,
			FromSnap: q.FromSnap, ToSnap: q.ToSnap,
		})
		if q.Limit > 0 && len(out) >= q.Limit {
			break
		}
	}
	return out, rows.Err()
}

func matchTo(v any, want string) bool {
	want = strings.TrimSpace(strings.ToLower(want))
	switch x := v.(type) {
	case string:
		return strings.EqualFold(x, want)
	case bool:
		return (x && (want == "true" || want == "1" || want == "yes")) ||
			(!x && (want == "false" || want == "0" || want == "no"))
	case int64:
		return fmt.Sprintf("%d", x) == want
	}
	return false
}

// NewCompany is one row returned by NewSince.
type NewCompany struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Batch         string `json:"batch"`
	Status        string `json:"status"`
	OneLiner      string `json:"one_liner"`
	LaunchedAt    int64  `json:"launched_at"`
	FirstSeenSnap string `json:"first_seen_snapshot"`
}

// NewSince returns companies present in the latest snapshot but absent from
// the snapshot at or before sinceSnap. sinceSnap must be a snapshot_id; the
// caller resolves dates via SnapshotAtOrBefore.
func NewSince(ctx context.Context, db *sql.DB, sinceSnap string, limit int) ([]NewCompany, error) {
	if err := EnsureSchema(db); err != nil {
		return nil, err
	}
	latest, err := LatestSnapshotID(ctx, db)
	if err != nil {
		return nil, err
	}
	if latest == "" || sinceSnap == "" || latest == sinceSnap {
		return nil, nil
	}
	q := `
SELECT a.slug,
       COALESCE(c.name, a.name),
       COALESCE(c.batch, a.batch),
       COALESCE(c.status, a.status, ''),
       COALESCE(c.one_liner, ''),
       COALESCE(c.launched_at, 0),
       a.snapshot_id
FROM companies_history a
LEFT JOIN companies c ON c.slug = a.slug
WHERE a.snapshot_id = ?
  AND a.slug NOT IN (
    SELECT slug FROM companies_history WHERE snapshot_id = ?
  )
ORDER BY COALESCE(c.launched_at, 0) DESC, a.slug ASC`
	args := []any{latest, sinceSnap}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NewCompany
	for rows.Next() {
		var n NewCompany
		if err := rows.Scan(&n.Slug, &n.Name, &n.Batch, &n.Status, &n.OneLiner, &n.LaunchedAt, &n.FirstSeenSnap); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
