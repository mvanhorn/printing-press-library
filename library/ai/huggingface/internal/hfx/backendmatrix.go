package hfx

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/huggingface-pp-cli/internal/hfdata"
)

// BackendEntry is one row in the backend-support matrix.
type BackendEntry struct {
	Feature       string `json:"feature"`
	Backend       string `json:"backend"`
	Supported     string `json:"supported"` // yes|no|partial|training-only|unknown
	Since         string `json:"since,omitempty"`
	Source        string `json:"source"`
	SourceChecked string `json:"source_checked"` // YYYY-MM-DD
	Notes         string `json:"notes,omitempty"`
	WikiPointer   string `json:"wiki_pointer,omitempty"`
}

// BackendMatrix is the full bundled (and optionally overridden) matrix.
type BackendMatrix struct {
	SchemaVersion int            `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	Notes         string         `json:"notes,omitempty"`
	Entries       []BackendEntry `json:"entries"`
}

// embedded data; var name follows the //go:embed directive in hfdata package.
// We re-export it here via the hfdata package which holds the JSON.
var (
	embeddedMatrixOnce bool
	embeddedMatrix     BackendMatrix
)

// LoadBackendMatrix returns the matrix to use, applying override layering:
// 1. explicit --backend-support path (if non-empty), if exists
// 2. <state-dir>/backend-support.override.json, if exists
// 3. embedded bundled matrix
func LoadBackendMatrix(stateDir, overridePath string) (BackendMatrix, string, error) {
	// 1. explicit override
	if overridePath != "" {
		m, err := readMatrixFile(overridePath)
		if err != nil {
			return BackendMatrix{}, "", fmt.Errorf("loading --backend-support override: %w", err)
		}
		return m, "override:" + overridePath, nil
	}
	// 2. state-dir override
	if stateDir != "" {
		ovr := filepath.Join(stateDir, "backend-support.override.json")
		if _, err := os.Stat(ovr); err == nil {
			m, err := readMatrixFile(ovr)
			if err != nil {
				return BackendMatrix{}, "", fmt.Errorf("loading state-dir override: %w", err)
			}
			return m, "override:" + ovr, nil
		}
	}
	// 3. embedded bundled
	m, err := loadEmbeddedMatrix()
	if err != nil {
		return BackendMatrix{}, "", err
	}
	return m, "bundled", nil
}

func readMatrixFile(path string) (BackendMatrix, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BackendMatrix{}, err
	}
	var m BackendMatrix
	if err := json.Unmarshal(data, &m); err != nil {
		return BackendMatrix{}, fmt.Errorf("parsing matrix at %s: %w", path, err)
	}
	return m, nil
}

func loadEmbeddedMatrix() (BackendMatrix, error) {
	if embeddedMatrixOnce {
		return embeddedMatrix, nil
	}
	data, err := hfdata.BackendSupportJSON()
	if err != nil {
		return BackendMatrix{}, err
	}
	var m BackendMatrix
	if err := json.Unmarshal(data, &m); err != nil {
		return BackendMatrix{}, fmt.Errorf("parsing embedded backend-support.json: %w", err)
	}
	embeddedMatrix = m
	embeddedMatrixOnce = true
	return m, nil
}

// MatrixAgeDays returns the staleness of the matrix in days, computed from
// the *oldest* source_checked across entries (the seed warns when oldest > 90).
// Returns -1 if no entry parses.
func MatrixAgeDays(m BackendMatrix) int {
	now := time.Now().UTC()
	oldest := time.Time{}
	for _, e := range m.Entries {
		if e.SourceChecked == "" {
			continue
		}
		t, err := time.Parse("2006-01-02", e.SourceChecked)
		if err != nil {
			continue
		}
		if oldest.IsZero() || t.Before(oldest) {
			oldest = t
		}
	}
	if oldest.IsZero() {
		return -1
	}
	return int(now.Sub(oldest).Hours() / 24)
}

// LookupVerdict returns the matrix entry for a given (feature, backend) pair.
// Backend matching is case-insensitive and tolerates "/turboquant"-style
// suffixes by trying the exact match first then falling back to the family.
func LookupVerdict(m BackendMatrix, feature, backend string) (BackendEntry, bool) {
	feature = strings.ToLower(strings.TrimSpace(feature))
	backend = strings.ToLower(strings.TrimSpace(backend))
	// exact match
	for _, e := range m.Entries {
		if strings.ToLower(e.Feature) == feature && strings.ToLower(e.Backend) == backend {
			return e, true
		}
	}
	// family fallback (strip /suffix)
	if i := strings.Index(backend, "/"); i > 0 {
		family := backend[:i]
		for _, e := range m.Entries {
			if strings.ToLower(e.Feature) == feature && strings.ToLower(e.Backend) == family {
				return e, true
			}
		}
	}
	return BackendEntry{Feature: feature, Backend: backend, Supported: "unknown"}, false
}

// _ keeps the embed package referenced (defensive against unused-import in
// CI when callers stub the matrix).
var _ embed.FS
