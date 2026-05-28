package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type KeywordSnapshot struct {
	ID         string    `json:"id"`
	Name       string    `json:"name,omitempty"`
	SourcePath string    `json:"source_path,omitempty"`
	SnapshotAt time.Time `json:"snapshot_at"`
	RowCount   int       `json:"row_count"`
	ImportedAt time.Time `json:"imported_at"`
}

func KeywordSnapshotID(sourcePath string, snapshotAt time.Time, rows []json.RawMessage) string {
	h := sha256.New()
	_, _ = h.Write([]byte(sourcePath))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(snapshotAt.UTC().Format(time.RFC3339)))
	for _, row := range rows {
		_, _ = h.Write(row)
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (s *Store) ImportKeywordSnapshot(ctx context.Context, id, name, sourcePath string, snapshotAt time.Time, rows []json.RawMessage) (KeywordSnapshot, error) {
	if snapshotAt.IsZero() {
		snapshotAt = time.Now().UTC()
	}
	if id == "" {
		id = KeywordSnapshotID(sourcePath, snapshotAt, rows)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return KeywordSnapshot{}, fmt.Errorf("starting keyword snapshot import: %w", err)
	}
	defer tx.Rollback()

	importedAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `DELETE FROM keyword_snapshot_rows WHERE snapshot_id = ?`, id); err != nil {
		return KeywordSnapshot{}, fmt.Errorf("clearing keyword snapshot rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO keyword_snapshots (id, name, source_path, snapshot_at, row_count, imported_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, source_path = excluded.source_path,
		 snapshot_at = excluded.snapshot_at, row_count = excluded.row_count, imported_at = excluded.imported_at`,
		id, name, sourcePath, snapshotAt.UTC().Format(time.RFC3339), len(rows), importedAt.Format(time.RFC3339),
	); err != nil {
		return KeywordSnapshot{}, fmt.Errorf("upserting keyword snapshot: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO keyword_snapshot_rows (snapshot_id, row_index, keyword, campaign, ad_group, bid, cpc, spend, sales, orders, clicks, data) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return KeywordSnapshot{}, fmt.Errorf("preparing keyword snapshot rows: %w", err)
	}
	defer stmt.Close()
	for i, row := range rows {
		var obj map[string]any
		if err := json.Unmarshal(row, &obj); err != nil {
			return KeywordSnapshot{}, fmt.Errorf("keyword snapshot row %d is not a JSON object: %w", i, err)
		}
		keyword := firstString(obj, "keyword", "Keyword")
		if keyword == "" {
			return KeywordSnapshot{}, fmt.Errorf("keyword snapshot row %d is missing keyword", i)
		}
		obj["date"] = snapshotAt.UTC().Format(time.RFC3339)
		rowWithDate, err := json.Marshal(obj)
		if err != nil {
			return KeywordSnapshot{}, fmt.Errorf("encoding keyword snapshot row %d: %w", i, err)
		}
		if _, err := stmt.ExecContext(ctx, id, i, keyword, firstString(obj, "campaign", "Campaign"), firstString(obj, "ad_group", "adGroup", "AdGroup"), numberValue(obj["bid"]), numberValue(obj["cpc"]), numberValue(obj["spend"]), numberValue(obj["sales"]), int(numberValue(obj["orders"])), int(numberValue(obj["clicks"])), string(rowWithDate)); err != nil {
			return KeywordSnapshot{}, fmt.Errorf("inserting keyword snapshot row %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return KeywordSnapshot{}, fmt.Errorf("committing keyword snapshot import: %w", err)
	}
	return KeywordSnapshot{ID: id, Name: name, SourcePath: sourcePath, SnapshotAt: snapshotAt.UTC(), RowCount: len(rows), ImportedAt: importedAt}, nil
}

func (s *Store) ListKeywordSnapshots(ctx context.Context) ([]KeywordSnapshot, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(name, ''), COALESCE(source_path, ''), snapshot_at, row_count, imported_at FROM keyword_snapshots ORDER BY snapshot_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing keyword snapshots: %w", err)
	}
	defer rows.Close()
	var out []KeywordSnapshot
	for rows.Next() {
		var item KeywordSnapshot
		var snapshotAt, importedAt string
		if err := rows.Scan(&item.ID, &item.Name, &item.SourcePath, &snapshotAt, &item.RowCount, &importedAt); err != nil {
			return nil, fmt.Errorf("scanning keyword snapshot: %w", err)
		}
		item.SnapshotAt, _ = time.Parse(time.RFC3339, snapshotAt)
		item.ImportedAt, _ = time.Parse(time.RFC3339, importedAt)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading keyword snapshots: %w", err)
	}
	return out, nil
}

func (s *Store) KeywordHistory(ctx context.Context, keyword string) ([]json.RawMessage, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, fmt.Errorf("keyword is required")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.data
		FROM keyword_snapshot_rows r
		JOIN keyword_snapshots s ON s.id = r.snapshot_id
		WHERE lower(r.keyword) = lower(?)
		ORDER BY s.snapshot_at ASC, r.row_index ASC`, keyword)
	if err != nil {
		return nil, fmt.Errorf("querying keyword history: %w", err)
	}
	defer rows.Close()
	var out []json.RawMessage
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scanning keyword history: %w", err)
		}
		out = append(out, json.RawMessage(raw))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading keyword history: %w", err)
	}
	return out, nil
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := obj[key]; ok {
			if s, ok := raw.(string); ok {
				return s
			}
		}
	}
	return ""
}

func numberValue(raw any) float64 {
	switch v := raw.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case json.Number:
		n, _ := v.Float64()
		return n
	default:
		return 0
	}
}
