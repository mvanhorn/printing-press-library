// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package workflow

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDecodePDFLabel(t *testing.T) {
	pdf := []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF\n")
	encoded := base64.StdEncoding.EncodeToString(pdf)
	decoded, err := DecodePDFLabel(encoded, int64(len(pdf)))
	if err != nil {
		t.Fatalf("decode valid PDF: %v", err)
	}
	if string(decoded) != string(pdf) {
		t.Fatalf("decoded=%q", decoded)
	}

	tests := []struct {
		name    string
		encoded string
		limit   int64
		want    error
	}{
		{"missing", "  ", 100, ErrLabelMissing},
		{"malformed", "%%%not-base64%%%", 100, ErrLabelMalformed},
		{"oversized", encoded, int64(len(pdf) - 1), ErrLabelTooLarge},
		{"not PDF", base64.StdEncoding.EncodeToString([]byte("plain text")), 100, ErrLabelNotPDF},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := DecodePDFLabel(test.encoded, test.limit)
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v, want %v", err, test.want)
			}
		})
	}
}

func TestWritePDFLabelAtomic(t *testing.T) {
	pdf := []byte("%PDF-1.4\n%%EOF\n")
	path := filepath.Join(t.TempDir(), "label.pdf")
	if err := WritePDFLabelAtomic(path, base64.StdEncoding.EncodeToString(pdf), 1024); err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(pdf) {
		t.Fatalf("stored=%q", stored)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%#o, want 0600", info.Mode().Perm())
	}
}
