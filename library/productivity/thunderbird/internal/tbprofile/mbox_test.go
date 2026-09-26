package tbprofile

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func inboxPath(profile string) string {
	return filepath.Join(profile, "ImapMail", "imap.example.com", "INBOX")
}

func scanAll(t *testing.T, path string, start int64) ([]RawMessage, int64) {
	t.Helper()
	var out []RawMessage
	end, err := ScanMbox(path, start, func(m RawMessage) error {
		m.Data = append([]byte(nil), m.Data...)
		out = append(out, m)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out, end
}

func TestIsFromLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"From - Mon Jan 06 09:00:00 2025\n", true},
		{"From - Mon Jan 06 09:00:00 2025\r\n", true},
		{"From alice@example.com Tue Jan 07 10:00:00 2025\n", true},
		{"From the desk of Carol, nothing else.\n", false},
		{">From - Mon Jan 06\n", false},
		{"Subject: From - x\n", false},
	}
	for _, tt := range tests {
		if got := IsFromLine([]byte(tt.line)); got != tt.want {
			t.Errorf("IsFromLine(%q) = %v", tt.line, got)
		}
	}
}

func TestScanMboxLFAndCRLF(t *testing.T) {
	for _, crlf := range []bool{false, true} {
		t.Run("crlf="+strconv.FormatBool(crlf), func(t *testing.T) {
			_, profile := tbtest.Fixture(t)
			path := inboxPath(profile)
			if crlf {
				tbtest.ToCRLF(t, path)
			}
			msgs, end := scanAll(t, path, 0)
			if len(msgs) != 8 {
				t.Fatalf("got %d raw messages, want 8 (expunged included)", len(msgs))
			}
			st, _ := os.Stat(path)
			if end != st.Size() {
				t.Fatalf("end %d != size %d", end, st.Size())
			}
			for i, m := range msgs {
				raw, err := ReadRaw(path, m.Offset, m.Length)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(raw, m.Data) {
					t.Fatalf("message %d: ReadRaw differs from scanned data", i)
				}
				if !bytes.HasPrefix(raw, []byte("X-Mozilla-Status:")) {
					t.Fatalf("message %d does not start at headers: %q", i, raw[:20])
				}
				if bytes.HasSuffix(raw, []byte("\n\n")) || bytes.HasSuffix(raw, []byte("\r\n\r\n")) {
					t.Fatalf("message %d keeps the separator blank line", i)
				}
			}
			second := ParseMessage(msgs[1].Data)
			if !bytes.Contains([]byte(second.BodyText), []byte("From the desk of Carol")) {
				t.Fatalf("body line starting with From split the message: %q", second.BodyText)
			}
		})
	}
}

func TestScanMboxIncremental(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	path := inboxPath(profile)
	all, end := scanAll(t, path, 0)
	extra := "From - Tue Jan 14 17:00:00 2025\nMessage-ID: <m9@example.com>\nSubject: Appended\n\nNew one.\n\n"
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(extra); err != nil {
		t.Fatal(err)
	}
	f.Close()
	got, end2 := scanAll(t, path, end)
	if len(got) != 1 || ParseMessage(got[0].Data).MessageID != "m9@example.com" {
		t.Fatalf("incremental scan = %d messages", len(got))
	}
	if got[0].Offset <= all[len(all)-1].Offset || end2 != end+int64(len(extra)) {
		t.Fatalf("offsets: new=%d end2=%d", got[0].Offset, end2)
	}
	none, end3 := scanAll(t, path, end2)
	if len(none) != 0 || end3 != end2 {
		t.Fatalf("scan at EOF returned %d messages", len(none))
	}
}

func TestScanMboxMissingFile(t *testing.T) {
	if _, err := ScanMbox(filepath.Join(t.TempDir(), "none"), 0, func(RawMessage) error { return nil }); err == nil {
		t.Fatal("expected error")
	}
}

func TestReadRawInvalid(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	tests := []struct {
		offset, length int64
	}{
		{-1, 10}, {0, 0}, {1 << 40, 10},
	}
	for _, tt := range tests {
		if _, err := ReadRaw(inboxPath(profile), tt.offset, tt.length); err == nil {
			t.Errorf("ReadRaw(%d,%d) succeeded", tt.offset, tt.length)
		}
	}
}

func TestMozillaStatus(t *testing.T) {
	tests := map[string]uint32{"0003": 3, " 1005 ": 0x1005, "zz": 0, "": 0}
	for in, want := range tests {
		if got := MozillaStatus(in); got != want {
			t.Errorf("MozillaStatus(%q) = %x", in, got)
		}
	}
}

func TestNormalizeMessageID(t *testing.T) {
	tests := map[string]string{
		" <a@b.c> ":         "a@b.c",
		"a@b.c":             "a@b.c",
		"<a@b.c> (comment)": "a@b.c",
		"":                  "",
	}
	for in, want := range tests {
		if got := NormalizeMessageID(in); got != want {
			t.Errorf("NormalizeMessageID(%q) = %q", in, got)
		}
	}
}

func TestParseMessageIDList(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"<a@x> <b@x>\t<c@x>", []string{"a@x", "b@x", "c@x"}},
		{"a@x b@x", []string{"a@x", "b@x"}},
		{"", []string{}},
	}
	for _, tt := range tests {
		got := ParseMessageIDList(tt.in)
		if len(got) != len(tt.want) {
			t.Fatalf("%q -> %v", tt.in, got)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%q[%d] = %q", tt.in, i, got[i])
			}
		}
	}
}

func TestThreadRoot(t *testing.T) {
	tests := []struct {
		id, irt string
		refs    []string
		want    string
	}{
		{"m8", "m7", []string{"m1", "m7"}, "m1"},
		{"m7", "m1", nil, "m1"},
		{"m1", "", nil, "m1"},
		{"m1", "", []string{""}, "m1"},
	}
	for _, tt := range tests {
		if got := ThreadRoot(tt.id, tt.irt, tt.refs); got != tt.want {
			t.Errorf("ThreadRoot(%v) = %q, want %q", tt, got, tt.want)
		}
	}
}

func TestShortHashAndKeys(t *testing.T) {
	a := MessageKey("account1", "INBOX", "m1@example.com", 10)
	tests := []struct {
		name string
		got  string
		same bool
	}{
		{"stable across offsets", MessageKey("account1", "INBOX", "m1@example.com", 999), true},
		{"folder changes key", MessageKey("account1", "Archives", "m1@example.com", 10), false},
		{"account changes key", MessageKey("account2", "INBOX", "m1@example.com", 10), false},
		{"no message-id uses offset", MessageKey("account1", "INBOX", "", 10), false},
	}
	for _, tt := range tests {
		if (tt.got == a) != tt.same {
			t.Errorf("%s: %q vs %q", tt.name, tt.got, a)
		}
	}
	if len(a) != 12 {
		t.Fatalf("key length %d", len(a))
	}
	if MessageKey("a", "f", "", 1) == MessageKey("a", "f", "", 2) {
		t.Fatal("offset fallback not distinct")
	}
	for i := 0; i < 2000; i++ {
		h := ShortHash(strconv.Itoa(i))
		if f, err := strconv.ParseFloat(h, 64); err == nil && f == 0 {
			t.Fatalf("ShortHash produced zero-numeric id %q", h)
		}
	}
	if ThreadID("m1@example.com") != ThreadID("m1@example.com") || ThreadID("m1@example.com") == ThreadID("m2@example.com") {
		t.Fatal("ThreadID not stable/distinct")
	}
}
