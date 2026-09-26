package tbprofile

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
)

// MaxBodyText caps the extracted body text.
const MaxBodyText = 64 * 1024

// Attachment is the metadata of one attachment part.
type Attachment struct {
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Inline      bool   `json:"inline,omitempty"`
}

// Message is a parsed RFC822 message.
type Message struct {
	Status          uint32
	Read            bool
	Replied         bool
	Flagged         bool
	Forwarded       bool
	Expunged        bool
	MessageID       string
	InReplyTo       string
	References      []string
	FromName        string
	FromAddr        string
	To              []string
	Cc              []string
	Subject         string
	Date            time.Time
	ListID          string
	ListUnsubscribe string
	Automated       bool
	AuthResults     string
	BodyText        string
	Attachments     []Attachment
	Header          textproto.MIMEHeader
}

var wordDecoder = &mime.WordDecoder{CharsetReader: charsetReader}

func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	enc, err := htmlindex.Get(strings.ToLower(strings.TrimSpace(charset)))
	if err != nil || enc == nil {
		return nil, fmt.Errorf("unsupported charset %q", charset)
	}
	return enc.NewDecoder().Reader(input), nil
}

// DecodeHeader decodes RFC2047 words and repairs raw 8-bit text.
func DecodeHeader(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = string(repairUTF8([]byte(s), false))
	if strings.Contains(s, "=?") {
		if out, err := wordDecoder.DecodeHeader(s); err == nil {
			s = out
		}
	}
	return s
}

// repairUTF8 keeps declared or mostly valid UTF-8 (bad bytes become U+FFFD) and reads the rest as windows-1252.
func repairUTF8(b []byte, declaredUTF8 bool) []byte {
	if utf8.Valid(b) {
		return b
	}
	if !declaredUTF8 && !mostlyUTF8(b) {
		if out, err := charmap.Windows1252.NewDecoder().Bytes(b); err == nil {
			return out
		}
	}
	return bytes.ToValidUTF8(b, []byte("�"))
}

func mostlyUTF8(b []byte) bool {
	multi, bad := 0, 0
	for len(b) > 0 {
		r, n := utf8.DecodeRune(b)
		if r == utf8.RuneError && n == 1 {
			bad++
		} else if n > 1 {
			multi++
		}
		b = b[n:]
	}
	return multi > 0 && multi >= bad
}

// MaxHeaderBytes caps the header block; anything beyond it is body.
const MaxHeaderBytes = 1 << 20

// SplitMessage separates the header block from the body, tolerating CRLF.
func SplitMessage(raw []byte) (textproto.MIMEHeader, []byte) {
	h := textproto.MIMEHeader{}
	var cur string
	var parts []string
	finish := func() {
		if cur != "" && len(parts) > 1 {
			vals := h[cur]
			vals[len(vals)-1] = strings.Join(parts, " ")
		}
		parts = parts[:0]
	}
	rest := raw
	for len(rest) > 0 && len(raw)-len(rest) < MaxHeaderBytes {
		i := bytes.IndexByte(rest, '\n')
		var line []byte
		if i < 0 {
			line, rest = rest, nil
		} else {
			line, rest = rest[:i], rest[i+1:]
		}
		line = bytes.TrimRight(line, "\r")
		if len(line) == 0 {
			break
		}
		if (line[0] == ' ' || line[0] == '\t') && cur != "" {
			parts = append(parts, strings.TrimSpace(string(line)))
			continue
		}
		finish()
		k, v, ok := bytes.Cut(line, []byte(":"))
		if !ok || len(k) == 0 || bytes.ContainsAny(k, " \t") {
			cur = ""
			continue
		}
		cur = textproto.CanonicalMIMEHeaderKey(string(k))
		val := strings.TrimSpace(string(v))
		h[cur] = append(h[cur], val)
		parts = append(parts, val)
	}
	finish()
	return h, rest
}

var emailRE = regexp.MustCompile(`[A-Za-z0-9._%+'-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

var addrParser = &mail.AddressParser{WordDecoder: wordDecoder}

// ParseAddresses returns the parsed addresses of a header value.
func ParseAddresses(v string) []*mail.Address {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	if list, err := addrParser.ParseList(v); err == nil {
		for _, a := range list {
			a.Address = strings.ToLower(a.Address)
			a.Name = UnquoteDisplayName(DecodeHeader(a.Name))
		}
		return list
	}
	var out []*mail.Address
	for _, e := range emailRE.FindAllString(v, -1) {
		out = append(out, &mail.Address{Address: strings.ToLower(e)})
	}
	return out
}

func addrList(v string) []string {
	out := make([]string, 0)
	for _, a := range ParseAddresses(v) {
		out = append(out, a.Address)
	}
	return out
}

// ParseDate parses a Date header, returning zero time when unparseable.
func ParseDate(v string) time.Time {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}
	}
	if t, err := mail.ParseDate(v); err == nil {
		return t
	}
	if i := strings.Index(v, " ("); i > 0 {
		if t, err := mail.ParseDate(v[:i]); err == nil {
			return t
		}
	}
	return time.Time{}
}

func headerToken(v string) string {
	v, _, _ = strings.Cut(v, ";")
	v, _, _ = strings.Cut(v, "(")
	return strings.ToLower(strings.TrimSpace(v))
}

// IsAutomatedHeader reports machine-generated or bulk mail: Auto-Submitted
// other than "no", Precedence bulk/list/junk, X-Auto-Response-Suppress,
// List-Id or List-Unsubscribe.
func IsAutomatedHeader(h textproto.MIMEHeader) bool {
	if v := headerToken(h.Get("Auto-Submitted")); v != "" && v != "no" {
		return true
	}
	switch headerToken(h.Get("Precedence")) {
	case "bulk", "list", "junk":
		return true
	}
	for _, k := range []string{"X-Auto-Response-Suppress", "List-Id", "List-Unsubscribe"} {
		if strings.TrimSpace(h.Get(k)) != "" {
			return true
		}
	}
	return false
}

// UnquoteDisplayName strips one pair of matching surrounding quotes from an
// RFC 5322 display name and unescapes \".
func UnquoteDisplayName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	return strings.ReplaceAll(s, `\"`, `"`)
}

// ParseMessage parses one RFC822 message (headers, flags, body text and
// attachment metadata).
func ParseMessage(raw []byte) *Message {
	h, body := SplitMessage(raw)
	m := &Message{Header: h}
	m.Status = MozillaStatus(h.Get("X-Mozilla-Status"))
	m.Read = m.Status&StatusRead != 0
	m.Replied = m.Status&StatusReplied != 0
	m.Flagged = m.Status&StatusFlagged != 0
	m.Expunged = m.Status&StatusExpunged != 0
	m.Forwarded = m.Status&StatusForwarded != 0
	m.MessageID = NormalizeMessageID(h.Get("Message-Id"))
	if irt := ParseMessageIDList(h.Get("In-Reply-To")); len(irt) > 0 {
		m.InReplyTo = irt[0]
	}
	m.References = ParseMessageIDList(h.Get("References"))
	if from := ParseAddresses(h.Get("From")); len(from) > 0 {
		m.FromAddr, m.FromName = from[0].Address, from[0].Name
	}
	m.To = addrList(h.Get("To"))
	m.Cc = addrList(h.Get("Cc"))
	m.Subject = DecodeHeader(h.Get("Subject"))
	m.Date = ParseDate(h.Get("Date"))
	m.ListID = DecodeHeader(h.Get("List-Id"))
	m.ListUnsubscribe = DecodeHeader(h.Get("List-Unsubscribe"))
	m.Automated = IsAutomatedHeader(h)
	m.AuthResults = strings.Join(h.Values("Authentication-Results"), "\n")
	var plain, htmlText string
	m.Attachments = make([]Attachment, 0)
	var cids []string
	walkParts(h, body, 0, func(p leafPart) {
		if p.attachment {
			m.Attachments = append(m.Attachments, Attachment{
				Index:       len(m.Attachments),
				Filename:    p.filename,
				ContentType: p.mediaType,
				SizeBytes:   decodedSize(p.encoding, p.body),
				Inline:      strings.HasPrefix(p.mediaType, "image/") && (p.disposition == "inline" || p.disposition == "" && p.contentID != ""),
			})
			cid := p.contentID
			if p.disposition == "attachment" {
				cid = ""
			}
			cids = append(cids, cid)
			return
		}
		switch p.mediaType {
		case "text/plain":
			if plain == "" {
				plain = decodeText(p)
			}
		case "text/html":
			if htmlText == "" {
				htmlText = decodeText(p)
			}
		}
	})
	refs := strings.ToLower(htmlText + "\n" + plain)
	for i, cid := range cids {
		if cid != "" && m.Attachments[i].ContentType != "message/rfc822" && strings.Contains(refs, "cid:"+cid) {
			m.Attachments[i].Inline = true
		}
	}
	text := plain
	if strings.TrimSpace(text) == "" && htmlText != "" {
		text = HTMLToText(htmlText)
	}
	m.BodyText = truncateUTF8(strings.TrimSpace(text), MaxBodyText)
	return m
}

type leafPart struct {
	header      textproto.MIMEHeader
	mediaType   string
	params      map[string]string
	encoding    string
	filename    string
	attachment  bool
	disposition string
	contentID   string
	body        []byte
}

func walkParts(h textproto.MIMEHeader, body []byte, depth int, visit func(leafPart)) {
	mediaType, params := parseMediaType(h.Get("Content-Type"))
	if !strings.Contains(mediaType, "/") {
		mediaType = "text/plain"
	}
	if strings.HasPrefix(mediaType, "multipart/") && params["boundary"] != "" && depth < 20 {
		for _, part := range splitMultipart(body, params["boundary"]) {
			ph, pb := SplitMessage(part)
			walkParts(ph, pb, depth+1, visit)
		}
		return
	}
	disp, dparams := parseMediaType(h.Get("Content-Disposition"))
	filename := DecodeHeader(dparams["filename"])
	if filename == "" {
		filename = DecodeHeader(params["name"])
	}
	attachment := disp == "attachment" || filename != ""
	if mediaType == "message/rfc822" {
		attachment = true
		if filename == "" {
			filename = "attached-message.eml"
		}
	}
	if !strings.HasPrefix(mediaType, "text/") && !strings.HasPrefix(mediaType, "multipart/") {
		attachment = true
	}
	visit(leafPart{
		header:      h,
		mediaType:   mediaType,
		params:      params,
		encoding:    strings.ToLower(strings.TrimSpace(h.Get("Content-Transfer-Encoding"))),
		filename:    filename,
		attachment:  attachment,
		disposition: disp,
		contentID:   strings.ToLower(strings.Trim(strings.TrimSpace(h.Get("Content-Id")), "<>")),
		body:        body,
	})
}

// parseMediaType keeps the params the strict parser rejects (boundary=----=_Part_1).
func parseMediaType(v string) (string, map[string]string) {
	if mt, params, err := mime.ParseMediaType(v); err == nil && mt != "" {
		return mt, params
	}
	parts := strings.Split(v, ";")
	params := map[string]string{}
	for _, p := range parts[1:] {
		k, val, ok := strings.Cut(p, "=")
		k = strings.ToLower(strings.TrimSpace(k))
		if !ok || k == "" {
			continue
		}
		if _, dup := params[k]; !dup {
			params[k] = strings.Trim(strings.TrimSpace(val), `"'`)
		}
	}
	return strings.ToLower(strings.TrimSpace(parts[0])), params
}

// splitMultipart returns sub-slices of body so nested parts are never copied.
func splitMultipart(body []byte, boundary string) [][]byte {
	delim := []byte("--" + boundary)
	var parts [][]byte
	start := -1
	for i := 0; i < len(body); {
		lineEnd := len(body)
		if j := bytes.IndexByte(body[i:], '\n'); j >= 0 {
			lineEnd = i + j + 1
		}
		if line := body[i:lineEnd]; bytes.HasPrefix(line, delim) {
			rest := bytes.TrimRight(line[len(delim):], " \t\r\n")
			closing := string(rest) == "--"
			if len(rest) == 0 || closing {
				if start >= 0 {
					end := i
					if end > start && body[end-1] == '\n' {
						end--
						if end > start && body[end-1] == '\r' {
							end--
						}
					}
					parts = append(parts, body[start:end])
				}
				if closing {
					return parts
				}
				start = lineEnd
			}
		}
		i = lineEnd
	}
	if start >= 0 && start < len(body) {
		parts = append(parts, body[start:])
	}
	return parts
}

func transferDecoder(encoding string, body []byte) io.Reader {
	switch encoding {
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, newBase64Cleaner(body))
	case "quoted-printable":
		return quotedprintable.NewReader(bytes.NewReader(body))
	default:
		return bytes.NewReader(body)
	}
}

func decodeTransfer(encoding string, body []byte) []byte {
	out, err := io.ReadAll(transferDecoder(encoding, body))
	if err != nil && len(out) == 0 {
		return body
	}
	return out
}

func decodedSize(encoding string, body []byte) int64 {
	n, _ := io.Copy(io.Discard, transferDecoder(encoding, body))
	return n
}

// base64Cleaner drops whitespace so the strict stdlib decoder accepts
// line-wrapped content.
type base64Cleaner struct {
	src []byte
}

func newBase64Cleaner(b []byte) *base64Cleaner { return &base64Cleaner{src: b} }

func (c *base64Cleaner) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) && len(c.src) > 0 {
		b := c.src[0]
		c.src = c.src[1:]
		if b == '\r' || b == '\n' || b == ' ' || b == '\t' {
			continue
		}
		p[n] = b
		n++
	}
	if n == 0 && len(c.src) == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func decodeText(p leafPart) string {
	data := decodeTransfer(p.encoding, p.body)
	cs := strings.ToLower(strings.TrimSpace(p.params["charset"]))
	if cs != "" && cs != "utf-8" && cs != "us-ascii" && cs != "utf8" {
		if enc, err := htmlindex.Get(cs); err == nil && enc != nil {
			if out, err := enc.NewDecoder().Bytes(data); err == nil {
				data = out
			}
		}
	}
	data = repairUTF8(data, cs == "utf-8" || cs == "utf8")
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

var (
	htmlDropRE  = regexp.MustCompile(`(?is)<(script|style|head)[^>]*>.*?</(script|style|head)>`)
	htmlBreakRE = regexp.MustCompile(`(?i)<(br|/p|/div|/tr|/li|/h[1-6]|/table)[^>]*>`)
	htmlTagRE   = regexp.MustCompile(`(?s)<[^>]*>`)
	spacesRE    = regexp.MustCompile(`[ \t\x{00a0}]+`)
	blankRunRE  = regexp.MustCompile(`\n\s*\n+`)
)

var (
	htmlBlockTagRE  = regexp.MustCompile(`(?i)<(/?)(blockquote|div)(?:\s[^>]*)?>`)
	htmlQuoteDivRE  = regexp.MustCompile(`(?i)\bclass\s*=\s*["']?[^"'>]*\bgmail_quote\b`)
	quoteOpen       = "\n\x01"
	quoteClose      = "\n\x02"
	quoteMarkerRepl = strings.NewReplacer("\x01", "", "\x02", "")
)

// markQuotes tags blockquote and Gmail quote-div boundaries so HTMLToText can prefix quoted lines with "> ".
func markQuotes(s string) string {
	var b strings.Builder
	var stack []bool
	last := 0
	for _, m := range htmlBlockTagRE.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(s[last:m[1]])
		last = m[1]
		tag := s[m[0]:m[1]]
		closing := m[3] > m[2]
		quote := strings.EqualFold(s[m[4]:m[5]], "blockquote") || htmlQuoteDivRE.MatchString(tag)
		switch {
		case closing && len(stack) > 0:
			if stack[len(stack)-1] {
				b.WriteString(quoteClose)
			}
			stack = stack[:len(stack)-1]
		case !closing && !strings.HasSuffix(tag, "/>"):
			stack = append(stack, quote)
			if quote {
				b.WriteString(quoteOpen)
			}
		}
	}
	b.WriteString(s[last:])
	return b.String()
}

// HTMLToText converts an HTML body to readable plain text; quoted blocks become "> " lines.
func HTMLToText(s string) string {
	s = htmlDropRE.ReplaceAllString(s, " ")
	s = markQuotes(s)
	s = htmlBreakRE.ReplaceAllString(s, "\n")
	s = htmlTagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = spacesRE.ReplaceAllString(s, " ")
	lines := strings.Split(s, "\n")
	depth := 0
	for i, l := range lines {
		depth += strings.Count(l, "\x01") - strings.Count(l, "\x02")
		l = strings.TrimSpace(quoteMarkerRepl.Replace(l))
		if l != "" && depth > 0 {
			l = strings.Repeat("> ", depth) + l
		}
		lines[i] = l
	}
	s = blankRunRE.ReplaceAllString(strings.Join(lines, "\n"), "\n\n")
	return strings.TrimSpace(s)
}

func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// ExtractAttachment returns the metadata and decoded bytes of the
// attachment at index (same numbering as Message.Attachments).
func ExtractAttachment(raw []byte, index int) (Attachment, []byte, error) {
	h, body := SplitMessage(raw)
	i := 0
	var found *leafPart
	walkParts(h, body, 0, func(p leafPart) {
		if !p.attachment || found != nil {
			return
		}
		if i == index {
			cp := p
			found = &cp
		}
		i++
	})
	if found == nil {
		return Attachment{}, nil, fmt.Errorf("attachment index %d not found (message has %d attachments)", index, i)
	}
	data := decodeTransfer(found.encoding, found.body)
	return Attachment{Index: index, Filename: found.filename, ContentType: found.mediaType, SizeBytes: int64(len(data))}, data, nil
}
