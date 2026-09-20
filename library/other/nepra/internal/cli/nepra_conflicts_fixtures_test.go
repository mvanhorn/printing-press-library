// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// Offline fixture loaders for the conflicts legs.
//
// The generation workbooks are byte-identical copies of
// internal/nepraparse/testdata/full-fy*.htm.gz and the PER span captures are
// gzipped copies of internal/nepraper/testdata/fy*.spans.tsv. They are COPIED
// rather than imported because a package's testdata is not reachable from a
// sibling package, and without them the gen, capacity and PER legs would have
// no offline test at all.

package cli

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// conflictsGenFixture returns one generation workbook exactly as NEPRA serves it,
// windows-1252 bytes and all. ParseWorkbook wants the UNDECODED bytes.
func conflictsGenFixture(t *testing.T, fy string) []byte {
	t.Helper()
	return ungzipFixture(t, filepath.Join("testdata", "gen-"+fy+".htm.gz"))
}

// perSpansFixture rebuilds a PER text layer from a committed span capture.
//
// It goes through nepraper.PageFromSpans, so the fixture exercises the same
// line grouping, cell gluing and content-stream ordering that
// nepraper.ExtractText does on the real PDF; only the pdf-library call is
// skipped. Page numbers are the original 1-indexed PDF pages, so a provenance
// assertion here matches the published document.
func perSpansFixture(t *testing.T, name string) *nepraper.Doc {
	t.Helper()
	raw := ungzipFixture(t, filepath.Join("testdata", "per-"+name+".spans.tsv.gz"))

	doc := &nepraper.Doc{}
	spans := map[int][]nepraper.Span{}
	var kept []int
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if text == "" {
			continue
		}
		cols := strings.Split(text, "\t")
		if strings.HasPrefix(cols[0], "#") {
			if len(cols) < 2 {
				continue
			}
			switch cols[0] {
			case "#pages":
				doc.NumPages = mustAtoi(t, cols[1])
			case "#creator":
				doc.Creator = cols[1]
			case "#producer":
				doc.Producer = cols[1]
			case "#page":
				kept = append(kept, mustAtoi(t, cols[1]))
			}
			continue
		}
		if len(cols) != 5 {
			t.Fatalf("%s:%d: want 5 columns, got %d", name, line, len(cols))
		}
		pg := mustAtoi(t, cols[0])
		spans[pg] = append(spans[pg], nepraper.Span{
			X: mustAtof(t, cols[1]), Y: mustAtof(t, cols[2]),
			W: mustAtof(t, cols[3]), Text: cols[4], Page: pg,
		})
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	for _, pg := range kept {
		doc.Pages = append(doc.Pages, nepraper.PageFromSpans(pg, spans[pg]))
	}
	if len(doc.Pages) == 0 {
		t.Fatalf("%s: fixture declared no pages", name)
	}
	return doc
}

func ungzipFixture(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path) // #nosec G304 -- test fixture path
	if err != nil {
		t.Fatalf("reading fixture %s: %v", path, err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip %s: %v", path, err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompressing %s: %v", path, err)
	}
	return out
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		t.Fatalf("bad int %q: %v", s, err)
	}
	return n
}

func mustAtof(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		t.Fatalf("bad float %q: %v", s, err)
	}
	return f
}
