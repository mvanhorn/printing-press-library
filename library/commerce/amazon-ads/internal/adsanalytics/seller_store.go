package adsanalytics

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

type SellerRevenueSummary struct {
	StorePath      string   `json:"store_path"`
	Revenue        float64  `json:"revenue"`
	MatchedRecords int      `json:"matched_records"`
	Source         string   `json:"source,omitempty"`
	Notes          []string `json:"notes,omitempty"`
}

func DefaultSellerStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "amazon-seller-pp-cli", "store.db")
}

func LoadSellerRevenue(storePath, asin string) (SellerRevenueSummary, error) {
	if storePath == "" {
		storePath = DefaultSellerStorePath()
	}
	summary := SellerRevenueSummary{StorePath: storePath}
	if storePath == "" {
		summary.Notes = append(summary.Notes, "could not resolve amazon-seller store path; TACOS requires total seller revenue")
		return summary, nil
	}
	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			summary.Notes = append(summary.Notes, "amazon-seller store not found; run amazon-seller-pp-cli sync or pass --total-revenue")
			return summary, nil
		}
		return summary, fmt.Errorf("checking seller store %s: %w", storePath, err)
	}

	db, err := sql.Open("sqlite", storePath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return summary, fmt.Errorf("opening seller store %s: %w", storePath, err)
	}
	defer db.Close()

	if tableUsableForRevenue(db, "orders", &summary) {
		summary.Source = "orders"
		if err := loadRevenueFromTable(db, "orders", "", asin, &summary); err != nil {
			return summary, err
		}
	}
	if summary.MatchedRecords == 0 && tableUsableForRevenue(db, "resources", &summary) {
		summary.Source = "resources:orders"
		if err := loadRevenueFromTable(db, "resources", "orders", asin, &summary); err != nil {
			return summary, err
		}
	}
	if summary.MatchedRecords == 0 && tableUsableForRevenue(db, "reports", &summary) {
		summary.Source = "reports"
		if err := loadRevenueFromTable(db, "reports", "", asin, &summary); err != nil {
			return summary, err
		}
	}
	if summary.MatchedRecords == 0 {
		if asin != "" {
			summary.Notes = append(summary.Notes, "no seller revenue records matched the ASIN; TACOS unavailable")
		} else {
			summary.Notes = append(summary.Notes, "seller store contained no recognizable revenue records; TACOS unavailable")
		}
	}
	return summary, nil
}

func tableUsableForRevenue(db *sql.DB, table string, summary *SellerRevenueSummary) bool {
	var name string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name); err != nil {
		return false
	}
	rows, err := db.Query(`PRAGMA table_info(` + quoteSQLiteIdent(table) + `)`)
	if err != nil {
		summary.Notes = append(summary.Notes, fmt.Sprintf("could not inspect seller store table %s: %v", table, err))
		return false
	}
	defer rows.Close()
	hasDataColumn := false
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultValue, &pk); err != nil {
			summary.Notes = append(summary.Notes, fmt.Sprintf("could not inspect seller store table %s: %v", table, err))
			return false
		}
		if colName == "data" {
			hasDataColumn = true
		}
	}
	if err := rows.Err(); err != nil {
		summary.Notes = append(summary.Notes, fmt.Sprintf("could not inspect seller store table %s: %v", table, err))
		return false
	}
	if !hasDataColumn {
		summary.Notes = append(summary.Notes, fmt.Sprintf("seller store table %s does not include a data column; TACOS revenue could not be read from that table", table))
		return false
	}
	return true
}

func loadRevenueFromTable(db *sql.DB, table, resourceType, asin string, summary *SellerRevenueSummary) error {
	query := `SELECT data FROM ` + quoteSQLiteIdent(table)
	args := []any{}
	if resourceType != "" {
		query += ` WHERE resource_type = ?`
		args = append(args, resourceType)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("querying seller store %s: %w", table, err)
	}
	defer rows.Close()

	malformedRows := 0
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return fmt.Errorf("scanning seller store %s row: %w", table, err)
		}
		var payload any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			malformedRows++
			continue
		}
		if asin != "" && !jsonContainsString(payload, asin) {
			continue
		}
		if amount, ok := extractRevenueAmount(payload); ok {
			summary.Revenue += amount
			summary.MatchedRecords++
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("reading seller store %s rows: %w", table, err)
	}
	if malformedRows > 0 {
		summary.Notes = append(summary.Notes, fmt.Sprintf("skipped %d malformed JSON row(s) in seller store table %s", malformedRows, table))
	}
	return nil
}

func quoteSQLiteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func extractRevenueAmount(v any) (float64, bool) {
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"OrderTotal", "orderTotal", "totalRevenue", "totalSales", "sales", "revenue", "itemPrice", "principal"} {
			if child, ok := x[key]; ok {
				if amount, ok := moneyAmount(child); ok {
					return amount, true
				}
			}
		}
		for _, child := range x {
			if amount, ok := extractRevenueAmount(child); ok {
				return amount, true
			}
		}
	case []any:
		total := 0.0
		matched := false
		for _, child := range x {
			if amount, ok := extractRevenueAmount(child); ok {
				total += amount
				matched = true
			}
		}
		return total, matched
	}
	return 0, false
}

func moneyAmount(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		amount, err := x.Float64()
		return amount, err == nil
	case string:
		return parseMoneyString(x)
	case map[string]any:
		for _, key := range []string{"Amount", "amount", "value", "Value"} {
			if child, ok := x[key]; ok {
				return moneyAmount(child)
			}
		}
	case []any:
		total := 0.0
		matched := false
		for _, child := range x {
			if amount, ok := moneyAmount(child); ok {
				total += amount
				matched = true
			}
		}
		return total, matched
	}
	return 0, false
}

func parseMoneyString(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.Trim(raw, "$")
	raw = strings.ReplaceAll(raw, ",", "")
	if raw == "" {
		return 0, false
	}
	amount, err := strconv.ParseFloat(raw, 64)
	return amount, err == nil
}

func jsonContainsString(v any, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return true
	}
	return jsonContainsStringFold(v, needle)
}

func jsonContainsStringFold(v any, needle string) bool {
	if needle == "" {
		return true
	}
	switch x := v.(type) {
	case string:
		return strings.Contains(strings.ToLower(x), needle)
	case map[string]any:
		for key, child := range x {
			if strings.Contains(strings.ToLower(key), needle) || jsonContainsStringFold(child, needle) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if jsonContainsStringFold(child, needle) {
				return true
			}
		}
	}
	return false
}
