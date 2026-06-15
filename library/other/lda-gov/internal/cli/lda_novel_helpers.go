// Copyright 2026 Mherzog4 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/lda-gov/internal/store"
	"github.com/spf13/cobra"
)

const ldaCLIName = "lda-gov-pp-cli"

type ldaRecord map[string]any

func openLDAStore(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath string, resources ...string) (*store.Store, string, error) {
	if err := validateDataSourceStrategy(flags, "local"); err != nil {
		return nil, dbPath, err
	}
	if dbPath == "" {
		dbPath = defaultDBPath(ldaCLIName)
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: %s sync --resources %s --db %s\n", dbPath, ldaCLIName, strings.Join(resources, ","), dbPath)
		return nil, dbPath, nil
	}
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, dbPath, fmt.Errorf("opening database: %w", err)
	}
	if !hintIfUnsynced(cmd, db, "") {
		hintIfStale(cmd, db, "", flags.maxAge)
	}
	return db, dbPath, nil
}

func listLDARecords(db *store.Store, resource string, limit int) ([]ldaRecord, error) {
	items, err := db.List(resource, limit)
	if err != nil {
		return nil, err
	}
	records := make([]ldaRecord, 0, len(items))
	for _, item := range items {
		rec, err := decodeLDARecord(item)
		if err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

func searchLDARecords(db *store.Store, query, resource string, limit int) ([]ldaRecord, error) {
	items, err := db.Search(query, limit, resource)
	if err != nil {
		return nil, err
	}
	records := make([]ldaRecord, 0, len(items))
	for _, item := range items {
		rec, err := decodeLDARecord(item)
		if err != nil {
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

func hintIfResourceEmpty(cmd *cobra.Command, db *store.Store, resource string) bool {
	resource = strings.TrimSpace(resource)
	if cmd == nil || db == nil || resource == "" {
		return false
	}
	count, err := db.Count(resource)
	if err != nil || count > 0 {
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "hint: local store has no %s records. Run '%s sync --resources %s' before trusting %s results.\n", resource, ldaCLIName, resource, resource)
	return true
}

func decodeLDARecord(data json.RawMessage) (ldaRecord, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var rec ldaRecord
	if err := dec.Decode(&rec); err != nil {
		return nil, err
	}
	return rec, nil
}

func writeLDARows(cmd *cobra.Command, flags *rootFlags, rows []map[string]any, columns []string) error {
	if rows == nil {
		rows = []map[string]any{}
	}
	if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly && !flags.csv) {
		return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
	}
	if flags.csv {
		return writeLDACSV(cmd, rows, columns)
	}
	return printAutoTable(cmd.OutOrStdout(), rows)
}

func writeLDAValue(cmd *cobra.Command, flags *rootFlags, value any) error {
	return printJSONFiltered(cmd.OutOrStdout(), value, flags)
}

func writeLDACSV(cmd *cobra.Command, rows []map[string]any, columns []string) error {
	if len(columns) == 0 {
		set := map[string]bool{}
		for _, row := range rows {
			for key := range row {
				set[key] = true
			}
		}
		for key := range set {
			columns = append(columns, key)
		}
		sort.Strings(columns)
	}
	w := csv.NewWriter(cmd.OutOrStdout())
	if err := w.Write(columns); err != nil {
		return err
	}
	for _, row := range rows {
		out := make([]string, len(columns))
		for i, col := range columns {
			out[i] = fmt.Sprint(row[col])
		}
		if err := w.Write(out); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func ldaString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprint(t)
	}
}

func ldaNumber(v any) float64 {
	switch t := v.(type) {
	case json.Number:
		f, _ := t.Float64()
		return f
	case float64:
		return t
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case string:
		clean := strings.ReplaceAll(strings.TrimSpace(t), ",", "")
		f, _ := strconv.ParseFloat(clean, 64)
		return f
	default:
		return 0
	}
}

func ldaAt(rec ldaRecord, path ...string) any {
	var cur any = rec
	for _, part := range path {
		var m map[string]any
		switch typed := cur.(type) {
		case ldaRecord:
			m = map[string]any(typed)
		case map[string]any:
			m = typed
		default:
			return nil
		}
		cur = m[part]
	}
	return cur
}

func ldaAtString(rec ldaRecord, paths ...[]string) string {
	for _, path := range paths {
		if s := strings.TrimSpace(ldaString(ldaAt(rec, path...))); s != "" {
			return s
		}
	}
	return ""
}

func ldaSlice(v any) []any {
	if items, ok := v.([]any); ok {
		return items
	}
	return nil
}

func ldaMap(v any) map[string]any {
	if m, ok := v.(ldaRecord); ok {
		return map[string]any(m)
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

func ldaContainsFold(haystack, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(haystack), needle)
}

func ldaYear(rec ldaRecord) int {
	return int(ldaNumber(ldaAt(rec, "filing_year")))
}

func ldaPeriod(rec ldaRecord) string {
	period := ldaString(ldaAt(rec, "filing_period_display"))
	if period == "" {
		period = ldaString(ldaAt(rec, "filing_period"))
	}
	return strings.ToUpper(strings.TrimSpace(period))
}

func ldaClientName(rec ldaRecord) string {
	return ldaAtString(rec, []string{"client", "name"}, []string{"client_name"}, []string{"name"})
}

func ldaRegistrantName(rec ldaRecord) string {
	return ldaAtString(rec, []string{"registrant", "name"}, []string{"registrant_name"}, []string{"name"})
}

func ldaLobbyistName(rec ldaRecord) string {
	if s := strings.TrimSpace(ldaString(ldaAt(rec, "lobbyist_full_display_name"))); s != "" {
		return s
	}
	parts := []string{
		ldaString(ldaAt(rec, "prefix")),
		ldaString(ldaAt(rec, "first_name")),
		ldaString(ldaAt(rec, "middle_name")),
		ldaString(ldaAt(rec, "last_name")),
		ldaString(ldaAt(rec, "suffix")),
	}
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			kept = append(kept, strings.TrimSpace(part))
		}
	}
	return strings.Join(kept, " ")
}

func ldaNestedLobbyistName(v any) string {
	m := ldaMap(v)
	if m == nil {
		return ""
	}
	if nested := ldaMap(m["lobbyist"]); nested != nil {
		return ldaLobbyistName(ldaRecord(nested))
	}
	return ldaLobbyistName(ldaRecord(m))
}

func ldaContributionItems(rec ldaRecord) []map[string]any {
	items := ldaSlice(ldaAt(rec, "contribution_items"))
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if m := ldaMap(item); m != nil {
			out = append(out, m)
		}
	}
	return out
}

func ldaIssueCodes(rec ldaRecord) []string {
	var out []string
	for _, activity := range ldaSlice(ldaAt(rec, "lobbying_activities")) {
		m := ldaMap(activity)
		if m == nil {
			continue
		}
		for _, key := range []string{"general_issue_code", "general_issue_code_display", "issue", "description"} {
			if s := strings.TrimSpace(ldaString(m[key])); s != "" {
				out = append(out, s)
				break
			}
		}
	}
	return out
}

func ldaMatchesYearRange(rec ldaRecord, fromYear, toYear int) bool {
	year := ldaYear(rec)
	if fromYear > 0 && year < fromYear {
		return false
	}
	if toYear > 0 && year > toYear {
		return false
	}
	return true
}

func ldaParseYear(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	year, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("invalid year %q", raw)
	}
	return year, nil
}

func ldaMatchesPeriod(rec ldaRecord, period string) bool {
	period = strings.ToLower(strings.TrimSpace(period))
	if period == "" {
		return true
	}
	value := strings.ToLower(ldaPeriod(rec))
	return strings.Contains(value, period) || strings.EqualFold(ldaString(ldaAt(rec, "filing_period")), period)
}

func ldaTopCount(counts map[string]int) string {
	type item struct {
		name  string
		count int
	}
	items := make([]item, 0, len(counts))
	for name, count := range counts {
		if strings.TrimSpace(name) != "" {
			items = append(items, item{name: name, count: count})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].name < items[j].name
		}
		return items[i].count > items[j].count
	})
	if len(items) == 0 {
		return ""
	}
	return fmt.Sprintf("%s (%d)", items[0].name, items[0].count)
}

func ldaNowDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

func countLDAResources(db *store.Store, resource string) int {
	var count int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type = ?`, resource).Scan(&count); err != nil && err != sql.ErrNoRows {
		return 0
	}
	return count
}
