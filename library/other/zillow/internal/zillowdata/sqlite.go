package zillowdata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var forbiddenSQLPattern = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|REPLACE|CREATE|DROP|ALTER|ATTACH|DETACH|VACUUM|REINDEX|ANALYZE)\b`)

func OpenDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func OpenReadOnlyDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(2)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS zillow_regions (
	metric TEXT NOT NULL,
	region_id INTEGER NOT NULL,
	region_name TEXT NOT NULL,
	region_type TEXT,
	state_name TEXT,
	size_rank INTEGER,
	PRIMARY KEY(metric, region_id)
);
CREATE TABLE IF NOT EXISTS zillow_observations (
	metric TEXT NOT NULL,
	region_id INTEGER NOT NULL,
	observed_date TEXT NOT NULL,
	value REAL NOT NULL,
	PRIMARY KEY(metric, region_id, observed_date)
);
CREATE INDEX IF NOT EXISTS idx_zillow_observations_region_date
	ON zillow_observations(region_id, observed_date);
CREATE TABLE IF NOT EXISTS zillow_releases (
	metric TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	source_url TEXT NOT NULL,
	etag TEXT,
	last_modified TEXT,
	sha256 TEXT NOT NULL,
	PRIMARY KEY(metric, fetched_at)
);
CREATE TABLE IF NOT EXISTS zillow_watches (
	name TEXT PRIMARY KEY,
	region_query TEXT NOT NULL,
	metrics_json TEXT NOT NULL,
	last_snapshot_json TEXT,
	updated_at TEXT NOT NULL
);`)
	return err
}

func SaveTable(ctx context.Context, db *sql.DB, table *Table) error {
	if table == nil {
		return errors.New("nil table")
	}
	observationCount := 0
	for _, row := range table.Rows {
		observationCount += len(row.Values)
	}
	if len(table.Rows) == 0 || observationCount == 0 {
		return fmt.Errorf("refusing to replace %q with an empty dataset", table.Dataset.Key)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM zillow_regions WHERE metric = ?`, table.Dataset.Key); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM zillow_observations WHERE metric = ?`, table.Dataset.Key); err != nil {
		return err
	}
	regionStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO zillow_regions(metric, region_id, region_name, region_type, state_name, size_rank)
		VALUES(?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer regionStmt.Close()
	observationStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO zillow_observations(metric, region_id, observed_date, value)
		VALUES(?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer observationStmt.Close()
	for _, row := range table.Rows {
		if _, err := regionStmt.ExecContext(ctx, table.Dataset.Key, row.RegionID, row.RegionName, row.RegionType, row.StateName, row.SizeRank); err != nil {
			return err
		}
		for date, value := range row.Values {
			if _, err := observationStmt.ExecContext(ctx, table.Dataset.Key, row.RegionID, date.Format("2006-01-02"), value); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO zillow_releases(metric, fetched_at, source_url, etag, last_modified, sha256)
		VALUES(?, ?, ?, ?, ?, ?)`,
		table.Dataset.Key, table.FetchedAt.Format(time.RFC3339Nano), table.SourceURL,
		table.ETag, table.LastModified, table.SHA256); err != nil {
		return err
	}
	return tx.Commit()
}

func QueryReadOnly(ctx context.Context, db *sql.DB, query string, maxRows int) ([]map[string]any, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("query is required")
	}
	trimmed := strings.TrimSuffix(query, ";")
	if strings.Contains(trimmed, ";") {
		return nil, errors.New("multiple SQL statements are not allowed")
	}
	first := strings.ToUpper(strings.Fields(trimmed)[0])
	switch first {
	case "SELECT", "WITH", "EXPLAIN":
	case "PRAGMA":
		upper := strings.ToUpper(trimmed)
		allowed := strings.HasPrefix(upper, "PRAGMA TABLE_INFO") ||
			strings.HasPrefix(upper, "PRAGMA INDEX_LIST") ||
			strings.HasPrefix(upper, "PRAGMA DATABASE_LIST") ||
			upper == "PRAGMA USER_VERSION" ||
			upper == "PRAGMA QUERY_ONLY"
		if !allowed {
			return nil, errors.New("only read-only schema PRAGMAs are allowed")
		}
	default:
		return nil, fmt.Errorf("read-only SQL only; %s is not allowed", first)
	}
	if keyword := forbiddenSQLPattern.FindString(trimmed); keyword != "" {
		return nil, fmt.Errorf("read-only SQL only; %s is not allowed", strings.ToUpper(keyword))
	}
	if maxRows <= 0 || maxRows > 10000 {
		maxRows = 1000
	}
	rows, err := db.QueryContext(ctx, trimmed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, min(maxRows, 100))
	for rows.Next() && len(out) < maxRows {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		record := make(map[string]any, len(columns))
		for i, column := range columns {
			value := values[i]
			if bytes, ok := value.([]byte); ok {
				value = string(bytes)
			}
			record[column] = value
		}
		out = append(out, record)
	}
	return out, rows.Err()
}

type Watch struct {
	Name         string             `json:"name"`
	Region       string             `json:"region"`
	Metrics      []string           `json:"metrics"`
	LastSnapshot map[string]float64 `json:"last_snapshot,omitempty"`
	UpdatedAt    time.Time          `json:"updated_at"`
}

func SaveWatch(ctx context.Context, db *sql.DB, watch Watch) error {
	metrics, _ := json.Marshal(watch.Metrics)
	snapshot, _ := json.Marshal(watch.LastSnapshot)
	_, err := db.ExecContext(ctx, `
		INSERT INTO zillow_watches(name, region_query, metrics_json, last_snapshot_json, updated_at)
		VALUES(?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			region_query=excluded.region_query,
			metrics_json=excluded.metrics_json,
			last_snapshot_json=excluded.last_snapshot_json,
			updated_at=excluded.updated_at`,
		watch.Name, watch.Region, string(metrics), nullableJSON(snapshot, watch.LastSnapshot != nil), watch.UpdatedAt.Format(time.RFC3339Nano))
	return err
}

func nullableJSON(data []byte, present bool) any {
	if !present {
		return nil
	}
	return string(data)
}

func LoadWatch(ctx context.Context, db *sql.DB, name string) (Watch, error) {
	var watch Watch
	var metricsText string
	var snapshotText sql.NullString
	var updated string
	err := db.QueryRowContext(ctx, `
		SELECT name, region_query, metrics_json, last_snapshot_json, updated_at
		FROM zillow_watches WHERE name = ?`, name).
		Scan(&watch.Name, &watch.Region, &metricsText, &snapshotText, &updated)
	if err != nil {
		return Watch{}, err
	}
	_ = json.Unmarshal([]byte(metricsText), &watch.Metrics)
	if snapshotText.Valid {
		_ = json.Unmarshal([]byte(snapshotText.String), &watch.LastSnapshot)
	}
	watch.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return watch, nil
}

func ListWatches(ctx context.Context, db *sql.DB) ([]Watch, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM zillow_watches ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Watch
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		watch, err := LoadWatch(ctx, db, name)
		if err != nil {
			return nil, err
		}
		out = append(out, watch)
	}
	return out, rows.Err()
}
