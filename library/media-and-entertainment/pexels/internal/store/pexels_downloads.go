// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import "database/sql"

// PexelsDownload is one row in the local download ledger. It records enough
// provenance to regenerate attribution (SOURCES.md, .meta.json sidecars)
// without re-hitting the API.
type PexelsDownload struct {
	MediaID         int64
	MediaType       string
	Query           string
	Photographer    string
	PhotographerURL string
	PageURL         string
	SrcURL          string
	FilePath        string
	AvgColor        string
	Alt             string
	DownloadedAt    string
}

// EnsurePexelsDownloads creates the download ledger table if it is absent.
func EnsurePexelsDownloads(db *sql.DB) error {
	const ddl = `CREATE TABLE IF NOT EXISTS pexels_downloads (
		media_id INTEGER NOT NULL,
		media_type TEXT NOT NULL,
		query TEXT,
		photographer TEXT,
		photographer_url TEXT,
		page_url TEXT,
		src_url TEXT,
		file_path TEXT,
		avg_color TEXT,
		alt TEXT,
		downloaded_at TEXT,
		PRIMARY KEY (media_id, media_type)
	)`
	_, err := db.Exec(ddl)
	return err
}

// InsertPexelsDownload upserts a download row (INSERT OR REPLACE on the
// composite primary key).
func InsertPexelsDownload(db *sql.DB, d PexelsDownload) error {
	const q = `INSERT OR REPLACE INTO pexels_downloads (
		media_id, media_type, query, photographer, photographer_url,
		page_url, src_url, file_path, avg_color, alt, downloaded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(q,
		d.MediaID, d.MediaType, d.Query, d.Photographer, d.PhotographerURL,
		d.PageURL, d.SrcURL, d.FilePath, d.AvgColor, d.Alt, d.DownloadedAt,
	)
	return err
}

// PexelsDownloadExists reports whether a row for (mediaID, mediaType) is
// already recorded.
func PexelsDownloadExists(db *sql.DB, mediaID int64, mediaType string) (bool, error) {
	const q = `SELECT 1 FROM pexels_downloads WHERE media_id = ? AND media_type = ? LIMIT 1`
	var one int
	err := db.QueryRow(q, mediaID, mediaType).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// AllPexelsDownloads returns ledger rows ordered by downloaded_at. When
// mediaTypes is non-empty the result is filtered to those types.
func AllPexelsDownloads(db *sql.DB, mediaTypes []string) ([]PexelsDownload, error) {
	q := `SELECT media_id, media_type, query, photographer, photographer_url,
		page_url, src_url, file_path, avg_color, alt, downloaded_at
		FROM pexels_downloads`
	var args []any
	if len(mediaTypes) > 0 {
		q += " WHERE media_type IN ("
		for i, t := range mediaTypes {
			if i > 0 {
				q += ", "
			}
			q += "?"
			args = append(args, t)
		}
		q += ")"
	}
	q += " ORDER BY downloaded_at"

	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]PexelsDownload, 0)
	for rows.Next() {
		var (
			d                                             PexelsDownload
			query, photographer, photographerURL, pageURL sql.NullString
			srcURL, filePath, avgColor, alt, downloadedAt sql.NullString
		)
		if err := rows.Scan(
			&d.MediaID, &d.MediaType, &query, &photographer, &photographerURL,
			&pageURL, &srcURL, &filePath, &avgColor, &alt, &downloadedAt,
		); err != nil {
			return nil, err
		}
		d.Query = query.String
		d.Photographer = photographer.String
		d.PhotographerURL = photographerURL.String
		d.PageURL = pageURL.String
		d.SrcURL = srcURL.String
		d.FilePath = filePath.String
		d.AvgColor = avgColor.String
		d.Alt = alt.String
		d.DownloadedAt = downloadedAt.String
		out = append(out, d)
	}
	return out, rows.Err()
}
