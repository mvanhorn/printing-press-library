package adsanalytics

import (
	"bytes"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

//go:embed report_schemas.toml
var reportSchemaFS embed.FS

type ReportSchema struct {
	Kind                string              `toml:"kind" json:"kind"`
	DisplayName         string              `toml:"display_name" json:"display_name"`
	AdProduct           string              `toml:"ad_product" json:"ad_product"`
	EntityLevel         string              `toml:"entity_level" json:"entity_level"`
	TimeUnit            string              `toml:"time_unit" json:"time_unit"`
	AttributionWindow   string              `toml:"attribution_window" json:"attribution_window"`
	RequiredColumns     []string            `toml:"required_columns" json:"required_columns"`
	ApplyCapableColumns []string            `toml:"apply_capable_columns" json:"apply_capable_columns"`
	ColumnAliases       map[string][]string `toml:"column_aliases" json:"column_aliases"`
	Units               map[string]string   `toml:"units" json:"units,omitempty"`
	Provenance          []string            `toml:"provenance" json:"provenance"`
	SampleHeader        string              `toml:"sample_header" json:"sample_header"`
	ExportPath          string              `toml:"export_path" json:"export_path"`
}

type ReportRecipe struct {
	Command       string   `toml:"command" json:"command"`
	AcceptedKinds []string `toml:"accepted_kinds" json:"accepted_kinds"`
	Description   string   `toml:"description" json:"description"`
}

type ReportRegistry struct {
	Schemas []ReportSchema `toml:"schemas" json:"schemas"`
	Recipes []ReportRecipe `toml:"recipes" json:"recipes"`
}

type ReportCandidate struct {
	Kind         string   `json:"kind"`
	DisplayName  string   `json:"display_name"`
	Confidence   float64  `json:"confidence"`
	Matched      []string `json:"matched"`
	Missing      []string `json:"missing"`
	EntityLevel  string   `json:"entity_level"`
	TimeUnit     string   `json:"time_unit"`
	AdProduct    string   `json:"ad_product"`
	ExportPath   string   `json:"export_path,omitempty"`
	SampleHeader string   `json:"sample_header,omitempty"`
}

type CanonicalRecord map[string]string

type SchemaValidation struct {
	Schema     ReportSchema      `json:"schema"`
	Candidates []ReportCandidate `json:"candidates,omitempty"`
	Missing    []string          `json:"missing,omitempty"`
	Partial    bool              `json:"partial,omitempty"`
	Warnings   []string          `json:"warnings,omitempty"`
}

type NormalizedSchemaReport struct {
	Kind       string            `json:"kind"`
	SourcePath string            `json:"source_path"`
	Rows       []CanonicalRecord `json:"rows"`
	Validation SchemaValidation  `json:"validation"`
}

func LoadReportRegistry() (ReportRegistry, error) {
	data, err := reportSchemaFS.ReadFile("report_schemas.toml")
	if err != nil {
		return ReportRegistry{}, err
	}
	var registry ReportRegistry
	if err := toml.Unmarshal(data, &registry); err != nil {
		return ReportRegistry{}, fmt.Errorf("parsing report schema registry: %w", err)
	}
	return registry, nil
}

func MustReportRegistry() ReportRegistry {
	registry, err := LoadReportRegistry()
	if err != nil {
		panic(err)
	}
	return registry
}

func (r ReportRegistry) Schema(kind string) (ReportSchema, bool) {
	kind = normalizeKind(kind)
	for _, schema := range r.Schemas {
		if normalizeKind(schema.Kind) == kind {
			return schema, true
		}
	}
	return ReportSchema{}, false
}

func (r ReportRegistry) Recipe(command string) (ReportRecipe, bool) {
	command = strings.TrimSpace(command)
	for _, recipe := range r.Recipes {
		if recipe.Command == command {
			return recipe, true
		}
	}
	return ReportRecipe{}, false
}

func DetectReportKind(path string, acceptedKinds []string) ([]ReportCandidate, error) {
	headers, _, err := readReportHeaderAndRows(path)
	if err != nil {
		return nil, err
	}
	registry, err := LoadReportRegistry()
	if err != nil {
		return nil, err
	}
	accepted := map[string]bool{}
	for _, kind := range acceptedKinds {
		accepted[normalizeKind(kind)] = true
	}
	var candidates []ReportCandidate
	for _, schema := range registry.Schemas {
		if len(accepted) > 0 && !accepted[normalizeKind(schema.Kind)] {
			continue
		}
		candidate := scoreSchemaCandidate(schema, headers)
		if candidate.Confidence > 0 {
			candidates = append(candidates, candidate)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence == candidates[j].Confidence {
			return candidates[i].Kind < candidates[j].Kind
		}
		return candidates[i].Confidence > candidates[j].Confidence
	})
	return candidates, nil
}

func NormalizeSchemaReport(path, kind string, acceptedKinds []string, allowPartial bool) (NormalizedSchemaReport, error) {
	registry, err := LoadReportRegistry()
	if err != nil {
		return NormalizedSchemaReport{}, err
	}
	headers, rawRows, err := readReportHeaderAndRows(path)
	if err != nil {
		return NormalizedSchemaReport{}, err
	}
	var schema ReportSchema
	var candidates []ReportCandidate
	if strings.TrimSpace(kind) != "" {
		var ok bool
		schema, ok = registry.Schema(kind)
		if !ok {
			return NormalizedSchemaReport{}, fmt.Errorf("unsupported --report-kind %q", kind)
		}
		candidates = []ReportCandidate{scoreSchemaCandidate(schema, headers)}
	} else {
		candidates, err = DetectReportKind(path, acceptedKinds)
		if err != nil {
			return NormalizedSchemaReport{}, err
		}
		if len(candidates) == 0 {
			return NormalizedSchemaReport{}, fmt.Errorf("could not detect report kind from %s; run reports recipe <command> and pass --report-kind", path)
		}
		if len(candidates) > 1 && candidates[0].Confidence < 0.90 && candidates[1].Confidence >= candidates[0].Confidence-0.10 {
			return NormalizedSchemaReport{}, fmt.Errorf("ambiguous report kind for %s: pass --report-kind (candidates: %s)", path, formatCandidateKinds(candidates))
		}
		var ok bool
		schema, ok = registry.Schema(candidates[0].Kind)
		if !ok {
			return NormalizedSchemaReport{}, fmt.Errorf("detected unsupported report kind %q", candidates[0].Kind)
		}
	}
	validation := SchemaValidation{
		Schema:     schema,
		Candidates: candidates,
	}
	missing := missingRequired(schema, headers)
	if len(missing) > 0 {
		validation.Missing = missing
		if !allowPartial {
			return NormalizedSchemaReport{}, fmt.Errorf("report %s is missing required columns %v for %s; run: reports recipe <command>", path, missing, schema.Kind)
		}
		validation.Partial = true
		validation.Warnings = append(validation.Warnings, fmt.Sprintf("allowing partial %s report with missing columns %v", schema.Kind, missing))
	}
	rows := make([]CanonicalRecord, 0, len(rawRows))
	for _, raw := range rawRows {
		rows = append(rows, normalizeRecord(schema, raw))
	}
	return NormalizedSchemaReport{
		Kind:       schema.Kind,
		SourcePath: path,
		Rows:       rows,
		Validation: validation,
	}, nil
}

func PerformanceRowsFromCanonical(records []CanonicalRecord) []PerformanceRow {
	rows := make([]PerformanceRow, 0, len(records))
	for _, record := range records {
		rows = append(rows, PerformanceRow{
			CampaignID:  record["campaignId"],
			Campaign:    record["campaignName"],
			AdGroup:     record["adGroupName"],
			ASIN:        firstNonEmpty(record["advertisedAsin"], record["asin"]),
			SKU:         record["advertisedSku"],
			Date:        record["date"],
			Spend:       parseNumber(record["cost"]),
			Sales:       parseNumber(record["sales"]),
			Orders:      int(parseNumber(record["orders"])),
			Clicks:      int(parseNumber(record["clicks"])),
			Impressions: int(parseNumber(record["impressions"])),
			Budget:      parseNumber(record["dailyBudget"]),
		})
	}
	return rows
}

func SearchTermRowsFromCanonical(records []CanonicalRecord) []SearchTermPerformance {
	rows := make([]SearchTermPerformance, 0, len(records))
	for _, record := range records {
		row := SearchTermPerformance{
			CampaignID:  record["campaignId"],
			Campaign:    record["campaignName"],
			AdGroupID:   record["adGroupId"],
			AdGroup:     record["adGroupName"],
			SearchTerm:  record["searchTerm"],
			Keyword:     record["keyword"],
			Spend:       parseNumber(record["cost"]),
			Sales:       parseNumber(record["sales"]),
			Conversions: int(parseNumber(record["orders"])),
			Clicks:      int(parseNumber(record["clicks"])),
			Impressions: int(parseNumber(record["impressions"])),
		}
		if row.SearchTerm != "" {
			rows = append(rows, row)
		}
	}
	return rows
}

func KeywordRowsFromCanonical(records []CanonicalRecord) []KeywordPerformance {
	rows := make([]KeywordPerformance, 0, len(records))
	for _, record := range records {
		row := KeywordPerformance{
			KeywordID:  record["keywordId"],
			CampaignID: record["campaignId"],
			Campaign:   record["campaignName"],
			AdGroupID:  record["adGroupId"],
			AdGroup:    record["adGroupName"],
			Keyword:    record["keyword"],
			MatchType:  record["matchType"],
			Date:       record["date"],
			Bid:        parseNumber(record["bid"]),
			CPC:        parseNumber(record["cpc"]),
			Spend:      parseNumber(record["cost"]),
			Sales:      parseNumber(record["sales"]),
			Orders:     int(parseNumber(record["orders"])),
			Clicks:     int(parseNumber(record["clicks"])),
		}
		if row.Keyword != "" {
			rows = append(rows, row)
		}
	}
	normalizeKeywordRows(rows)
	return rows
}

func readReportHeaderAndRows(path string) ([]string, []map[string]string, error) {
	data, err := ReadReportFile(path)
	if err != nil {
		return nil, nil, err
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil, fmt.Errorf("report %s is empty", path)
	}
	switch trimmed[0] {
	case '[':
		var rows []map[string]any
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		return headersAndStringRows(rows), stringifyRows(rows), nil
	case '{':
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &envelope); err != nil {
			return nil, nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		for _, key := range []string{"rows", "data", "items", "records", "results", "campaigns", "keywords", "search_terms", "products"} {
			if raw := envelope[key]; len(raw) > 0 {
				var rows []map[string]any
				if err := json.Unmarshal(raw, &rows); err == nil {
					return headersAndStringRows(rows), stringifyRows(rows), nil
				}
			}
		}
		var row map[string]any
		if err := json.Unmarshal(trimmed, &row); err != nil {
			return nil, nil, fmt.Errorf("parsing JSON report %s: %w", path, err)
		}
		rows := []map[string]any{row}
		return headersAndStringRows(rows), stringifyRows(rows), nil
	default:
		return readDelimitedRows(trimmed)
	}
}

func readDelimitedRows(data []byte) ([]string, []map[string]string, error) {
	firstLineEnd := bytes.IndexByte(data, '\n')
	if firstLineEnd < 0 {
		firstLineEnd = len(data)
	}
	firstLine := string(data[:firstLineEnd])
	delimiter := ','
	if strings.Count(firstLine, "\t") > strings.Count(firstLine, ",") {
		delimiter = '\t'
	}
	cr := csv.NewReader(bytes.NewReader(data))
	cr.Comma = delimiter
	cr.FieldsPerRecord = -1
	records, err := cr.ReadAll()
	if err != nil {
		return nil, nil, err
	}
	if len(records) < 2 {
		return nil, nil, fmt.Errorf("delimited report must include a header and at least one row")
	}
	headers := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, record := range records[1:] {
		row := map[string]string{}
		for i, header := range headers {
			if i < len(record) {
				row[header] = strings.TrimSpace(record[i])
			}
		}
		rows = append(rows, row)
	}
	return headers, rows, nil
}

func headersAndStringRows(rows []map[string]any) []string {
	seen := map[string]bool{}
	var headers []string
	for _, row := range rows {
		for key := range row {
			if !seen[key] {
				seen[key] = true
				headers = append(headers, key)
			}
		}
	}
	sort.Strings(headers)
	return headers
}

func stringifyRows(rows []map[string]any) []map[string]string {
	out := make([]map[string]string, 0, len(rows))
	for _, row := range rows {
		item := map[string]string{}
		for key, value := range row {
			switch v := value.(type) {
			case string:
				item[key] = v
			case float64, bool, json.Number:
				item[key] = fmt.Sprint(v)
			case nil:
				item[key] = ""
			default:
				data, _ := json.Marshal(v)
				item[key] = string(data)
			}
		}
		out = append(out, item)
	}
	return out
}

func scoreSchemaCandidate(schema ReportSchema, headers []string) ReportCandidate {
	headerSet := map[string]bool{}
	for _, header := range headers {
		headerSet[normalizeHeader(header)] = true
	}
	matched, missing := matchedMissing(schema, headerSet)
	required := len(schema.RequiredColumns)
	confidence := 0.0
	if required > 0 {
		confidence = float64(len(matched)) / float64(required)
	}
	return ReportCandidate{
		Kind:         schema.Kind,
		DisplayName:  schema.DisplayName,
		Confidence:   confidence,
		Matched:      matched,
		Missing:      missing,
		EntityLevel:  schema.EntityLevel,
		TimeUnit:     schema.TimeUnit,
		AdProduct:    schema.AdProduct,
		ExportPath:   schema.ExportPath,
		SampleHeader: schema.SampleHeader,
	}
}

func matchedMissing(schema ReportSchema, headerSet map[string]bool) ([]string, []string) {
	var matched []string
	var missing []string
	for _, required := range schema.RequiredColumns {
		if schemaColumnPresent(schema, required, headerSet) {
			matched = append(matched, required)
		} else {
			missing = append(missing, required)
		}
	}
	return matched, missing
}

func missingRequired(schema ReportSchema, headers []string) []string {
	headerSet := map[string]bool{}
	for _, header := range headers {
		headerSet[normalizeHeader(header)] = true
	}
	_, missing := matchedMissing(schema, headerSet)
	return missing
}

func schemaColumnPresent(schema ReportSchema, canonical string, headerSet map[string]bool) bool {
	if headerSet[normalizeHeader(canonical)] {
		return true
	}
	for _, alias := range schema.ColumnAliases[canonical] {
		if headerSet[normalizeHeader(alias)] {
			return true
		}
	}
	return false
}

func normalizeRecord(schema ReportSchema, raw map[string]string) CanonicalRecord {
	out := CanonicalRecord{}
	rawByNorm := map[string]string{}
	for key, value := range raw {
		rawByNorm[normalizeHeader(key)] = value
	}
	for canonical := range schema.ColumnAliases {
		out[canonical] = firstAliasValue(canonical, schema.ColumnAliases[canonical], rawByNorm)
	}
	for _, required := range schema.RequiredColumns {
		if _, ok := out[required]; !ok {
			out[required] = firstAliasValue(required, schema.ColumnAliases[required], rawByNorm)
		}
	}
	return out
}

func firstAliasValue(canonical string, aliases []string, raw map[string]string) string {
	names := append([]string{canonical}, aliases...)
	for _, name := range names {
		if value, ok := raw[normalizeHeader(name)]; ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func formatCandidateKinds(candidates []ReportCandidate) string {
	limit := len(candidates)
	if limit > 4 {
		limit = 4
	}
	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s %.2f", candidates[i].Kind, candidates[i].Confidence))
	}
	return strings.Join(parts, ", ")
}

func normalizeKind(kind string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(kind)), "_", "-")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func EncodeJSONRows(w io.Writer, rows []CanonicalRecord) error {
	enc := json.NewEncoder(w)
	return enc.Encode(rows)
}
