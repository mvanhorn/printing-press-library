// Package discovery implements anonymous, bounded eplus website reads.
package discovery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Row = map[string]any

var jst = time.FixedZone("Asia/Tokyo", 9*3600)
var dateRE = regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})`)
var dateTimeRE = regexp.MustCompile(`(\d{4})/(\d{1,2})/(\d{1,2})[^0-9]*?(\d{1,2}):(\d{2})`)
var clockRE = regexp.MustCompile(`(\d{1,2}):(\d{2})`)
var englishDateRE = regexp.MustCompile(`(?i)(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2}),\s*(\d{4})(?:\s*\([^)]*\))?(?:\s+(\d{1,2}):(\d{2}))?`)

func nullable(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func obj(v any) Row {
	m, _ := v.(map[string]any)
	if m == nil {
		return Row{}
	}
	return m
}
func list(v any) []any {
	a, _ := v.([]any)
	if a == nil {
		return []any{}
	}
	return a
}
func newSession() Row {
	return Row{"id": nil, "event_id": nil, "name": nil, "url": nil, "date": nil, "date_end": nil, "doors_at": nil, "start_at": nil, "end_at": nil, "timezone": "Asia/Tokyo", "venue": nil, "region": nil, "sales": []Row{}, "tickets": []Row{}, "fees": []string{}, "eligibility": []string{}, "payment": []string{}, "collection": []string{}, "notes": []string{}, "overseas_bookability": "unknown", "inventory": "unknown", "terms_status": "unknown"}
}
func dateParts(y, m, d string) string {
	t, e := time.ParseInLocation("2006-1-2", y+"-"+m+"-"+d, jst)
	if e != nil {
		return ""
	}
	return t.Format("2006-01-02")
}
func parseDate(s string) string {
	if len(s) == 8 {
		if t, e := time.ParseInLocation("20060102", s, jst); e == nil {
			return t.Format("2006-01-02")
		}
	}
	if m := dateRE.FindStringSubmatch(s); m != nil {
		return dateParts(m[1], m[2], m[3])
	}
	if m := englishDateRE.FindStringSubmatch(s); m != nil {
		if t, e := time.ParseInLocation("January 2, 2006", strings.Title(strings.ToLower(m[1]))+" "+m[2]+", "+m[3], jst); e == nil {
			return t.Format("2006-01-02")
		}
	}
	if t, e := time.ParseInLocation("2006-01-02", s, jst); e == nil {
		return t.Format("2006-01-02")
	}
	return ""
}

// at preserves the service date while normalizing explicit 24:xx/25:xx overnight times.
func at(date, clock string) any {
	if date == "" || clock == "" {
		return nil
	}
	clock = strings.ReplaceAll(clock, ":", "")
	if len(clock) == 3 {
		clock = "0" + clock
	}
	if len(clock) != 4 {
		return nil
	}
	h, e := strconv.Atoi(clock[:2])
	m, e2 := strconv.Atoi(clock[2:])
	if e != nil || e2 != nil || h < 0 || h > 47 || m < 0 || m > 59 {
		return nil
	}
	d, e := time.ParseInLocation("2006-01-02", date, jst)
	if e != nil {
		return nil
	}
	return d.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute).Format(time.RFC3339)
}
func compactDateTime(s string) any {
	if len(s) != 14 {
		return nil
	}
	if t, e := time.ParseInLocation("20060102150405", s, jst); e == nil {
		return t.Format(time.RFC3339)
	}
	return nil
}
func window(s string) (any, any) {
	m := dateTimeRE.FindAllStringSubmatch(s, -1)
	if len(m) >= 2 {
		return at(dateParts(m[0][1], m[0][2], m[0][3]), m[0][4]+":"+m[0][5]), at(dateParts(m[1][1], m[1][2], m[1][3]), m[1][4]+":"+m[1][5])
	}
	e := englishDateRE.FindAllStringSubmatch(s, -1)
	if len(e) >= 2 {
		return at(parseDate(e[0][0]), e[0][4]+":"+e[0][5]), at(parseDate(e[1][0]), e[1][4]+":"+e[1][5])
	}
	return nil, nil
}
func saleKind(label, code string) string {
	if code == "000" || strings.Contains(label, "抽選") || strings.Contains(label, "プレオーダー") {
		return "lottery"
	}
	if strings.Contains(strings.ToLower(label), "request") || strings.Contains(label, "リクエスト") {
		return "request"
	}
	if code == "001" || code == "002" || strings.Contains(label, "先着") || strings.Contains(label, "一般発売") {
		return "first_come"
	}
	return "unknown"
}

// accepting is a sale-round state. It does not imply available seats.
func saleState(label, code, kind string, cancelled, handled bool) (string, string) {
	if cancelled || strings.Contains(label, "休演") || strings.Contains(label, "中止") {
		return "cancelled", "unavailable"
	}
	if !handled || strings.Contains(label, "扱いなし") {
		return "not_handled", "unknown"
	}
	if strings.Contains(label, "受付前") || code == "2" || code == "3" {
		return "upcoming", "unknown"
	}
	if strings.Contains(label, "受付終了") || code == "4" || code == "5" {
		return "closed", "unknown"
	}
	if kind == "lottery" && (strings.Contains(label, "受付中") || strings.Contains(label, "予定枚数終了") || code == "0" || code == "1") {
		return "accepting", "unknown"
	}
	if strings.Contains(label, "予定枚数終了") || strings.Contains(strings.ToLower(label), "sold out") || code == "1" {
		return "sold_out", "unavailable"
	}
	if strings.Contains(label, "空席あり") || code == "0" {
		if kind == "first_come" {
			return "accepting", "available"
		}
		return "accepting", "unknown"
	}
	if strings.Contains(label, "受付中") {
		return "accepting", "unknown"
	}
	return "unknown", "unknown"
}
func newSale(id, name, kind, status, inventory string, start, end any, url any) Row {
	deadline := any(nil)
	if kind == "lottery" {
		deadline = end
	}
	return Row{"id": id, "name": nullable(name), "kind": kind, "status": status, "inventory": inventory, "starts_at": start, "ends_at": end, "lottery_deadline": deadline, "booking_url": url, "eligibility": []string{}, "phase": salePhase(name, kind), "fees": []string{}, "payment": []string{}, "collection": []string{}, "terms_status": "unknown"}
}
func ValidateDate(s string) error {
	if s != "" && parseDate(s) != s {
		return fmt.Errorf("date must be YYYY-MM-DD: %q", s)
	}
	return nil
}

func salePhase(name, kind string) string {
	if strings.Contains(name, "一般") {
		return "general_sale"
	}
	if strings.Contains(name, "先行") || strings.Contains(strings.ToLower(name), "presale") {
		return "presale"
	}
	if kind == "request" {
		return "request"
	}
	return "unknown"
}

// fallbackID is tied to source facts rather than the article's position.
func fallbackID(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "derived-" + hex.EncodeToString(h[:8])
}
