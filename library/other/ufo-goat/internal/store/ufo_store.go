// Package store — custom UFO data layer methods.
// These extend the generated store with typed queries for the UAP file manifest.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// UFOFile represents a fully typed UAP file record.
type UFOFile struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Type             string `json:"type"`
	Agency           string `json:"agency"`
	ReleaseDate      string `json:"release_date"`
	IncidentDate     string `json:"incident_date"`
	ParsedDate       string `json:"parsed_date,omitempty"`
	IncidentLocation string `json:"incident_location"`
	Description      string `json:"description"`
	Redacted         bool   `json:"redacted"`
	DownloadURL      string `json:"download_url"`
	ThumbnailURL     string `json:"thumbnail_url,omitempty"`
	DVIDSVideoID     string `json:"dvids_video_id,omitempty"`
	VideoTitle       string `json:"video_title,omitempty"`
	VideoPairing     string `json:"video_pairing,omitempty"`
	PDFPairing       string `json:"pdf_pairing,omitempty"`
	ModalImage       string `json:"modal_image,omitempty"`
	PDFImageLink     string `json:"pdf_image_link,omitempty"`
	SyncedAt         string `json:"synced_at,omitempty"`
	Downloaded       bool   `json:"downloaded,omitempty"`
}

// FileFilter holds filter criteria for listing files.
type FileFilter struct {
	Agency   string
	Type     string
	Location string
	After    string // parsed date >= after
	Before   string // parsed date <= before
	Redacted *bool
	Limit    int
}

// LocationSummary aggregates incidents by location.
type LocationSummary struct {
	Location  string   `json:"location"`
	Count     int      `json:"count"`
	Agencies  []string `json:"agencies"`
	DateRange string   `json:"date_range"`
}

// FilePair represents a video-PDF pairing.
type FilePair struct {
	VideoID    string `json:"video_id"`
	VideoTitle string `json:"video_title"`
	PDFID      string `json:"pdf_id"`
	PDFTitle   string `json:"pdf_title"`
	Agency     string `json:"agency"`
}

// EnsureUFOSchema creates the extended schema for UFO file data.
// Called during sync to add columns and FTS5 index.
func (s *Store) EnsureUFOSchema() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Use a pinned connection so all DDL runs on the same conn
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring connection for schema: %w", err)
	}
	defer conn.Close()

	columns := []struct{ name, decl string }{
		{"title", "TEXT"},
		{"release_date", "TEXT"},
		{"incident_date", "TEXT"},
		{"parsed_date", "TEXT"},
		{"incident_location", "TEXT"},
		{"description", "TEXT"},
		{"download_url", "TEXT"},
		{"thumbnail_url", "TEXT"},
		{"dvids_video_id", "TEXT"},
		{"video_title", "TEXT"},
		{"video_pairing", "TEXT"},
		{"pdf_pairing", "TEXT"},
		{"modal_image", "TEXT"},
		{"pdf_image_link", "TEXT"},
		{"downloaded", "INTEGER DEFAULT 0"},
	}

	// Use PRAGMA table_info on the pinned connection
	existingCols := make(map[string]bool)
	pragmaRows, pragmaErr := conn.QueryContext(ctx, `PRAGMA table_info(files)`)
	if pragmaErr == nil {
		for pragmaRows.Next() {
			var cid int
			var name, typ string
			var notnull, pk int
			var dflt sql.NullString
			if scanErr := pragmaRows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); scanErr != nil {
				continue
			}
			existingCols[name] = true
		}
		pragmaRows.Close()
	}

	for _, col := range columns {
		if existingCols[col.name] {
			continue
		}
		stmt := fmt.Sprintf(`ALTER TABLE files ADD COLUMN "%s" %s`, col.name, col.decl)
		_, alterErr := conn.ExecContext(ctx, stmt)
		if alterErr != nil {
			if !strings.Contains(alterErr.Error(), "duplicate column") {
				return fmt.Errorf("adding column %s: %w", col.name, alterErr)
			}
		}
	}

	// Create FTS5 virtual table for full-text search (standalone, not content-synced)
	conn.ExecContext(ctx, `DROP TABLE IF EXISTS files_fts`)
	_, ftsErr := conn.ExecContext(ctx, `CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(
		id, title, description, incident_location, agency, video_title,
		tokenize='porter unicode61'
	)`)
	if ftsErr != nil {
		if !strings.Contains(ftsErr.Error(), "already exists") {
			return fmt.Errorf("creating FTS5 index: %w", ftsErr)
		}
	}

	return nil
}

// UpsertUFOFile inserts or updates a typed UAP file record.
func (s *Store) UpsertUFOFile(f UFOFile) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Marshal full record as JSON for the data column
	data, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshaling file: %w", err)
	}

	redacted := 0
	if f.Redacted {
		redacted = 1
	}

	_, err = tx.Exec(
		`INSERT INTO files (id, data, synced_at, agency, type, location, redacted, title, release_date, incident_date, parsed_date, incident_location, description, download_url, thumbnail_url, dvids_video_id, video_title, video_pairing, pdf_pairing, modal_image, pdf_image_link, downloaded)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   data = excluded.data, synced_at = excluded.synced_at, agency = excluded.agency,
		   type = excluded.type, location = excluded.location, redacted = excluded.redacted,
		   title = excluded.title, release_date = excluded.release_date,
		   incident_date = excluded.incident_date, parsed_date = excluded.parsed_date,
		   incident_location = excluded.incident_location, description = excluded.description,
		   download_url = excluded.download_url, thumbnail_url = excluded.thumbnail_url,
		   dvids_video_id = excluded.dvids_video_id, video_title = excluded.video_title,
		   video_pairing = excluded.video_pairing, pdf_pairing = excluded.pdf_pairing,
		   modal_image = excluded.modal_image, pdf_image_link = excluded.pdf_image_link,
		   downloaded = excluded.downloaded`,
		f.ID, string(data), time.Now().Format(time.RFC3339),
		f.Agency, f.Type, f.IncidentLocation, redacted,
		f.Title, f.ReleaseDate, f.IncidentDate, f.ParsedDate,
		f.IncidentLocation, f.Description, f.DownloadURL, f.ThumbnailURL,
		f.DVIDSVideoID, f.VideoTitle, f.VideoPairing, f.PDFPairing,
		f.ModalImage, f.PDFImageLink, 0,
	)
	if err != nil {
		return fmt.Errorf("upserting file: %w", err)
	}

	// Also upsert into the generic resources table for compatibility
	if err := s.upsertGenericResourceTx(tx, "files", f.ID, json.RawMessage(data)); err != nil {
		return fmt.Errorf("upserting generic resource: %w", err)
	}

	return tx.Commit()
}

// UpsertUFOFileBatch inserts multiple files in a single transaction.
func (s *Store) UpsertUFOFileBatch(files []UFOFile) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	count := 0
	for _, f := range files {
		data, err := json.Marshal(f)
		if err != nil {
			continue
		}

		redacted := 0
		if f.Redacted {
			redacted = 1
		}

		_, err = tx.Exec(
			`INSERT INTO files (id, data, synced_at, agency, type, location, redacted, title, release_date, incident_date, parsed_date, incident_location, description, download_url, thumbnail_url, dvids_video_id, video_title, video_pairing, pdf_pairing, modal_image, pdf_image_link, downloaded)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET
			   data = excluded.data, synced_at = excluded.synced_at, agency = excluded.agency,
			   type = excluded.type, location = excluded.location, redacted = excluded.redacted,
			   title = excluded.title, release_date = excluded.release_date,
			   incident_date = excluded.incident_date, parsed_date = excluded.parsed_date,
			   incident_location = excluded.incident_location, description = excluded.description,
			   download_url = excluded.download_url, thumbnail_url = excluded.thumbnail_url,
			   dvids_video_id = excluded.dvids_video_id, video_title = excluded.video_title,
			   video_pairing = excluded.video_pairing, pdf_pairing = excluded.pdf_pairing,
			   modal_image = excluded.modal_image, pdf_image_link = excluded.pdf_image_link,
			   downloaded = excluded.downloaded`,
			f.ID, string(data), time.Now().Format(time.RFC3339),
			f.Agency, f.Type, f.IncidentLocation, redacted,
			f.Title, f.ReleaseDate, f.IncidentDate, f.ParsedDate,
			f.IncidentLocation, f.Description, f.DownloadURL, f.ThumbnailURL,
			f.DVIDSVideoID, f.VideoTitle, f.VideoPairing, f.PDFPairing,
			f.ModalImage, f.PDFImageLink, 0,
		)
		if err != nil {
			continue
		}

		// Also upsert generic resource
		_ = s.upsertGenericResourceTx(tx, "files", f.ID, json.RawMessage(data))
		count++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

// RebuildFTS rebuilds the FTS5 index from the files table.
func (s *Store) RebuildFTS() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// Drop and recreate the FTS table to ensure clean index
	s.db.Exec(`DROP TABLE IF EXISTS files_fts`)
	_, err := s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS files_fts USING fts5(
		id, title, description, incident_location, agency, video_title,
		tokenize='porter unicode61'
	)`)
	if err != nil {
		return fmt.Errorf("creating FTS5 table: %w", err)
	}

	// Populate from files table
	_, err = s.db.Exec(`INSERT INTO files_fts (id, title, description, incident_location, agency, video_title)
		SELECT id, COALESCE(title,''), COALESCE(description,''), COALESCE(incident_location,''), COALESCE(agency,''), COALESCE(video_title,'')
		FROM files`)
	if err != nil {
		return fmt.Errorf("populating FTS5 index: %w", err)
	}

	return nil
}

// ListUFOFiles returns files matching the given filter.
func (s *Store) ListUFOFiles(filter FileFilter) ([]UFOFile, error) {
	query := `SELECT id, COALESCE(title,''), COALESCE(type,''), COALESCE(agency,''),
		COALESCE(release_date,''), COALESCE(incident_date,''), COALESCE(parsed_date,''),
		COALESCE(incident_location,''), COALESCE(description,''), COALESCE(redacted,0),
		COALESCE(download_url,''), COALESCE(thumbnail_url,''), COALESCE(dvids_video_id,''),
		COALESCE(video_title,''), COALESCE(video_pairing,''), COALESCE(pdf_pairing,''),
		COALESCE(modal_image,''), COALESCE(pdf_image_link,''), COALESCE(synced_at,''),
		COALESCE(downloaded,0)
		FROM files WHERE 1=1`
	var args []any

	if filter.Agency != "" {
		query += ` AND LOWER(agency) = LOWER(?)`
		args = append(args, filter.Agency)
	}
	if filter.Type != "" {
		query += ` AND LOWER(type) = LOWER(?)`
		args = append(args, filter.Type)
	}
	if filter.Location != "" {
		query += ` AND LOWER(incident_location) LIKE LOWER(?)`
		args = append(args, "%"+filter.Location+"%")
	}
	if filter.After != "" {
		query += ` AND parsed_date >= ?`
		args = append(args, filter.After)
	}
	if filter.Before != "" {
		query += ` AND parsed_date <= ?`
		args = append(args, filter.Before)
	}
	if filter.Redacted != nil {
		if *filter.Redacted {
			query += ` AND redacted = 1`
		} else {
			query += ` AND (redacted = 0 OR redacted IS NULL)`
		}
	}

	query += ` ORDER BY COALESCE(NULLIF(parsed_date,''), '9999-99-99') ASC, title ASC`

	if filter.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, filter.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing files: %w", err)
	}
	defer rows.Close()

	return scanUFOFiles(rows)
}

// GetUFOFileByID returns a single file by exact ID or partial title match.
func (s *Store) GetUFOFileByID(idOrTitle string) (*UFOFile, error) {
	// Try exact ID match first
	row := s.db.QueryRow(
		`SELECT id, COALESCE(title,''), COALESCE(type,''), COALESCE(agency,''),
		COALESCE(release_date,''), COALESCE(incident_date,''), COALESCE(parsed_date,''),
		COALESCE(incident_location,''), COALESCE(description,''), COALESCE(redacted,0),
		COALESCE(download_url,''), COALESCE(thumbnail_url,''), COALESCE(dvids_video_id,''),
		COALESCE(video_title,''), COALESCE(video_pairing,''), COALESCE(pdf_pairing,''),
		COALESCE(modal_image,''), COALESCE(pdf_image_link,''), COALESCE(synced_at,''),
		COALESCE(downloaded,0)
		FROM files WHERE id = ?`, idOrTitle)

	f, err := scanSingleUFOFile(row)
	if err == nil {
		return f, nil
	}

	// Try partial ID match
	row = s.db.QueryRow(
		`SELECT id, COALESCE(title,''), COALESCE(type,''), COALESCE(agency,''),
		COALESCE(release_date,''), COALESCE(incident_date,''), COALESCE(parsed_date,''),
		COALESCE(incident_location,''), COALESCE(description,''), COALESCE(redacted,0),
		COALESCE(download_url,''), COALESCE(thumbnail_url,''), COALESCE(dvids_video_id,''),
		COALESCE(video_title,''), COALESCE(video_pairing,''), COALESCE(pdf_pairing,''),
		COALESCE(modal_image,''), COALESCE(pdf_image_link,''), COALESCE(synced_at,''),
		COALESCE(downloaded,0)
		FROM files WHERE id LIKE ?`, idOrTitle+"%")

	f, err = scanSingleUFOFile(row)
	if err == nil {
		return f, nil
	}

	// Try partial title match
	row = s.db.QueryRow(
		`SELECT id, COALESCE(title,''), COALESCE(type,''), COALESCE(agency,''),
		COALESCE(release_date,''), COALESCE(incident_date,''), COALESCE(parsed_date,''),
		COALESCE(incident_location,''), COALESCE(description,''), COALESCE(redacted,0),
		COALESCE(download_url,''), COALESCE(thumbnail_url,''), COALESCE(dvids_video_id,''),
		COALESCE(video_title,''), COALESCE(video_pairing,''), COALESCE(pdf_pairing,''),
		COALESCE(modal_image,''), COALESCE(pdf_image_link,''), COALESCE(synced_at,''),
		COALESCE(downloaded,0)
		FROM files WHERE LOWER(title) LIKE LOWER(?)`, "%"+idOrTitle+"%")

	f, err = scanSingleUFOFile(row)
	if err != nil {
		return nil, fmt.Errorf("file %q not found", idOrTitle)
	}
	return f, nil
}

// SearchUFOFiles performs full-text search across titles, descriptions, locations.
func (s *Store) SearchUFOFiles(query string, limit int) ([]UFOFile, error) {
	if limit <= 0 {
		limit = 50
	}

	// Try FTS5 first
	rows, err := s.db.Query(
		`SELECT f.id, COALESCE(f.title,''), COALESCE(f.type,''), COALESCE(f.agency,''),
		COALESCE(f.release_date,''), COALESCE(f.incident_date,''), COALESCE(f.parsed_date,''),
		COALESCE(f.incident_location,''), COALESCE(f.description,''), COALESCE(f.redacted,0),
		COALESCE(f.download_url,''), COALESCE(f.thumbnail_url,''), COALESCE(f.dvids_video_id,''),
		COALESCE(f.video_title,''), COALESCE(f.video_pairing,''), COALESCE(f.pdf_pairing,''),
		COALESCE(f.modal_image,''), COALESCE(f.pdf_image_link,''), COALESCE(f.synced_at,''),
		COALESCE(f.downloaded,0)
		FROM files_fts fts
		JOIN files f ON fts.id = f.id
		WHERE files_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		// Fallback to LIKE search if FTS fails
		return s.searchUFOFilesLike(query, limit)
	}
	defer rows.Close()

	files, err := scanUFOFiles(rows)
	if err != nil || len(files) == 0 {
		// Fallback to LIKE search
		return s.searchUFOFilesLike(query, limit)
	}
	return files, nil
}

func (s *Store) searchUFOFilesLike(query string, limit int) ([]UFOFile, error) {
	pattern := "%" + query + "%"
	rows, err := s.db.Query(
		`SELECT id, COALESCE(title,''), COALESCE(type,''), COALESCE(agency,''),
		COALESCE(release_date,''), COALESCE(incident_date,''), COALESCE(parsed_date,''),
		COALESCE(incident_location,''), COALESCE(description,''), COALESCE(redacted,0),
		COALESCE(download_url,''), COALESCE(thumbnail_url,''), COALESCE(dvids_video_id,''),
		COALESCE(video_title,''), COALESCE(video_pairing,''), COALESCE(pdf_pairing,''),
		COALESCE(modal_image,''), COALESCE(pdf_image_link,''), COALESCE(synced_at,''),
		COALESCE(downloaded,0)
		FROM files
		WHERE LOWER(title) LIKE LOWER(?)
		   OR LOWER(description) LIKE LOWER(?)
		   OR LOWER(incident_location) LIKE LOWER(?)
		   OR LOWER(video_title) LIKE LOWER(?)
		   OR LOWER(agency) LIKE LOWER(?)
		ORDER BY title ASC
		LIMIT ?`, pattern, pattern, pattern, pattern, pattern, limit)
	if err != nil {
		return nil, fmt.Errorf("searching files: %w", err)
	}
	defer rows.Close()
	return scanUFOFiles(rows)
}

// GetTimeline returns files ordered by parsed incident date.
func (s *Store) GetTimeline(after, before string) ([]UFOFile, error) {
	query := `SELECT id, COALESCE(title,''), COALESCE(type,''), COALESCE(agency,''),
		COALESCE(release_date,''), COALESCE(incident_date,''), COALESCE(parsed_date,''),
		COALESCE(incident_location,''), COALESCE(description,''), COALESCE(redacted,0),
		COALESCE(download_url,''), COALESCE(thumbnail_url,''), COALESCE(dvids_video_id,''),
		COALESCE(video_title,''), COALESCE(video_pairing,''), COALESCE(pdf_pairing,''),
		COALESCE(modal_image,''), COALESCE(pdf_image_link,''), COALESCE(synced_at,''),
		COALESCE(downloaded,0)
		FROM files WHERE parsed_date != '' AND parsed_date IS NOT NULL`
	var args []any

	if after != "" {
		query += ` AND parsed_date >= ?`
		args = append(args, after)
	}
	if before != "" {
		query += ` AND parsed_date <= ?`
		args = append(args, before)
	}

	query += ` ORDER BY parsed_date ASC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying timeline: %w", err)
	}
	defer rows.Close()
	return scanUFOFiles(rows)
}

// GetPairs returns video-PDF pairings.
func (s *Store) GetPairs() ([]FilePair, error) {
	// Find files that have video_pairing or pdf_pairing set
	rows, err := s.db.Query(
		`SELECT v.id, COALESCE(v.title,''), p.id, COALESCE(p.title,''), COALESCE(v.agency,'')
		FROM files v
		JOIN files p ON (
			(v.video_pairing != '' AND LOWER(v.video_pairing) = LOWER(p.title))
			OR (v.pdf_pairing != '' AND LOWER(v.pdf_pairing) = LOWER(p.title))
			OR (p.video_pairing != '' AND LOWER(p.video_pairing) = LOWER(v.title))
			OR (p.pdf_pairing != '' AND LOWER(p.pdf_pairing) = LOWER(v.title))
		)
		WHERE v.type = 'VID' AND p.type = 'PDF'
		GROUP BY v.id, p.id`)
	if err != nil {
		// Fallback: show files that have pairings listed
		return s.getPairsFallback()
	}
	defer rows.Close()

	var pairs []FilePair
	for rows.Next() {
		var p FilePair
		if err := rows.Scan(&p.VideoID, &p.VideoTitle, &p.PDFID, &p.PDFTitle, &p.Agency); err != nil {
			continue
		}
		pairs = append(pairs, p)
	}

	if len(pairs) == 0 {
		return s.getPairsFallback()
	}
	return pairs, rows.Err()
}

func (s *Store) getPairsFallback() ([]FilePair, error) {
	// List files that have video_pairing or pdf_pairing fields set
	rows, err := s.db.Query(
		`SELECT id, COALESCE(title,''), COALESCE(type,''), COALESCE(agency,''),
		 COALESCE(video_pairing,''), COALESCE(pdf_pairing,'')
		 FROM files
		 WHERE (video_pairing != '' AND video_pairing IS NOT NULL)
		    OR (pdf_pairing != '' AND pdf_pairing IS NOT NULL)
		 ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("querying pairs: %w", err)
	}
	defer rows.Close()

	var pairs []FilePair
	for rows.Next() {
		var id, title, ftype, agency, videoPairing, pdfPairing string
		if err := rows.Scan(&id, &title, &ftype, &agency, &videoPairing, &pdfPairing); err != nil {
			continue
		}
		if ftype == "VID" && pdfPairing != "" {
			pairs = append(pairs, FilePair{
				VideoID:    id,
				VideoTitle: title,
				PDFTitle:   pdfPairing,
				Agency:     agency,
			})
		} else if ftype == "PDF" && videoPairing != "" {
			pairs = append(pairs, FilePair{
				PDFTitle:   title,
				PDFID:      id,
				VideoTitle: videoPairing,
				Agency:     agency,
			})
		}
	}
	return pairs, rows.Err()
}

// GetLocations aggregates incidents by location.
func (s *Store) GetLocations() ([]LocationSummary, error) {
	rows, err := s.db.Query(
		`SELECT COALESCE(incident_location,'Unknown') as loc,
		 COUNT(*) as cnt,
		 GROUP_CONCAT(DISTINCT agency) as agencies,
		 MIN(parsed_date) as min_date,
		 MAX(parsed_date) as max_date
		 FROM files
		 WHERE incident_location != '' AND incident_location IS NOT NULL
		 GROUP BY LOWER(incident_location)
		 ORDER BY cnt DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying locations: %w", err)
	}
	defer rows.Close()

	var locations []LocationSummary
	for rows.Next() {
		var loc, agenciesStr string
		var cnt int
		var minDate, maxDate sql.NullString
		if err := rows.Scan(&loc, &cnt, &agenciesStr, &minDate, &maxDate); err != nil {
			continue
		}
		agencies := strings.Split(agenciesStr, ",")
		dateRange := ""
		if minDate.Valid && maxDate.Valid && minDate.String != "" && maxDate.String != "" {
			if minDate.String == maxDate.String {
				dateRange = minDate.String
			} else {
				dateRange = minDate.String + " to " + maxDate.String
			}
		}
		locations = append(locations, LocationSummary{
			Location:  loc,
			Count:     cnt,
			Agencies:  agencies,
			DateRange: dateRange,
		})
	}
	return locations, rows.Err()
}

// GetAgencySummary returns a summary of files by agency.
func (s *Store) GetAgencySummary() ([]map[string]any, error) {
	rows, err := s.db.Query(
		`SELECT COALESCE(agency,'Unknown') as ag,
		 COUNT(*) as cnt,
		 SUM(CASE WHEN type='PDF' THEN 1 ELSE 0 END) as pdfs,
		 SUM(CASE WHEN type='VID' THEN 1 ELSE 0 END) as vids,
		 SUM(CASE WHEN type='IMG' THEN 1 ELSE 0 END) as imgs,
		 MIN(CASE WHEN parsed_date != '' AND parsed_date IS NOT NULL THEN parsed_date END) as min_date,
		 MAX(CASE WHEN parsed_date != '' AND parsed_date IS NOT NULL THEN parsed_date END) as max_date
		 FROM files
		 GROUP BY LOWER(agency)
		 ORDER BY cnt DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying agencies: %w", err)
	}
	defer rows.Close()

	var agencies []map[string]any
	for rows.Next() {
		var ag string
		var cnt, pdfs, vids, imgs int
		var minDate, maxDate sql.NullString
		if err := rows.Scan(&ag, &cnt, &pdfs, &vids, &imgs, &minDate, &maxDate); err != nil {
			continue
		}
		dateRange := ""
		if minDate.Valid && maxDate.Valid && minDate.String != "" && maxDate.String != "" {
			if minDate.String == maxDate.String {
				dateRange = minDate.String
			} else {
				dateRange = minDate.String + " - " + maxDate.String
			}
		}
		agencies = append(agencies, map[string]any{
			"agency":     ag,
			"count":      cnt,
			"pdfs":       pdfs,
			"videos":     vids,
			"images":     imgs,
			"date_range": dateRange,
		})
	}
	return agencies, rows.Err()
}

// GetNewFiles returns files synced after the given timestamp.
func (s *Store) GetNewFiles(since time.Time) ([]UFOFile, error) {
	rows, err := s.db.Query(
		`SELECT id, COALESCE(title,''), COALESCE(type,''), COALESCE(agency,''),
		COALESCE(release_date,''), COALESCE(incident_date,''), COALESCE(parsed_date,''),
		COALESCE(incident_location,''), COALESCE(description,''), COALESCE(redacted,0),
		COALESCE(download_url,''), COALESCE(thumbnail_url,''), COALESCE(dvids_video_id,''),
		COALESCE(video_title,''), COALESCE(video_pairing,''), COALESCE(pdf_pairing,''),
		COALESCE(modal_image,''), COALESCE(pdf_image_link,''), COALESCE(synced_at,''),
		COALESCE(downloaded,0)
		FROM files WHERE synced_at > ?
		ORDER BY synced_at DESC`, since.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("querying new files: %w", err)
	}
	defer rows.Close()
	return scanUFOFiles(rows)
}

// MarkDownloaded marks a file as downloaded.
func (s *Store) MarkDownloaded(id string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`UPDATE files SET downloaded = 1 WHERE id = ?`, id)
	return err
}

// GetFileCount returns total file count.
func (s *Store) GetFileCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&count)
	return count, err
}

func scanUFOFiles(rows *sql.Rows) ([]UFOFile, error) {
	var files []UFOFile
	for rows.Next() {
		var f UFOFile
		var redacted int
		var downloaded int
		if err := rows.Scan(
			&f.ID, &f.Title, &f.Type, &f.Agency,
			&f.ReleaseDate, &f.IncidentDate, &f.ParsedDate,
			&f.IncidentLocation, &f.Description, &redacted,
			&f.DownloadURL, &f.ThumbnailURL, &f.DVIDSVideoID,
			&f.VideoTitle, &f.VideoPairing, &f.PDFPairing,
			&f.ModalImage, &f.PDFImageLink, &f.SyncedAt,
			&downloaded,
		); err != nil {
			continue
		}
		f.Redacted = redacted != 0
		f.Downloaded = downloaded != 0
		files = append(files, f)
	}
	return files, rows.Err()
}

func scanSingleUFOFile(row *sql.Row) (*UFOFile, error) {
	var f UFOFile
	var redacted int
	var downloaded int
	if err := row.Scan(
		&f.ID, &f.Title, &f.Type, &f.Agency,
		&f.ReleaseDate, &f.IncidentDate, &f.ParsedDate,
		&f.IncidentLocation, &f.Description, &redacted,
		&f.DownloadURL, &f.ThumbnailURL, &f.DVIDSVideoID,
		&f.VideoTitle, &f.VideoPairing, &f.PDFPairing,
		&f.ModalImage, &f.PDFImageLink, &f.SyncedAt,
		&downloaded,
	); err != nil {
		return nil, err
	}
	f.Redacted = redacted != 0
	f.Downloaded = downloaded != 0
	return &f, nil
}
