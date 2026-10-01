package asoview

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Object = map[string]any

func object(v any) Object {
	m, _ := v.(map[string]any)
	if m == nil {
		return Object{}
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
func text(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	default:
		return ""
	}
}
func nullable(v any) any {
	s := strings.TrimSpace(text(v))
	if s == "" || s == "-" {
		return nil
	}
	return s
}
func flag(v any) bool { b, _ := v.(bool); return b }
func integer(v any) *int64 {
	s := strings.ReplaceAll(strings.TrimSpace(text(v)), ",", "")
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}
func price(v any, unit any, basis string, date any) Object {
	return Object{"amount": integer(v), "currency": "JPY", "unit": nullable(unit), "basis": basis, "date": date, "party_specific": false}
}
func strptr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func idString(v any) string { return text(v) }

var idPattern = regexp.MustCompile(`^(ticket[0-9]{10}|pln[0-9]{10})$`)

func ParseID(input string) (kind, id string, err error) {
	id = input
	if strings.Contains(input, "://") {
		u, e := url.Parse(input)
		if e != nil || u.Scheme != "https" || u.Host != "www.asoview.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return "", "", fail(2, "use a source ID or canonical https://www.asoview.com/item/.../ URL")
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) != 3 || parts[0] != "item" {
			return "", "", fail(2, "expected canonical Asoview product URL")
		}
		id = parts[2]
		kind = parts[1]
	}
	if !idPattern.MatchString(id) {
		return "", "", fail(2, "invalid product ID %q; expected ticket0000049223 or pln3000044589", input)
	}
	expected := "ticket"
	if strings.HasPrefix(id, "pln") {
		expected = "activity"
	}
	if kind != "" && kind != expected {
		return "", "", fail(2, "product URL kind and source ID disagree")
	}
	return expected, id, nil
}
func BookingURL(id string) string {
	kind, code, err := ParseID(id)
	if err != nil {
		return ""
	}
	return Origin + "/item/" + kind + "/" + code + "/"
}
func ParseDate(s string) (time.Time, error) {
	t, e := time.Parse("2006-01-02", s)
	if e != nil || t.Format("2006-01-02") != s {
		return t, fail(2, "invalid --date %q; use YYYY-MM-DD", s)
	}
	return t, nil
}
func ParseMonth(s string) (time.Time, error) {
	t, e := time.Parse("2006-01", s)
	if e != nil || t.Format("2006-01") != s {
		return t, fail(2, "invalid --month %q; use YYYY-MM", s)
	}
	return t, nil
}
func NormalizeTime(s string) string {
	t, e := time.Parse("15:04", s)
	if e == nil {
		return t.Format("15:04")
	}
	t, e = time.Parse("3:04", s)
	if e == nil {
		return t.Format("15:04")
	}
	return s
}

var ageRange = regexp.MustCompile(`([0-9]+)\s*歳?\s*[〜～~－-]\s*([0-9]+)\s*歳`)
var ageMin = regexp.MustCompile(`([0-9]+)\s*歳\s*(?:以上|から|[〜～~])`)
var ageMax = regexp.MustCompile(`([0-9]+)\s*歳\s*(?:以下|まで)`)

func AgeBand(label string) Object {
	out := Object{"text": strptr(label), "minimum_years": nil, "maximum_years": nil}
	if m := ageRange.FindStringSubmatch(label); len(m) == 3 {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		out["minimum_years"] = a
		out["maximum_years"] = b
		return out
	}
	if m := ageMin.FindStringSubmatch(label); len(m) == 2 {
		a, _ := strconv.Atoi(m[1])
		out["minimum_years"] = a
	}
	if m := ageMax.FindStringSubmatch(label); len(m) == 2 {
		a, _ := strconv.Atoi(m[1])
		out["maximum_years"] = a
	}
	return out
}
func decodeJSON(raw []byte, v any) error {
	d := json.NewDecoder(strings.NewReader(string(raw)))
	d.UseNumber()
	if e := d.Decode(v); e != nil {
		return fail(5, "Asoview response schema is invalid JSON: %v", e)
	}
	return nil
}
func optionsFrom(raw []any, dated bool, date any) ([]Object, error) {
	out := []Object{}
	basis := "advertised_band"
	if dated {
		basis = "date_specific_band"
	}
	for _, v := range raw {
		m := object(v)
		label := text(m["categoryLabel"])
		if label == "" {
			label = text(m["feeLabel"])
		}
		id := text(m["id"])
		if id == "" {
			id = text(m["basicFeeNumber"])
		}
		amount := m["sellingPrice"]
		regular := m["regularPrice"]
		if dated {
			amount = m["sellingFee"]
			regular = m["regularFee"]
		}
		if strings.TrimSpace(id) == "" || strings.TrimSpace(label) == "" {
			return nil, fail(5, "Asoview price band schema missing source ID or label")
		}
		if amount != nil {
			n := integer(amount)
			if n == nil || *n < 0 {
				return nil, fail(5, "Asoview price band amount invalid")
			}
		}
		out = append(out, Object{"id": strptr(id), "code": nullable(m["basicFeeCode"]), "name_ja": strptr(label), "age_band": AgeBand(label), "price": price(amount, m["unit"], basis, date), "regular_amount": integer(regular), "variable_price": m["isFluctuating"], "allocation": integer(m["allocation"]), "label_type": nullable(m["feeLabelType"]), "dependency_type": nullable(m["feeLabelDependencyType"])})
	}
	return out, nil
}
func validateQuantity(q int) error {
	if q < 1 || q > 50 {
		return fail(2, "--quantity must be 1..50")
	}
	return nil
}
func validLimit(n, max int) error {
	if n < 1 || n > max {
		return fail(2, "--limit must be 1..%d", max)
	}
	return nil
}
