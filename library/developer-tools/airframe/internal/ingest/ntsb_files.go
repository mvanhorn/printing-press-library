// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package ingest

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// extractMDBFromZip opens a one-file NTSB zip (avall.zip or PRE1982.zip)
// and writes the contained .mdb to a fresh tmp file. Returns the tmp path;
// caller is responsible for removing it.
func extractMDBFromZip(zipPath string) (string, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("opening NTSB zip %s: %w", zipPath, err)
	}
	defer zr.Close()

	var mdbFile *zip.File
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".mdb") {
			mdbFile = f
			break
		}
	}
	if mdbFile == nil {
		return "", fmt.Errorf("no .mdb file found inside %s", zipPath)
	}

	src, err := mdbFile.Open()
	if err != nil {
		return "", fmt.Errorf("opening %s in zip: %w", mdbFile.Name, err)
	}
	defer src.Close()

	dst, err := os.CreateTemp("", "airframe-ntsb-*.mdb")
	if err != nil {
		return "", fmt.Errorf("tmp file for mdb: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(dst.Name())
		return "", fmt.Errorf("copying mdb out of zip: %w", err)
	}
	if err := dst.Close(); err != nil {
		os.Remove(dst.Name())
		return "", err
	}
	return dst.Name(), nil
}

// extractCSV opens an MDB-table-as-CSV stream via the supplied extractor
// and configures a permissive csv.Reader. Caller closes the returned closer.
func extractCSV(ctx context.Context, extract MDBExtractor, mdbPath, table string) (*csv.Reader, io.Closer, error) {
	rc, err := extract(ctx, mdbPath, table)
	if err != nil {
		return nil, nil, err
	}
	cr := csv.NewReader(rc)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = true
	cr.ReuseRecord = true
	return cr, rc, nil
}

func ingestNTSBEvents(ctx context.Context, tx *sql.Tx, extract MDBExtractor, mdbPath string, progress ProgressFunc) (int, error) {
	cr, closer, err := extractCSV(ctx, extract, mdbPath, "events")
	if err != nil {
		return 0, err
	}
	defer closer.Close()

	header, err := cr.Read()
	if err != nil {
		return 0, fmt.Errorf("events header: %w", err)
	}
	idx := columnIndex(header)

	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO events (
		event_id, event_type, event_date, event_city, event_state, event_country,
		latitude, longitude, highest_injury,
		total_fatal, total_serious, total_minor, total_uninjured,
		weather, light_condition, phase_of_flight, ntsb_report_no, probable_cause
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,NULL)`)
	if err != nil {
		return 0, fmt.Errorf("preparing events insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("events row %d: %w", count, err)
		}
		evID := col(row, idx, "EV_ID")
		if evID == "" {
			continue
		}
		_, err = stmt.ExecContext(ctx,
			evID,
			nullableStr(col(row, idx, "EV_TYPE")),
			nullableStr(ntsbDate(col(row, idx, "EV_DATE"))),
			nullableStr(col(row, idx, "EV_CITY")),
			nullableStr(col(row, idx, "EV_STATE")),
			nullableStr(col(row, idx, "EV_COUNTRY")),
			parseFloat(col(row, idx, "LATITUDE")),
			parseFloat(col(row, idx, "LONGITUDE")),
			nullableStr(col(row, idx, "EV_HIGHEST_INJURY")),
			parseInt(col(row, idx, "INJ_TOT_F")),
			parseInt(col(row, idx, "INJ_TOT_S")),
			parseInt(col(row, idx, "INJ_TOT_M")),
			parseInt(col(row, idx, "INJ_TOT_N")),
			nullableStr(col(row, idx, "WX_COND_BASIC")),
			nullableStr(col(row, idx, "LIGHT_COND")),
			nullableStr(col(row, idx, "PHASE_FLT_SPEC")),
			nullableStr(col(row, idx, "NTSB_NO")),
		)
		if err != nil {
			return count, fmt.Errorf("insert events (%s): %w", evID, err)
		}
		count++
		if count%20000 == 0 {
			progress("ingest_progress", map[string]any{"table": "events", "rows": count})
		}
	}
	return count, nil
}

func ingestNTSBAircraft(ctx context.Context, tx *sql.Tx, extract MDBExtractor, mdbPath string, progress ProgressFunc) (int, error) {
	cr, closer, err := extractCSV(ctx, extract, mdbPath, "aircraft")
	if err != nil {
		return 0, err
	}
	defer closer.Close()

	header, err := cr.Read()
	if err != nil {
		return 0, fmt.Errorf("aircraft header: %w", err)
	}
	idx := columnIndex(header)

	stmt, err := tx.PrepareContext(ctx, `INSERT OR REPLACE INTO event_aircraft (
		event_id, aircraft_idx, registration, make_model_code,
		damage, operator_name, far_part, flight_phase
	) VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing event_aircraft insert: %w", err)
	}
	defer stmt.Close()

	count := 0
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("aircraft row %d: %w", count, err)
		}
		evID := col(row, idx, "EV_ID")
		acKeyStr := col(row, idx, "AIRCRAFT_KEY")
		if evID == "" || acKeyStr == "" {
			continue
		}
		acKey := parseInt(acKeyStr)
		if acKey == nil {
			continue
		}
		// We do not have an FAA-style make_model_code here. NTSB stores
		// free-text acft_make + acft_model. Leave make_model_code NULL;
		// downstream queries can match on (acft_make, acft_model) text
		// or on registration → aircraft.make_model_code via the FAA join.
		regis := normalizeNNumberMaybe(col(row, idx, "REGIS_NO"))
		_, err = stmt.ExecContext(ctx,
			evID, acKey,
			nullableStr(regis),
			nil, // make_model_code intentionally NULL on the NTSB side
			nullableStr(col(row, idx, "DAMAGE")),
			nullableStr(col(row, idx, "OPER_NAME")),
			nullableStr(col(row, idx, "FAR_PART")),
			nullableStr(col(row, idx, "PHASE_FLT_SPEC")),
		)
		if err != nil {
			return count, fmt.Errorf("insert event_aircraft (%s/%v): %w", evID, acKey, err)
		}
		count++
		if count%20000 == 0 {
			progress("ingest_progress", map[string]any{"table": "event_aircraft", "rows": count})
		}
	}
	return count, nil
}

// ingestNTSBNarratives reads NTSB narratives. The NTSB schema keys
// narratives by (ev_id, Aircraft_Key); our store keeps one row per event.
// We pick the first narrative seen per event_id; subsequent rows for the
// same event are ignored. For multi-aircraft events the narrative text is
// usually identical across keys.
func ingestNTSBNarratives(ctx context.Context, tx *sql.Tx, extract MDBExtractor, mdbPath string, fullNarratives bool, progress ProgressFunc) (int, error) {
	cr, closer, err := extractCSV(ctx, extract, mdbPath, "narratives")
	if err != nil {
		return 0, err
	}
	defer closer.Close()

	header, err := cr.Read()
	if err != nil {
		return 0, fmt.Errorf("narratives header: %w", err)
	}
	idx := columnIndex(header)

	stmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO narratives (event_id, summary, full_zstd) VALUES (?,?,?)`)
	if err != nil {
		return 0, fmt.Errorf("preparing narratives insert: %w", err)
	}
	defer stmt.Close()

	var enc *zstd.Encoder
	if fullNarratives {
		enc, err = zstd.NewWriter(nil)
		if err != nil {
			return 0, fmt.Errorf("zstd encoder: %w", err)
		}
		defer enc.Close()
	}

	count := 0
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("narratives row %d: %w", count, err)
		}
		evID := col(row, idx, "EV_ID")
		if evID == "" {
			continue
		}
		full := col(row, idx, "NARR_ACCF")
		if full == "" {
			full = col(row, idx, "NARR_INC")
		}
		cause := col(row, idx, "NARR_CAUSE")
		// Build the summary: probable cause if present, else the first
		// MaxSummaryChars of the factual narrative. Probable cause is
		// often more useful as a one-liner.
		summary := strings.TrimSpace(cause)
		if summary == "" {
			summary = truncateUTF8(full, MaxSummaryChars)
		} else {
			summary = truncateUTF8(summary, MaxSummaryChars)
		}

		var blob any
		if fullNarratives && full != "" {
			blob = enc.EncodeAll([]byte(full), nil)
		}

		_, err = stmt.ExecContext(ctx, evID, nullableStr(summary), blob)
		if err != nil {
			return count, fmt.Errorf("insert narratives (%s): %w", evID, err)
		}
		count++
		if count%20000 == 0 {
			progress("ingest_progress", map[string]any{"table": "narratives", "rows": count})
		}
	}
	return count, nil
}

// normalizeNNumberMaybe is normalizeNNumber but tolerant of non-US tails
// (which NTSB also covers). Foreign registrations like "G-OABC" or "JA8119"
// pass through unchanged.
func normalizeNNumberMaybe(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	// Already prefixed with N
	if strings.HasPrefix(s, "N") {
		return s
	}
	// Contains a non-N letter prefix (e.g., "G-", "JA", "VH-") → foreign tail
	for _, r := range s {
		if r >= 'A' && r <= 'Z' && r != 'N' {
			return s
		}
		break
	}
	// Pure digits → US tail, prefix N
	return "N" + s
}
