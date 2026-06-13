// sheets push — write keyword/research data to a Google Sheets spreadsheet.
// Reads JSON from stdin (or --input file) and appends rows to a target sheet.
// Column headers are auto-derived from the first row's keys, or overridden by
// --columns (comma-separated) to enforce a specific order.
package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/api/sheets/v4"
)

func newSheetsCmd(_ *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sheets",
		Short: "Push SEMrush data to Google Sheets (push to your existing client templates)",
	}
	cmd.AddCommand(newSheetsPushCmd())
	cmd.AddCommand(newSheetsInfoCmd())
	return cmd
}

func newSheetsPushCmd() *cobra.Command {
	var (
		input        string
		sheetID      string
		sheetTab     string
		startCell    string
		columnsFlag  string
		appendMode   bool
		replaceMode  bool
		includeHead  bool
		emitURL      bool
	)
	cmd := &cobra.Command{
		Use:   "push <sheet-id>",
		Short: "Append rows from JSON (stdin or --input) to a Google Sheet tab",
		Long: "Reads JSON or CSV from --input (or stdin) and writes rows to a " +
			"Google Sheet tab.\n\n" +
			"INPUT FORMATS (auto-detected):\n" +
			"  • JSON array of objects, or wrapped envelope with " +
			"'list'/'keywords'/'items'/'data' array\n" +
			"  • CSV with header row — handles UTF-8 BOM, comma/semicolon/tab " +
			"delimiters (auto-detected), lazy-quoted fields. Perfect for raw " +
			"SEMrush UI exports (Top Keywords, etc.).\n\n" +
			"Column order auto-derives from the first row's keys, or use --columns " +
			"to enforce a specific order. Column header aliases handle most common " +
			"SEMrush CSV header names (Keyword, Position, Volume, KD, CPC, URL, etc.).\n\n" +
			"Sheet ID is the long string in the URL: " +
			"docs.google.com/spreadsheets/d/<SHEET_ID>/edit",
		Example: strings.Trim(`
  # Pipe KMT output directly to your template
  semrush-pp-cli keyword-magic "tiles" --domain nationaltiles.com.au --database au \
    | semrush-pp-cli sheets push 1abc...xyz --tab "Keywords" --columns phrase,volume,difficulty,domain_difficulty,domain_position

  # Append from a file
  semrush-pp-cli sheets push 1abc...xyz --tab "Research" --input research.json

  # Replace (overwrite from A1)
  semrush-pp-cli sheets push 1abc...xyz --tab "Results" --start A1 --no-append --input results.json
`, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			sheetID = args[0]
			rows, headers, err := loadSheetData(input, columnsFlag)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return fmt.Errorf("no rows to push (input was empty or shape wasn't an array)")
			}

			ctx := cmd.Context()
			svc, err := loadGoogleSheetsService(ctx)
			if err != nil {
				return err
			}

			rng := buildSheetRange(sheetTab, startCell)
			var written int
			switch {
			case replaceMode:
				// Header-aware clear-then-write: reads row 1 of the target tab,
				// maps row fields into template column order, clears A2:end,
				// writes data at A2. Same logic as `client onboard`.
				written, err = replaceRowsMatchingTemplate(svc, sheetID, sheetTab, rows)
			case appendMode:
				written, err = appendRowsToSheet(ctx, svc, sheetID, rng, headers, rows, includeHead)
			default:
				written, err = writeRowsToSheet(ctx, svc, sheetID, rng, headers, rows, includeHead)
			}
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d rows to %s!%s\n", written, sheetID, rng)
			if emitURL {
				fmt.Fprintf(cmd.OutOrStdout(), "https://docs.google.com/spreadsheets/d/%s/edit\n", sheetID)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "Path to JSON file (default: read from stdin)")
	cmd.Flags().StringVar(&sheetTab, "tab", "Sheet1", "Sheet tab name (e.g. 'Keywords', 'Research')")
	cmd.Flags().StringVar(&startCell, "start", "A1", "Anchor cell for append/replace (A1-style)")
	cmd.Flags().StringVar(&columnsFlag, "columns", "", "Comma-separated column order (overrides auto-detected). Use to match an existing template's column layout.")
	cmd.Flags().BoolVar(&appendMode, "append", true, "Append below existing data (default). Use --append=false to Update from --start.")
	cmd.Flags().BoolVar(&replaceMode, "replace", false, "Clear everything below row 1 of the tab, then write rows fresh (preserves headers). Use for client deliverables.")
	cmd.Flags().BoolVar(&includeHead, "header", true, "Include a header row with column names")
	cmd.Flags().BoolVar(&emitURL, "url", true, "Print the Google Sheets URL after writing")
	return cmd
}

func newSheetsInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <sheet-id>",
		Short: "Show sheet metadata (title, tabs, dimensions) — useful for finding tab names",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadGoogleSheetsService(cmd.Context())
			if err != nil {
				return err
			}
			ss, err := svc.Spreadsheets.Get(args[0]).Do()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\n", ss.Properties.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "Locale: %s, Timezone: %s\n", ss.Properties.Locale, ss.Properties.TimeZone)
			fmt.Fprintln(cmd.OutOrStdout(), "Tabs:")
			for _, s := range ss.Sheets {
				p := s.Properties
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  (id=%d, rows=%d, cols=%d)\n",
					p.Title, p.SheetId, p.GridProperties.RowCount, p.GridProperties.ColumnCount)
			}
			return nil
		},
	}
	return cmd
}

// loadSheetData reads the input and normalizes it into:
//   - rows: a slice of map[string]any (one per record)
//   - headers: the ordered column names to write
//
// Auto-detects JSON vs CSV by sniffing the first non-whitespace byte. CSV
// inputs handle UTF-8 BOM, comma/semicolon/tab delimiters (auto-detected from
// the first line), and lazy-quoted fields. Header row of the CSV becomes the
// keys of each row map.
//
// If columnsFlag is non-empty, those become the header order (case-insensitive
// match against record keys). Otherwise, derive headers from the union of keys
// in the first record (sorted for determinism).
func loadSheetData(inputPath, columnsFlag string) ([]map[string]any, []string, error) {
	var data []byte
	var err error
	if inputPath != "" {
		data, err = os.ReadFile(inputPath)
		if err != nil {
			return nil, nil, err
		}
	} else {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, nil, err
		}
	}
	// Detect CSV vs JSON by sniffing first non-whitespace byte. JSON starts
	// with '[' or '{'; CSV with a printable header character.
	if isCSVInput(data) {
		return parseCSVInput(data, columnsFlag)
	}
	data = stripJSONEnvelope(data)
	// Try array first
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err == nil {
		return rows, deriveHeaders(rows, columnsFlag), nil
	}
	// Try wrapped object that contains an array under known keys
	var wrapper map[string]any
	if err := json.Unmarshal(data, &wrapper); err == nil {
		for _, key := range []string{"list", "keywords", "items", "data", "results"} {
			if v, ok := wrapper[key]; ok {
				if arr, ok := v.([]any); ok {
					rows = make([]map[string]any, 0, len(arr))
					for _, item := range arr {
						if m, ok := item.(map[string]any); ok {
							rows = append(rows, m)
						}
					}
					if len(rows) > 0 {
						return rows, deriveHeaders(rows, columnsFlag), nil
					}
				}
				// Could be a nested map
				if m, ok := v.(map[string]any); ok {
					for _, k2 := range []string{"list", "keywords", "items"} {
						if v2, ok := m[k2]; ok {
							if arr2, ok := v2.([]any); ok {
								for _, item := range arr2 {
									if mm, ok := item.(map[string]any); ok {
										rows = append(rows, mm)
									}
								}
								if len(rows) > 0 {
									return rows, deriveHeaders(rows, columnsFlag), nil
								}
							}
						}
					}
				}
			}
		}
	}
	return nil, nil, fmt.Errorf("input JSON was not an array of objects nor a known envelope (list/keywords/items/data/results)")
}

// isCSVInput sniffs the first non-whitespace, non-BOM byte. Returns true for
// likely-CSV (alphanumeric or quote — typical header row), false for JSON
// (starts with [ or {) or empty input.
func isCSVInput(data []byte) bool {
	// Strip UTF-8 BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	i := 0
	for i < len(data) && (data[i] == ' ' || data[i] == '\t' || data[i] == '\n' || data[i] == '\r') {
		i++
	}
	if i >= len(data) {
		return false
	}
	first := data[i]
	// JSON markers
	if first == '[' || first == '{' {
		return false
	}
	// Likely CSV header characters
	return (first >= 'A' && first <= 'Z') || (first >= 'a' && first <= 'z') || first == '"' || first == '\''
}

// parseCSVInput parses a CSV body into row maps and a header order. Detects
// comma/semicolon/tab delimiter from the first line (whichever has the most
// occurrences). Strips UTF-8 BOM. Tolerates ragged rows (missing trailing
// fields). Each row's keys are the CSV's header strings (trimmed).
func parseCSVInput(data []byte, columnsFlag string) ([]map[string]any, []string, error) {
	// Strip UTF-8 BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	// Detect delimiter from first non-empty line
	firstLine := data
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		firstLine = data[:idx]
	}
	commaN := bytes.Count(firstLine, []byte(","))
	semiN := bytes.Count(firstLine, []byte(";"))
	tabN := bytes.Count(firstLine, []byte("\t"))
	delim := ','
	switch {
	case semiN > commaN && semiN >= tabN:
		delim = ';'
	case tabN > commaN && tabN > semiN:
		delim = '\t'
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1 // allow ragged rows
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("parsing CSV: %w", err)
	}
	if len(records) == 0 {
		return nil, nil, fmt.Errorf("CSV input was empty")
	}
	if len(records) == 1 {
		return nil, nil, fmt.Errorf("CSV input has only a header row, no data")
	}

	headers := make([]string, len(records[0]))
	for i, h := range records[0] {
		headers[i] = strings.TrimSpace(h)
	}

	rows := make([]map[string]any, 0, len(records)-1)
	for _, rec := range records[1:] {
		row := make(map[string]any, len(headers))
		for i, h := range headers {
			if i < len(rec) {
				row[h] = strings.TrimSpace(rec[i])
			}
		}
		rows = append(rows, row)
	}
	return rows, deriveHeaders(rows, columnsFlag), nil
}

// stripJSONEnvelope removes the {"meta":..., "results":<x>} wrapper that the
// SEMrush CSV-converted path adds, when present, returning just the inner value.
func stripJSONEnvelope(data []byte) []byte {
	var env struct {
		Meta    map[string]any  `json:"meta"`
		Results json.RawMessage `json:"results"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return data
	}
	if env.Meta != nil && len(env.Results) > 0 {
		return env.Results
	}
	return data
}

func deriveHeaders(rows []map[string]any, columnsFlag string) []string {
	if columnsFlag != "" {
		parts := strings.Split(columnsFlag, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if h := strings.TrimSpace(p); h != "" {
				out = append(out, h)
			}
		}
		return out
	}
	// Union of keys across all rows, sorted for determinism. Prefer common
	// SEMrush keyword-data fields in a natural order if present.
	preferred := []string{
		"phrase", "Keyword", "keyword",
		"volume", "Search Volume",
		"cpc", "CPC",
		"difficulty", "Keyword Difficulty Index",
		"domain_difficulty", "domain_position", "domain_traffic", "domain_relevance",
		"competition_level", "Competition", "intents", "results", "Number of Results",
	}
	seen := map[string]bool{}
	for _, r := range rows {
		for k := range r {
			seen[k] = true
		}
	}
	var ordered []string
	for _, p := range preferred {
		if seen[p] {
			ordered = append(ordered, p)
			delete(seen, p)
		}
	}
	remaining := make([]string, 0, len(seen))
	for k := range seen {
		remaining = append(remaining, k)
	}
	sort.Strings(remaining)
	return append(ordered, remaining...)
}

func buildSheetRange(tab, start string) string {
	if tab == "" {
		tab = "Sheet1"
	}
	// Quote tab if it has spaces/specials
	if strings.ContainsAny(tab, " '!\"$%&()") {
		tab = "'" + strings.ReplaceAll(tab, "'", "''") + "'"
	}
	return tab + "!" + start
}

func appendRowsToSheet(ctx interface{}, svc *sheets.Service, sheetID, rng string, headers []string, rows []map[string]any, includeHead bool) (int, error) {
	body := buildValueRange(headers, rows, includeHead)
	call := svc.Spreadsheets.Values.Append(sheetID, rng, body).
		ValueInputOption("USER_ENTERED").
		InsertDataOption("INSERT_ROWS")
	_, err := call.Do()
	if err != nil {
		return 0, fmt.Errorf("sheets append: %w", err)
	}
	written := len(body.Values)
	return written, nil
}

func writeRowsToSheet(ctx interface{}, svc *sheets.Service, sheetID, rng string, headers []string, rows []map[string]any, includeHead bool) (int, error) {
	body := buildValueRange(headers, rows, includeHead)
	call := svc.Spreadsheets.Values.Update(sheetID, rng, body).ValueInputOption("USER_ENTERED")
	_, err := call.Do()
	if err != nil {
		return 0, fmt.Errorf("sheets update: %w", err)
	}
	return len(body.Values), nil
}

// replaceRowsBelowHeaderInSheet clears every row below row 1 of `tab`, then
// writes `rows` starting at A2. Used by `client onboard` so a freshly-cloned
// template becomes a clean client deliverable instead of carrying the template's
// example data below the new rows.
//
// The header row (row 1) is preserved as-is — `rows` must be in the same
// column order as the template's row-1 headers (`headers` parameter is used
// only for field→column mapping).
func replaceRowsBelowHeaderInSheet(ctx interface{}, svc *sheets.Service, sheetID, tab string, headers []string, rows []map[string]any) (int, error) {
	// Quote tab name if it contains special chars
	quotedTab := tab
	if strings.ContainsAny(tab, " '!\"$%&()") {
		quotedTab = "'" + strings.ReplaceAll(tab, "'", "''") + "'"
	}
	// Clear everything below row 1. ZZ:open-row notation clears columns A
	// through ZZ across every row from 2 onward — covers any template width.
	clearRange := quotedTab + "!A2:ZZ"
	if _, err := svc.Spreadsheets.Values.Clear(sheetID, clearRange, &sheets.ClearValuesRequest{}).Do(); err != nil {
		return 0, fmt.Errorf("clearing %s: %w", clearRange, err)
	}
	// Now write the new rows starting at A2, no header row.
	writeRange := quotedTab + "!A2"
	body := buildValueRange(headers, rows, false)
	if _, err := svc.Spreadsheets.Values.Update(sheetID, writeRange, body).ValueInputOption("USER_ENTERED").Do(); err != nil {
		return 0, fmt.Errorf("writing to %s: %w", writeRange, err)
	}
	return len(body.Values), nil
}

// templateHeaderAliases maps a normalized template header name (lowercase,
// trimmed) to a prioritized list of data-field keys. Used by header-aware
// sheet writing — the template's row 1 defines the column ORDER, and the
// CLI maps its data fields into those columns by name. Headers not in this
// map are left blank in the output (e.g. manual-curation columns like
// "Topic", "Category", "Subcategory", "Mapping").
var templateHeaderAliases = map[string][]string{
	"keyword":           {"phrase", "Keyword", "keyword"},
	"phrase":            {"phrase", "Phrase"},
	"volume":            {"volume", "Volume", "Search Volume"},
	"search volume":     {"volume", "Search Volume"},
	"kd":                {"difficulty", "KD", "Keyword Difficulty"},
	"difficulty":        {"difficulty", "Difficulty"},
	"keyword difficulty": {"difficulty", "Keyword Difficulty"},
	// PKD intentionally NOT aliased — the user opted to drop PKD from
	// deliverables. If the template still has a PKD column, the
	// header-aware mapper leaves it blank, and the user can delete the
	// column from the master template at their leisure.
	//   "pkd":               {"domain_difficulty", "PKD"},
	//   "personal kd":       {"domain_difficulty", "Personal KD"},
	//   "personal keyword difficulty": {"domain_difficulty"},
	// Position / Traffic prefer the *_display fields so the not-ranking
	// sentinel (255) shows as "100+" and the corresponding traffic cell
	// blanks out. Falls through to the raw values if a caller hasn't run
	// deriveDisplayFields (e.g. direct sheets push of foreign data).
	"position":          {"_position_display", "domain_position", "Position"},
	"current position":  {"_position_display", "domain_position", "Position"},
	"est traffic":       {"_traffic_display", "domain_traffic", "Est Traffic", "Traffic", "Estimated clicks"},
	"estimated traffic": {"_traffic_display", "domain_traffic", "Estimated Traffic", "Traffic"},
	"traffic":           {"_traffic_display", "domain_traffic", "Traffic"},
	// SEMrush UI exports their "Traffic" column variously as "Estimated clicks"
	// in newer exports and "Traffic" in older. Both should land here.
	"estimated clicks":  {"domain_traffic", "Traffic", "_traffic_display", "Estimated clicks"},
	"est clicks":        {"domain_traffic", "Traffic", "Estimated clicks"},
	"clicks":            {"domain_traffic", "Traffic", "Clicks"},
	"cpc":               {"cpc", "CPC"},
	"type":              {"_type", "Type", "Intent"},
	"intent":            {"_type", "Intent"},
	"seed":              {"_seed", "Seed"},
	"mode":              {"_mode", "Mode"},
	"competition":       {"competition_level", "Competition"},
	"number of results": {"results", "Number of Results"},
	"results":           {"results", "Results"},
	// Trend prefers the summary string ("Growing" / "Declining" / etc.) over
	// the raw 12-float array, since the array isn't useful in a sheet cell.
	"trends":            {"_trend_summary", "Trends"},
	"trend":             {"_trend_summary", "Trend"},
	"trend summary":     {"_trend_summary"},
	"trending":          {"_trend_summary"},
	"url":               {"URL", "Url", "url", "Ranking URL", "Landing Page"},
	"ranking url":       {"Ranking URL", "URL"},
	"landing page":      {"Landing Page", "URL"},
	"relevance":         {"domain_relevance", "Relevance"},
	// SEMrush UI CSV exports have these column names verbatim. Aliasing them
	// so a CSV pushed via `sheets push --input client-keywords.csv` lands in
	// the right columns of any template that uses our standard headers.
	"previous position":     {"Previous Position", "Pp"},
	"position difference":   {"Position Difference", "Pd"},
	"traffic (%)":           {"Traffic (%)", "Traffic", "domain_traffic"},
	"traffic %":             {"Traffic (%)", "Traffic"},
	"costs":                 {"Costs", "Traffic Cost"},
	"timestamp":             {"Timestamp"},
	"serp features by keyword": {"SERP Features by Keyword"},
	"keyword intents":       {"Keyword Intents", "_type", "Type"},
}

// readTemplateHeaders reads row 1 of `tab` and returns the header strings in
// column order. Returns empty slice if row 1 is empty.
func readTemplateHeaders(svc *sheets.Service, sheetID, tab string) ([]string, error) {
	quotedTab := tab
	if strings.ContainsAny(tab, " '!\"$%&()") {
		quotedTab = "'" + strings.ReplaceAll(tab, "'", "''") + "'"
	}
	rng := quotedTab + "!1:1"
	resp, err := svc.Spreadsheets.Values.Get(sheetID, rng).Do()
	if err != nil {
		return nil, err
	}
	if len(resp.Values) == 0 {
		return nil, nil
	}
	row := resp.Values[0]
	out := make([]string, len(row))
	for i, v := range row {
		out[i] = fmt.Sprintf("%v", v)
	}
	return out, nil
}

// replaceRowsMatchingTemplate is the header-aware variant of
// replaceRowsBelowHeaderInSheet. Reads row 1 of `tab`, maps each data row's
// fields into the template's column order via templateHeaderAliases, then
// clears below row 1 and writes the mapped rows at A2. Unknown headers
// (e.g. "Topic", "Category") are left blank in each row.
func replaceRowsMatchingTemplate(svc *sheets.Service, sheetID, tab string, rows []map[string]any) (int, error) {
	headers, err := readTemplateHeaders(svc, sheetID, tab)
	if err != nil {
		return 0, fmt.Errorf("reading template headers from %s: %w", tab, err)
	}
	if len(headers) == 0 {
		return 0, fmt.Errorf("tab %q has no header row (row 1 is empty)", tab)
	}

	// Build [][]any in template column order
	values := make([][]any, 0, len(rows))
	for _, r := range rows {
		out := make([]any, len(headers))
		for i, h := range headers {
			norm := strings.ToLower(strings.TrimSpace(h))
			aliases, ok := templateHeaderAliases[norm]
			if !ok {
				out[i] = "" // unmapped header — leave blank
				continue
			}
			var v any
			for _, a := range aliases {
				if val, present := r[a]; present {
					v = val
					break
				}
			}
			out[i] = cellValue(v)
		}
		values = append(values, out)
	}

	quotedTab := tab
	if strings.ContainsAny(tab, " '!\"$%&()") {
		quotedTab = "'" + strings.ReplaceAll(tab, "'", "''") + "'"
	}
	clearRange := quotedTab + "!A2:ZZ"
	if _, err := svc.Spreadsheets.Values.Clear(sheetID, clearRange, &sheets.ClearValuesRequest{}).Do(); err != nil {
		return 0, fmt.Errorf("clearing %s: %w", clearRange, err)
	}
	writeRange := quotedTab + "!A2"
	body := &sheets.ValueRange{Values: values}
	if _, err := svc.Spreadsheets.Values.Update(sheetID, writeRange, body).ValueInputOption("USER_ENTERED").Do(); err != nil {
		return 0, fmt.Errorf("writing to %s: %w", writeRange, err)
	}
	return len(values), nil
}

func buildValueRange(headers []string, rows []map[string]any, includeHead bool) *sheets.ValueRange {
	values := make([][]any, 0, len(rows)+1)
	if includeHead {
		headerRow := make([]any, len(headers))
		for i, h := range headers {
			headerRow[i] = h
		}
		values = append(values, headerRow)
	}
	for _, r := range rows {
		row := make([]any, len(headers))
		for i, h := range headers {
			v := lookupKeyCaseInsensitive(r, h)
			row[i] = cellValue(v)
		}
		values = append(values, row)
	}
	return &sheets.ValueRange{Values: values}
}

func lookupKeyCaseInsensitive(m map[string]any, key string) any {
	if v, ok := m[key]; ok {
		return v
	}
	lower := strings.ToLower(key)
	for k, v := range m {
		if strings.ToLower(k) == lower {
			return v
		}
	}
	return nil
}

// cellValue converts JSON values to forms Sheets understands. Slices/maps
// become compact JSON strings to keep one cell per record.
func cellValue(v any) any {
	switch t := v.(type) {
	case nil:
		return ""
	case string, float64, int, bool:
		return t
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}
