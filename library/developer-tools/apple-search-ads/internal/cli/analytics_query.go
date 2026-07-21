// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

// analyticsCacheRow is a row from the reporting_cache table (possibly aggregated).
type analyticsCacheRow struct {
	GroupKey    string  `json:"group_key"`
	Impressions int64   `json:"impressions"`
	Taps        int64   `json:"taps"`
	Installs    int64   `json:"installs"`
	Spend       float64 `json:"spend"`
	CPA         float64 `json:"cpa"`
	TTR         float64 `json:"ttr"` // taps/impressions*100 (Apple Search Ads definition)
	CVR         float64 `json:"cvr"` // installs/taps*100 (conversion rate)
}

// allowedGroupBy is a safelist for --group-by values (prevents SQL injection via column interpolation).
var allowedGroupBy = map[string]string{
	"campaign":    "campaign_name",
	"campaign_id": "campaign_id",
	"date":        "date",
	"granularity": "granularity",
}

// unsupportedGroupBy explains dimensions that are valid-sounding but not available in campaign-level reports.
var unsupportedGroupBy = map[string]string{
	"keyword_text": "keyword_text is not available in campaign-level reports (the sync-cache command pulls campaign granularity only); use --group-by campaign or campaign_id",
	"match_type":   "match_type is not available in campaign-level reports; use --group-by campaign or campaign_id",
}

func defaultAnalyticsDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "apple-search-ads-pp-cli", "data.db")
}

// analyticsDateRange returns YYYY-MM-DD start and end date strings for the last N days.
func analyticsDateRange(days int) (string, string) {
	end := time.Now().UTC()
	start := end.AddDate(0, 0, -days)
	return start.Format("2006-01-02"), end.Format("2006-01-02")
}

// reportingRow is a parsed row from the Apple Search Ads reporting API response.
type reportingRow struct {
	Date         string
	CampaignID   string
	CampaignName string
	Impressions  int64
	Taps         int64
	Installs     int64
	Spend        float64
	CPA          float64
	TTR          float64 // taps/impressions*100
	CVR          float64 // installs/taps*100
}

// parseReportingRows parses the Apple Search Ads /reports/campaigns response.
// The response structure is: {"data": {"reportingDataResponse": {"row": [...]}}}
// Each row has "metadata" (campaignId, campaignName) and "granularity" (array of day rows).
func parseReportingRows(data json.RawMessage) ([]reportingRow, error) {
	// Unwrap nested data structures.
	payload := data
	for _, key := range []string{"data", "reportingDataResponse"} {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(payload, &nested); err == nil {
			if v, ok := nested[key]; ok {
				payload = v
			}
		}
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(payload, &wrapper); err != nil {
		return nil, fmt.Errorf("unexpected response shape: %w", err)
	}

	rowsRaw, ok := wrapper["row"]
	if !ok {
		return nil, nil
	}

	var apiRows []map[string]json.RawMessage
	if err := json.Unmarshal(rowsRaw, &apiRows); err != nil {
		return nil, fmt.Errorf("parsing rows array: %w", err)
	}

	var results []reportingRow
	for _, apiRow := range apiRows {
		meta := analyticsExtractStringMap(apiRow["metadata"])
		granularityRows := analyticsExtractGranularityRows(apiRow["granularity"])
		campaignID := meta["campaignId"]
		campaignName := meta["campaignName"]
		for _, gr := range granularityRows {
			spend := analyticsParseFloat(gr["localSpend"])
			impressions := analyticsParseInt(gr["impressions"])
			taps := analyticsParseInt(gr["taps"])
			installs := analyticsParseInt(gr["installs"])
			row := reportingRow{
				Date:         gr["date"],
				CampaignID:   campaignID,
				CampaignName: campaignName,
				Impressions:  impressions,
				Taps:         taps,
				Installs:     installs,
				Spend:        spend,
			}
			if installs > 0 {
				row.CPA = spend / float64(installs)
			}
			if impressions > 0 {
				row.TTR = float64(taps) / float64(impressions) * 100
			}
			if taps > 0 {
				row.CVR = float64(installs) / float64(taps) * 100
			}
			results = append(results, row)
		}
	}
	return results, nil
}

func analyticsExtractStringMap(raw json.RawMessage) map[string]string {
	result := map[string]string{}
	if raw == nil {
		return result
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return result
	}
	for k, v := range m {
		switch val := v.(type) {
		case string:
			result[k] = val
		case float64:
			result[k] = fmt.Sprintf("%g", val)
		}
	}
	return result
}

func analyticsExtractGranularityRows(raw json.RawMessage) []map[string]string {
	if raw == nil {
		return nil
	}
	var arr []map[string]interface{}
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	var results []map[string]string
	for _, item := range arr {
		row := map[string]string{}
		for k, v := range item {
			switch val := v.(type) {
			case string:
				row[k] = val
			case float64:
				row[k] = fmt.Sprintf("%g", val)
			case map[string]interface{}:
				// localSpend: {"amount": "1.23", "currency": "USD"}
				if amt, ok := val["amount"].(string); ok {
					row[k] = amt
				} else if amt, ok := val["amount"].(float64); ok {
					row[k] = fmt.Sprintf("%g", amt)
				}
			}
		}
		results = append(results, row)
	}
	return results
}

func analyticsParseInt(s string) int64 {
	if s == "" {
		return 0
	}
	var n int64
	fmt.Sscanf(s, "%d", &n) // #nosec G104 -- zero-value fallback is correct for unparseable API fields
	return n
}

func analyticsParseFloat(s string) float64 {
	if s == "" {
		return 0
	}
	var f float64
	fmt.Sscanf(s, "%f", &f) // #nosec G104 -- zero-value fallback is correct for unparseable API fields
	return f
}

func newNovelAnalyticsSyncCacheCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagCampaignID string
	var flagGranularity string
	var flagDBPath string

	cmd := &cobra.Command{
		Use:   "sync-cache",
		Short: "Download reporting data from the API into a local SQLite cache",
		Long: `Pull Apple Search Ads campaign-level reporting metrics from the API and
store them in a local SQLite database. Once cached, use 'analytics query'
to run offline aggregations without consuming API quota.`,
		Example:     "  apple-search-ads-pp-cli analytics sync-cache --days 30\n  apple-search-ads-pp-cli analytics sync-cache --campaign-id 12345 --days 7",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flagGranularity != "daily" && flagGranularity != "weekly" && flagGranularity != "monthly" {
				return fmt.Errorf("--granularity must be daily, weekly, or monthly (got %q)", flagGranularity)
			}
			if flagDays < 1 || flagDays > 365 {
				return fmt.Errorf("--days must be between 1 and 365 (got %d)", flagDays)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			startDate, endDate := analyticsDateRange(flagDays)
			granLabel := strings.ToUpper(flagGranularity)

			reqBody := map[string]any{
				"startTime":   startDate,
				"endTime":     endDate,
				"granularity": granLabel,
				"selector": map[string]any{
					"orderBy": []map[string]string{
						{"field": "localSpend", "sortOrder": "DESCENDING"},
					},
					"pagination": map[string]int{"offset": 0, "limit": 1000},
				},
				"returnRowTotals":            false,
				"returnRecordsWithNoMetrics": false,
			}

			if flagCampaignID != "" {
				sel := reqBody["selector"].(map[string]any)
				sel["conditions"] = []map[string]any{
					{"field": "campaignId", "operator": "EQUALS", "values": []string{flagCampaignID}},
				}
			}

			data, _, err := c.Post(cmd.Context(), "/reports/campaigns", reqBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			rows, err := parseReportingRows(data)
			if err != nil {
				return fmt.Errorf("parsing reporting response: %w", err)
			}
			if len(rows) >= 1000 {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: reporting response hit the 1000-row limit; the cache may be incomplete — consider narrowing the date range or filtering by campaign")
			}

			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = defaultAnalyticsDBPath()
			}
			if err := os.MkdirAll(filepath.Dir(dbPath), 0o750); err != nil { // #nosec G301 -- user-owned data dir; group/other access is not needed
				return fmt.Errorf("creating db directory: %w", err)
			}

			db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
			if err != nil {
				return fmt.Errorf("opening analytics cache: %w", err)
			}
			defer db.Close()

			_, err = db.ExecContext(cmd.Context(), `CREATE TABLE IF NOT EXISTS reporting_cache (
				date TEXT NOT NULL, campaign_id TEXT NOT NULL, campaign_name TEXT NOT NULL DEFAULT '',
				impressions INTEGER NOT NULL DEFAULT 0, taps INTEGER NOT NULL DEFAULT 0,
				installs INTEGER NOT NULL DEFAULT 0, spend REAL NOT NULL DEFAULT 0.0,
				cpa REAL NOT NULL DEFAULT 0.0, ttr REAL NOT NULL DEFAULT 0.0,
				cvr REAL NOT NULL DEFAULT 0.0,
				granularity TEXT NOT NULL DEFAULT 'daily',
				synced_at TEXT NOT NULL DEFAULT (datetime('now')),
				PRIMARY KEY (date, campaign_id, granularity)
			)`)
			if err != nil {
				return fmt.Errorf("creating reporting_cache table: %w", err)
			}
			// Idempotent migration for databases created before the cvr column was added.
			_, _ = db.ExecContext(cmd.Context(), `ALTER TABLE reporting_cache ADD COLUMN cvr REAL NOT NULL DEFAULT 0.0`) // #nosec G104 -- error means column already exists

			upsertSQL := `INSERT INTO reporting_cache
				(date, campaign_id, campaign_name, impressions, taps, installs, spend, cpa, ttr, cvr, granularity, synced_at)
				VALUES (?,?,?,?,?,?,?,?,?,?,?,datetime('now'))
				ON CONFLICT(date, campaign_id, granularity) DO UPDATE SET
				campaign_name=excluded.campaign_name, impressions=excluded.impressions,
				taps=excluded.taps, installs=excluded.installs, spend=excluded.spend,
				cpa=excluded.cpa, ttr=excluded.ttr, cvr=excluded.cvr, synced_at=excluded.synced_at`

			tx, err := db.BeginTx(cmd.Context(), nil)
			if err != nil {
				return fmt.Errorf("beginning transaction: %w", err)
			}
			stmt, err := tx.PrepareContext(cmd.Context(), upsertSQL)
			if err != nil {
				_ = tx.Rollback() // #nosec G104 -- rollback error is secondary to the prepare error already being returned
				return fmt.Errorf("preparing upsert: %w", err)
			}
			defer stmt.Close()

			count := 0
			for _, row := range rows {
				if _, err := stmt.ExecContext(cmd.Context(),
					row.Date, row.CampaignID, row.CampaignName,
					row.Impressions, row.Taps, row.Installs,
					row.Spend, row.CPA, row.TTR, row.CVR,
					flagGranularity,
				); err != nil {
					_ = tx.Rollback() // #nosec G104 -- rollback error is secondary to the insert error already being returned
					return fmt.Errorf("inserting row: %w", err)
				}
				count++
			}
			if err := tx.Commit(); err != nil {
				return fmt.Errorf("committing transaction: %w", err)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":      "ok",
				"rows_synced": count,
				"days":        flagDays,
				"granularity": flagGranularity,
				"db_path":     dbPath,
			}, flags)
		},
	}

	cmd.Flags().IntVar(&flagDays, "days", 30, "Number of days of history to sync (1-365)")
	cmd.Flags().StringVar(&flagCampaignID, "campaign-id", "", "Optional: filter sync to a specific campaign ID")
	cmd.Flags().StringVar(&flagGranularity, "granularity", "daily", "Report granularity: daily, weekly, or monthly")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Path to SQLite cache (default: ~/.local/share/apple-search-ads-pp-cli/data.db)")
	return cmd
}

func newNovelAnalyticsQueryCmd(flags *rootFlags) *cobra.Command {
	var flagGroupBy string
	var flagLimit int
	var flagDays int
	var flagDBPath string

	cmd := &cobra.Command{
		Use:   "query",
		Short: "Query locally cached reporting data without API rate limit friction.",
		Long: `Run aggregated queries against the local reporting cache populated by
'analytics sync-cache'. Supports grouping by campaign, date, or granularity.
If the cache is empty, run 'analytics sync-cache' first.`,
		Example:     "  apple-search-ads-pp-cli analytics query --group-by campaign --limit 20\n  apple-search-ads-pp-cli analytics query --group-by date --days 7",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := flagDBPath
			if dbPath == "" {
				dbPath = defaultAnalyticsDBPath()
			}

			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				return fmt.Errorf("analytics cache not found at %s — run 'analytics sync-cache' first", dbPath)
			}

			db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(5000)")
			if err != nil {
				return fmt.Errorf("opening analytics cache: %w", err)
			}
			defer db.Close()

			var tableCount int
			if err := db.QueryRowContext(cmd.Context(),
				`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='reporting_cache'`).
				Scan(&tableCount); err != nil || tableCount == 0 {
				return fmt.Errorf("reporting_cache table not found — run 'analytics sync-cache' first")
			}

			groupCol := "campaign_name"
			if flagGroupBy != "" {
				key := strings.ToLower(flagGroupBy)
				if msg, unsupported := unsupportedGroupBy[key]; unsupported {
					return fmt.Errorf("%s", msg)
				}
				col, ok := allowedGroupBy[key]
				if !ok {
					allowed := make([]string, 0, len(allowedGroupBy))
					for k := range allowedGroupBy {
						allowed = append(allowed, k)
					}
					return fmt.Errorf("--group-by must be one of: %s", strings.Join(allowed, ", "))
				}
				groupCol = col
			}

			var querySQL string
			var queryArgs []any
			if flagDays > 0 {
				querySQL = fmt.Sprintf(`
					SELECT %s AS group_key,
					       SUM(impressions) AS impressions, SUM(taps) AS taps, SUM(installs) AS installs,
					       ROUND(SUM(spend),2) AS spend,
					       CASE WHEN SUM(installs)>0 THEN ROUND(SUM(spend)/SUM(installs),2) ELSE 0 END AS cpa,
					       CASE WHEN SUM(impressions)>0 THEN ROUND(CAST(SUM(taps) AS REAL)/SUM(impressions)*100,2) ELSE 0 END AS ttr,
					       CASE WHEN SUM(taps)>0 THEN ROUND(CAST(SUM(installs) AS REAL)/SUM(taps)*100,2) ELSE 0 END AS cvr
					FROM reporting_cache
					WHERE date >= date('now','-%d days')
					GROUP BY %s ORDER BY spend DESC LIMIT ?`, groupCol, flagDays, groupCol)
				queryArgs = []any{flagLimit}
			} else {
				querySQL = fmt.Sprintf(`
					SELECT %s AS group_key,
					       SUM(impressions) AS impressions, SUM(taps) AS taps, SUM(installs) AS installs,
					       ROUND(SUM(spend),2) AS spend,
					       CASE WHEN SUM(installs)>0 THEN ROUND(SUM(spend)/SUM(installs),2) ELSE 0 END AS cpa,
					       CASE WHEN SUM(impressions)>0 THEN ROUND(CAST(SUM(taps) AS REAL)/SUM(impressions)*100,2) ELSE 0 END AS ttr,
					       CASE WHEN SUM(taps)>0 THEN ROUND(CAST(SUM(installs) AS REAL)/SUM(taps)*100,2) ELSE 0 END AS cvr
					FROM reporting_cache
					GROUP BY %s ORDER BY spend DESC LIMIT ?`, groupCol, groupCol)
				queryArgs = []any{flagLimit}
			}

			sqlRows, err := db.QueryContext(cmd.Context(), querySQL, queryArgs...)
			if err != nil {
				return fmt.Errorf("querying analytics cache: %w", err)
			}
			defer sqlRows.Close()

			var results []analyticsCacheRow
			for sqlRows.Next() {
				var r analyticsCacheRow
				if err := sqlRows.Scan(&r.GroupKey, &r.Impressions, &r.Taps, &r.Installs, &r.Spend, &r.CPA, &r.TTR, &r.CVR); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				results = append(results, r)
			}
			if err := sqlRows.Err(); err != nil {
				return fmt.Errorf("iterating rows: %w", err)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No data in analytics cache — run 'analytics sync-cache' first")
				return printJSONFiltered(cmd.OutOrStdout(), []analyticsCacheRow{}, flags)
			}

			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}

	cmd.Flags().StringVar(&flagGroupBy, "group-by", "campaign", "Group results by: campaign, campaign_id, date, granularity")
	cmd.Flags().IntVar(&flagLimit, "limit", 20, "Maximum number of result rows")
	cmd.Flags().IntVar(&flagDays, "days", 0, "Filter to last N days (0 = all cached data)")
	cmd.Flags().StringVar(&flagDBPath, "db", "", "Path to SQLite cache (default: ~/.local/share/apple-search-ads-pp-cli/data.db)")
	return cmd
}
