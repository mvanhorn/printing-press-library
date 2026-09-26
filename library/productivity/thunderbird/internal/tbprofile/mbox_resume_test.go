package tbprofile

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const resumeMbox = "From - Mon Jan 06 09:00:00 2025\nSubject: a\n\nline\nFrom - Tue Jan 07 10:00:00 2025\nstill body\n\nFrom - Wed Jan 08 11:00:00 2025\nSubject: c\n\nc body\n"

func writeMbox(t *testing.T, content string, crlf bool) string {
	t.Helper()
	if crlf {
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}
	p := filepath.Join(t.TempDir(), "INBOX")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestScanMboxResumeMatchesFullScan(t *testing.T) {
	for _, crlf := range []bool{false, true} {
		t.Run("crlf="+strconv.FormatBool(crlf), func(t *testing.T) {
			path := writeMbox(t, resumeMbox, crlf)
			raw, _ := os.ReadFile(path)
			full, _ := scanAll(t, path, 0)
			if len(full) != 2 {
				t.Fatalf("full scan = %d messages", len(full))
			}
			for _, marker := range []string{"From - Tue", "From - Wed", "still body"} {
				k := int64(strings.Index(string(raw), marker))
				var want []RawMessage
				for _, m := range full {
					if m.Start >= k {
						want = append(want, m)
					}
				}
				got, _ := scanAll(t, path, k)
				if len(got) != len(want) {
					t.Fatalf("resume at %q: %d messages, full scan has %d", marker, len(got), len(want))
				}
				for i := range got {
					if got[i].Start != want[i].Start || got[i].Offset != want[i].Offset || got[i].Length != want[i].Length {
						t.Fatalf("resume at %q: %+v != %+v", marker, got[i], want[i])
					}
				}
			}
		})
	}
}

func TestBoundaryAt(t *testing.T) {
	path := writeMbox(t, resumeMbox, false)
	raw, _ := os.ReadFile(path)
	s := string(raw)
	tests := []struct {
		name string
		off  int64
		want bool
	}{
		{"from line", 0, true},
		{"blank line", int64(strings.Index(s, "\nFrom - Wed")), true},
		{"eof", int64(len(s)), true},
		{"mid body", int64(strings.Index(s, "still body")), false},
		{"mid line", 3, false},
	}
	for _, tt := range tests {
		if got, err := BoundaryAt(path, tt.off); err != nil || got != tt.want {
			t.Errorf("%s: BoundaryAt = %v, %v", tt.name, got, err)
		}
	}
}

func TestReadMessageAt(t *testing.T) {
	content := "From - Mon Jan 06 09:00:00 2025\nSubject: a\n\nbody a\n\nFrom - Tue Jan 07 10:00:00 2025\nMessage-ID: <b@example.com>\nSubject: b\n\nbody b\n"
	path := writeMbox(t, content, false)
	aOff := int64(strings.Index(content, "Subject: a"))
	bOff := int64(strings.Index(content, "Message-ID"))
	tests := []struct {
		name    string
		off     int64
		length  int64
		msgID   string
		wantErr error
	}{
		{"no id at a message start", aOff, 10, "", nil},
		{"no id, shifted offset", aOff + 3, 10, "", ErrMessageMoved},
		{"no id, offset inside body", int64(strings.Index(content, "body a")), 6, "", ErrMessageMoved},
		{"id matches", bOff, 40, "b@example.com", nil},
		{"id differs", bOff, 40, "c@example.com", ErrMessageMoved},
	}
	for _, tt := range tests {
		_, err := ReadMessageAt(path, tt.off, tt.length, tt.msgID)
		if !errors.Is(err, tt.wantErr) && !(err == nil && tt.wantErr == nil) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.wantErr)
		}
	}
}
