// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/store"
)

const (
	// NTSBAvallSource is the sync_meta.source identifier for the main NTSB
	// avall.zip dataset (1982 → present).
	NTSBAvallSource = "ntsb_avall"
	// NTSBPre1982Source is the sync_meta.source identifier for the optional
	// PRE1982.zip historical dataset.
	NTSBPre1982Source = "ntsb_pre1982"

	NTSBAvallURL   = "https://data.ntsb.gov/avdata/FileDirectory/DownloadFile?fileID=C%3A%5Cavdata%5Cavall.zip"
	NTSBPre1982URL = "https://data.ntsb.gov/avdata/FileDirectory/DownloadFile?fileID=C%3A%5Cavdata%5CPRE1982.zip"

	// MaxSummaryChars is the truncation point for the Core profile narrative
	// summary. Covers the probable-cause-style opener without storing the
	// full multi-paragraph factual narrative.
	MaxSummaryChars = 500
)

// NTSBOptions controls which optional tiers are populated.
type NTSBOptions struct {
	IncludePre1982 bool
	FullNarratives bool
	Force          bool
}

// NTSBResult is the per-source outcome.
type NTSBResult struct {
	Skipped         bool           `json:"skipped"`
	BytesDownloaded int64          `json:"bytes_downloaded"`
	Counts          map[string]int `json:"counts,omitempty"`
	Duration        time.Duration  `json:"duration_ms"`
}

// MDBExtractor produces a CSV byte stream for a single MDB table. The real
// implementation shells out to mdb-export; tests inject a fake.
type MDBExtractor func(ctx context.Context, mdbPath, table string) (io.ReadCloser, error)

// NewShellMDBExtractor returns an MDBExtractor backed by the mdb-export
// binary from the `mdbtools` package. It is the production extractor.
// Doctor checks for mdb-export on PATH and emits an install hint; this
// function returns a typed error if invoked without it.
func NewShellMDBExtractor() MDBExtractor {
	return func(ctx context.Context, mdbPath, table string) (io.ReadCloser, error) {
		if _, err := exec.LookPath("mdb-export"); err != nil {
			return nil, ErrMDBToolsMissing
		}
		cmd := exec.CommandContext(ctx, "mdb-export", "-D", "%Y-%m-%d %H:%M:%S", mdbPath, table)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("mdb-export stdout pipe: %w", err)
		}
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("starting mdb-export: %w", err)
		}
		return &commandReader{ReadCloser: stdout, cmd: cmd}, nil
	}
}

// ErrMDBToolsMissing signals that `mdb-export` is not on PATH. Doctor reports
// this with a precise install hint; sync surfaces it from SyncNTSB.
var ErrMDBToolsMissing = errors.New("mdb-export not found on PATH — install mdbtools (brew install mdbtools / apt install mdbtools / AUR mdbtools)")

type commandReader struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (cr *commandReader) Close() error {
	closeErr := cr.ReadCloser.Close()
	waitErr := cr.cmd.Wait()
	if waitErr != nil {
		return waitErr
	}
	return closeErr
}

// SyncNTSB downloads the NTSB avall.zip, runs the supplied MDBExtractor to
// produce CSV streams for events/aircraft/narratives, and replaces those
// tables in the store. PRE1982 is processed identically when enabled.
func SyncNTSB(ctx context.Context, st *store.Store, opts NTSBOptions, extract MDBExtractor, progress ProgressFunc) (NTSBResult, error) {
	if extract == nil {
		extract = NewShellMDBExtractor()
		// Pre-flight: bail before downloading 90 MB if the user's environment
		// can't process the result. Custom extractors (e.g., tests) skip this.
		if _, err := exec.LookPath("mdb-export"); err != nil {
			return NTSBResult{}, ErrMDBToolsMissing
		}
	}
	if progress == nil {
		progress = func(string, map[string]any) {}
	}

	res, err := syncOneNTSBSource(ctx, st, NTSBAvallSource, NTSBAvallURL, opts, extract, progress)
	if err != nil {
		return res, err
	}
	if opts.IncludePre1982 {
		pre, err := syncOneNTSBSource(ctx, st, NTSBPre1982Source, NTSBPre1982URL, opts, extract, progress)
		if err != nil {
			return res, fmt.Errorf("pre1982: %w", err)
		}
		// Aggregate counts. PRE1982 rows are merged into the same tables.
		for k, v := range pre.Counts {
			res.Counts[k] += v
		}
		res.BytesDownloaded += pre.BytesDownloaded
	}
	return res, nil
}

func syncOneNTSBSource(ctx context.Context, st *store.Store, source, url string, opts NTSBOptions, extract MDBExtractor, progress ProgressFunc) (NTSBResult, error) {
	start := time.Now()
	result := NTSBResult{Counts: map[string]int{}}

	prevLM, _, err := lookupSyncMeta(ctx, st, source)
	if err != nil {
		return result, err
	}
	ifMod := ""
	if !opts.Force {
		ifMod = prevLM
	}
	progress("download_start", map[string]any{"source": source, "url": url, "if_modified_since": ifMod})

	skipped, lastModified, tmpPath, n, err := downloadToTempFile(ctx, url, ifMod, "airframe-"+source+"-*.zip")
	if err != nil {
		return result, err
	}
	result.BytesDownloaded = n
	if skipped {
		result.Skipped = true
		result.Duration = time.Since(start)
		progress("sync_skip", map[string]any{"source": source, "reason": "unchanged"})
		return result, nil
	}
	defer os.Remove(tmpPath)
	progress("download_done", map[string]any{"source": source, "bytes": n, "last_modified": lastModified})

	// Extract the single .mdb file from the zip to a sibling tmp file. mdb-export
	// works on file paths, not stdin.
	mdbPath, err := extractMDBFromZip(tmpPath)
	if err != nil {
		return result, err
	}
	defer os.Remove(mdbPath)

	// Reset the three tables. Single transaction across all three so a
	// failure mid-way leaves the prior state intact.
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

	// On the avall source we wipe; on PRE1982 we delete only the pre-1982
	// slice (avall rows are 1982-present, PRE1982 rows are pre-1982 — they
	// don't overlap). PRE1982 is opt-in so this only runs by request.
	//
	// PATCH: previously the PRE1982 branch skipped the wipe entirely, which
	// meant re-running `sync --source ntsb --include-pre1982` after the
	// upstream file changed left deleted rows in place and either silently
	// skipped or PK-collided on modified rows. The targeted DELETE keeps
	// re-ingest idempotent without touching the 1982-present rows owned by
	// the avall source. The FK cascades on event_aircraft + narratives
	// (declared in schemaV1 with ON DELETE CASCADE; _foreign_keys=ON is
	// set in the open DSN) clean up the child rows.
	if source == NTSBAvallSource {
		for _, tbl := range []string{"events", "event_aircraft", "narratives"} {
			if _, err := tx.ExecContext(ctx, "DELETE FROM "+tbl); err != nil {
				return result, fmt.Errorf("clearing %s: %w", tbl, err)
			}
		}
	} else if source == NTSBPre1982Source {
		if _, err := tx.ExecContext(ctx, "DELETE FROM events WHERE event_date < '1982-01-01'"); err != nil {
			return result, fmt.Errorf("clearing pre-1982 events: %w", err)
		}
	}

	nEv, err := ingestNTSBEvents(ctx, tx, extract, mdbPath, progress)
	if err != nil {
		return result, fmt.Errorf("events: %w", err)
	}
	result.Counts["events"] = nEv

	nAc, err := ingestNTSBAircraft(ctx, tx, extract, mdbPath, progress)
	if err != nil {
		return result, fmt.Errorf("aircraft: %w", err)
	}
	result.Counts["event_aircraft"] = nAc

	nNa, err := ingestNTSBNarratives(ctx, tx, extract, mdbPath, opts.FullNarratives, progress)
	if err != nil {
		return result, fmt.Errorf("narratives: %w", err)
	}
	result.Counts["narratives"] = nNa

	profile := "core"
	if opts.FullNarratives {
		profile += "+full-narratives"
	}
	if opts.IncludePre1982 && source == NTSBAvallSource {
		profile += "+pre1982-ready"
	}
	if err := upsertSyncMetaTx(ctx, tx, store.SyncMetaRow{
		Source:          source,
		SourceURL:       url,
		LastModified:    lastModified,
		LastSyncedAt:    time.Now().UTC().Format(time.RFC3339),
		RowCount:        int64(result.Counts["events"]),
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
	progress("source_done", map[string]any{"source": source, "counts": result.Counts, "duration_ms": result.Duration.Milliseconds()})
	return result, nil
}

// truncateUTF8 returns s truncated to at most maxChars runes (not bytes),
// preserving UTF-8 safety. Trailing whitespace is removed.
func truncateUTF8(s string, maxChars int) string {
	if maxChars <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxChars {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(string(runes[:maxChars]))
}

// parseInt is a tolerant integer parser; "" or non-numeric → nil (NULL in DB).
func parseInt(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return v
}

// parseFloat is a tolerant float parser; "" or non-numeric → nil.
func parseFloat(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return v
}

// ntsbDate normalizes NTSB date columns. mdb-export with -D "%Y-%m-%d %H:%M:%S"
// emits "2024-03-15 14:30:00"; we keep just the date prefix. Other older
// exports use M/D/YYYY; we convert those too. Empty → "".
func ntsbDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		return s[:10]
	}
	// Try slash-separated US-style dates. SplitN drops any trailing time portion.
	first := strings.SplitN(s, " ", 2)[0]
	if strings.Contains(first, "/") {
		for _, layout := range []string{"1/2/2006", "01/02/2006", "1/2/06", "01/02/06"} {
			if t, err := time.Parse(layout, first); err == nil {
				return t.Format("2006-01-02")
			}
		}
	}
	return s
}
