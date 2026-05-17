package salestech

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// CSVImportRow is one parsed CSV row ready to be turned into one ST
// estimate-create + N estimate-put-item calls. The CLI command wraps each
// row into the generated client's Create + PutItem flow.
type CSVImportRow struct {
	LineNumber  int               `json:"line_number"`
	JobID       int64             `json:"job_id,omitempty"`
	ProjectID   int64             `json:"project_id,omitempty"`
	Name        string            `json:"name"`
	Summary     string            `json:"summary,omitempty"`
	Tax         float64           `json:"tax,omitempty"`
	SoldByID    int64             `json:"sold_by_id,omitempty"`
	IsRecommend bool              `json:"is_recommended,omitempty"`
	Items       []CSVImportItem   `json:"items"`
	Errors      []string          `json:"errors,omitempty"`
	Warnings    []string          `json:"warnings,omitempty"`
	Raw         map[string]string `json:"-"`
}

// CSVImportItem is one CSV line item — translated to one ST estimate-item
// put call.
type CSVImportItem struct {
	SkuID       int64   `json:"sku_id"`
	SkuName     string  `json:"sku_name,omitempty"`
	Qty         float64 `json:"qty"`
	UnitRate    float64 `json:"unit_rate"`
	Description string  `json:"description,omitempty"`
}

// ImportCSV reads a CSV stream and returns one CSVImportRow per estimate
// row. Multiple rows in the CSV with the same `estimate_key` column collapse
// into one estimate with multiple items; an empty estimate_key is treated
// as a unique row.
//
// Required columns (header line, case-insensitive):
//
//	estimate_key  - any string; rows sharing the same key merge into one
//	                estimate (use for "multi-line quote in a sheet").
//	estimate_name - the estimate Name (defaults to "Imported quote <N>" if
//	                blank on the first row of a key).
//	sku_id        - integer; the ST pricebook SKU id (look up via
//	                servicetitan-pricebook-pp-cli search if needed).
//	qty           - decimal.
//	unit_rate     - decimal.
//
// Optional columns:
//
//	job_id, project_id, summary, tax, sold_by_id, is_recommended,
//	sku_name (for human readability in dry-run output only),
//	description (per-line-item).
//
// Rows with missing required values land with non-empty Errors; the caller
// decides whether to continue (the CLI's --strict flag).
func ImportCSV(r io.Reader) ([]CSVImportRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // tolerate ragged rows
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"sku_id", "qty", "unit_rate"}
	for _, k := range required {
		if _, ok := colIdx[k]; !ok {
			return nil, fmt.Errorf("missing required CSV column %q (header: %s)", k, strings.Join(header, ","))
		}
	}

	get := func(rec []string, key string) string {
		idx, ok := colIdx[key]
		if !ok || idx >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[idx])
	}

	byKey := map[string]*CSVImportRow{}
	var order []string
	lineNumber := 1 // header was line 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading line %d: %w", lineNumber+1, err)
		}
		lineNumber++

		key := get(rec, "estimate_key")
		if key == "" {
			key = fmt.Sprintf("__row%d", lineNumber)
		}

		row, exists := byKey[key]
		if !exists {
			row = &CSVImportRow{LineNumber: lineNumber, Raw: map[string]string{}}
			byKey[key] = row
			order = append(order, key)
			for k, idx := range colIdx {
				if idx < len(rec) {
					row.Raw[k] = rec[idx]
				}
			}
		}

		// Per-row header fields (only set on first sighting of the key).
		if row.Name == "" {
			row.Name = get(rec, "estimate_name")
		}
		if row.Summary == "" {
			row.Summary = get(rec, "summary")
		}
		if row.JobID == 0 {
			row.JobID, _ = parseInt64(get(rec, "job_id"))
		}
		if row.ProjectID == 0 {
			row.ProjectID, _ = parseInt64(get(rec, "project_id"))
		}
		if row.SoldByID == 0 {
			row.SoldByID, _ = parseInt64(get(rec, "sold_by_id"))
		}
		if row.Tax == 0 {
			row.Tax, _ = strconv.ParseFloat(get(rec, "tax"), 64)
		}
		if !row.IsRecommend {
			row.IsRecommend = parseBool(get(rec, "is_recommended"))
		}

		// Line item from this CSV line.
		item := CSVImportItem{
			SkuName:     get(rec, "sku_name"),
			Description: get(rec, "description"),
		}
		var err1, err2, err3 error
		item.SkuID, err1 = parseInt64(get(rec, "sku_id"))
		item.Qty, err2 = strconv.ParseFloat(get(rec, "qty"), 64)
		item.UnitRate, err3 = strconv.ParseFloat(get(rec, "unit_rate"), 64)
		if err1 != nil || item.SkuID <= 0 {
			row.Errors = append(row.Errors, fmt.Sprintf("line %d: sku_id required (got %q)", lineNumber, get(rec, "sku_id")))
		}
		if err2 != nil || item.Qty <= 0 {
			row.Errors = append(row.Errors, fmt.Sprintf("line %d: qty must be positive (got %q)", lineNumber, get(rec, "qty")))
		}
		if err3 != nil {
			row.Errors = append(row.Errors, fmt.Sprintf("line %d: unit_rate required (got %q)", lineNumber, get(rec, "unit_rate")))
		}
		if item.SkuID > 0 && item.Qty > 0 {
			row.Items = append(row.Items, item)
		}
	}

	out := make([]CSVImportRow, 0, len(order))
	for i, key := range order {
		row := byKey[key]
		if row.Name == "" {
			row.Name = fmt.Sprintf("Imported quote %d", i+1)
			row.Warnings = append(row.Warnings, "estimate_name was blank; defaulted to "+row.Name)
		}
		if row.JobID == 0 && row.ProjectID == 0 {
			row.Warnings = append(row.Warnings,
				"neither job_id nor project_id set — the ST API requires at least one for Estimates_Create")
		}
		if len(row.Items) == 0 {
			row.Errors = append(row.Errors, "no valid line items in row")
		}
		out = append(out, *row)
	}
	return out, nil
}

func parseInt64(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "y", "1":
		return true
	}
	return false
}

// CreateRequestPayload returns the JSON body the generated client's
// Estimates_Create wants for one CSVImportRow.
func (r CSVImportRow) CreateRequestPayload() map[string]any {
	body := map[string]any{
		"name":          r.Name,
		"summary":       r.Summary,
		"isRecommended": r.IsRecommend,
		"tax":           r.Tax,
	}
	if r.JobID > 0 {
		body["jobId"] = r.JobID
	}
	if r.ProjectID > 0 {
		body["projectId"] = r.ProjectID
	}
	if r.SoldByID > 0 {
		body["soldBy"] = r.SoldByID
	}
	items := make([]map[string]any, 0, len(r.Items))
	for _, it := range r.Items {
		items = append(items, map[string]any{
			"skuId":       it.SkuID,
			"qty":         it.Qty,
			"unitRate":    it.UnitRate,
			"description": it.Description,
		})
	}
	body["items"] = items
	return body
}
