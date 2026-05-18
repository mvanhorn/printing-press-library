// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package ingest

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/store"
)

const (
	// FAASource is the sync_meta.source identifier for the FAA dataset.
	FAASource = "faa_master"
	// FAAURL is the FAA Releasable Aircraft Database zip. Refreshed nightly
	// at 23:30 Central. HTTP HEAD is unreliable here (Akamai cache returns
	// a stale error page); use GET with If-Modified-Since instead.
	FAAURL = "https://registry.faa.gov/database/ReleasableAircraft.zip"
)

// FAAOptions controls which optional tiers are populated.
type FAAOptions struct {
	IncludeDereg     bool
	IncludeAddresses bool
	Force            bool // ignore If-Modified-Since short-circuit
}

// FAAResult is what SyncFAA returns.
type FAAResult struct {
	Skipped         bool           `json:"skipped"`
	BytesDownloaded int64          `json:"bytes_downloaded"`
	LastModified    string         `json:"last_modified,omitempty"`
	Counts          map[string]int `json:"counts,omitempty"`
	Duration        time.Duration  `json:"duration_ms"`
}

// ProgressFunc receives streaming sync events suitable for NDJSON output.
type ProgressFunc func(event string, payload map[string]any)

// SyncFAA downloads the FAA Releasable Aircraft Database, replaces the FAA
// tables in the store with its contents, and updates sync_meta. If the
// server's Last-Modified header matches the value previously stored in
// sync_meta, the download is skipped unless opts.Force is set.
func SyncFAA(ctx context.Context, st *store.Store, opts FAAOptions, progress ProgressFunc) (FAAResult, error) {
	start := time.Now()
	result := FAAResult{Counts: map[string]int{}}

	if progress == nil {
		progress = func(string, map[string]any) {}
	}

	prevLastModified, _, err := lookupSyncMeta(ctx, st, FAASource)
	if err != nil {
		return result, err
	}
	ifModSince := ""
	if !opts.Force {
		ifModSince = prevLastModified
	}

	progress("download_start", map[string]any{"source": FAASource, "url": FAAURL, "if_modified_since": ifModSince})

	skipped, lastModified, tmpPath, n, err := downloadToTempFile(ctx, FAAURL, ifModSince, "airframe-faa-*.zip")
	if err != nil {
		return result, err
	}
	result.BytesDownloaded = n
	if skipped {
		result.Skipped = true
		result.Duration = time.Since(start)
		progress("sync_skip", map[string]any{"source": FAASource, "reason": "unchanged"})
		return result, nil
	}
	defer os.Remove(tmpPath)
	result.LastModified = lastModified

	progress("download_done", map[string]any{"source": FAASource, "bytes": n, "last_modified": lastModified})

	// Read the zip's central directory and find the CSVs we care about.
	zr, err := zip.OpenReader(tmpPath)
	if err != nil {
		return result, fmt.Errorf("opening FAA zip: %w", err)
	}
	defer zr.Close()

	files := indexZipFiles(zr.File)

	st.Lock()
	defer st.Unlock()
	tx, err := st.DB().BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.ExecContext(ctx, `DELETE FROM aircraft`); err != nil {
		return result, fmt.Errorf("clearing aircraft: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM make_model`); err != nil {
		return result, fmt.Errorf("clearing make_model: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM engine`); err != nil {
		return result, fmt.Errorf("clearing engine: %w", err)
	}
	if opts.IncludeDereg {
		if _, err := tx.ExecContext(ctx, `DELETE FROM dereg`); err != nil {
			return result, fmt.Errorf("clearing dereg: %w", err)
		}
	}

	// ACFTREF + ENGINE first (referenced by MASTER) to keep the conceptual
	// order obvious; insert order does not enforce FK in our schema.
	if f, ok := files["ACFTREF.txt"]; ok {
		n, err := ingestACFTREF(ctx, tx, f, progress)
		if err != nil {
			return result, err
		}
		result.Counts["make_model"] = n
	}
	if f, ok := files["ENGINE.txt"]; ok {
		n, err := ingestENGINE(ctx, tx, f, progress)
		if err != nil {
			return result, err
		}
		result.Counts["engine"] = n
	}
	if f, ok := files["MASTER.txt"]; ok {
		n, err := ingestMASTER(ctx, tx, f, opts.IncludeAddresses, progress)
		if err != nil {
			return result, err
		}
		result.Counts["aircraft"] = n
	}
	if opts.IncludeDereg {
		if f, ok := files["DEREG.txt"]; ok {
			n, err := ingestDEREG(ctx, tx, f, progress)
			if err != nil {
				return result, err
			}
			result.Counts["dereg"] = n
		}
	}

	// Replace the sync_meta row last so a failure earlier leaves the prior
	// last_modified intact (next sync still tries to refresh).
	profile := "core"
	if opts.IncludeAddresses {
		profile += "+addresses"
	}
	if opts.IncludeDereg {
		profile += "+dereg"
	}
	if err := upsertSyncMetaTx(ctx, tx, store.SyncMetaRow{
		Source:          FAASource,
		SourceURL:       FAAURL,
		LastModified:    lastModified,
		LastSyncedAt:    time.Now().UTC().Format(time.RFC3339),
		RowCount:        int64(result.Counts["aircraft"]),
		BytesDownloaded: n,
		SchemaProfile:   profile,
	}); err != nil {
		return result, err
	}

	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit tx: %w", err)
	}
	committed = true
	result.Duration = time.Since(start)
	progress("source_done", map[string]any{"source": FAASource, "counts": result.Counts, "duration_ms": result.Duration.Milliseconds()})
	return result, nil
}

func indexZipFiles(fs []*zip.File) map[string]*zip.File {
	out := make(map[string]*zip.File, len(fs))
	for _, f := range fs {
		out[f.Name] = f
	}
	return out
}

// openZipCSV returns an io.ReadCloser positioned past any UTF-8 BOM. Caller
// is responsible for closing.
func openZipCSV(zf *zip.File) (io.ReadCloser, error) {
	rc, err := zf.Open()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", zf.Name, err)
	}
	br := &bomStripper{r: rc}
	return &readCloser{Reader: br, Closer: rc}, nil
}

type bomStripper struct {
	r       io.Reader
	checked bool
}

func (b *bomStripper) Read(p []byte) (int, error) {
	if !b.checked {
		b.checked = true
		// Peek 3 bytes for BOM.
		head := make([]byte, 3)
		n, err := io.ReadFull(b.r, head)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return 0, err
		}
		if n == 3 && head[0] == 0xEF && head[1] == 0xBB && head[2] == 0xBF {
			// Drop BOM
		} else {
			// Prepend what we read back into the stream.
			b.r = io.MultiReader(strings.NewReader(string(head[:n])), b.r)
		}
	}
	return b.r.Read(p)
}

type readCloser struct {
	io.Reader
	io.Closer
}

// newCSVReader configures a csv.Reader tuned for FAA bulk files.
//
//   - FieldsPerRecord = -1 because every FAA row carries a trailing comma
//     (an empty 15th field on ACFTREF, etc.) that breaks the default check.
//   - LazyQuotes = false despite intuition: FAA's ACFTREF has 67 rows like
//     `BABY ACE "D"` with bare quotes mid-field. With LazyQuotes=true Go's
//     parser keeps a quoted-field state open across newlines and silently
//     swallows tens of thousands of subsequent rows. With LazyQuotes=false
//     the parser errors precisely on those 67 lines, which the per-table
//     loops tolerate (log + skip) instead of aborting.
//   - ReuseRecord = true is safe here because every per-row value is
//     captured via the immutable string returned from col(); the slice
//     reuse is transparent.
func newCSVReader(r io.Reader) *csv.Reader {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.LazyQuotes = false
	cr.ReuseRecord = true
	return cr
}

// columnIndex maps trimmed/uppercased column names → 0-based positions.
func columnIndex(header []string) map[string]int {
	out := make(map[string]int, len(header))
	for i, h := range header {
		out[strings.ToUpper(strings.TrimSpace(h))] = i
	}
	return out
}

func col(row []string, idx map[string]int, name string) string {
	i, ok := idx[name]
	if !ok || i < 0 || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func colInt(row []string, idx map[string]int, name string) (int, bool) {
	s := col(row, idx, name)
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}

// nullableInt returns a pointer for INTEGER columns that accept NULL. Go's
// database/sql lets us pass nil for NULL; sending 0 would lie.
func nullableInt(row []string, idx map[string]int, name string) any {
	v, ok := colInt(row, idx, name)
	if !ok {
		return nil
	}
	return v
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// normalizeNNumber prepends 'N' to a bare FAA N-number. FAA CSVs store the
// number without the 'N' prefix; users (and NTSB) write it with the prefix.
func normalizeNNumber(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "N") {
		return s
	}
	return "N" + s
}
