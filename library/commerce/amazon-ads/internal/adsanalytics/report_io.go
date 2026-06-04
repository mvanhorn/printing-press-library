package adsanalytics

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type NormalizedReport struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	SourcePath string            `json:"source_path"`
	RowCount   int               `json:"row_count"`
	Rows       []json.RawMessage `json:"rows"`
}

func ReadReportFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading report %s: %w", path, err)
	}
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b || strings.HasSuffix(strings.ToLower(path), ".gz") {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("opening gzip report %s: %w", path, err)
		}
		defer zr.Close()
		expanded, err := io.ReadAll(zr)
		if err != nil {
			return nil, fmt.Errorf("decompressing gzip report %s: %w", path, err)
		}
		data = expanded
	}
	return data, nil
}

func NormalizeReport(path, kind string) (NormalizedReport, error) {
	kind = strings.TrimSpace(strings.ToLower(kind))
	if kind == "" {
		kind = "generic"
	}
	var rows []json.RawMessage
	var err error
	switch kind {
	case "search-terms", "search_terms":
		kind = "search-terms"
		var parsed []SearchTermPerformance
		parsed, err = LoadSearchTermReport(path)
		rows = marshalRows(parsed)
	case "performance", "campaign-performance", "product-performance", "placement-performance":
		kind = "performance"
		var parsed []PerformanceRow
		parsed, err = LoadPerformanceReport(path)
		rows = marshalRows(parsed)
	case "keyword-performance", "keywords":
		kind = "keyword-performance"
		var parsed []KeywordPerformance
		parsed, err = LoadKeywordPerformanceReport(path)
		rows = marshalRows(parsed)
	case "generic", "auto":
		kind = "generic"
		rows, err = normalizeGenericReport(path)
	default:
		return NormalizedReport{}, fmt.Errorf("unsupported report kind %q: use search-terms, performance, keyword-performance, or generic", kind)
	}
	if err != nil {
		return NormalizedReport{}, err
	}
	report := NormalizedReport{
		Kind:       kind,
		SourcePath: path,
		RowCount:   len(rows),
		Rows:       rows,
	}
	report.ID = normalizedReportID(report)
	return report, nil
}

func marshalRows[T any](items []T) []json.RawMessage {
	rows := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		data, err := json.Marshal(item)
		if err == nil {
			rows = append(rows, data)
		}
	}
	return rows
}

func normalizeGenericReport(path string) ([]json.RawMessage, error) {
	data, err := ReadReportFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("report %s is empty", path)
	}
	if strings.HasPrefix(trimmed, "[") {
		var rows []json.RawMessage
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		return rows, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(data, &envelope); err != nil {
			return nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		for _, key := range []string{"rows", "data", "items", "records", "results"} {
			if raw := envelope[key]; len(raw) > 0 {
				var rows []json.RawMessage
				if err := json.Unmarshal(raw, &rows); err == nil {
					return rows, nil
				}
			}
		}
		return []json.RawMessage{json.RawMessage(data)}, nil
	}
	return normalizeCSVReport(strings.NewReader(trimmed))
}

func normalizeCSVReport(r io.Reader) ([]json.RawMessage, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV report must include a header and at least one row")
	}
	headers := make([]string, len(records[0]))
	for i, h := range records[0] {
		headers[i] = normalizeHeader(h)
	}
	rows := make([]json.RawMessage, 0, len(records)-1)
	for _, record := range records[1:] {
		obj := map[string]any{}
		for i, header := range headers {
			if header == "" || i >= len(record) {
				continue
			}
			obj[header] = strings.TrimSpace(record[i])
		}
		data, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		rows = append(rows, data)
	}
	return rows, nil
}

func normalizedReportID(report NormalizedReport) string {
	h := sha256.New()
	_, _ = h.Write([]byte(report.Kind))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(filepath.Base(report.SourcePath)))
	_, _ = h.Write([]byte{0})
	for _, row := range report.Rows {
		_, _ = h.Write(row)
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
