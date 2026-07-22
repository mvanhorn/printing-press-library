// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/pricing"
	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/store"
)

func newNovelSpendCmd(flags *rootFlags) *cobra.Command {
	var flagBy string
	var flagState string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "spend",
		Short:       "SQL over local booking, invoice, and Magic-task history by category, tasker, source, or month",
		Example:     "  human-goat-pp-cli spend --by source --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !commandHasChangedFlags(cmd) {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate local spend")
				return nil
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("spend does not accept positional arguments"))
			}
			flagBy = strings.ToLower(strings.TrimSpace(flagBy))
			switch flagBy {
			case "source", "category", "tasker", "month":
			default:
				return usageErr(fmt.Errorf("--by must be one of source, category, tasker, month"))
			}

			if dbPath == "" {
				dbPath = defaultDBPath("human-goat-pp-cli")
			}
			if _, err := os.Stat(dbPath); err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: human-goat-pp-cli sync --db %s\n", dbPath, dbPath)
					if flags.asJSON || flags.agent {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					}
					return nil
				}
				return fmt.Errorf("stat local mirror: %w", err)
			}

			s, err := store.OpenReadOnlyContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local mirror: %w", err)
			}
			defer s.Close()

			rows, err := s.Query(`SELECT resource_type, data, updated_at FROM resources WHERE resource_type IN ('bookings', 'invoices', 'magic', 'tasks')`)
			if err != nil {
				return fmt.Errorf("query local spend rows: %w", err)
			}

			resourceRows := make([]spendResourceRow, 0)
			for rows.Next() {
				var row spendResourceRow
				var data []byte
				if err := rows.Scan(&row.ResourceType, &data, &row.UpdatedAt); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan local spend row: %w", err)
				}
				row.Data = append(json.RawMessage(nil), data...)
				resourceRows = append(resourceRows, row)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("read local spend rows: %w", err)
			}
			if err := rows.Close(); err != nil {
				return fmt.Errorf("close local spend rows: %w", err)
			}

			out := aggregateSpendRows(resourceRows, flagBy, flagState)
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			tableRows := make([][]string, 0, len(out))
			for _, row := range out {
				tableRows = append(tableRows, []string{row.Group, strconv.Itoa(row.Count), strconv.Itoa(row.TotalCents)})
			}
			if err := flags.printTable(cmd, []string{"GROUP", "COUNT", "TOTAL_CENTS"}, tableRows); err != nil {
				return err
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no local spend data yet; sync bookings/invoices first.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "source", "Aggregate by source, category, tasker, or month")
	cmd.Flags().StringVar(&flagState, "state", "", "State for CA/MA service-fee-only pricing when estimating TaskRabbit all-in totals")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

type spendResourceRow struct {
	ResourceType string
	Data         json.RawMessage
	UpdatedAt    string
}

type spendOutputRow struct {
	Group      string `json:"group"`
	Count      int    `json:"count"`
	TotalCents int    `json:"total_cents"`
}

func aggregateSpendRows(rows []spendResourceRow, by, state string) []spendOutputRow {
	type aggregate struct {
		count      int
		totalCents int
	}
	aggregates := make(map[string]aggregate)
	for _, row := range rows {
		decoded := decodeSpendData(row.Data)
		group := spendGroup(row, decoded, by)
		if group == "" {
			group = "unknown"
		}
		agg := aggregates[group]
		agg.count++
		if cents, ok := spendAmountCentsForRow(row, decoded, state); ok {
			agg.totalCents += cents
		}
		aggregates[group] = agg
	}

	groups := make([]string, 0, len(aggregates))
	for group := range aggregates {
		groups = append(groups, group)
	}
	sort.Strings(groups)

	out := make([]spendOutputRow, 0, len(groups))
	for _, group := range groups {
		agg := aggregates[group]
		out = append(out, spendOutputRow{
			Group:      group,
			Count:      agg.count,
			TotalCents: agg.totalCents,
		})
	}
	return out
}

func decodeSpendData(data json.RawMessage) any {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil
	}
	return decoded
}

func spendGroup(row spendResourceRow, decoded any, by string) string {
	switch by {
	case "source":
		if row.ResourceType == "magic" {
			return "magic"
		}
		return "taskrabbit"
	case "category":
		return findSpendString(decoded, "category", "category_name", "categoryName", "task_template", "task_template_name", "taskTemplateName")
	case "tasker":
		return findSpendString(decoded, "tasker", "tasker_name", "taskerName", "rabbit_name", "rabbitName", "display_name", "displayName")
	case "month":
		if month := spendMonth(findSpendString(decoded, "date", "created_at", "createdAt", "completed_at", "completedAt", "submitted_at", "submittedAt", "scheduled_at", "scheduledAt", "invoice_date", "invoiceDate", "updated_at", "updatedAt")); month != "" {
			return month
		}
		return spendMonth(row.UpdatedAt)
	default:
		return ""
	}
}

func findSpendString(value any, keys ...string) string {
	targets := make(map[string]bool, len(keys))
	for _, key := range keys {
		targets[normalizeSpendKey(key)] = true
	}
	return findSpendStringInValue(value, targets)
}

func findSpendStringInValue(value any, targets map[string]bool) string {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if !targets[normalizeSpendKey(key)] {
				continue
			}
			if s := spendStringValue(item); s != "" {
				return s
			}
			if s := findSpendString(item, "name", "title", "display_name", "displayName"); s != "" {
				return s
			}
		}
		for _, item := range v {
			if s := findSpendStringInValue(item, targets); s != "" {
				return s
			}
		}
	case []any:
		for _, item := range v {
			if s := findSpendStringInValue(item, targets); s != "" {
				return s
			}
		}
	}
	return ""
}

func spendStringValue(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return ""
	}
}

// spendAmountCentsForRow resolves the spend amount for a row. An explicit
// billed/all-in amount (e.g. an invoice row's real total) always wins so a rate
// estimate never shadows a known charge. Only when no explicit amount exists
// does it estimate a TaskRabbit booking's all-in total: base hourly rate ->
// all-in fee transform -> multiplied by the booked duration (so multi-hour
// bookings are not counted at a single hour).
func spendAmountCentsForRow(row spendResourceRow, decoded any, state string) (int, bool) {
	if cents, ok := findSpendAmountCents(decoded); ok {
		return cents, true
	}
	if row.ResourceType != "magic" {
		if base, ok := findPosterHourlyRateCents(decoded); ok {
			allInHourly := pricing.AllIn(base, state).AllInCents
			hours := findBookingHours(decoded)
			return int(math.Round(float64(allInHourly) * hours)), true
		}
	}
	return 0, false
}

// findBookingHours derives a TaskRabbit booking's billable hours from a
// duration_seconds or hours field, defaulting to 1 hour (the TaskRabbit
// minimum billing unit) when neither is present.
func findBookingHours(value any) float64 {
	if secs, ok := findNumericField(value, "duration_seconds"); ok && secs > 0 {
		if h := secs / 3600.0; h >= 1 {
			return h
		}
	}
	if h, ok := findNumericField(value, "hours"); ok && h >= 1 {
		return h
	}
	return 1
}

// findNumericField recursively finds the first numeric value for key.
func findNumericField(value any, key string) (float64, bool) {
	switch v := value.(type) {
	case map[string]any:
		if item, ok := v[key]; ok {
			if raw, _, ok := spendNumericValue(item); ok {
				return raw, true
			}
		}
		for _, item := range v {
			if raw, ok := findNumericField(item, key); ok {
				return raw, true
			}
		}
	case []any:
		for _, item := range v {
			if raw, ok := findNumericField(item, key); ok {
				return raw, true
			}
		}
	}
	return 0, false
}

// findPosterHourlyRateCents extracts a TaskRabbit poster_hourly_rate_cents value
// (the pre-fee base rate) from a decoded row, recursing into nested objects.
func findPosterHourlyRateCents(value any) (int, bool) {
	switch v := value.(type) {
	case map[string]any:
		if item, ok := v["poster_hourly_rate_cents"]; ok {
			if cents, ok := spendCentsValue("poster_hourly_rate_cents", item); ok {
				return cents, true
			}
		}
		for _, item := range v {
			if cents, ok := findPosterHourlyRateCents(item); ok {
				return cents, true
			}
		}
	case []any:
		for _, item := range v {
			if cents, ok := findPosterHourlyRateCents(item); ok {
				return cents, true
			}
		}
	}
	return 0, false
}

func findSpendAmountCents(value any) (int, bool) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if spendAmountKey(key) {
				if cents, ok := spendCentsValue(key, item); ok {
					return cents, true
				}
			}
		}
		for _, item := range v {
			if cents, ok := findSpendAmountCents(item); ok {
				return cents, true
			}
		}
	case []any:
		for _, item := range v {
			if cents, ok := findSpendAmountCents(item); ok {
				return cents, true
			}
		}
	}
	return 0, false
}

func spendAmountKey(key string) bool {
	normalized := normalizeSpendKey(key)
	return normalized == "amount" || normalized == "allin" || normalized == "amountcents" || normalized == "allincents"
}

func spendCentsValue(key string, value any) (int, bool) {
	raw, stringValue, ok := spendNumericValue(value)
	if !ok {
		return 0, false
	}
	normalized := normalizeSpendKey(key)
	if strings.HasSuffix(normalized, "cents") {
		return roundSpendCents(raw), true
	}
	if strings.Contains(stringValue, ".") || strings.Contains(stringValue, "$") {
		return roundSpendCents(raw * 100), true
	}
	return roundSpendCents(raw), true
}

func spendNumericValue(value any) (float64, string, bool) {
	switch v := value.(type) {
	case json.Number:
		f, err := v.Float64()
		return f, v.String(), err == nil
	case float64:
		return v, strconv.FormatFloat(v, 'f', -1, 64), true
	case int:
		return float64(v), strconv.Itoa(v), true
	case string:
		clean := strings.TrimSpace(v)
		clean = strings.TrimPrefix(clean, "$")
		clean = strings.ReplaceAll(clean, ",", "")
		clean = strings.TrimSuffix(clean, "/hr")
		clean = strings.TrimSpace(clean)
		f, err := strconv.ParseFloat(clean, 64)
		return f, v, err == nil
	default:
		return 0, "", false
	}
}

func roundSpendCents(v float64) int {
	if v < 0 {
		return int(v - 0.5)
	}
	return int(v + 0.5)
}

func normalizeSpendKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	replacer := strings.NewReplacer("_", "", "-", "", " ", "")
	return replacer.Replace(key)
}

func spendMonth(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("2006-01") && value[4] == '-' {
		if _, err := time.Parse("2006-01", value[:7]); err == nil {
			return value[:7]
		}
	}
	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Format("2006-01")
		}
	}
	return ""
}
