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
	"time"

	_ "modernc.org/sqlite"
)

type SellerRevenueSummary struct {
	StorePath      string   `json:"store_path"`
	Revenue        float64  `json:"revenue"`
	MatchedRecords int      `json:"matched_records"`
	Source         string   `json:"source,omitempty"`
	Freshness      string   `json:"freshness,omitempty"`
	Notes          []string `json:"notes,omitempty"`
}

type SellerStoreValidation struct {
	StorePath        string   `json:"store_path"`
	Exists           bool     `json:"exists"`
	MarketplaceMatch *bool    `json:"marketplace_match,omitempty"`
	AccountMatch     *bool    `json:"account_match,omitempty"`
	DateOverlap      *bool    `json:"date_overlap,omitempty"`
	Freshness        string   `json:"freshness,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
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
	if freshness := sellerStoreFreshness(db); !freshness.IsZero() {
		summary.Freshness = freshness.Format(time.RFC3339)
	}

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

func ValidateSellerStore(storePath, expectedMarketplace, expectedAccount, startDate, endDate string) (SellerStoreValidation, error) {
	if storePath == "" {
		storePath = DefaultSellerStorePath()
	}
	validation := SellerStoreValidation{StorePath: storePath}
	if storePath == "" {
		return validation, fmt.Errorf("could not resolve amazon-seller store path")
	}
	if _, err := os.Stat(storePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return validation, fmt.Errorf("amazon-seller store not found at %s; run amazon-seller-pp-cli sync or pass --total-revenue", storePath)
		}
		return validation, fmt.Errorf("checking seller store %s: %w", storePath, err)
	}
	validation.Exists = true
	db, err := sql.Open("sqlite", storePath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return validation, fmt.Errorf("opening seller store %s: %w", storePath, err)
	}
	defer db.Close()
	if freshness := sellerStoreFreshness(db); !freshness.IsZero() {
		validation.Freshness = freshness.Format(time.RFC3339)
	}
	if expectedMarketplace != "" {
		if actual, ok := sellerStoreMetaValue(db, "marketplace", "marketplace_id", "marketplaceId"); ok {
			match := strings.EqualFold(actual, expectedMarketplace)
			validation.MarketplaceMatch = &match
			if !match {
				return validation, fmt.Errorf("seller store marketplace %q does not match ads marketplace %q", actual, expectedMarketplace)
			}
		} else {
			validation.Warnings = append(validation.Warnings, "seller store marketplace metadata unavailable; marketplace match could not be verified")
		}
	}
	if expectedAccount != "" {
		if actual, ok := sellerStoreMetaValue(db, "account", "account_id", "accountId", "profile_id", "profileId"); ok {
			match := actual == expectedAccount
			validation.AccountMatch = &match
			if !match {
				return validation, fmt.Errorf("seller store account/profile %q does not match ads profile %q", actual, expectedAccount)
			}
		} else {
			validation.Warnings = append(validation.Warnings, "seller store account metadata unavailable; account match could not be verified")
		}
	}
	if startDate != "" || endDate != "" {
		overlap, known := sellerStoreDateOverlap(db, startDate, endDate)
		if known {
			validation.DateOverlap = &overlap
			if !overlap {
				return validation, fmt.Errorf("seller store dates do not overlap ads report range %s..%s", startDate, endDate)
			}
		} else {
			validation.Warnings = append(validation.Warnings, "seller store date coverage unavailable; date overlap could not be verified")
		}
	}
	return validation, nil
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

func sellerStoreFreshness(db *sql.DB) time.Time {
	var best time.Time
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		return best
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			continue
		}
		for _, column := range sellerStoreColumns(db, table) {
			if column != "updated_at" && column != "updatedAt" && column != "created_at" && column != "createdAt" && column != "fetched_at" && column != "fetchedAt" {
				continue
			}
			var raw sql.NullString
			query := `SELECT MAX(` + quoteSQLiteIdent(column) + `) FROM ` + quoteSQLiteIdent(table)
			if err := db.QueryRow(query).Scan(&raw); err != nil || !raw.Valid {
				continue
			}
			if ts, ok := parseSellerDate(raw.String); ok && ts.After(best) {
				best = ts
			}
		}
	}
	if err := rows.Err(); err != nil {
		return best
	}
	return best
}

func sellerStoreColumns(db *sql.DB, table string) []string {
	rows, err := db.Query(`PRAGMA table_info(` + quoteSQLiteIdent(table) + `)`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var colName, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &colName, &colType, &notNull, &defaultValue, &pk); err == nil {
			cols = append(cols, colName)
		}
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return cols
}

func sellerStoreMetaValue(db *sql.DB, keys ...string) (string, bool) {
	for _, table := range []string{"metadata", "meta", "profiles", "accounts"} {
		if !sellerTableExists(db, table) {
			continue
		}
		cols := sellerStoreColumns(db, table)
		for _, key := range keys {
			if stringSliceContains(cols, key) {
				var value sql.NullString
				if err := db.QueryRow(`SELECT ` + quoteSQLiteIdent(key) + ` FROM ` + quoteSQLiteIdent(table) + ` WHERE ` + quoteSQLiteIdent(key) + ` IS NOT NULL LIMIT 1`).Scan(&value); err == nil && value.Valid && value.String != "" {
					return value.String, true
				}
			}
		}
		if stringSliceContains(cols, "key") && stringSliceContains(cols, "value") {
			for _, key := range keys {
				var value sql.NullString
				if err := db.QueryRow(`SELECT value FROM `+quoteSQLiteIdent(table)+` WHERE key = ? LIMIT 1`, key).Scan(&value); err == nil && value.Valid && value.String != "" {
					return value.String, true
				}
			}
		}
	}
	return "", false
}

func sellerStoreDateOverlap(db *sql.DB, startDate, endDate string) (bool, bool) {
	reportStart, _ := parseSellerDate(startDate)
	reportEnd, _ := parseSellerDate(endDate)
	if reportStart.IsZero() && reportEnd.IsZero() {
		return false, false
	}
	var minDate, maxDate time.Time
	for _, table := range []string{"orders", "resources", "reports"} {
		if !sellerTableExists(db, table) || !stringSliceContains(sellerStoreColumns(db, table), "data") {
			continue
		}
		tableMin, tableMax, ok := sellerStoreDateRange(db, table)
		if !ok {
			continue
		}
		if minDate.IsZero() || tableMin.Before(minDate) {
			minDate = tableMin
		}
		if maxDate.IsZero() || tableMax.After(maxDate) {
			maxDate = tableMax
		}
	}
	if minDate.IsZero() && maxDate.IsZero() {
		return false, false
	}
	if reportStart.IsZero() {
		reportStart = reportEnd
	}
	if reportEnd.IsZero() {
		reportEnd = reportStart
	}
	return !reportEnd.Before(minDate) && !reportStart.After(maxDate), true
}

func sellerStoreDateRange(db *sql.DB, table string) (time.Time, time.Time, bool) {
	query := `WITH extracted(value) AS (
	SELECT node.value
	FROM ` + quoteSQLiteIdent(table) + ` AS source,
		json_tree(CASE WHEN json_valid(source.data) THEN source.data ELSE '{}' END) AS node
	WHERE node.type = 'text'
		AND (
			lower(COALESCE(node.key, '')) LIKE '%date%'
			OR lower(COALESCE(node.key, '')) LIKE '%purchase%'
			OR lower(COALESCE(node.key, '')) LIKE '%lastupdate%'
		)
		AND node.value GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]*'
)
SELECT MIN(value), MAX(value) FROM extracted WHERE value IS NOT NULL AND TRIM(value) <> ''`
	var minRaw, maxRaw sql.NullString
	if err := db.QueryRow(query).Scan(&minRaw, &maxRaw); err != nil {
		return time.Time{}, time.Time{}, false
	}
	minDate, minOK := parseSellerDate(minRaw.String)
	maxDate, maxOK := parseSellerDate(maxRaw.String)
	if !minRaw.Valid || !maxRaw.Valid || !minOK || !maxOK {
		return time.Time{}, time.Time{}, false
	}
	return minDate, maxDate, true
}

func parseSellerDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

func sellerTableExists(db *sql.DB, table string) bool {
	var name string
	return db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name) == nil
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
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
