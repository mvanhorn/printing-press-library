// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Package cdcpdf extracts text from CDC Pakistan's report PDFs.
//
// WHY TWO BACKENDS. CDC changed its production pipeline between vintages:
//
//	2024 and earlier : /Producer "Microsoft Excel 2016"   PDF 1.5, object streams
//	2025 and later   : /Producer "Microsoft: Print To PDF" PDF 1.7, classic xref
//
// Pure-Go readers cope with the first and not the second. Measured against the
// live 2025-11-30 vintage: github.com/ledongthuc/pdf fails with "malformed PDF:
// reading at offset 0: stream not present"; github.com/dslipak/pdf hangs
// indefinitely; pdfcpu extracts raw content streams rather than text. No system
// extractor (pdftotext, mutool, qpdf, gs) is present by default on macOS.
//
// macOS ships PDFKit, which renders every vintage correctly, so the primary
// backend shells out to `swift` following the Swift-subprocess-bridge pattern
// used elsewhere in the Printing Press for macOS framework APIs. Measured on the
// 659-page / 16.5 MB part A: 3,562,556 characters in 3.5 seconds.
//
// The pure-Go backend remains as a fallback so non-Darwin builds stay useful for
// Excel-era vintages instead of failing outright. Whichever backend produced the
// text is recorded as row provenance -- a silent backend swap must never look
// like a schema change.
package cdcpdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	golangpdf "github.com/ledongthuc/pdf"
)

// Backend names recorded as provenance alongside every extracted row.
const (
	BackendPDFKit = "pdfkit-swift"
	BackendPureGo = "ledongthuc-go"
)

// ErrNoBackend means no extractor on this host can read the file. It is
// deliberately distinct from "the file extracted to zero text", because
// conflating an unreadable file with an empty one is how a real vintage gets
// recorded as "CDC published nothing".
var ErrNoBackend = errors.New("cdcpdf: no usable PDF text backend on this host")

// Result carries extracted text plus the provenance needed to interpret it.
type Result struct {
	Text     string `json:"-"`
	Backend  string `json:"backend"`
	Pages    int    `json:"pages"`
	Chars    int    `json:"chars"`
	Producer string `json:"producer,omitempty"`
}

// swiftExtractor is the inline Swift program. PDFKit's per-page `string`
// property is the only extraction path that handles print-driver output, which
// is what CDC now publishes.
const swiftExtractor = `
import Foundation
import Quartz

let args = CommandLine.arguments
guard args.count > 1, let doc = PDFDocument(url: URL(fileURLWithPath: args[1])) else {
    FileHandle.standardError.write("cdcpdf: PDFKit could not open the document\n".data(using: .utf8)!)
    exit(2)
}
var out = ""
for i in 0..<doc.pageCount {
    if let p = doc.page(at: i), let s = p.string { out += s }
}
FileHandle.standardOutput.write("\u{1}PAGES=\(doc.pageCount)\u{1}".data(using: .utf8)!)
FileHandle.standardOutput.write(out.data(using: .utf8)!)
`

// Available reports whether a given backend can run on this host. Callers use it
// to explain a limitation up front rather than failing mid-extraction.
func Available(backend string) bool {
	switch backend {
	case BackendPDFKit:
		if runtime.GOOS != "darwin" {
			return false
		}
		_, err := exec.LookPath("swift")
		return err == nil
	case BackendPureGo:
		return true
	}
	return false
}

// Extract reads all text from path, preferring PDFKit and falling back to the
// pure-Go reader. ctx bounds the subprocess; a large report takes seconds, not
// minutes, so an unbounded context here would mask a hang.
func Extract(ctx context.Context, path string) (*Result, error) {
	producer, _ := readProducer(path)

	var firstErr error
	if Available(BackendPDFKit) {
		res, err := extractPDFKit(ctx, path)
		if err == nil {
			res.Producer = producer
			return res, nil
		}
		firstErr = fmt.Errorf("%s: %w", BackendPDFKit, err)
	}

	res, err := extractPureGo(path)
	if err == nil {
		res.Producer = producer
		return res, nil
	}
	pureErr := fmt.Errorf("%s: %w", BackendPureGo, err)

	if firstErr != nil {
		return nil, fmt.Errorf("%w: %v; %v", ErrNoBackend, firstErr, pureErr)
	}
	return nil, fmt.Errorf("%w: %v (PDFKit unavailable: needs macOS with swift on PATH)", ErrNoBackend, pureErr)
}

func extractPDFKit(ctx context.Context, path string) (*Result, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	script, err := os.CreateTemp("", "cdcpdf-*.swift")
	if err != nil {
		return nil, err
	}
	defer os.Remove(script.Name())
	if _, err := script.WriteString(swiftExtractor); err != nil {
		script.Close()
		return nil, err
	}
	if err := script.Close(); err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, "swift", script.Name(), abs)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, errors.New(msg)
	}

	// The Swift side frames the page count in \x01 delimiters so a report whose
	// own text contains "PAGES=" cannot spoof it.
	raw := stdout.String()
	pages := 0
	if strings.HasPrefix(raw, "\x01") {
		if end := strings.Index(raw[1:], "\x01"); end >= 0 {
			hdr := raw[1 : 1+end]
			raw = raw[end+2:]
			pages, _ = strconv.Atoi(strings.TrimPrefix(hdr, "PAGES="))
		}
	}
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("PDFKit returned no text (document may be image-only)")
	}
	return &Result{Text: raw, Backend: BackendPDFKit, Pages: pages, Chars: len(raw)}, nil
}

func extractPureGo(path string) (res *Result, err error) {
	// ledongthuc panics on some malformed cross-reference tables rather than
	// returning an error, and a panic here would take down the whole command.
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("panic while reading PDF: %v", r)
		}
	}()

	f, r, err := golangpdf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rd, err := r.GetPlainText()
	if err != nil {
		return nil, err
	}
	b, err := io.ReadAll(rd)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(b)) == "" {
		return nil, errors.New("no text extracted")
	}
	return &Result{Text: string(b), Backend: BackendPureGo, Pages: r.NumPage(), Chars: len(b)}, nil
}

// readProducer pulls /Producer out of the trailer without a full parse. It is
// load-bearing: the producer string is how a vintage's layout family is
// identified, and CDC's switch from Excel to a print driver changed the column
// set without changing the file name.
func readProducer(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	idx := bytes.Index(b, []byte("/Producer"))
	if idx < 0 {
		return "", nil
	}
	rest := b[idx:]
	if len(rest) > 512 {
		rest = rest[:512]
	}
	open := bytes.IndexAny(rest, "(<")
	if open < 0 {
		return "", nil
	}
	closer := byte(')')
	if rest[open] == '<' {
		closer = '>'
	}
	end := bytes.IndexByte(rest[open+1:], closer)
	if end < 0 {
		return "", nil
	}
	val := rest[open+1 : open+1+end]
	// Print-To-PDF writes UTF-16BE with a BOM; Excel writes plain bytes.
	if len(val) >= 2 && val[0] == 0xFE && val[1] == 0xFF {
		var sb strings.Builder
		for i := 2; i+1 < len(val); i += 2 {
			sb.WriteRune(rune(int(val[i])<<8 | int(val[i+1])))
		}
		return strings.TrimSpace(sb.String()), nil
	}
	return strings.TrimSpace(string(val)), nil
}
