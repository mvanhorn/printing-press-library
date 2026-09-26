package tbprofile

import (
	"encoding/base64"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseMediaTypeLenient(t *testing.T) {
	tests := []struct {
		in, wantType string
		want         map[string]string
	}{
		{`multipart/mixed; boundary=----=_Part_1`, "multipart/mixed", map[string]string{"boundary": "----=_Part_1"}},
		{`text/plain; charset="utf-8"; name='a b.txt'; x=[bad]`, "text/plain", map[string]string{"charset": "utf-8", "name": "a b.txt"}},
		{`multipart/alternative; boundary="ok"`, "multipart/alternative", map[string]string{"boundary": "ok"}},
	}
	for _, tt := range tests {
		mt, params := parseMediaType(tt.in)
		if mt != tt.wantType {
			t.Errorf("%q: type %q", tt.in, mt)
		}
		for k, v := range tt.want {
			if params[k] != v {
				t.Errorf("%q: %s = %q, want %q", tt.in, k, params[k], v)
			}
		}
	}
}

func TestParseMessageUnquotedBoundary(t *testing.T) {
	raw := "Subject: x\nContent-Type: multipart/mixed; boundary=----=_Part_1\n\n" +
		"------=_Part_1\nContent-Type: text/plain; charset=utf-8\n\nhello body\n" +
		"------=_Part_1\nContent-Type: application/pdf\nContent-Disposition: attachment; filename=r=1.pdf\n\nPDF\n" +
		"------=_Part_1--\n"
	m := ParseMessage([]byte(raw))
	if m.BodyText != "hello body" || len(m.Attachments) != 1 || m.Attachments[0].Filename != "r=1.pdf" {
		t.Fatalf("body %q attachments %+v", m.BodyText, m.Attachments)
	}
}

func TestInvalidUTF8KeepsAccents(t *testing.T) {
	body := "Caff\xc3\xa8 e cr\xc3\xa8me \xff fine"
	for _, cs := range []string{"utf-8", ""} {
		raw := "Subject: x\nContent-Type: text/plain"
		if cs != "" {
			raw += "; charset=" + cs
		}
		m := ParseMessage([]byte(raw + "\n\n" + body + "\n"))
		if m.BodyText != "Caffè e crème � fine" {
			t.Errorf("charset %q: body %q", cs, m.BodyText)
		}
	}
	if m := ParseMessage([]byte("Subject: x\nContent-Type: text/plain\n\ncaf\xe9\n")); m.BodyText != "café" {
		t.Errorf("latin-1 bytes: %q", m.BodyText)
	}
	if m := ParseMessage([]byte("Subject: x\nContent-Type: text/plain; charset=utf-8\n\ncaf\xe9\n")); m.BodyText != "caf�" {
		t.Errorf("declared utf-8 with a bad byte: %q", m.BodyText)
	}
	if got := DecodeHeader("Caff\xc3\xa8 \xff"); got != "Caffè �" {
		t.Errorf("header: %q", got)
	}
	if got := DecodeHeader("caf\xe9"); got != "café" {
		t.Errorf("latin-1 header: %q", got)
	}
}

func TestSplitMessageHeaderCap(t *testing.T) {
	junk := strings.Repeat("X-Junk: aaaa\n", MaxHeaderBytes/13+1000)
	h, body := SplitMessage([]byte("Subject: x\n" + junk + "\nbody\n"))
	if h.Get("Subject") != "x" || !strings.HasPrefix(string(body), "X-Junk") || len(h["X-Junk"])*13 > MaxHeaderBytes {
		t.Fatalf("junk headers %d, body starts %q", len(h["X-Junk"]), string(body[:min(len(body), 10)]))
	}
}

func TestSplitMessageLongFoldingIsLinear(t *testing.T) {
	folded := "Subject: a\n" + strings.Repeat(" b\n", MaxHeaderBytes/3) + "\nbody\n"
	start := time.Now()
	h, _ := SplitMessage([]byte(folded))
	if d := time.Since(start); d > 3*time.Second {
		t.Fatalf("folding took %v", d)
	}
	if v := h.Get("Subject"); !strings.HasPrefix(v, "a b b") || len(v) < MaxHeaderBytes/2 {
		t.Fatalf("subject len %d", len(v))
	}
}

func TestNestedMultipartDoesNotCopyPerLevel(t *testing.T) {
	payload := base64.StdEncoding.EncodeToString(make([]byte, 1<<20))
	part := "Content-Type: application/octet-stream\nContent-Transfer-Encoding: base64\n\n" + payload + "\n"
	for i := 0; i < 12; i++ {
		b := "b" + string(rune('a'+i))
		part = "Content-Type: multipart/mixed; boundary=" + b + "\n\n--" + b + "\n" + part + "--" + b + "--\n"
	}
	raw := []byte("Subject: x\n" + part)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	m := ParseMessage(raw)
	runtime.ReadMemStats(&after)
	if len(m.Attachments) != 1 || m.Attachments[0].SizeBytes != 1<<20 {
		t.Fatalf("attachments %+v", m.Attachments)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 4*uint64(len(raw)) {
		t.Fatalf("allocated %d bytes for a %d byte message", alloc, len(raw))
	}
}

func TestSplitMultipart(t *testing.T) {
	body := "preamble\r\n--B\r\nA: 1\r\n\r\none\r\n--Bx\r\nnot a delimiter\r\n--B \r\n\r\ntwo\r\n--B--\r\nepilogue"
	got := splitMultipart([]byte(body), "B")
	if len(got) != 2 || string(got[0]) != "A: 1\r\n\r\none\r\n--Bx\r\nnot a delimiter" || string(got[1]) != "\r\ntwo" {
		t.Fatalf("parts = %q", got)
	}
	if got := splitMultipart([]byte("--B\nunterminated"), "B"); len(got) != 1 || string(got[0]) != "unterminated" {
		t.Fatalf("unterminated = %q", got)
	}
}
