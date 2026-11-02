// Package voice implements the observed, read-only Google Voice web protocol.
package voice

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

type Account struct {
	ID      string `json:"id"`
	Version string `json:"version,omitempty"`
}
type Contact struct {
	Name    string `json:"name,omitempty"`
	Phone   string `json:"phone"`
	Blocked *bool  `json:"blocked,omitempty"`
}
type AttachmentVariant struct {
	SizeSpec int64 `json:"size_spec"`
	Width    int64 `json:"width"`
	Height   int64 `json:"height"`
}
type Attachment struct {
	ID       string              `json:"id"`
	MIME     string              `json:"mime_type,omitempty"`
	Width    int64               `json:"width,omitempty"`
	Height   int64               `json:"height,omitempty"`
	Metadata []AttachmentVariant `json:"metadata"`
}
type Message struct {
	ID          string       `json:"id"`
	ThreadID    string       `json:"thread_id"`
	Timestamp   string       `json:"timestamp"`
	TimestampMS int64        `json:"timestamp_ms"`
	Direction   string       `json:"direction"`
	Kind        string       `json:"kind"`
	Type        int64        `json:"source_type"`
	Text        string       `json:"text"`
	Duration    float64      `json:"duration_seconds,omitempty"`
	Transcript  string       `json:"transcript,omitempty"`
	Attachments []Attachment `json:"attachments"`
}
type Thread struct {
	ID          string    `json:"id"`
	Read        *bool     `json:"read"`
	IsText      *bool     `json:"is_text"`
	Contacts    []Contact `json:"participants"`
	Folders     []int64   `json:"folders"`
	Messages    []Message `json:"messages"`
	OlderCursor string    `json:"older_cursor"`
}
type Page struct {
	Threads          []Thread `json:"threads"`
	NextCursor       string   `json:"next_cursor"`
	Version          string   `json:"version,omitempty"`
	UnknownReadState bool     `json:"read_state_unknown,omitempty"`
}

func array(v any) []any { a, _ := v.([]any); return a }
func at(v any, n int) any {
	a := array(v)
	if n < 0 || n >= len(a) {
		return nil
	}
	return a[n]
}
func str(v any) string { s, _ := v.(string); return s }
func num(v any) int64  { n, _ := integer(v); return n }
func repeated(v any, label string) ([]any, error) {
	if v == nil {
		return nil, nil
	}
	a, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("Voice %s changed from an array; refusing page", label)
	}
	return a, nil
}
func scalar(v any, label string) (string, error) {
	if v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("Voice %s changed from a string; refusing page", label)
	}
	return s, nil
}
func numeric(v any) (float64, error) {
	if v == nil {
		return 0, nil
	}
	var n float64
	var e error
	switch x := v.(type) {
	case json.Number:
		n, e = x.Float64()
	case float64:
		n = x
	case string:
		n, e = strconv.ParseFloat(x, 64)
	default:
		e = fmt.Errorf("not numeric")
	}
	if e != nil || math.IsInf(n, 0) || math.IsNaN(n) {
		return 0, fmt.Errorf("Voice numeric field is malformed; refusing page")
	}
	return n, nil
}
func integer(v any) (int64, error) {
	if v == nil {
		return 0, nil
	}
	if s, ok := v.(string); ok {
		if n, e := strconv.ParseInt(s, 10, 64); e == nil {
			return n, nil
		}
	}
	if n, ok := v.(json.Number); ok {
		if i, e := n.Int64(); e == nil {
			return i, nil
		}
	}
	n, e := numeric(v)
	if e != nil || math.Trunc(n) != n || n >= float64(math.MaxInt64) || n < float64(math.MinInt64) {
		return 0, fmt.Errorf("Voice integer field is malformed; refusing page")
	}
	return int64(n), nil
}
func booleanValue(v any) (*bool, error) {
	if v == nil {
		return nil, nil
	}
	if b, ok := v.(bool); ok {
		return &b, nil
	}
	n, e := integer(v)
	if e != nil || (n != 0 && n != 1) {
		return nil, fmt.Errorf("Voice boolean field is malformed; refusing page")
	}
	b := n == 1
	return &b, nil
}
func decode(raw []byte) ([]any, error) {
	var v []any
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	if e := d.Decode(&v); e != nil || v == nil {
		return nil, fmt.Errorf("Voice response is not a protobuf JSON array")
	}
	var extra any
	if e := d.Decode(&extra); e != io.EOF {
		return nil, fmt.Errorf("Voice response has trailing JSON data")
	}
	return v, nil
}
func DecodeAccount(raw []byte) (Account, error) {
	a, e := decode(raw)
	if e != nil {
		return Account{}, e
	}
	account, e := repeated(at(a, 0), "account")
	if e != nil {
		return Account{}, e
	}
	id, e := scalar(at(account, 0), "account ID")
	if e != nil || id == "" {
		return Account{}, fmt.Errorf("Voice account response lacks a stable account ID")
	}
	version, e := repeated(at(account, 12), "account version")
	if e != nil {
		return Account{}, e
	}
	token, e := scalar(at(version, 0), "account version token")
	return Account{ID: id, Version: token}, e
}
func decodeContact(v any) (Contact, error) {
	a, e := repeated(v, "contact")
	if e != nil {
		return Contact{}, e
	}
	name, e := scalar(at(a, 0), "contact name")
	if e != nil {
		return Contact{}, e
	}
	phone, e := scalar(at(a, 1), "contact phone")
	if e != nil {
		return Contact{}, e
	}
	blocked, e := booleanValue(at(a, 6))
	return Contact{Name: name, Phone: phone, Blocked: blocked}, e
}
func decodeAttachment(v any) (Attachment, error) {
	a, e := repeated(v, "attachment")
	out := Attachment{Metadata: []AttachmentVariant{}}
	if e != nil {
		return out, e
	}
	out.ID, e = scalar(at(a, 1), "attachment ID")
	if e != nil {
		return out, e
	}
	out.MIME, e = scalar(at(a, 0), "attachment MIME")
	if e != nil {
		return out, e
	}
	variants, e := repeated(at(a, 3), "attachment metadata")
	if e != nil {
		return out, e
	}
	original := false
	var area int64
	for _, v := range variants {
		m, e := repeated(v, "attachment variant")
		if e != nil {
			return out, e
		}
		size, e := integer(at(m, 0))
		if e != nil {
			return out, e
		}
		width, e := integer(at(m, 1))
		if e != nil {
			return out, e
		}
		height, e := integer(at(m, 2))
		if e != nil {
			return out, e
		}
		if width < 0 || height < 0 || width > 1000000 || height > 1000000 {
			return out, fmt.Errorf("Voice attachment dimensions are invalid")
		}
		out.Metadata = append(out.Metadata, AttachmentVariant{size, width, height})
		if size == 1 || (!original && width*height > area) {
			out.Width = width
			out.Height = height
			area = width * height
			if size == 1 {
				original = true
			}
		}
	}
	return out, nil
}
func decodeMessage(v any, thread string) (Message, error) {
	a, e := repeated(v, "message")
	m := Message{ThreadID: thread, Direction: "unknown", Kind: "unknown", Attachments: []Attachment{}}
	if e != nil {
		return m, e
	}
	m.ID, e = scalar(at(a, 0), "message ID")
	if e != nil {
		return m, e
	}
	m.TimestampMS, e = integer(at(a, 1))
	if e != nil || m.ID == "" || m.TimestampMS <= 0 || m.TimestampMS > 253402300799999 {
		return m, fmt.Errorf("Voice message lacks a valid ID or timestamp; refusing page")
	}
	m.Timestamp = time.UnixMilli(m.TimestampMS).UTC().Format(time.RFC3339Nano)
	m.Type, e = integer(at(a, 4))
	if e != nil {
		return m, e
	}
	m.Text, e = scalar(at(a, 9), "message text")
	if e != nil {
		return m, e
	}
	m.Duration, e = numeric(at(a, 8))
	if e != nil || m.Duration < 0 {
		return m, fmt.Errorf("Voice message duration is invalid")
	}
	switch m.Type {
	case 0:
		m.Kind = "missed_call"
		m.Direction = "inbound"
	case 1:
		m.Kind = "call"
		m.Direction = "inbound"
	case 2:
		m.Kind = "voicemail"
		m.Direction = "inbound"
	case 7, 14:
		m.Kind = "call"
		m.Direction = "outbound"
	case 10:
		m.Kind = "text"
		m.Direction = "inbound"
	case 11:
		m.Kind = "text"
		m.Direction = "outbound"
	}
	if at(a, 14) != nil {
		mm, e := repeated(at(a, 14), "MMS")
		if e != nil {
			return m, e
		}
		m.Kind = "mms"
		text, e := scalar(at(mm, 0), "MMS text")
		if e != nil {
			return m, e
		}
		if text != "" {
			m.Text = text
		}
		attachments, e := repeated(at(mm, 2), "MMS attachments")
		if e != nil {
			return m, e
		}
		for _, v := range attachments {
			a, e := decodeAttachment(v)
			if e != nil {
				return m, e
			}
			m.Attachments = append(m.Attachments, a)
		}
	}
	transcript, e := repeated(at(a, 6), "transcript")
	if e != nil {
		return m, e
	}
	tokens, e := repeated(at(transcript, 1), "transcript tokens")
	if e != nil {
		return m, e
	}
	words := []string{}
	for _, token := range tokens {
		a, e := repeated(token, "transcript token")
		if e != nil {
			return m, e
		}
		// Voice's positional protobuf JSON is PBLite, where byte fields are
		// literal UTF-8 strings. Do not apply canonical protojson base64 rules.
		word, e := scalar(at(a, 0), "transcript text")
		if e != nil {
			return m, e
		}
		if word != "" {
			words = append(words, word)
		}
	}
	m.Transcript = strings.Join(words, " ")
	return m, nil
}
func decodeThread(v any) (Thread, error) {
	a, e := repeated(v, "thread")
	t := Thread{Contacts: []Contact{}, Folders: []int64{}, Messages: []Message{}}
	if e != nil {
		return t, e
	}
	t.ID, e = scalar(at(a, 0), "thread ID")
	if e != nil || t.ID == "" {
		return t, fmt.Errorf("Voice thread response lacks a stable thread ID")
	}
	t.Read, e = booleanValue(at(a, 1))
	if e != nil {
		return t, e
	}
	t.IsText, e = booleanValue(at(a, 7))
	if e != nil {
		return t, e
	}
	t.OlderCursor, e = scalar(at(a, 3), "older-history cursor")
	if e != nil {
		return t, e
	}
	contacts, e := repeated(at(a, 4), "thread contacts")
	if e != nil {
		return t, e
	}
	for _, v := range contacts {
		c, e := decodeContact(v)
		if e != nil {
			return t, e
		}
		t.Contacts = append(t.Contacts, c)
	}
	if len(t.Contacts) == 0 {
		phones, e := repeated(at(a, 8), "thread phones")
		if e != nil {
			return t, e
		}
		for _, v := range phones {
			s, e := scalar(v, "thread phone")
			if e != nil {
				return t, e
			}
			if s != "" {
				t.Contacts = append(t.Contacts, Contact{Phone: s})
			}
		}
	}
	folders, e := repeated(at(a, 5), "thread folders")
	if e != nil {
		return t, e
	}
	for _, v := range folders {
		n, e := integer(v)
		if e != nil {
			return t, e
		}
		t.Folders = append(t.Folders, n)
	}
	messages, e := repeated(at(a, 2), "thread messages")
	if e != nil {
		return t, e
	}
	for _, v := range messages {
		m, e := decodeMessage(v, t.ID)
		if e != nil {
			return t, e
		}
		t.Messages = append(t.Messages, m)
	}
	return t, nil
}
func DecodePage(raw []byte, detail bool) (Page, error) {
	a, e := decode(raw)
	p := Page{Threads: []Thread{}}
	if e != nil {
		return p, e
	}
	if detail {
		t, e := decodeThread(at(a, 0))
		if e != nil {
			return p, e
		}
		p.Threads = append(p.Threads, t)
		p.NextCursor = t.OlderCursor
		return p, nil
	}
	threads, e := repeated(at(a, 0), "inventory")
	if e != nil {
		return p, e
	}
	p.NextCursor, e = scalar(at(a, 1), "inventory cursor")
	if e != nil {
		return p, e
	}
	p.Version, e = scalar(at(a, 2), "inventory version")
	if e != nil {
		return p, e
	}
	for _, v := range threads {
		t, e := decodeThread(v)
		if e != nil {
			return p, e
		}
		p.Threads = append(p.Threads, t)
	}
	return p, nil
}
