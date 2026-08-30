package store

import (
	"encoding/json"
)

// PDFRecord is the durable local catalog shape for an indexed PDF.
type PDFRecord struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Pages    int    `json:"pages"`
	SHA256   string `json:"sha256"`
	Modified string `json:"modified,omitempty"`
	Text     string `json:"text,omitempty"`
}

// UpsertPDF stores a catalog record in the generic local resource table while
// keeping a typed entry point for callers and future schema migrations.
func (s *Store) UpsertPDF(record PDFRecord) error {
	payload, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return s.Upsert("pdf_catalog", record.ID, payload)
}

// SearchPDF searches indexed PDF records through the store's FTS-aware search
// path and returns typed JSON records for agent-friendly output.
func (s *Store) SearchPDF(query string, limit int) ([]json.RawMessage, error) {
	return s.Search(query, limit, "pdf_catalog")
}
