// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backlog.csv")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write CSV: %v", err)
	}
	return path
}

func TestParseExternalCSV(t *testing.T) {
	cases := []struct {
		name    string
		csv     string
		want    []externalRow
		wantErr string // substring of the expected error, empty for success
	}{
		{
			name: "minutes with fractional value, default type",
			csv:  "date,minutes,description\n2026-01-15,1.5,podcast walk\n",
			want: []externalRow{{Date: "2026-01-15", Description: "podcast walk", TimeSeconds: 90, Type: "watching"}},
		},
		{
			name: "hours column and explicit type",
			csv:  "Type,Hours,Date\nlistening,2,2026-02-01\n",
			want: []externalRow{{Date: "2026-02-01", TimeSeconds: 7200, Type: "listening"}},
		},
		{
			name: "seconds column",
			csv:  "date,seconds\n2026-03-01,45\n",
			want: []externalRow{{Date: "2026-03-01", TimeSeconds: 45, Type: "watching"}},
		},
		{
			name: "blank date rows are skipped",
			csv:  "date,minutes\n2026-01-01,10\n,999\n2026-01-02,20\n",
			want: []externalRow{
				{Date: "2026-01-01", TimeSeconds: 600, Type: "watching"},
				{Date: "2026-01-02", TimeSeconds: 1200, Type: "watching"},
			},
		},
		{
			name:    "missing date column",
			csv:     "when,minutes\n2026-01-01,10\n",
			wantErr: "'date' column",
		},
		{
			name:    "missing duration column",
			csv:     "date,description\n2026-01-01,walk\n",
			wantErr: "'seconds', 'minutes', or 'hours'",
		},
		{
			name: "zero-duration rows are skipped, not fatal",
			csv:  "date,minutes\n2026-01-01,10\n2026-01-02,0\n2026-01-03,20\n",
			want: []externalRow{
				{Date: "2026-01-01", TimeSeconds: 600, Type: "watching"},
				{Date: "2026-01-03", TimeSeconds: 1200, Type: "watching"},
			},
		},
		{
			// Malformed quoting on file line 3. Before the line counter fix
			// this was reported as line 2.
			name:    "malformed row names its file line",
			csv:     "date,minutes\n2026-01-01,10\n2026-01-02,\"unclosed\n",
			wantErr: "CSV line 3:",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseExternalCSV(writeCSV(t, tc.csv))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got rows %+v", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseExternalCSV: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
