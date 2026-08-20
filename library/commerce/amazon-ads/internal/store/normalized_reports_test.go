package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestImportNormalizedReport(t *testing.T) {
	t.Parallel()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	rows := []json.RawMessage{
		json.RawMessage(`{"campaign":"Core","spend":10}`),
		json.RawMessage(`{"campaign":"Scale","spend":20}`),
	}
	meta, err := s.ImportNormalizedReport(context.Background(), "report-1", "performance", "perf.csv.gz", rows)
	if err != nil {
		t.Fatalf("ImportNormalizedReport returned error: %v", err)
	}
	if meta.RowCount != 2 || meta.Kind != "performance" {
		t.Fatalf("meta = %+v", meta)
	}

	var reportCount, rowCount int
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM normalized_reports WHERE id = ?`, "report-1").Scan(&reportCount); err != nil {
		t.Fatalf("count normalized_reports: %v", err)
	}
	if err := s.DB().QueryRow(`SELECT COUNT(*) FROM normalized_report_rows WHERE report_id = ?`, "report-1").Scan(&rowCount); err != nil {
		t.Fatalf("count normalized_report_rows: %v", err)
	}
	if reportCount != 1 || rowCount != 2 {
		t.Fatalf("reportCount=%d rowCount=%d, want 1 and 2", reportCount, rowCount)
	}
}
