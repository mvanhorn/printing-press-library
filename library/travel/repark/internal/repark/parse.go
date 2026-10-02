package repark

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/repark/internal/cliutil"
	"golang.org/x/net/html"
)

var windowRE = regexp.MustCompile(`([0-9]{1,2}):([0-9]{2})\s*[-～~－]\s*([0-9]{1,2}):([0-9]{2})`)
var unitRateRE = regexp.MustCompile(`([0-9,]+)\s*分\s*/\s*([0-9,]+)\s*円`)
var amountRE = regexp.MustCompile(`([0-9,]+)\s*円`)
var elapsedRE = regexp.MustCompile(`(?:入庫後|駐車後)\s*([0-9]+)\s*時間`)
var dayRE = regexp.MustCompile(`【([^】]+)】`)
var numberRE = regexp.MustCompile(`[0-9]+`)
var negatedBayDifferenceRE = regexp.MustCompile(`異な(?:りません|らない|って(?:いません|おりません|いない))|違い(?:は|が)?(?:ありません|ない|無い|なし|無し)|違(?:いません|わない)`)

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func hasClass(n *html.Node, c string) bool {
	for _, a := range strings.Fields(attr(n, "class")) {
		if a == c {
			return true
		}
	}
	return false
}
func walk(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}
func nodes(n *html.Node, fn func(*html.Node) bool) []*html.Node {
	out := []*html.Node{}
	walk(n, func(x *html.Node) {
		if fn(x) {
			out = append(out, x)
		}
	})
	return out
}
func text(n *html.Node) string {
	var b strings.Builder
	var visit func(*html.Node)
	visit = func(x *html.Node) {
		if x.Type == html.ElementNode && (x.Data == "script" || x.Data == "style" || x.Data == "noscript") {
			return
		}
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		if x.Type == html.ElementNode && (x.Data == "br" || x.Data == "p" || x.Data == "div" || x.Data == "span") {
			b.WriteByte('\n')
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(n)
	return strings.TrimSpace(b.String())
}
func clean(s string) string { return strings.Join(strings.Fields(cliutil.CleanText(s)), " ") }
func fragment(s string) string {
	n, e := html.Parse(strings.NewReader(s))
	if e != nil {
		return clean(s)
	}
	return text(n)
}
func input(n *html.Node, id string) string {
	for _, x := range nodes(n, func(x *html.Node) bool { return x.Data == "input" && (attr(x, "id") == id || attr(x, "name") == id) }) {
		return attr(x, "value")
	}
	return ""
}
func byClass(n *html.Node, c string) []*html.Node {
	return nodes(n, func(x *html.Node) bool { return x.Type == html.ElementNode && hasClass(x, c) })
}
func onlyText(n *html.Node, c string) string {
	xs := byClass(n, c)
	if len(xs) > 0 {
		return clean(text(xs[0]))
	}
	return ""
}
func number(s string) *int {
	s = strings.ReplaceAll(s, ",", "")
	v, e := strconv.Atoi(s)
	if e != nil || v < 0 {
		return nil
	}
	return &v
}
func numeric(s string) *float64 {
	v, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if e != nil || v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
func boolp(v bool) *bool       { return &v }
func stamp(t time.Time) string { return t.In(JST).Format(time.RFC3339) }
func fullWidth(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '０' && r <= '９' {
			return r - '０' + '0'
		}
		return r
	}, s)
}

func occupancy(code, label string, t time.Time) Occupancy {
	categories := map[string]string{"0": "available", "1": "crowded", "2": "full"}
	labels := map[string]string{"0": "空車", "1": "混雑", "2": "満車"}
	if code == "" {
		for k, v := range labels {
			if label == v {
				code = k
			}
		}
	}
	cat := categories[code]
	if cat == "" {
		cat = "unknown"
	}
	if label == "" {
		label = labels[code]
	}
	if label == "" {
		label = "満空情報なし"
	}
	return Occupancy{Category: cat, LabelJA: label, SourceCode: code, ObservedAt: stamp(t)}
}

func parseWindow(s string) (string, string, bool) {
	m := windowRE.FindStringSubmatch(fullWidth(s))
	if len(m) == 0 {
		return "", "", false
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[3])
	am, _ := strconv.Atoi(m[2])
	bm, _ := strconv.Atoi(m[4])
	if a > 23 || b > 24 || am > 59 || bm > 59 || (b == 24 && bm != 0) {
		return "", "", false
	}
	return fmt.Sprintf("%02d:%02d", a, am), fmt.Sprintf("%02d:%02d", b, bm), b*60+bm < a*60+am
}
func parseRate(day, window, charge, note string) Rate {
	a, b, overnight := parseWindow(window)
	r := Rate{DayType: clean(day), Start: a, End: b, Overnight: overnight, SourceText: clean(window + " " + charge), SourceNote: clean(note)}
	if m := unitRateRE.FindStringSubmatch(fullWidth(charge)); len(m) > 0 {
		r.IntervalMinutes = number(m[1])
		r.AmountJPY = number(m[2])
	}
	return r
}
func maximums(s string) ([]Maximum, string) {
	application := "unspecified"
	n := fullWidth(s)
	if strings.Contains(n, "繰り返し") || strings.Contains(n, "繰返し") {
		application = "repeating"
	}
	if strings.Contains(n, "1回限り") || strings.Contains(n, "一回限り") || strings.Contains(n, "1回のみ") {
		application = "one_time"
	}
	out := []Maximum{}
	for _, line := range strings.Split(strings.ReplaceAll(fragment(s), "\r", ""), "\n") {
		line = clean(line)
		if !strings.Contains(line, "円") {
			continue
		}
		v := Maximum{Kind: "unparsed", Application: application, SourceText: line}
		normalized := fullWidth(line)
		if m := dayRE.FindStringSubmatch(line); len(m) > 0 {
			v.DayType = m[1]
		}
		amounts := amountRE.FindAllStringSubmatch(normalized, -1)
		scoped := strings.Contains(normalized, "番") || strings.Contains(normalized, "車室")
		if len(amounts) == 1 && !scoped {
			v.AmountJPY = number(amounts[0][1])
		}
		if a, b, o := parseWindow(normalized); a != "" {
			v.Kind = "time_window"
			v.Start = a
			v.End = b
			v.Overnight = o
		}
		if m := elapsedRE.FindStringSubmatch(normalized); len(m) > 0 {
			v.Kind = "elapsed_after_entry"
			v.ElapsedHours = number(m[1])
		}
		if strings.Contains(normalized, "当日") || strings.Contains(normalized, "当日24時") {
			v.Kind = "calendar_day"
		}
		if len(amounts) != 1 || scoped {
			v.Kind = "unparsed"
			v.AmountJPY = nil
		}
		out = append(out, v)
	}
	return out, application
}
func sourceDeclaresBayVariation(s string) bool {
	for _, clause := range strings.FieldsFunc(s, func(r rune) bool { return r == '。' || r == ';' || r == '；' || r == '\n' }) {
		byBay := strings.Contains(clause, "車室ごと") || strings.Contains(clause, "車室毎") || strings.Contains(clause, "車室により") || strings.Contains(clause, "車室によって")
		if byBay && !negatedBayDifferenceRE.MatchString(clause) && (strings.Contains(clause, "異な") || strings.Contains(clause, "違い") || strings.Contains(clause, "違う")) {
			return true
		}
	}
	return false
}

func parseLimits(s string) Limits {
	l := Limits{SourceText: clean(s)}
	normalized := fullWidth(s)
	for _, p := range []struct {
		name   string
		target **float64
	}{{"高さ", &l.HeightM}, {"長さ", &l.LengthM}, {"幅", &l.WidthM}, {"重量", &l.WeightT}} {
		r := regexp.MustCompile(regexp.QuoteMeta(p.name) + `\s*([0-9]+(?:\.[0-9]+)?)\s*([mt])`)
		if m := r.FindStringSubmatch(normalized); len(m) > 0 {
			*p.target = numeric(m[1])
		}
	}
	if sourceDeclaresBayVariation(s) {
		l.PerBay = boolp(true)
	}
	if l.PerBay != nil && *l.PerBay {
		l.Note = l.SourceText
	}
	return l
}
func tableValues(n *html.Node) map[string]string {
	values := map[string]string{}
	for _, row := range nodes(n, func(x *html.Node) bool { return x.Data == "tr" }) {
		key := ""
		for c := row.FirstChild; c != nil; c = c.NextSibling {
			if c.Data == "th" {
				key = clean(text(c))
			} else if c.Data == "td" && key != "" {
				values[key] = clean(text(c))
				key = ""
			}
		}
	}
	return values
}

func parseDetail(body []byte, id string, t time.Time) (Lot, error) {
	n, e := html.Parse(strings.NewReader(string(body)))
	if e != nil {
		return Lot{}, e
	}
	if input(n, "park_code") != id {
		return Lot{}, fmt.Errorf("source detail did not identify %s; lot may be unavailable or the page format changed", id)
	}
	values := tableValues(n)
	lot := Lot{ID: id, Name: values["駐車場名"], Address: values["所在地"], SourceURL: DetailURL(id), ObservedAt: stamp(t), Rates: []Rate{}, Maximums: []Maximum{}}
	if lot.Name == "" || lot.Address == "" {
		return Lot{}, fmt.Errorf("source detail lacks parking identity for %s", id)
	}
	if m := numberRE.FindString(fullWidth(values["収容台数"])); m != "" {
		lot.Capacity = number(m)
	}
	lot.Hours = Hours{Text: values["営業時間"]}
	if lot.Hours.Text == "24時間" {
		lot.Hours.Open24H = boolp(true)
	}
	lot.Limits = parseLimits(values["車両制限"])
	lot.Fit = AssessFit(lot.Limits, Vehicle{})
	lat := numeric(input(n, "latitude"))
	lon := numeric(input(n, "longitude"))
	if lat != nil && lon != nil {
		p := Coordinates{*lat, *lon}
		if ValidateCoordinates(p) == nil {
			lot.Coordinates = &p
		}
	}
	lot.Occupancy = occupancy(input(n, "status"), "", t)
	for _, section := range byClass(n, "lv3") {
		day := ""
		hs := nodes(section, func(x *html.Node) bool { return x.Data == "h3" })
		if len(hs) > 0 {
			day = clean(text(hs[0]))
		}
		for _, band := range byClass(section, "unit-inner-quarter-wide") {
			window := onlyText(band, "unit-inner-quarter-charge")
			charge := onlyText(band, "unit-inner-quarter-charge-note")
			if window != "" && charge != "" {
				lot.Rates = append(lot.Rates, parseRate(day, window, charge, ""))
			}
		}
	}
	for _, section := range byClass(n, "lv2") {
		for _, h := range nodes(section, func(x *html.Node) bool { return x.Data == "h2" }) {
			if clean(text(h)) == "料金体系" {
				lot.SourceRateText = clean(text(section))
				break
			}
		}
		if lot.SourceRateText != "" {
			break
		}
	}
	lot.PricingParseStatus = pricingStatus(lot.Rates)
	for _, p := range nodes(n, func(x *html.Node) bool { return x.Data == "p" }) {
		s := strings.TrimSpace(text(p))
		if strings.HasPrefix(s, "最大料金") {
			lot.ChargeNote = s
			break
		}
	}
	lot.Maximums, lot.MaximumApplication = maximums(lot.ChargeNote)
	for _, a := range nodes(n, func(x *html.Node) bool { return x.Data == "a" }) {
		if attr(a, "href") == "/parking_user/time/result/calculation/?park="+id {
			lot.CalculatorURL = Origin + attr(a, "href")
		}
	}
	if strings.Contains(lot.SourceRateText+lot.ChargeNote, "税込") {
		lot.TaxIncluded = boolp(true)
	} else if strings.Contains(lot.SourceRateText+lot.ChargeNote, "税抜") {
		lot.TaxIncluded = boolp(false)
	}
	return lot, nil
}

type marker struct {
	ID          string          `json:"park_code"`
	Name        string          `json:"park_name"`
	Address     string          `json:"full_address"`
	Latitude    string          `json:"latitude"`
	Longitude   string          `json:"longitude"`
	Capacity    string          `json:"capacity"`
	Status      string          `json:"status"`
	Height      string          `json:"height_limit"`
	Length      string          `json:"length_limit"`
	Width       string          `json:"width_limit"`
	Weight      string          `json:"weight_limit"`
	LimitNote   string          `json:"limit_note"`
	OpenType    string          `json:"open_type"`
	OpenStart   string          `json:"open_start_time"`
	OpenEnd     string          `json:"open_end_time"`
	OpenNote    string          `json:"open_time_note"`
	ChargeNote  string          `json:"charge_note"`
	Attention   string          `json:"attention"`
	ImportDate  string          `json:"import_date"`
	UpdatedAt   string          `json:"updated_at"`
	ServiceCode string          `json:"service_code"`
	Charges     json.RawMessage `json:"charges"`
}

func parseMarkers(body []byte, t time.Time) ([]Lot, error) {
	lots, _, err := parseMarkersBounded(body, t, 0)
	return lots, err
}

func parseMarkersBounded(body []byte, t time.Time, limit int) ([]Lot, int, error) {
	var rows []marker
	if e := json.Unmarshal(body, &rows); e != nil {
		return nil, 0, fmt.Errorf("invalid source markers JSON: %w", e)
	}
	if rows == nil && strings.TrimSpace(string(body)) != "[]" {
		return nil, 0, fmt.Errorf("source markers response is not an array")
	}
	total := len(rows)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	out := make([]Lot, 0, len(rows))
	for _, r := range rows {
		if !idPattern.MatchString(r.ID) || r.Name == "" {
			return nil, 0, fmt.Errorf("source marker lacks canonical parking identity")
		}
		lat := numeric(r.Latitude)
		lon := numeric(r.Longitude)
		if lat == nil || lon == nil {
			return nil, 0, fmt.Errorf("source marker %s has invalid coordinates", r.ID)
		}
		point := Coordinates{*lat, *lon}
		if e := ValidateCoordinates(point); e != nil {
			return nil, 0, e
		}
		l := Lot{ID: r.ID, Name: clean(r.Name), Address: clean(r.Address), Coordinates: &point, Capacity: number(r.Capacity), SourceURL: DetailURL(r.ID), ObservedAt: stamp(t), SourceImportDate: r.ImportDate, SourceUpdatedAt: r.UpdatedAt, Attention: clean(fragment(r.Attention)), Rates: []Rate{}}
		l.Occupancy = occupancy(r.Status, "", t)
		l.Limits = Limits{HeightM: numeric(r.Height), LengthM: numeric(r.Length), WidthM: numeric(r.Width), WeightT: numeric(r.Weight), Note: clean(fragment(r.LimitNote))}
		if sourceDeclaresBayVariation(l.Limits.Note) {
			l.Limits.PerBay = boolp(true)
		}
		limitParts := []string{}
		for _, p := range []struct{ label, value, unit string }{{"高さ", r.Height, "m"}, {"長さ", r.Length, "m"}, {"幅", r.Width, "m"}, {"重量", r.Weight, "t"}} {
			if p.value != "" {
				limitParts = append(limitParts, p.label+p.value+p.unit)
			}
		}
		l.Limits.SourceText = strings.Join(limitParts, "、")
		l.Hours = Hours{SourceType: r.OpenType, Start: r.OpenStart, End: r.OpenEnd, Note: clean(fragment(r.OpenNote))}
		if r.OpenType == "1" {
			l.Hours.Text = "24時間"
			l.Hours.Open24H = boolp(true)
		} else if r.OpenStart != "" || r.OpenEnd != "" {
			l.Hours.Text = clean(r.OpenStart + "-" + r.OpenEnd + " " + r.OpenNote)
		}
		groups, err := orderedSourceValues(r.Charges)
		if err != nil {
			return nil, 0, fmt.Errorf("source %s tariff groups: %w", r.ID, err)
		}
		for _, rawGroup := range groups {
			var g struct {
				Name    string          `json:"name"`
				Charges json.RawMessage `json:"charges"`
			}
			if err := json.Unmarshal(rawGroup, &g); err != nil {
				return nil, 0, fmt.Errorf("source %s tariff group: %w", r.ID, err)
			}
			bands, err := orderedSourceValues(g.Charges)
			if err != nil {
				return nil, 0, fmt.Errorf("source %s rate bands: %w", r.ID, err)
			}
			for _, rawBand := range bands {
				var b struct {
					Time   string `json:"time"`
					Charge string `json:"charge"`
					Note   string `json:"note"`
				}
				if err := json.Unmarshal(rawBand, &b); err != nil {
					return nil, 0, fmt.Errorf("source %s rate band: %w", r.ID, err)
				}
				l.Rates = append(l.Rates, parseRate(g.Name, b.Time, b.Charge, b.Note))
				l.SourceRateText += clean(g.Name+" "+b.Time+" "+b.Charge+" "+b.Note) + "\n"
			}
		}
		l.SourceRateText = strings.TrimSpace(l.SourceRateText)
		l.PricingParseStatus = pricingStatus(l.Rates)
		l.ChargeNote = strings.TrimSpace(fragment(r.ChargeNote))
		l.Maximums, l.MaximumApplication = maximums(r.ChargeNote)
		if strings.Contains(l.SourceRateText+r.ChargeNote, "税込") {
			l.TaxIncluded = boolp(true)
		} else if strings.Contains(l.SourceRateText+r.ChargeNote, "税抜") {
			l.TaxIncluded = boolp(false)
		}
		if len(r.ServiceCode) > 14 && r.ServiceCode[14] == '1' {
			l.CalculatorURL = Origin + "/parking_user/time/result/calculation/?park=" + r.ID
		}
		l.Fit = AssessFit(l.Limits, Vehicle{})
		out = append(out, l)
	}
	return out, total, nil
}

// PHP encodes non-contiguous numeric array keys as a JSON object. Weekday
// groups are commonly keyed 1/3 or 1/4, while all-day groups use an array.
func orderedSourceValues(raw json.RawMessage) ([]json.RawMessage, error) {
	trim := strings.TrimSpace(string(raw))
	if trim == "" || trim == "null" {
		return []json.RawMessage{}, nil
	}
	if strings.HasPrefix(trim, "[") {
		var a []json.RawMessage
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		return a, nil
	}
	if strings.HasPrefix(trim, "{") {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, err
		}
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			a, e := strconv.Atoi(keys[i])
			b, f := strconv.Atoi(keys[j])
			if e == nil && f == nil {
				return a < b
			}
			return keys[i] < keys[j]
		})
		a := make([]json.RawMessage, 0, len(keys))
		for _, k := range keys {
			a = append(a, m[k])
		}
		return a, nil
	}
	return nil, fmt.Errorf("expected source array or indexed object")
}

func pricingStatus(rates []Rate) string {
	if len(rates) == 0 {
		return "source_structure_not_normalized"
	}
	for _, r := range rates {
		if r.Start == "" || r.End == "" || r.AmountJPY == nil || r.IntervalMinutes == nil {
			return "partially_normalized_source_text_preserved"
		}
	}
	return "rate_bands_normalized_maximum_conditions_preserved"
}
