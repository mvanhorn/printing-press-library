package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/restaurant365-odata/internal/client"
	"github.com/spf13/cobra"
)

type r365View struct {
	Name        string `json:"name"`
	Resource    string `json:"resource"`
	DateField   string `json:"date_field,omitempty"`
	RowVersion  bool   `json:"row_version"`
	Description string `json:"description,omitempty"`
}

type r365Field struct {
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`
	Nullable string `json:"nullable,omitempty"`
}

type r365ViewSummary struct {
	Name        string `json:"name"`
	Resource    string `json:"resource"`
	FieldCount  int    `json:"field_count"`
	DateField   string `json:"date_field,omitempty"`
	RowVersion  bool   `json:"row_version"`
	SyncPattern string `json:"sync_pattern"`
}

type r365SampleSummary struct {
	View           string           `json:"view"`
	Requested      int              `json:"requested"`
	Returned       int              `json:"returned"`
	Columns        []string         `json:"columns"`
	ValuesRedacted bool             `json:"values_redacted"`
	Rows           []map[string]any `json:"rows,omitempty"`
}

type r365BackfillPlan struct {
	View        string             `json:"view"`
	Strategy    string             `json:"strategy"`
	DateField   string             `json:"date_field,omitempty"`
	ChunkDays   int                `json:"chunk_days,omitempty"`
	Watermark   string             `json:"watermark,omitempty"`
	LocationID  string             `json:"location_id,omitempty"`
	Requests    int                `json:"requests"`
	Chunks      []r365BackfillStep `json:"chunks"`
	Recommended string             `json:"recommended_next_command,omitempty"`
}

type r365BackfillStep struct {
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Filter string `json:"filter"`
	Reason string `json:"reason"`
}

var r365Views = []r365View{
	{Name: "Company", Resource: "company", Description: "Company dimension rows"},
	{Name: "Employee", Resource: "employee", Description: "Employee reporting rows"},
	{Name: "EntityDeleted", Resource: "entity-deleted", RowVersion: true, Description: "Deletion tombstones"},
	{Name: "GLAccount", Resource: "glaccount", Description: "General ledger account rows"},
	{Name: "Item", Resource: "item", Description: "Item dimension rows"},
	{Name: "JobTitle", Resource: "job-title", Description: "Job title rows"},
	{Name: "LaborDetail", Resource: "labor-detail", DateField: "dateWorked", Description: "Labor detail rows"},
	{Name: "Location", Resource: "location", Description: "Location dimension rows"},
	{Name: "PayrollSummary", Resource: "payroll-summary", DateField: "payrollStart", Description: "Payroll summary rows"},
	{Name: "POSEmployee", Resource: "posemployee", Description: "POS employee rows"},
	{Name: "SalesDetail", Resource: "sales-detail", DateField: "date", Description: "Sales detail rows"},
	{Name: "SalesEmployee", Resource: "sales-employee", DateField: "date", Description: "Sales employee rows"},
	{Name: "SalesPayment", Resource: "sales-payment", DateField: "date", Description: "Sales payment rows"},
	{Name: "Transaction", Resource: "transaction", DateField: "date", RowVersion: true, Description: "Transaction header rows"},
	{Name: "TransactionDetail", Resource: "transaction-detail", RowVersion: true, Description: "Transaction detail rows"},
}

func newR365ListViewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list-views",
		Short:   "List documented Restaurant365 OData views",
		Example: "  restaurant365-odata-pp-cli list-views --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			fields := map[string][]r365Field{}
			if c, err := flags.newClient(); err == nil {
				if parsed, err := fetchR365Metadata(cmd.Context(), c); err == nil {
					fields = parsed
				}
			}

			out := make([]r365ViewSummary, 0, len(r365Views))
			for _, view := range r365Views {
				viewFields := r365FieldsForView(fields, view.Name)
				out = append(out, r365ViewSummary{
					Name:        view.Name,
					Resource:    view.Resource,
					FieldCount:  len(viewFields),
					DateField:   view.DateField,
					RowVersion:  view.RowVersion,
					SyncPattern: r365SyncPattern(view),
				})
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"views": out})
			}
			rows := make([][]string, 0, len(out))
			for _, view := range out {
				rows = append(rows, []string{view.Name, view.Resource, strconv.Itoa(view.FieldCount), view.SyncPattern})
			}
			return flags.printTable(cmd, []string{"VIEW", "RESOURCE", "FIELDS", "SYNC"}, rows)
		},
	}
	return cmd
}

func newR365DescribeViewCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "describe-view <view>",
		Short:   "Describe a Restaurant365 OData view from $metadata",
		Example: "  restaurant365-odata-pp-cli describe-view SalesDetail --agent",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			view, err := canonicalR365View(args[0])
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}
			var fields []r365Field
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			metadata, err := fetchR365Metadata(cmd.Context(), c)
			if err == nil {
				fields = r365FieldsForView(metadata, view.Name)
			}
			out := map[string]any{
				"name":         view.Name,
				"resource":     view.Resource,
				"date_field":   view.DateField,
				"row_version":  view.RowVersion,
				"sync_pattern": r365SyncPattern(view),
				"fields":       fields,
				"field_count":  len(fields),
			}
			return flags.printJSON(cmd, out)
		},
	}
	return cmd
}

func newR365SampleCmd(flags *rootFlags) *cobra.Command {
	var viewName string
	var limit int
	var filter string
	var includeValues bool

	cmd := &cobra.Command{
		Use:     "sample",
		Short:   "Safely sample a Restaurant365 OData view",
		Example: "  restaurant365-odata-pp-cli sample --view Location --limit 5 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			view, err := canonicalR365View(viewName)
			if err != nil {
				return usageErr(err)
			}
			if limit < 1 || limit > 100 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 100"))
			}
			if dryRunOK(flags) {
				return nil
			}
			rows, err := fetchR365Rows(cmd.Context(), flags, view.Name, limit, filter, "")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			summary := newR365SampleSummary(view.Name, limit, rows, includeValues)
			return flags.printJSON(cmd, summary)
		},
	}
	cmd.Flags().StringVar(&viewName, "view", "Location", "Restaurant365 OData view to sample")
	cmd.Flags().IntVar(&limit, "limit", 5, "Maximum rows to inspect (1-100)")
	cmd.Flags().StringVar(&filter, "filter", "", "Optional OData $filter expression")
	cmd.Flags().BoolVar(&includeValues, "include-values", false, "Include row values in output; default only reports columns and counts")
	return cmd
}

func newR365BackfillPlanCmd(flags *rootFlags) *cobra.Command {
	var viewName string
	var fromValue string
	var toValue string
	var chunkDays int
	var watermark string
	var locationID string

	cmd := &cobra.Command{
		Use:   "backfill-plan",
		Short: "Plan safe Restaurant365 OData backfill chunks",
		Example: `  restaurant365-odata-pp-cli backfill-plan --view SalesDetail --from 2026-05-01 --to 2026-05-31 --agent
  restaurant365-odata-pp-cli backfill-plan --view Transaction --watermark 0 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			view, err := canonicalR365View(viewName)
			if err != nil {
				return usageErr(err)
			}
			if chunkDays < 1 || chunkDays > 31 {
				return usageErr(fmt.Errorf("--chunk-days must be between 1 and 31"))
			}
			var from, to time.Time
			if fromValue != "" || toValue != "" {
				from, err = parseR365Date(fromValue)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --from: %w", err))
				}
				to, err = parseR365Date(toValue)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --to: %w", err))
				}
			}
			plan, err := planR365Backfill(view.Name, from, to, chunkDays, locationID)
			if err != nil {
				return usageErr(err)
			}
			if watermark != "" {
				plan.Watermark = watermark
			}
			if view.RowVersion && fromValue == "" && toValue == "" {
				filter := "rowVersion gt 0"
				if watermark != "" {
					if _, err := strconv.ParseUint(watermark, 10, 64); err != nil {
						return usageErr(fmt.Errorf("--watermark must be a non-negative integer, got %q", watermark))
					}
					filter = "rowVersion gt " + watermark
				}
				plan.Strategy = "rowVersion"
				plan.Chunks = []r365BackfillStep{{Filter: filter, Reason: "rowVersion captures inserts, updates, and back-dated edits"}}
				plan.Requests = 1
			}
			return flags.printJSON(cmd, plan)
		},
	}
	cmd.Flags().StringVar(&viewName, "view", "SalesDetail", "Restaurant365 OData view")
	cmd.Flags().StringVar(&fromValue, "from", "", "Inclusive start date YYYY-MM-DD")
	cmd.Flags().StringVar(&toValue, "to", "", "Inclusive end date YYYY-MM-DD")
	cmd.Flags().IntVar(&chunkDays, "chunk-days", 31, "Maximum days per date-window chunk")
	cmd.Flags().StringVar(&watermark, "watermark", "", "Starting rowVersion watermark for rowVersion-backed views")
	cmd.Flags().StringVar(&locationID, "location-id", "", "Optional location ID to include in date-window filters")
	return cmd
}

func newR365DeletedRecordsCmd(flags *rootFlags) *cobra.Command {
	var entity string
	var sinceRowVersion string
	var limit int
	var includeValues bool

	cmd := &cobra.Command{
		Use:     "deleted-records",
		Short:   "Inspect Restaurant365 EntityDeleted tombstones",
		Example: "  restaurant365-odata-pp-cli deleted-records --since-row-version 0 --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 || limit > 1000 {
				return usageErr(fmt.Errorf("--limit must be between 1 and 1000"))
			}
			filterParts := []string{}
			if sinceRowVersion != "" {
				if _, err := strconv.ParseUint(sinceRowVersion, 10, 64); err != nil {
					return usageErr(fmt.Errorf("--since-row-version must be a non-negative integer, got %q", sinceRowVersion))
				}
				filterParts = append(filterParts, "rowVersion gt "+sinceRowVersion)
			}
			if entity != "" {
				filterParts = append(filterParts, "entityName eq '"+escapeODataString(entity)+"'")
			}
			rows, err := fetchR365Rows(cmd.Context(), flags, "EntityDeleted", limit, strings.Join(filterParts, " and "), "rowVersion")
			if err != nil {
				return classifyAPIError(err, flags)
			}
			counts := map[string]int{}
			for _, row := range rows {
				name, _ := row["entityName"].(string)
				if name == "" {
					name = "unknown"
				}
				counts[name]++
			}
			out := map[string]any{"view": "EntityDeleted", "returned": len(rows), "counts_by_entity": counts, "values_redacted": !includeValues}
			if includeValues {
				out["rows"] = rows
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&entity, "entity", "", "Filter tombstones to one R365 entity name")
	cmd.Flags().StringVar(&sinceRowVersion, "since-row-version", "", "Only tombstones with rowVersion greater than this value")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum tombstones to inspect")
	cmd.Flags().BoolVar(&includeValues, "include-values", false, "Include tombstone IDs and row values; default returns counts only")
	return cmd
}

func newR365ExportCmd(flags *rootFlags) *cobra.Command {
	var viewName string
	var fromValue string
	var toValue string
	var filter string
	var format string
	var output string
	var limit int

	cmd := &cobra.Command{
		Use:     "export",
		Short:   "Export a Restaurant365 OData view to JSONL or CSV",
		Example: "  restaurant365-odata-pp-cli export --view SalesDetail --from 2026-05-01 --to 2026-05-31 --output sales.jsonl --dry-run --agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			view, err := canonicalR365View(viewName)
			if err != nil {
				return usageErr(err)
			}
			if output == "" {
				return usageErr(fmt.Errorf("--output is required"))
			}
			if format != "jsonl" && format != "csv" {
				return usageErr(fmt.Errorf("--format must be jsonl or csv"))
			}
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			queryFilters := []string{filter}
			plannedChunks := 0
			if filter == "" && fromValue != "" && toValue != "" {
				from, err := parseR365Date(fromValue)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --from: %w", err))
				}
				to, err := parseR365Date(toValue)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --to: %w", err))
				}
				plan, err := planR365Backfill(view.Name, from, to, 31, "")
				if err != nil {
					return usageErr(err)
				}
				queryFilters = make([]string, 0, len(plan.Chunks))
				if len(plan.Chunks) > 0 {
					for _, chunk := range plan.Chunks {
						queryFilters = append(queryFilters, chunk.Filter)
					}
				}
				plannedChunks = len(queryFilters)
			}
			if len(queryFilters) == 0 {
				queryFilters = []string{""}
			}
			if queryFilters[0] == "" && view.DateField != "" {
				return usageErr(fmt.Errorf("%s exports require --from/--to or --filter to avoid unsafe large pulls", view.Name))
			}
			if flags.dryRun {
				out := map[string]any{"dry_run": true, "view": view.Name, "output": output, "format": format, "filters": queryFilters}
				if plannedChunks > 0 {
					out["chunks"] = plannedChunks
				}
				return flags.printJSON(cmd, out)
			}
			rows := []map[string]any{}
			for _, queryFilter := range queryFilters {
				chunkRows, err := fetchR365RowsAll(cmd.Context(), flags, view.Name, limit, queryFilter, "")
				if err != nil {
					return classifyAPIError(err, flags)
				}
				rows = append(rows, chunkRows...)
			}
			if err := writeR365Export(output, format, rows); err != nil {
				return err
			}
			out := map[string]any{"view": view.Name, "rows": len(rows), "output": output, "format": format}
			if plannedChunks > 0 {
				out["chunks"] = plannedChunks
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&viewName, "view", "Location", "Restaurant365 OData view")
	cmd.Flags().StringVar(&fromValue, "from", "", "Inclusive start date YYYY-MM-DD for date-window exports")
	cmd.Flags().StringVar(&toValue, "to", "", "Inclusive end date YYYY-MM-DD for date-window exports")
	cmd.Flags().StringVar(&filter, "filter", "", "Explicit OData $filter expression")
	cmd.Flags().StringVar(&format, "format", "jsonl", "Export format: jsonl or csv")
	cmd.Flags().StringVar(&output, "output", "", "Output file path")
	cmd.Flags().IntVar(&limit, "limit", 1000, "Rows to fetch per export page")
	return cmd
}

func canonicalR365View(input string) (r365View, error) {
	key := normalizeR365ViewName(input)
	for _, view := range r365Views {
		if normalizeR365ViewName(view.Name) == key || normalizeR365ViewName(view.Resource) == key {
			return view, nil
		}
	}
	return r365View{}, fmt.Errorf("unknown R365 view %q", input)
}

func normalizeR365ViewName(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	replacer := strings.NewReplacer("-", "", "_", "", " ", "")
	return replacer.Replace(input)
}

func r365FieldsForView(metadata map[string][]r365Field, viewName string) []r365Field {
	if len(metadata) == 0 {
		return nil
	}
	if fields := metadata[viewName]; fields != nil {
		return fields
	}
	viewKey := normalizeR365ViewName(viewName)
	for name, fields := range metadata {
		if normalizeR365ViewName(name) == viewKey {
			return fields
		}
	}
	return nil
}

func r365SyncPattern(view r365View) string {
	switch {
	case view.RowVersion:
		return "rowVersion incremental"
	case view.DateField != "":
		return "date-window refresh"
	default:
		return "small dimension full refresh"
	}
}

func fetchR365Rows(ctx context.Context, flags *rootFlags, viewName string, limit int, filter, orderBy string) ([]map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	rows, _, err := fetchR365RowsPage(ctx, c, viewName, limit, filter, orderBy, 0)
	return rows, err
}

func fetchR365RowsAll(ctx context.Context, flags *rootFlags, viewName string, limit int, filter, orderBy string) ([]map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	rows := []map[string]any{}
	skip := 0
	for {
		pageRows, nextSkip, err := fetchR365RowsPage(ctx, c, viewName, limit, filter, orderBy, skip)
		if err != nil {
			return nil, err
		}
		rows = append(rows, pageRows...)
		if nextSkip < 0 || len(pageRows) == 0 {
			break
		}
		skip = nextSkip
	}
	return rows, nil
}

func fetchR365RowsPage(ctx context.Context, c *client.Client, viewName string, limit int, filter, orderBy string, skip int) ([]map[string]any, int, error) {
	params := map[string]string{"$top": strconv.Itoa(limit)}
	if skip > 0 {
		params["$skip"] = strconv.Itoa(skip)
	}
	if filter != "" {
		params["$filter"] = filter
	}
	if orderBy != "" {
		params["$orderby"] = orderBy
	}
	data, err := c.GetNoCache(ctx, "/"+viewName, params)
	if err != nil {
		return nil, -1, err
	}
	rows, nextSkip, err := extractODataRowsPage(data, limit, skip)
	return rows, nextSkip, err
}

func extractODataRows(data json.RawMessage) ([]map[string]any, error) {
	rows, _, err := extractODataRowsPage(data, 0, 0)
	return rows, err
}

func extractODataRowsPage(data json.RawMessage, limit int, skip int) ([]map[string]any, int, error) {
	if isDryRunResponse(data) {
		return nil, -1, nil
	}
	var envelope struct {
		Value    []map[string]any `json:"value"`
		NextLink string           `json:"@odata.nextLink"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Value != nil {
		nextSkip := nextR365Skip(envelope.NextLink, len(envelope.Value), limit, skip)
		return envelope.Value, nextSkip, nil
	}
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err == nil {
		nextSkip := -1
		if limit > 0 && len(rows) >= limit {
			nextSkip = skip + len(rows)
		}
		return rows, nextSkip, nil
	}
	return nil, -1, fmt.Errorf("response did not contain an OData value array")
}

func nextR365Skip(nextLink string, rowCount int, limit int, currentSkip int) int {
	if nextLink != "" {
		if parsed, err := url.Parse(nextLink); err == nil {
			if raw := parsed.Query().Get("$skip"); raw != "" {
				if next, err := strconv.Atoi(raw); err == nil && next > currentSkip {
					return next
				}
			}
		}
	}
	if limit > 0 && rowCount >= limit {
		return currentSkip + rowCount
	}
	return -1
}

func newR365SampleSummary(view string, requested int, rows []map[string]any, includeValues bool) r365SampleSummary {
	columns := collectR365Columns(rows)
	summary := r365SampleSummary{
		View:           view,
		Requested:      requested,
		Returned:       len(rows),
		Columns:        columns,
		ValuesRedacted: !includeValues,
	}
	if includeValues {
		summary.Rows = rows
	}
	return summary
}

func collectR365Columns(rows []map[string]any) []string {
	set := map[string]bool{}
	for _, row := range rows {
		for key := range row {
			set[key] = true
		}
	}
	columns := make([]string, 0, len(set))
	for key := range set {
		columns = append(columns, key)
	}
	sort.Strings(columns)
	return columns
}

func parseR365Date(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}
	return time.Parse("2006-01-02", value)
}

func planR365Backfill(viewName string, from, to time.Time, chunkDays int, locationID string) (r365BackfillPlan, error) {
	view, err := canonicalR365View(viewName)
	if err != nil {
		return r365BackfillPlan{}, err
	}
	if from.IsZero() || to.IsZero() {
		return r365BackfillPlan{View: view.Name, Strategy: r365SyncPattern(view), Requests: 0}, nil
	}
	if to.Before(from) {
		return r365BackfillPlan{}, fmt.Errorf("--to must be on or after --from")
	}
	if view.DateField == "" {
		return r365BackfillPlan{}, fmt.Errorf("%s does not have a known date field; use rowVersion or explicit filters", view.Name)
	}

	plan := r365BackfillPlan{
		View:        view.Name,
		Strategy:    "date-window",
		DateField:   view.DateField,
		ChunkDays:   chunkDays,
		LocationID:  locationID,
		Recommended: "run export or sync per chunk; keep chunks small for R365 OData stability",
	}
	for cursor := from; !cursor.After(to); {
		end := cursor.AddDate(0, 0, chunkDays-1)
		if end.After(to) {
			end = to
		}
		filter := fmt.Sprintf("%s ge %s and %s le %s", view.DateField, r365StartOfDay(cursor), view.DateField, r365EndOfDay(end))
		if locationID != "" {
			filter += " and locationId eq '" + escapeODataString(locationID) + "'"
		}
		plan.Chunks = append(plan.Chunks, r365BackfillStep{
			From:   cursor.Format("2006-01-02"),
			To:     end.Format("2006-01-02"),
			Filter: filter,
			Reason: "bounded date-window request for R365 OData",
		})
		cursor = end.AddDate(0, 0, 1)
	}
	plan.Requests = len(plan.Chunks)
	return plan, nil
}

func fetchR365Metadata(ctx context.Context, c *client.Client) (map[string][]r365Field, error) {
	raw, err := c.GetWithHeadersNoCache(ctx, "/$metadata", nil, map[string]string{"Accept": "application/xml"})
	if err != nil {
		return nil, err
	}
	return parseR365Metadata([]byte(raw))
}

func parseR365Metadata(raw []byte) (map[string][]r365Field, error) {
	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	fields := map[string][]r365Field{}
	var current string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fields, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "EntityType":
			current = attrValue(start, "Name")
		case "Property":
			if current == "" {
				continue
			}
			fields[current] = append(fields[current], r365Field{
				Name:     attrValue(start, "Name"),
				Type:     attrValue(start, "Type"),
				Nullable: attrValue(start, "Nullable"),
			})
		}
	}
	return fields, nil
}

func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func writeR365Export(path, format string, rows []map[string]any) error {
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	keepTemp := true
	closed := false
	closeFile := func() error {
		if closed {
			return nil
		}
		closed = true
		return file.Close()
	}
	defer func() {
		_ = closeFile()
		if keepTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	switch format {
	case "jsonl":
		enc := json.NewEncoder(file)
		for _, row := range rows {
			if err := enc.Encode(row); err != nil {
				return err
			}
		}
	case "csv":
		columns := collectR365Columns(rows)
		writer := csv.NewWriter(file)
		if err := writer.Write(columns); err != nil {
			return err
		}
		for _, row := range rows {
			record := make([]string, len(columns))
			for i, column := range columns {
				if row[column] != nil {
					record[i] = fmt.Sprint(row[column])
				}
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}
	}
	if err := closeFile(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	keepTemp = false
	return nil
}

func escapeODataString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func r365StartOfDay(value time.Time) string {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
}

func r365EndOfDay(value time.Time) string {
	return time.Date(value.Year(), value.Month(), value.Day(), 23, 59, 59, 999999999, time.UTC).Format(time.RFC3339Nano)
}
