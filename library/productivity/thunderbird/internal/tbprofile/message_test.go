package tbprofile

import (
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile/tbtest"
)

func fixtureMessages(t *testing.T) map[string]*Message {
	t.Helper()
	_, profile := tbtest.Fixture(t)
	msgs, _ := scanAll(t, inboxPath(profile), 0)
	out := map[string]*Message{}
	for _, m := range msgs {
		pm := ParseMessage(m.Data)
		out[pm.MessageID] = pm
	}
	return out
}

func TestParseMessageFixtures(t *testing.T) {
	msgs := fixtureMessages(t)
	tests := []struct {
		id    string
		check func(t *testing.T, m *Message)
	}{
		{"m1@example.com", func(t *testing.T, m *Message) {
			if !m.Read || !m.Replied || m.Flagged || m.Expunged {
				t.Errorf("flags = %+v", m)
			}
			if m.FromAddr != "alice@example.com" || m.FromName != "Alice Example" || m.Subject != "Project kickoff" {
				t.Errorf("from/subject = %q %q %q", m.FromAddr, m.FromName, m.Subject)
			}
			if !m.Date.Equal(time.Date(2025, 1, 6, 8, 0, 0, 0, time.UTC)) {
				t.Errorf("date = %v", m.Date)
			}
			if !strings.Contains(m.AuthResults, "dkim=pass") || !strings.Contains(m.AuthResults, "spf=pass") {
				t.Errorf("auth = %q", m.AuthResults)
			}
			if m.BodyText != "Hi Bob, the kickoff is on Monday." {
				t.Errorf("body = %q", m.BodyText)
			}
		}},
		{"m2@example.com", func(t *testing.T, m *Message) {
			if m.Read || !m.Flagged {
				t.Errorf("unread flagged expected: %+v", m)
			}
			if strings.Join(m.Cc, ",") != "dave@example.com,erin@example.com" || strings.Join(m.To, ",") != "bob@example.com" {
				t.Errorf("to/cc = %v %v", m.To, m.Cc)
			}
		}},
		{"m3@example.com", func(t *testing.T, m *Message) {
			if !m.Expunged {
				t.Error("expunged flag not set")
			}
		}},
		{"m4@example.com", func(t *testing.T, m *Message) {
			if len(m.Attachments) != 1 {
				t.Fatalf("attachments = %+v", m.Attachments)
			}
			a := m.Attachments[0]
			if a.Filename != "report.pdf" || a.ContentType != "application/pdf" || a.SizeBytes != int64(len("Hello attachment world!")) || a.Index != 0 {
				t.Errorf("attachment = %+v", a)
			}
			if m.BodyText != "Here is the report from the Café." {
				t.Errorf("qp body = %q", m.BodyText)
			}
		}},
		{"m5@example.com", func(t *testing.T, m *Message) {
			if m.Subject != "Café crème" || m.FromName != "José Example" {
				t.Errorf("rfc2047 = %q / %q", m.Subject, m.FromName)
			}
			if m.BodyText != "Un café crème, s'il vous plaît." {
				t.Errorf("latin1 body = %q", m.BodyText)
			}
		}},
		{"m6@example.com", func(t *testing.T, m *Message) {
			if m.ListID != "Example News <news.example.com>" || !strings.HasPrefix(m.ListUnsubscribe, "<mailto:unsubscribe@example.com>") {
				t.Errorf("list headers = %q %q", m.ListID, m.ListUnsubscribe)
			}
			if m.BodyText != "Top stories & updates\nRead more" {
				t.Errorf("html body = %q", m.BodyText)
			}
			if m.Read {
				t.Error("newsletter should be unread")
			}
		}},
		{"m8@example.com", func(t *testing.T, m *Message) {
			if m.InReplyTo != "m7@example.com" || strings.Join(m.References, ",") != "m1@example.com,m7@example.com" {
				t.Errorf("threading = %q %v", m.InReplyTo, m.References)
			}
			if ThreadRoot(m.MessageID, m.InReplyTo, m.References) != "m1@example.com" {
				t.Error("thread root")
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			m, ok := msgs[tt.id]
			if !ok {
				t.Fatalf("message %s not parsed", tt.id)
			}
			tt.check(t, m)
		})
	}
}

func TestDecodeHeader(t *testing.T) {
	tests := map[string]string{
		"=?iso-8859-1?Q?Caf=E9?=":   "Café",
		"=?UTF-8?B?Q2Fmw6k=?=":      "Café",
		"=?windows-1252?Q?=80uro?=": "€uro",
		"plain text":                "plain text",
		"caf\xe9 raw":               "café raw",
		"=?unknown-cs?Q?x?=":        "=?unknown-cs?Q?x?=",
		"":                          "",
	}
	for in, want := range tests {
		if got := DecodeHeader(in); got != want {
			t.Errorf("DecodeHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitMessage(t *testing.T) {
	h, body := SplitMessage([]byte("Subject: a\r\n folded\r\nBad line\r\nX-Empty:\r\n\r\nbody\r\n"))
	if h.Get("Subject") != "a folded" || string(body) != "body\r\n" {
		t.Fatalf("h=%v body=%q", h, body)
	}
	if _, ok := h["Bad line"]; ok {
		t.Fatal("invalid header line kept")
	}
	if vals, ok := h["X-Empty"]; !ok || vals[0] != "" {
		t.Fatalf("empty header = %v", vals)
	}
}

func TestIsAutomatedHeader(t *testing.T) {
	tests := []struct {
		hdr  string
		want bool
	}{
		{"Auto-Submitted: auto-generated", true},
		{"Auto-Submitted: Auto-Replied; owner-email=x@example.com", true},
		{"Auto-Submitted: no", false},
		{"Auto-Submitted: No (human)", false},
		{"Precedence: bulk", true},
		{"Precedence: List", true},
		{"Precedence: junk", true},
		{"Precedence: first-class", false},
		{"X-Auto-Response-Suppress: All", true},
		{"List-Id: <news.example.com>", true},
		{"List-Unsubscribe: <mailto:u@example.com>", true},
		{"Subject: hello", false},
	}
	for _, tt := range tests {
		h, _ := SplitMessage([]byte(tt.hdr + "\n\nbody\n"))
		if got := IsAutomatedHeader(h); got != tt.want {
			t.Errorf("%q = %v, want %v", tt.hdr, got, tt.want)
		}
		if m := ParseMessage([]byte("From: a@example.com\n" + tt.hdr + "\n\nbody\n")); m.Automated != tt.want {
			t.Errorf("ParseMessage %q Automated = %v", tt.hdr, m.Automated)
		}
	}
}

func TestUnquoteDisplayName(t *testing.T) {
	tests := []struct{ in, want string }{
		{`"Jane Doe"`, "Jane Doe"},
		{`'Jane Doe'`, "Jane Doe"},
		{` "Say \"hi\"" `, `Say "hi"`},
		{`'Jane`, "'Jane"},
		{`"Jane'`, `"Jane'`},
		{`O'Brien`, "O'Brien"},
		{``, ""},
	}
	for _, tt := range tests {
		if got := UnquoteDisplayName(tt.in); got != tt.want {
			t.Errorf("%q = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestParseAddresses(t *testing.T) {
	tests := []struct {
		in    string
		addrs []string
		name  string
	}{
		{`"Erin E." <Erin@Example.com>, dave@example.com`, []string{"erin@example.com", "dave@example.com"}, "Erin E."},
		{`=?iso-8859-1?Q?Jos=E9?= <jose@example.com>`, []string{"jose@example.com"}, "José"},
		{`'Jane Doe' <jane@example.com>`, []string{"jane@example.com"}, "Jane Doe"},
		{`=?utf-8?Q?=22Quoted_Name=22?= <q@example.com>`, []string{"q@example.com"}, "Quoted Name"},
		{`=?utf-8?Q?Say_=5C=22hi=5C=22?= <s@example.com>`, []string{"s@example.com"}, `Say "hi"`},
		{`broken <<a@example.com>> junk, b@example.com`, []string{"a@example.com", "b@example.com"}, ""},
		{``, nil, ""},
	}
	for _, tt := range tests {
		got := ParseAddresses(tt.in)
		if len(got) != len(tt.addrs) {
			t.Fatalf("%q -> %d addrs", tt.in, len(got))
		}
		for i, a := range got {
			if a.Address != tt.addrs[i] {
				t.Errorf("%q[%d] = %q", tt.in, i, a.Address)
			}
		}
		if len(got) > 0 && got[0].Name != tt.name {
			t.Errorf("%q name = %q", tt.in, got[0].Name)
		}
	}
}

func TestParseDate(t *testing.T) {
	want := time.Date(2025, 1, 7, 10, 0, 0, 0, time.UTC)
	tests := map[string]bool{
		"Tue, 07 Jan 2025 10:00:00 +0000":       true,
		"Tue, 07 Jan 2025 10:00:00 +0000 (UTC)": true,
		"not a date":                            false,
	}
	for in, ok := range tests {
		got := ParseDate(in)
		if ok != got.Equal(want) || !ok && !got.IsZero() {
			t.Errorf("ParseDate(%q) = %v", in, got)
		}
	}
}

func TestHTMLToText(t *testing.T) {
	tests := map[string]string{
		"<p>a &amp; b</p><p>c</p>":                     "a & b\nc",
		"<script>x()</script>hi<br>there":              "hi\nthere",
		"<div>  spaced   out </div>\n\n\n<div>x</div>": "spaced out\n\nx",
	}
	for in, want := range tests {
		if got := HTMLToText(in); got != want {
			t.Errorf("HTMLToText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBodyTruncation(t *testing.T) {
	body := strings.Repeat("é", MaxBodyText)
	m := ParseMessage([]byte("Subject: big\nContent-Type: text/plain; charset=utf-8\n\n" + body))
	if len(m.BodyText) > MaxBodyText || !strings.HasPrefix(body, m.BodyText) {
		t.Fatalf("truncated len %d", len(m.BodyText))
	}
}

func TestExtractAttachment(t *testing.T) {
	_, profile := tbtest.Fixture(t)
	msgs, _ := scanAll(t, inboxPath(profile), 0)
	var raw []byte
	for _, m := range msgs {
		if strings.Contains(string(m.Data), "<m4@example.com>") {
			raw = m.Data
		}
	}
	tests := []struct {
		index   int
		wantErr bool
	}{
		{0, false}, {1, true}, {-1, true},
	}
	for _, tt := range tests {
		a, data, err := ExtractAttachment(raw, tt.index)
		if tt.wantErr {
			if err == nil {
				t.Errorf("index %d: expected error", tt.index)
			}
			continue
		}
		if err != nil || string(data) != "Hello attachment world!" || a.Filename != "report.pdf" || a.SizeBytes != 23 {
			t.Errorf("index %d: %+v %q %v", tt.index, a, data, err)
		}
	}
}

func TestBodyCharsetDecoding(t *testing.T) {
	tests := []struct {
		charset, body, want string
	}{
		{"iso-8859-2", "Za=BF=F3=B3=E6", "Zażółć"},
		{"koi8-r", "=F0=D2=C9=D7=C5=D4", "Привет"},
		{"utf-8", "Caf=C3=A9", "Café"},
	}
	for _, tt := range tests {
		m := ParseMessage([]byte("Subject: x\nContent-Type: text/plain; charset=" + tt.charset + "\nContent-Transfer-Encoding: quoted-printable\n\n" + tt.body + "\n"))
		if m.BodyText != tt.want {
			t.Errorf("%s body = %q, want %q", tt.charset, m.BodyText, tt.want)
		}
	}
}

func TestNonTextSinglePartIsAttachment(t *testing.T) {
	m := ParseMessage([]byte("Subject: x\nContent-Type: image/png\nContent-Transfer-Encoding: base64\n\niVBORw0K\n"))
	if len(m.Attachments) != 1 || m.Attachments[0].ContentType != "image/png" || m.Attachments[0].SizeBytes != 6 {
		t.Fatalf("attachments = %+v", m.Attachments)
	}
}
