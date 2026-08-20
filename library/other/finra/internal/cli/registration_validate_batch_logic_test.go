// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseCRDFile(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{"plain newline list", "1234567\n7654321\n\n  9999999  \n", []string{"1234567", "7654321", "9999999"}, false},
		{"csv with crd column", "name,crd\nAlice,1234567\nBob,7654321\n", []string{"1234567", "7654321"}, false},
		{"csv with CRD column different case", "CRD,name\n1234567,Alice\n", []string{"1234567"}, false},
		{"empty input", "", nil, false},
		{"single bare crd, no header keyword", "1234567", []string{"1234567"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseCRDFile([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseCRDFile(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCRDFile(%q) unexpected error: %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseCRDFile(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseCRDCSVMissingColumn(t *testing.T) {
	t.Parallel()
	// The header contains "crd" as a substring (crd_number), which routes
	// parsing into the CSV path, but no column is an exact (case-insensitive)
	// match for "crd" — this must surface as an error, not a silent empty
	// result or a misparsed value.
	_, err := parseCRDFile([]byte("name,crd_number\nAlice,1234567\n"))
	if err == nil {
		t.Fatalf("expected error for CSV missing an exact 'crd' column")
	}
}

// TestValidateBatchRejectsOversizedFile guards against loading an
// unbounded --file fully into memory: a file above
// maxRegistrationValidateBatchFileBytes must be rejected with a clear usage
// error before any read of its contents, rather than attempting
// os.ReadFile on an arbitrarily large file.
func TestValidateBatchRejectsOversizedFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "oversized-crds.txt")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating fixture file: %v", err)
	}
	if err := f.Truncate(maxRegistrationValidateBatchFileBytes + 1); err != nil {
		f.Close()
		t.Fatalf("truncating fixture file to oversized length: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("closing fixture file: %v", err)
	}

	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"registration", "validate-batch", "--file", path})

	err = cmd.Execute()
	if err == nil {
		t.Fatalf("expected an error for an oversized --file, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Fatalf("error = %v, want it to mention the file being too large", err)
	}
}
