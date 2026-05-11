package cli

import (
	"strings"
	"testing"
)

func TestResolveSendText(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		stdin   bool
		args    []string
		wantErr bool
	}{
		{name: "text flag", text: "hello", wantErr: false},
		{name: "positional", args: []string{"hi", "world"}, wantErr: false},
		{name: "text wins over positional (shell-quote artifact)", text: "x", args: []string{"y"}, wantErr: false},
		{name: "text + stdin conflict", text: "x", stdin: true, wantErr: true},
		{name: "no input", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveSendText(tt.text, tt.stdin, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveSendText error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestTelegramHTMLEscape(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{in: "plain text", want: "plain text"},
		{in: "<b>keep</b>", want: "<b>keep</b>"},
		{in: "<script>bad</script>", want: "&lt;script&gt;bad&lt;/script&gt;"},
		{in: "<b>ok</b><script>bad</script>", want: "<b>ok</b>&lt;script&gt;bad&lt;/script&gt;"},
		{in: "5 < 6 & 7 > 8", want: "5 &lt; 6 &amp; 7 &gt; 8"},
		{in: `<a href="https://x">link</a>`, want: `<a href="https://x">link</a>`},
	}
	for _, tt := range tests {
		got := telegramHTMLEscape(tt.in)
		if got != tt.want {
			t.Errorf("telegramHTMLEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSplitMessage(t *testing.T) {
	short := strings.Repeat("hello ", 10) // ~60 chars
	parts := splitMessage(short, 4096)
	if len(parts) != 1 {
		t.Errorf("short text should produce 1 part, got %d", len(parts))
	}

	// 10_000-char text should split into 3+ parts with each part <=4096.
	long := strings.Repeat("Lorem ipsum dolor sit amet, consectetur. ", 250)
	if len(long) <= 4096 {
		t.Fatalf("test setup: expected long >4096, got %d", len(long))
	}
	parts = splitMessage(long, 4096)
	if len(parts) < 2 {
		t.Errorf("long text should split into multiple parts, got %d", len(parts))
	}
	for i, p := range parts {
		if len(p) > 4096 {
			t.Errorf("part %d exceeds limit: %d", i, len(p))
		}
	}

	// Pathological no-spaces input still splits at raw byte boundary.
	noSpaces := strings.Repeat("X", 5000)
	parts = splitMessage(noSpaces, 4096)
	if len(parts) < 2 {
		t.Errorf("no-spaces text should still split, got %d parts", len(parts))
	}
}

func TestLintTelegramHTML(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		okExp bool
	}{
		{"plain text", "hello world", true},
		{"allowed tag", "<b>bold</b>", true},
		{"allowed nested", "<b>x <i>y</i> z</b>", true},
		{"link allowed", `<a href="https://x">link</a>`, true},
		{"disallowed tag", "<unknown>x</unknown>", false},
		{"unterminated", "x < y", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := lintTelegramHTML(tt.text)
			if r.OK != tt.okExp {
				t.Errorf("lintTelegramHTML(%q).OK=%v, want %v (issue=%q)", tt.text, r.OK, tt.okExp, r.Issue)
			}
		})
	}
}

func TestParseSince(t *testing.T) {
	tests := []struct {
		in      string
		wantErr bool
	}{
		{in: "today"},
		{in: "1h"},
		{in: "24h"},
		{in: "7d"},
		{in: "30m"},
		{in: "", wantErr: true},
		{in: "1y", wantErr: true},
		{in: "abc", wantErr: true},
	}
	for _, tt := range tests {
		_, err := parseSince(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseSince(%q) err=%v wantErr=%v", tt.in, err, tt.wantErr)
		}
	}
}

func TestContentHash(t *testing.T) {
	a := contentHash("hello")
	b := contentHash("hello")
	c := contentHash("world")
	if a != b {
		t.Errorf("same input should produce same hash: %q vs %q", a, b)
	}
	if a == c {
		t.Errorf("different inputs should produce different hashes")
	}
	if len(a) != 16 {
		t.Errorf("hash length should be 16 hex chars, got %d", len(a))
	}
}
