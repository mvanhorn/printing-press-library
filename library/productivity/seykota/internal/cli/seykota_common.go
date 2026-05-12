// Copyright 2026 kjuju600. Licensed under Apache-2.0. See LICENSE.

// Shared plumbing for the seykota content-archive commands (search, faq,
// tsp, risk, timeline, cite, index, sql). Not generated.
package cli

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/seykota/internal/store"
)

// snapshotGz is a vendored, gzipped SQLite database holding the crawled
// seykota.com archive (FAQ months, TSP sections, the risk essay). It is
// shipped with the binary so search/faq/tsp/risk work with zero network on
// first run. A near-empty placeholder is committed during development and
// replaced by `index build` writing data/snapshot.db (then gzipped) before
// release; restoreFromSnapshot ignores anything below restoreMinBytes.
//
//go:embed data/snapshot.db.gz
var snapshotGz []byte

const restoreMinBytes = 4096

// corpusDBPath resolves the on-disk path for the local archive DB: the
// --db flag if set, else the standard data-dir location.
func corpusDBPath(dbFlag string) string {
	if strings.TrimSpace(dbFlag) != "" {
		return dbFlag
	}
	return defaultDBPath("seykota-pp-cli")
}

// openCorpus opens the local archive store, creates the corpus schema if
// needed, and — when the corpus is empty and a real vendored snapshot is
// embedded — restores it. Returns a friendly error if there is nothing to
// search and no snapshot bundled.
func openCorpus(ctx context.Context, dbFlag string) (*store.Store, error) {
	path := corpusDBPath(dbFlag)
	s, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("opening local archive at %s: %w", path, err)
	}
	if err := s.EnsureCorpus(ctx); err != nil {
		s.Close()
		return nil, err
	}
	if s.CorpusEmpty(ctx) {
		if len(snapshotGz) >= restoreMinBytes {
			s.Close()
			if err := restoreFromSnapshot(path); err != nil {
				return nil, fmt.Errorf("restoring bundled archive: %w", err)
			}
			s, err = store.OpenWithContext(ctx, path)
			if err != nil {
				return nil, err
			}
			if err := s.EnsureCorpus(ctx); err != nil {
				s.Close()
				return nil, err
			}
		}
		if s.CorpusEmpty(ctx) {
			s.Close()
			return nil, fmt.Errorf("local archive at %s is empty and no offline snapshot is bundled — run 'seykota-pp-cli index build' to fetch the archive from seykota.com", path)
		}
	}
	return s, nil
}

// openCorpusReadOnlyOptional opens the store for read commands but does NOT
// hard-fail when empty — used by `doctor`-style status. (Currently unused;
// kept minimal.)

func restoreFromSnapshot(dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	zr, err := gzip.NewReader(bytes.NewReader(snapshotGz))
	if err != nil {
		return fmt.Errorf("reading bundled snapshot: %w", err)
	}
	defer zr.Close()
	// remove any stale sidecars from a half-open DB before overwriting
	for _, sfx := range []string{"", "-wal", "-shm", "-journal"} {
		_ = os.Remove(dest + sfx)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, zr); err != nil {
		return err
	}
	return nil
}

// emitJSON writes v as filtered JSON (honoring --select / --compact / etc).
func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// wantsJSON reports whether output should be JSON: --json, --agent, or a
// non-terminal stdout with no competing format flag.
func wantsJSON(cmd *cobra.Command, flags *rootFlags) bool {
	if flags.asJSON {
		return true
	}
	if !isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain {
		return true
	}
	return false
}

// printRows writes a simple aligned table to the command's stdout.
func printRows(cmd *cobra.Command, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	return tw.Flush()
}

// clip normalizes whitespace and shortens s to n runes with an ellipsis.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n < 1 {
		n = 1
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}
