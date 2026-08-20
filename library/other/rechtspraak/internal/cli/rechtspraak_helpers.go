// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/rechtspraak/internal/rechtspraak"
)

// PRINTING_PRESS_VERIFY=1 is set by `printing-press verify`. We avoid live
// HTTP in that environment to keep verify offline-safe.
func isVerifyEnv() bool {
	return os.Getenv("PRINTING_PRESS_VERIFY") == "1"
}

func mustHTTP() *rechtspraak.HTTP {
	return rechtspraak.NewHTTP()
}

// vocabCacheDir returns the local cache directory for vocab files.
func vocabCacheDir() string {
	if d := os.Getenv("RECHTSPRAAK_PP_CACHE_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "rechtspraak-pp-cli")
	}
	return filepath.Join(home, ".cache", "rechtspraak-pp-cli")
}

// cacheFileMaxAge is how long a cached vocab is considered fresh. The
// underlying vocabularies change rarely (Instanties updates on Wet
// Herziening events, Rechtsgebieden / Proceduresoorten basically never).
const cacheFileMaxAge = 14 * 24 * time.Hour

func vocabCachePath(name string) string {
	_ = os.MkdirAll(vocabCacheDir(), 0o755)
	return filepath.Join(vocabCacheDir(), name+".json")
}

func readCacheJSON(path string, out any) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if time.Since(st.ModTime()) > cacheFileMaxAge {
		return fmt.Errorf("cache stale")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(out)
}

func writeCacheJSON(path string, v any) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var (
	courtIdxMu  sync.Mutex
	courtIdxVal *rechtspraak.CourtIndex
)

// getCourtIndex returns a cached CourtIndex, fetching from the API once and
// reusing on subsequent calls within the same process.
func getCourtIndex(ctx context.Context) (*rechtspraak.CourtIndex, error) {
	courtIdxMu.Lock()
	defer courtIdxMu.Unlock()
	if courtIdxVal != nil {
		return courtIdxVal, nil
	}
	var courts []rechtspraak.Court
	cache := vocabCachePath("courts")
	if err := readCacheJSON(cache, &courts); err != nil {
		if isVerifyEnv() {
			courtIdxVal = rechtspraak.NewCourtIndex(nil)
			return courtIdxVal, nil
		}
		var err error
		courts, err = mustHTTP().Courts(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetch courts vocab: %w", err)
		}
		_ = writeCacheJSON(cache, courts)
	}
	courtIdxVal = rechtspraak.NewCourtIndex(courts)
	return courtIdxVal, nil
}

var (
	subjIdxMu  sync.Mutex
	subjIdxVal *rechtspraak.SubjectIndex
)

func getSubjectIndex(ctx context.Context) (*rechtspraak.SubjectIndex, error) {
	subjIdxMu.Lock()
	defer subjIdxMu.Unlock()
	if subjIdxVal != nil {
		return subjIdxVal, nil
	}
	var subjects []rechtspraak.Subject
	cache := vocabCachePath("subjects")
	if err := readCacheJSON(cache, &subjects); err != nil {
		if isVerifyEnv() {
			subjIdxVal = rechtspraak.NewSubjectIndex(nil)
			return subjIdxVal, nil
		}
		var err error
		subjects, err = mustHTTP().Subjects(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetch subjects vocab: %w", err)
		}
		_ = writeCacheJSON(cache, subjects)
	}
	subjIdxVal = rechtspraak.NewSubjectIndex(subjects)
	return subjIdxVal, nil
}

var (
	procIdxMu  sync.Mutex
	procIdxVal *rechtspraak.ProcedureIndex
)

func getProcedureIndex(ctx context.Context) (*rechtspraak.ProcedureIndex, error) {
	procIdxMu.Lock()
	defer procIdxMu.Unlock()
	if procIdxVal != nil {
		return procIdxVal, nil
	}
	var procedures []rechtspraak.Procedure
	cache := vocabCachePath("procedures")
	if err := readCacheJSON(cache, &procedures); err != nil {
		if isVerifyEnv() {
			procIdxVal = rechtspraak.NewProcedureIndex(nil)
			return procIdxVal, nil
		}
		var err error
		procedures, err = mustHTTP().Procedures(ctx)
		if err != nil {
			return nil, fmt.Errorf("fetch procedures vocab: %w", err)
		}
		_ = writeCacheJSON(cache, procedures)
	}
	procIdxVal = rechtspraak.NewProcedureIndex(procedures)
	return procIdxVal, nil
}

var (
	relDefMu  sync.Mutex
	relDefVal []rechtspraak.RelationDef
)

func getRelationDefs(ctx context.Context) ([]rechtspraak.RelationDef, error) {
	relDefMu.Lock()
	defer relDefMu.Unlock()
	if relDefVal != nil {
		return relDefVal, nil
	}
	cache := vocabCachePath("relations")
	if err := readCacheJSON(cache, &relDefVal); err == nil {
		return relDefVal, nil
	}
	if isVerifyEnv() {
		return nil, nil
	}
	rels, err := mustHTTP().Relations(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch relations vocab: %w", err)
	}
	relDefVal = rels
	_ = writeCacheJSON(cache, rels)
	return rels, nil
}

// writeJSONOut encodes v as indented JSON to the command's stdout.
// Common helper for novel commands' JSON output.
func writeJSONOut(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// shouldEmitJSON decides whether to emit JSON for a novel command. Returns
// true when --json/--agent is set OR stdout is not a terminal (pipe/file).
func shouldEmitJSON(w io.Writer, flags *rootFlags) bool {
	if flags == nil {
		return false
	}
	if flags.asJSON {
		return true
	}
	if !isTerminal(w) && !flags.csv && !flags.quiet && !flags.plain {
		return true
	}
	return false
}

// boundCtx returns a context derived from parent, bounded by the
// rootFlags --timeout if it is positive. Hand-written novel commands
// (chain, citations, dossier, narrow, watch, uitspraken search) make many
// sequential HTTP calls through the typed rechtspraak.HTTP client rather
// than through the generic internal/client (which already consumes
// flags.timeout). Without this helper, --timeout silently does nothing on
// those commands — the contract advertised in --help is not honoured. The
// helper always returns a non-nil cancel func so callers can `defer cancel()`
// unconditionally without nil checks.
func boundCtx(parent context.Context, flags *rootFlags) (context.Context, context.CancelFunc) {
	if flags == nil || flags.timeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, flags.timeout)
}
