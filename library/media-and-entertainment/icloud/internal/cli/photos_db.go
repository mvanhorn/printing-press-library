// Copyright 2026 Matias Sanchez Moises and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// PATCH(macos-only-guard): returns configErr on non-Darwin so the error is structured.
// CoreData timestamps count seconds since Jan 1 2001 00:00:00 UTC.
const coreDataEpoch int64 = 978307200

// Asset is one row from ZASSET joined with ZADDITIONALASSETATTRIBUTES.
type Asset struct {
	UUID      string
	Filename  string
	SizeBytes int64
	Kind      int // 0=image, 1=video
	Date      time.Time
}

func (a Asset) IsVideo() bool   { return a.Kind == 1 }
func (a Asset) SizeGB() float64 { return float64(a.SizeBytes) / (1 << 30) }
func (a Asset) SizeMB() float64 { return float64(a.SizeBytes) / (1 << 20) }

func (a Asset) TypeLabel() string {
	if a.IsVideo() {
		return "video"
	}
	return "photo"
}

// StorageRow summarises assets grouped by an arbitrary label.
type StorageRow struct {
	Label     string `json:"label"`
	Count     int64  `json:"count"`
	SizeBytes int64  `json:"size_bytes"`
}

func (r StorageRow) SizeGB() float64 { return float64(r.SizeBytes) / (1 << 30) }

func defaultLibraryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Pictures", "Photos Library.photoslibrary", "database", "Photos.sqlite")
}

func openPhotosDB(libraryPath string) (*sql.DB, error) {
	if runtime.GOOS != "darwin" {
		return nil, configErr(fmt.Errorf(
			"icloud-pp-cli requires macOS — this is %s", runtime.GOOS,
		))
	}
	if libraryPath == "" {
		libraryPath = defaultLibraryPath()
	}
	if _, err := os.Stat(libraryPath); err != nil {
		return nil, fmt.Errorf(
			"Photos library not found at %s\n\nUse --library to specify a custom path.",
			libraryPath,
		)
	}
	u := &url.URL{
		Scheme:   "file",
		Path:     libraryPath,
		RawQuery: "mode=ro&_busy_timeout=5000&_query_only=1",
	}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, fmt.Errorf("cannot open Photos library: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("cannot read Photos library: %w\n\nTry quitting Photos.app and running again.", err)
	}
	return db, nil
}

// queryLargestVideos returns up to limit videos sorted largest-first.
// File sizes and original filenames live in ZADDITIONALASSETATTRIBUTES.
func queryLargestVideos(db *sql.DB, limit int, year, month int) ([]Asset, error) {
	q := `
		SELECT
			COALESCE(a.ZUUID, ''),
			COALESCE(aa.ZORIGINALFILENAME, a.ZFILENAME, ''),
			COALESCE(aa.ZORIGINALFILESIZE, 0),
			COALESCE(a.ZKIND, 0),
			COALESCE(a.ZDATECREATED, 0)
		FROM ZASSET a
		JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
		WHERE a.ZKIND = 1
		  AND a.ZTRASHEDSTATE = 0
		  AND aa.ZORIGINALFILESIZE > 0
	`
	args := []any{}
	if year > 0 {
		q += fmt.Sprintf(" AND CAST(strftime('%%Y', datetime(a.ZDATECREATED + %d, 'unixepoch')) AS INTEGER) = ?", coreDataEpoch)
		args = append(args, year)
	}
	if month > 0 {
		q += fmt.Sprintf(" AND CAST(strftime('%%m', datetime(a.ZDATECREATED + %d, 'unixepoch')) AS INTEGER) = ?", coreDataEpoch)
		args = append(args, month)
	}
	q += " ORDER BY aa.ZORIGINALFILESIZE DESC"
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}

	return scanAssets(db, q, args...)
}

// queryStorageByType returns totals grouped by ZKIND (photo vs video).
func queryStorageByType(db *sql.DB) ([]StorageRow, error) {
	q := `
		SELECT
			CASE a.ZKIND WHEN 1 THEN 'video' ELSE 'photo' END,
			COUNT(*),
			SUM(COALESCE(aa.ZORIGINALFILESIZE, 0))
		FROM ZASSET a
		JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
		WHERE a.ZTRASHEDSTATE = 0
		GROUP BY a.ZKIND
		ORDER BY SUM(COALESCE(aa.ZORIGINALFILESIZE, 0)) DESC
	`
	return scanStorageRows(db, q)
}

// queryStorageByYear returns totals grouped by year.
func queryStorageByYear(db *sql.DB) ([]StorageRow, error) {
	q := fmt.Sprintf(`
		SELECT
			strftime('%%Y', datetime(a.ZDATECREATED + %d, 'unixepoch')),`, coreDataEpoch) + `
			COUNT(*),
			SUM(COALESCE(aa.ZORIGINALFILESIZE, 0))
		FROM ZASSET a
		JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
		WHERE a.ZTRASHEDSTATE = 0
		GROUP BY 1
		ORDER BY 1 DESC
	`
	return scanStorageRows(db, q)
}

// queryTopFiles returns the heaviest files across all (or a filtered) media type.
func queryTopFiles(db *sql.DB, limit int, mediaType string) ([]Asset, error) {
	kindFilter := ""
	switch mediaType {
	case "video":
		kindFilter = "AND a.ZKIND = 1"
	case "photo":
		kindFilter = "AND a.ZKIND = 0"
	}

	q := fmt.Sprintf(`
		SELECT
			COALESCE(a.ZUUID, ''),
			COALESCE(aa.ZORIGINALFILENAME, a.ZFILENAME, ''),
			COALESCE(aa.ZORIGINALFILESIZE, 0),
			COALESCE(a.ZKIND, 0),
			COALESCE(a.ZDATECREATED, 0)
		FROM ZASSET a
		JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
		WHERE a.ZTRASHEDSTATE = 0
		  AND aa.ZORIGINALFILESIZE > 0
		  %s
		ORDER BY aa.ZORIGINALFILESIZE DESC
	`, kindFilter)

	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	return scanAssets(db, q)
}

// querySensitiveAssets returns assets Apple's on-device ML has flagged as containing
// nudity (ZSCREENTIMEDEVICEIMAGESENSITIVITY = 1). Results are shuffled randomly so
// repeated calls with the same limit produce a varied sample. Pass limit=0 for all.
func querySensitiveAssets(db *sql.DB, limit int, mediaType string) ([]Asset, error) {
	kindFilter := ""
	switch mediaType {
	case "video":
		kindFilter = "AND a.ZKIND = 1"
	case "photo":
		kindFilter = "AND a.ZKIND = 0"
	}

	q := fmt.Sprintf(`
		SELECT
			COALESCE(a.ZUUID, ''),
			COALESCE(aa.ZORIGINALFILENAME, a.ZFILENAME, ''),
			COALESCE(aa.ZORIGINALFILESIZE, 0),
			COALESCE(a.ZKIND, 0),
			COALESCE(a.ZDATECREATED, 0)
		FROM ZASSET a
		JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
		JOIN ZMEDIAANALYSISASSETATTRIBUTES m ON m.ZASSET = a.Z_PK
		WHERE a.ZTRASHEDSTATE = 0
		  AND m.ZSCREENTIMEDEVICEIMAGESENSITIVITY = 1
		  %s
		ORDER BY RANDOM()
	`, kindFilter)
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	return scanAssets(db, q)
}

// queryByUUIDs returns assets matching the given UUIDs (used by the delete command).
// Batches requests in chunks of 999 to stay within SQLite's SQLITE_LIMIT_VARIABLE_NUMBER.
func queryByUUIDs(db *sql.DB, uuids []string) ([]Asset, error) {
	if len(uuids) == 0 {
		return nil, nil
	}
	const batchSize = 999
	var out []Asset
	for i := 0; i < len(uuids); i += batchSize {
		end := i + batchSize
		if end > len(uuids) {
			end = len(uuids)
		}
		batch := uuids[i:end]
		placeholders := make([]string, len(batch))
		args := make([]any, len(batch))
		for j, u := range batch {
			placeholders[j] = "?"
			args[j] = u
		}
		q := fmt.Sprintf(`
			SELECT
				COALESCE(a.ZUUID, ''),
				COALESCE(aa.ZORIGINALFILENAME, a.ZFILENAME, ''),
				COALESCE(aa.ZORIGINALFILESIZE, 0),
				COALESCE(a.ZKIND, 0),
				COALESCE(a.ZDATECREATED, 0)
			FROM ZASSET a
			JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
			WHERE a.ZUUID IN (%s)
			  AND a.ZTRASHEDSTATE = 0
		`, strings.Join(placeholders, ","))
		assets, err := scanAssets(db, q, args...)
		if err != nil {
			return nil, err
		}
		out = append(out, assets...)
	}
	return out, nil
}

// queryTotals returns a single summary row across all non-trashed assets.
func queryTotals(db *sql.DB) (count int64, sizeBytes int64, err error) {
	row := db.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(COALESCE(aa.ZORIGINALFILESIZE, 0)), 0)
		FROM ZASSET a
		JOIN ZADDITIONALASSETATTRIBUTES aa ON aa.ZASSET = a.Z_PK
		WHERE a.ZTRASHEDSTATE = 0
	`)
	err = row.Scan(&count, &sizeBytes)
	return
}

// ── internal helpers ──────────────────────────────────────────────────────────

func scanAssets(db *sql.DB, q string, args ...any) ([]Asset, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Asset
	for rows.Next() {
		var a Asset
		var created float64
		if err := rows.Scan(&a.UUID, &a.Filename, &a.SizeBytes, &a.Kind, &created); err != nil {
			return nil, fmt.Errorf("scan asset row: %w", err)
		}
		a.Date = time.Unix(int64(created)+coreDataEpoch, 0)
		out = append(out, a)
	}
	return out, rows.Err()
}

func scanStorageRows(db *sql.DB, q string, args ...any) ([]StorageRow, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StorageRow
	for rows.Next() {
		var r StorageRow
		if err := rows.Scan(&r.Label, &r.Count, &r.SizeBytes); err != nil {
			return nil, fmt.Errorf("scan storage row: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
