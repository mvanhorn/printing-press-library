package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type NormalizedReportImport struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	SourcePath string    `json:"source_path"`
	RowCount   int       `json:"row_count"`
	ImportedAt time.Time `json:"imported_at"`
}

func (s *Store) ImportNormalizedReport(ctx context.Context, id, kind, sourcePath string, rows []json.RawMessage) (NormalizedReportImport, error) {
	if id == "" {
		return NormalizedReportImport{}, fmt.Errorf("normalized report id is required")
	}
	if kind == "" {
		return NormalizedReportImport{}, fmt.Errorf("normalized report kind is required")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return NormalizedReportImport{}, fmt.Errorf("starting normalized report import: %w", err)
	}
	defer tx.Rollback()

	importedAt := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `DELETE FROM normalized_report_rows WHERE report_id = ?`, id); err != nil {
		return NormalizedReportImport{}, fmt.Errorf("clearing normalized report rows: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO normalized_reports (id, kind, source_path, row_count, imported_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET kind = excluded.kind, source_path = excluded.source_path,
		 row_count = excluded.row_count, imported_at = excluded.imported_at`,
		id, kind, sourcePath, len(rows), importedAt.Format(time.RFC3339),
	); err != nil {
		return NormalizedReportImport{}, fmt.Errorf("upserting normalized report: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO normalized_report_rows (report_id, row_index, data) VALUES (?, ?, ?)`)
	if err != nil {
		return NormalizedReportImport{}, fmt.Errorf("preparing normalized report rows: %w", err)
	}
	defer stmt.Close()
	for i, row := range rows {
		if !json.Valid(row) {
			return NormalizedReportImport{}, fmt.Errorf("normalized row %d is not valid JSON", i)
		}
		if _, err := stmt.ExecContext(ctx, id, i, string(row)); err != nil {
			return NormalizedReportImport{}, fmt.Errorf("inserting normalized row %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return NormalizedReportImport{}, fmt.Errorf("committing normalized report import: %w", err)
	}
	return NormalizedReportImport{
		ID:         id,
		Kind:       kind,
		SourcePath: sourcePath,
		RowCount:   len(rows),
		ImportedAt: importedAt,
	}, nil
}
