// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.
// Download ledger: a small sidecar (--dir/.extron-downloads.json) recording
// what was downloaded, when, and the catalog revision/size at download time.
// `literature updates` diffs revisions against the catalog; `catalog verify`
// checks on-disk sizes.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/extron/internal/extron"
)

// downloadRecord is one entry of the download ledger.
type downloadRecord struct {
	File         string `json:"file"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	URL          string `json:"url"`
	Rev          string `json:"rev"`
	Date         string `json:"date"`
	Size         string `json:"size"`
	SizeBytes    int64  `json:"size_bytes"`
	DownloadedAt string `json:"downloaded_at"`
}

func ledgerPath(dir string) string {
	return filepath.Join(dir, ".extron-downloads.json")
}

// loadLedger reads the ledger; a missing or empty ledger yields an empty list.
func loadLedger(dir string) ([]downloadRecord, error) {
	data, err := os.ReadFile(ledgerPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return []downloadRecord{}, nil
		}
		return nil, fmt.Errorf("reading download ledger: %w", err)
	}
	var recs []downloadRecord
	if len(data) == 0 {
		return []downloadRecord{}, nil
	}
	if err := json.Unmarshal(data, &recs); err != nil {
		return nil, fmt.Errorf("decoding download ledger: %w", err)
	}
	return recs, nil
}

// resolveLedgerPath joins a ledger file path (stored relative to --dir) onto
// the download directory, refusing paths that escape it.
func resolveLedgerPath(dir, file string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("empty ledger file path")
	}
	clean := filepath.Clean(file)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("ledger file path escapes download dir: %q", file)
	}
	return filepath.Join(dir, clean), nil
}

// readLedgerLocked loads the ledger under the same exclusive lock the writer
// uses, so verification/update reads cannot observe a half-committed or
// concurrently-replaced ledger.
func readLedgerLocked(dir string) ([]downloadRecord, error) {
	var recs []downloadRecord
	err := withLedgerLock(dir, func() error {
		var lerr error
		recs, lerr = loadLedger(dir)
		return lerr
	})
	return recs, err
}

func saveLedger(dir string, recs []downloadRecord) error {
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := ledgerPath(dir)
	tmp, err := os.CreateTemp(dir, ".extron-downloads-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// upsertLedgerRecords merges new records into the ledger by file path. The
// read-merge-write runs under an exclusive lock so concurrent CLI processes
// sharing --dir cannot silently drop each other's entries.
func upsertLedgerRecords(dir string, recs []downloadRecord) error {
	return withLedgerLock(dir, func() error {
		existing, err := loadLedger(dir)
		if err != nil {
			return err
		}
		byFile := make(map[string]int, len(existing))
		for i, r := range existing {
			byFile[r.File] = i
		}
		for _, r := range recs {
			if i, ok := byFile[r.File]; ok {
				existing[i] = r
			} else {
				existing = append(existing, r)
				byFile[r.File] = len(existing) - 1
			}
		}
		return saveLedger(dir, existing)
	})
}

func newDownloadRecord(doc extron.Doc, file string, sizeBytes int64) downloadRecord {
	return downloadRecord{
		File:         file,
		Title:        doc.Title,
		Category:     doc.Category,
		URL:          doc.URL,
		Rev:          doc.Rev,
		Date:         doc.Date,
		Size:         doc.Size,
		SizeBytes:    sizeBytes,
		DownloadedAt: nowRFC3339(),
	}
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
