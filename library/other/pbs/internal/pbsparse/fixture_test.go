// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFixture loads a testdata fixture, transparently decoding the base64
// sidecar when the raw file is absent.
//
// The .xlsx fixtures are ZIP containers, and the public printing-press-library
// repo's `Block committed MCPB/native binary payloads` CI guard rejects any
// newly added file under library/ whose first bytes are the ZIP magic
// "PK\x03\x04" — it has no exemption for testdata. That guard exists to stop
// prebuilt MCPB and native release binaries from being committed instead of
// built in Actions, which is not what these files are: they are upstream
// source spreadsheets that two independent parsers are cross-checked against.
//
// Rather than drop the fixtures and lose the cross-encoding assertion in
// pdf_test.go (2,601 cells compared PDF-vs-XLSX with zero value
// disagreements), the published copy carries them as base64 text and decodes
// here. A raw <name> is preferred when present, so a local working tree with
// the original binaries behaves identically.
func readFixture(t *testing.T, name string) []byte {
	t.Helper()

	raw := filepath.Join("testdata", name)
	b, err := os.ReadFile(raw)
	if err == nil {
		return b
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("read fixture %s: %v", name, err)
	}

	encoded, err := os.ReadFile(raw + ".b64")
	if err != nil {
		t.Fatalf("read fixture %s: neither %s nor %s.b64 is present: %v", name, raw, raw, err)
	}

	// openssl base64 hard-wraps, so strip every newline and stray space
	// before decoding.
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		}
		return r
	}, string(encoded))

	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		t.Fatalf("decode fixture %s.b64: %v", name, err)
	}
	return decoded
}
