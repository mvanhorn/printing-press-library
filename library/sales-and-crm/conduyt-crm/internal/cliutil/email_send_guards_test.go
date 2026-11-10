package cliutil

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// specBodyKeys reads the request-body property names of one operation straight from upstream/openapi.yaml.
func specBodyKeys(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			RequestBody struct {
				Content map[string]struct {
					Schema struct {
						Properties map[string]any `yaml:"properties"`
					} `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"requestBody"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	props := doc.Paths[path]["post"].RequestBody.Content["application/json"].Schema.Properties
	if len(props) == 0 {
		t.Fatalf("no request body properties for POST %s in upstream/openapi.yaml", path)
	}
	var keys []string
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// The key lists the pass-through filter accepts are exactly the properties the public spec declares. A key the spec accepts is
// never refused, and a key it dropped is never let through.
func TestEmailSendKeysMatchPublicSpec(t *testing.T) {
	for path, got := range map[string][]string{"/email/send": EmailSendKeys, "/email/send-bulk": EmailSendBulkKeys} {
		sorted := append([]string{}, got...)
		sort.Strings(sorted)
		if want := specBodyKeys(t, path); !reflect.DeepEqual(sorted, want) {
			t.Errorf("%s: filter accepts %v, spec declares %v", path, sorted, want)
		}
	}
}

func TestCheckEmailSendBodyKeys(t *testing.T) {
	if err := CheckEmailSendBodyKeys("email.send", map[string]any{"to": "a@b", "html": nil, "cc": []any{}}); err != nil {
		t.Errorf("known keys refused: %v", err)
	}
	err := CheckEmailSendBodyKeys("email.send", map[string]any{"to": "a@b", "body": "x", "bodyHtml": "y"})
	if err == nil || !strings.Contains(err.Error(), `"body", "bodyHtml"`) || !strings.Contains(err.Error(), "accepted keys: bcc, cc, contactId") {
		t.Errorf("unknown keys: %v", err)
	}
	if err := CheckEmailSendBodyKeys("email.send-bulk", map[string]any{"sandbox": true}); err == nil {
		t.Errorf("a query name must be refused unless the caller passes it as extra")
	}
	if err := CheckEmailSendBodyKeys("email.send-bulk", map[string]any{"sandbox": true}, "sandbox"); err != nil {
		t.Errorf("extra key refused: %v", err)
	}
	if err := CheckEmailSendBodyKeys("email.other", nil); err == nil {
		t.Errorf("unknown endpoint id must not pass silently")
	}
}

// specSendProp is one send-body property of upstream/openapi.yaml. reprint637: the full public document writes a nullable
// field as anyOf [string-with-bounds, null], so the bounds are read from the string branch when the property has none itself.
type specSendProp struct {
	MaxLength int    `yaml:"maxLength"`
	Pattern   string `yaml:"pattern"`
	AnyOf     []struct {
		Type      any    `yaml:"type"`
		MaxLength int    `yaml:"maxLength"`
		Pattern   string `yaml:"pattern"`
	} `yaml:"anyOf"`
}

func (p specSendProp) bounds() (int, string) {
	if p.MaxLength != 0 || p.Pattern != "" {
		return p.MaxLength, p.Pattern
	}
	for _, b := range p.AnyOf {
		if b.Type == "string" {
			return b.MaxLength, b.Pattern
		}
	}
	return 0, ""
}

// specSendProps returns the property schemas of POST /email/send from upstream/openapi.yaml.
func specSendProps(t *testing.T) map[string]specSendProp {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	type prop = specSendProp
	var doc struct {
		Paths map[string]map[string]struct {
			RequestBody struct {
				Content map[string]struct {
					Schema struct {
						Properties map[string]prop `yaml:"properties"`
					} `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"requestBody"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Paths["/email/send"]["post"].RequestBody.Content["application/json"].Schema.Properties
}

// Every limit CheckEmailSendFlags enforces is the one the live spec declares, so a refresh that moves a limit fails here.
func TestEmailSendLimitsMatchPublicSpec(t *testing.T) {
	p := specSendProps(t)
	for name, c := range map[string]struct {
		prop    string
		max     int
		pattern string
	}{
		"to":           {"to", EmailSendToMax, EmailSendToPattern},
		"subject":      {"subject", EmailSendSubjectMax, EmailSendSubjectRE},
		"html":         {"html", EmailSendHTMLMax, ""},
		"text":         {"text", EmailSendTextMax, ""},
		"contactId":    {"contactId", 0, EmailSendContactIDRE},
		"replyTo":      {"replyTo", 0, EmailSendReplyToRE},
		"sendDomainId": {"sendDomainId", 0, EmailSendSendDomainRE},
	} {
		gotMax, gotPattern := p[c.prop].bounds()
		if gotMax != c.max || gotPattern != c.pattern {
			t.Errorf("%s: code has maxLength=%d pattern=%q, spec has maxLength=%d pattern=%q", name, c.max, c.pattern, gotMax, gotPattern)
		}
	}
}

func okSendBody() EmailSendFlagBody {
	return EmailSendFlagBody{To: "a@b.co", Subject: "S", ContactID: "6f1c0000-0000-4000-8000-0000000000a1", HTML: "<p>x</p>"}
}

// One test per limit: the value at the limit passes, one past it is refused; characters outside the BMP count as two.
func TestCheckEmailSendFlagsLimits(t *testing.T) {
	at := func(n int) string { return strings.Repeat("a", n) }
	cases := []struct {
		name   string
		set    func(*EmailSendFlagBody, string)
		okVal  string
		badVal string
	}{
		{"to max", func(b *EmailSendFlagBody, v string) { b.To = v }, at(EmailSendToMax-2) + "@b", at(EmailSendToMax-1) + "@b"},
		{"subject max", func(b *EmailSendFlagBody, v string) { b.Subject = v }, at(EmailSendSubjectMax), at(EmailSendSubjectMax + 1)},
		{"html max", func(b *EmailSendFlagBody, v string) { b.HTML = v }, at(EmailSendHTMLMax), at(EmailSendHTMLMax + 1)},
		{"text max", func(b *EmailSendFlagBody, v string) { b.Text = v }, at(EmailSendTextMax), at(EmailSendTextMax + 1)},
		{"subject astral counts twice", func(b *EmailSendFlagBody, v string) { b.Subject = v }, strings.Repeat("\U0001F600", EmailSendSubjectMax/2), strings.Repeat("\U0001F600", EmailSendSubjectMax/2+1)},
		{"to needs @", func(b *EmailSendFlagBody, v string) { b.To = v }, "a@b", "ab"},
		{"subject non-blank", func(b *EmailSendFlagBody, v string) { b.Subject = v }, "x", " \t\n"},
		{"contact uuid", func(b *EmailSendFlagBody, v string) { b.ContactID = v }, "6f1c0000-0000-4000-8000-0000000000a1", " 6f1c0000-0000-4000-8000-0000000000a1"},
		{"reply-to max after trim", func(b *EmailSendFlagBody, v string) { b.ReplyTo, b.HasReplyTo = v, true }, "  " + at(254) + "  ", at(255)},
		{"send-domain uuid", func(b *EmailSendFlagBody, v string) { b.SendDomainID, b.HasSendDomainID = v, true }, " 6f1c0000-0000-4000-8000-0000000000a4 ", "not-a-uuid"},
	}
	for _, c := range cases {
		ok := okSendBody()
		c.set(&ok, c.okVal)
		if err := CheckEmailSendFlags(ok); err != nil {
			t.Errorf("%s: value at the limit refused: %v", c.name, err)
		}
		bad := okSendBody()
		c.set(&bad, c.badVal)
		if err := CheckEmailSendFlags(bad); err == nil {
			t.Errorf("%s: value past the limit accepted", c.name)
		}
	}
	// blank reply-to and send-domain are valid per the spec patterns when given; absent ones are not checked
	b := okSendBody()
	b.ReplyTo, b.HasReplyTo, b.SendDomainID, b.HasSendDomainID = "", true, "", true
	if err := CheckEmailSendFlags(b); err != nil {
		t.Errorf("blank optional values: %v", err)
	}
	// a no-break space is JavaScript whitespace: the spec accepts it around a reply-to
	b.ReplyTo = "\u00a0a@b.co\u00a0"
	if err := CheckEmailSendFlags(b); err != nil {
		t.Errorf("no-break space padding: %v", err)
	}
	b = okSendBody()
	b.HTML, b.Text = "", ""
	if err := CheckEmailSendFlags(b); err == nil {
		t.Errorf("no body accepted")
	}
}

// Reply-to is the field whose spec pattern counts with ".{0,252}", which ECMAScript measures in UTF-16 code units and which stops at
// line terminators. A Go regexp copy counted runes: 128 astral characters (256 units) passed here and were refused by the API.
func TestCheckEmailSendFlagsReplyToUTF16AndLineTerminators(t *testing.T) {
	astral := "\U0001F600"
	try := func(v string) error {
		b := okSendBody()
		b.ReplyTo, b.HasReplyTo = v, true
		return CheckEmailSendFlags(b)
	}
	if err := try(strings.Repeat(astral, 127)); err != nil {
		t.Errorf("127 astral characters (254 units) refused: %v", err)
	}
	if err := try(strings.Repeat(astral, 128)); err == nil {
		t.Errorf("128 astral characters (256 units) accepted")
	}
	// BMP boundary: 254 units pass, 255 do not; an astral pair at the edge of the budget counts 2
	if err := try(strings.Repeat("a", 254)); err != nil {
		t.Errorf("254 BMP characters refused: %v", err)
	}
	if err := try(strings.Repeat("a", 255)); err == nil {
		t.Errorf("255 BMP characters accepted")
	}
	if err := try(strings.Repeat("a", 253) + astral); err == nil {
		t.Errorf("253 BMP + 1 astral (255 units) accepted")
	}
	if err := try(strings.Repeat("a", 252) + astral); err != nil {
		t.Errorf("252 BMP + 1 astral (254 units) refused: %v", err)
	}
	// padding is not counted, whatever whitespace it is made of
	if err := try("\u3000\ufeff " + strings.Repeat(astral, 127) + "\n\r\u2028\u2029\t"); err != nil {
		t.Errorf("padded 254-unit value refused: %v", err)
	}
	// a line terminator inside the trimmed value never matches ECMAScript "."; Go's "." matches \r, U+2028 and U+2029
	for _, lt := range []string{"\n", "\r", "\r\n", "\u2028", "\u2029"} {
		if err := try("a@b" + lt + "c.co"); err == nil {
			t.Errorf("embedded line terminator %q accepted", lt)
		}
	}
	// other whitespace inside is fine for "."
	if err := try("a@b \u00a0\tc.co"); err != nil {
		t.Errorf("inner spaces refused: %v", err)
	}
}

// lengthSetters say how to put n astral characters into each body property the spec bounds; pre is the fixed UTF-16 length the
// setter adds (to needs its "@"). A property the spec bounds that is not listed here fails TestEveryBoundedSendFieldCountsUTF16, so
// a new bounded field cannot ship without a check that it counts code units.
var lengthSetters = map[string]struct {
	pre int
	set func(*EmailSendFlagBody, string)
}{
	"to":      {1, func(b *EmailSendFlagBody, v string) { b.To = "@" + v }},
	"subject": {0, func(b *EmailSendFlagBody, v string) { b.Subject = v }},
	"html":    {0, func(b *EmailSendFlagBody, v string) { b.HTML = v }},
	"text":    {0, func(b *EmailSendFlagBody, v string) { b.Text = v }},
	"replyTo": {0, func(b *EmailSendFlagBody, v string) { b.ReplyTo, b.HasReplyTo = v, true }},
}

// Derives every length-bounded property (maxLength, or a ".{a,b}" in its pattern) of both send bodies from upstream/openapi.yaml and
// proves the Go side measures it in UTF-16 code units: the largest astral value that fits passes, one more character is refused.
func TestEveryBoundedSendFieldCountsUTF16(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "upstream", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	type prop = specSendProp
	var doc struct {
		Paths map[string]map[string]struct {
			RequestBody struct {
				Content map[string]struct {
					Schema struct {
						Properties map[string]prop `yaml:"properties"`
					} `yaml:"schema"`
				} `yaml:"content"`
			} `yaml:"requestBody"`
		} `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	dotRepeat := regexp.MustCompile(`\.\{(\d+),(\d+)\}`)
	seen, seenBulk := 0, 0
	for _, path := range []string{"/email/send", "/email/send-bulk"} {
		for name, p := range doc.Paths[path]["post"].RequestBody.Content["application/json"].Schema.Properties {
			max, pattern := p.bounds()
			if m := dotRepeat.FindStringSubmatch(pattern); m != nil {
				hi, _ := strconv.Atoi(m[2])
				max = hi + 2 // the \S on each side of the repeat
			}
			if max == 0 {
				continue
			}
			seen++
			if path == "/email/send-bulk" {
				// reprint637: send-bulk's bounded field is replyTo, checked by its own helper
				if name != "replyTo" {
					t.Errorf("%s %s is bounded by the spec (%d units) and has no UTF-16 length check here", path, name, max)
					continue
				}
				seenBulk++
				fits, over := strings.Repeat("\U0001F600", max/2), strings.Repeat("\U0001F600", max/2+1)
				if err := CheckEmailSendBulkReplyTo(fits); err != nil {
					t.Errorf("send-bulk replyTo: %d units refused: %v", max, err)
				}
				if err := CheckEmailSendBulkReplyTo(over); err == nil {
					t.Errorf("send-bulk replyTo: %d units accepted", max+2)
				}
				continue
			}
			c, ok := lengthSetters[name]
			if path != "/email/send" || !ok {
				t.Errorf("%s %s is bounded by the spec (%d units) and has no UTF-16 length check here", path, name, max)
				continue
			}
			n := (max - c.pre) / 2
			okB, badB := okSendBody(), okSendBody()
			c.set(&okB, strings.Repeat("\U0001F600", n))
			c.set(&badB, strings.Repeat("\U0001F600", n+1))
			if err := CheckEmailSendFlags(okB); err != nil {
				t.Errorf("%s: %d astral characters (%d units, max %d) refused: %v", name, n, c.pre+2*n, max, err)
			}
			if err := CheckEmailSendFlags(badB); err == nil {
				t.Errorf("%s: %d astral characters (%d units, max %d) accepted", name, n+1, c.pre+2*n+2, max)
			}
		}
	}
	if seenBulk != 1 {
		t.Errorf("spec shows %d bounded send-bulk fields, want 1 (replyTo)", seenBulk)
	}
	if seen-seenBulk < len(lengthSetters) {
		t.Errorf("spec shows %d bounded send fields, the setters cover %d: a bound was dropped or the scan is blind", seen, len(lengthSetters))
	}
}

// The guards file counts length in one place. A rune count or a byte length of a flag value would pass the astral tests only by luck.
func TestGuardsCountLengthOnlyThroughUTF16Helper(t *testing.T) {
	src, err := os.ReadFile("email_send_guards.go")
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(src), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") || strings.HasPrefix(line, "func utf16Len") {
			continue
		}
		for _, bad := range []string{"RuneCount", "[]rune(", "len(b.", "len(s)", "len(t)"} {
			if strings.Contains(line, bad) {
				t.Errorf("email_send_guards.go:%d counts length without utf16Len (%s): %s", i+1, bad, line)
			}
		}
	}
}
