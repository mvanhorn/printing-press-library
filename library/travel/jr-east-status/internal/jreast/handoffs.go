package jreast

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const PlannedURL = "https://www.jreast.co.jp/suspend/"
const CertificateURL = Origin + "/delay_certificate/"
const CertificateEnglishURL = Origin + "/delay_certificate/e/"
const CertificateCoverageURL = Origin + "/delay_certificate/e/rosen.html"

type Planned struct {
	LineNoticeFacts   []Notice   `json:"line_notice_facts,omitempty"`
	LineID            string     `json:"line_id"`
	Title             string     `json:"source_title"`
	DateExpressions   []string   `json:"source_date_expressions"`
	Year              *int       `json:"calendar_year"`
	Tables            [][]string `json:"source_table_facts"`
	Replacement       string     `json:"replacement_transport"`
	SourceURL         string     `json:"source_url"`
	ScheduleGuarantee bool       `json:"schedule_guarantee"`
}

// ParsePlanned returns selected construction table facts; missing years stay unknown.
func ParsePlanned(body []byte, line Line) ([]Planned, bool, error) {
	doc, e := html.Parse(strings.NewReader(string(body)))
	if e != nil {
		return nil, false, e
	}
	sections := walk(doc, func(n *html.Node) bool { return n.Data == "section" && class(n, "mb80") })
	if len(sections) == 0 {
		return nil, false, fmt.Errorf("JR East planned-work page has no recognized sections")
	}
	if len(sections) > 100 {
		return nil, false, fmt.Errorf("planned-work section scan exceeds 100")
	}
	out := make([]Planned, 0)
	clipped := false
	for _, sec := range sections {
		hs := walk(sec, func(n *html.Node) bool { return n.Data == "h2" })
		if len(hs) == 0 {
			continue
		}
		title := text(hs[0])
		if !plannedTitleMatches(line, title) {
			continue
		}
		if len(out) >= 6 {
			clipped = true
			continue
		}
		p := Planned{LineID: line.ID, Title: cut(title, 90), DateExpressions: make([]string, 0), Tables: make([][]string, 0), Replacement: "unknown", SourceURL: PlannedURL}
		if len([]rune(title)) > 90 {
			clipped = true
		}
		all := normText(text(sec))
		p.Year = commonCalendarYear(dateJA.FindAllString(all, -1))
		if strings.Contains(all, "代行輸送は行いません") {
			p.Replacement = "not_provided"
		}
		trs := walk(sec, func(n *html.Node) bool { return n.Data == "tr" })
		for _, tr := range trs {
			cells := walk(tr, func(n *html.Node) bool { return n.Data == "td" })
			if len(cells) == 0 {
				continue
			}
			if len(p.Tables) >= 12 {
				clipped = true
				break
			}
			row := make([]string, 0)
			for _, td := range cells {
				cell := text(td)
				if len([]rune(cell)) > 160 {
					clipped = true
				}
				row = append(row, cut(cell, 160))
			}
			if len(row) > 6 {
				return nil, false, fmt.Errorf("planned-work table has unsupported width")
			}
			p.Tables = append(p.Tables, row)
		}
		p.DateExpressions = dateJA.FindAllString(all, 13)
		if len(p.DateExpressions) > 12 {
			p.DateExpressions = p.DateExpressions[:12]
			clipped = true
		}
		if p.DateExpressions == nil {
			p.DateExpressions = make([]string, 0)
		}
		for _, n := range line.Notices {
			if n.Planned && n.Language == "ja" {
				p.LineNoticeFacts = append(p.LineNoticeFacts, n)
			}
		}
		out = append(out, p)
	}
	return out, clipped, nil
}

func plannedTitleMatches(line Line, title string) bool {
	if line.NameJA != "" && strings.Contains(Normalize(title), Normalize(line.NameJA)) {
		return true
	}
	// The official construction page uses the formal main-line name while
	// the Kanto operational page uses 東海道線 for this native line ID.
	return line.ID == "kanto:tokaidoline" && strings.Contains(Normalize(title), "東海道本線")
}

type CertificateSlot struct {
	ID               string `json:"slot_id"`
	Window           string `json:"slot_window_jst"`
	State            string `json:"publication_state"`
	DisplayMinutes   *int   `json:"display_delay_minutes"`
	LowerBound       bool   `json:"display_is_lower_bound"`
	ActualTrainDelay *int   `json:"actual_train_delay_minutes"`
	Date             string `json:"source_date_jst,omitempty"`
	URL              string `json:"published_url,omitempty"`
}

type Certificate struct {
	SourceCode     string            `json:"source_certificate_code,omitempty"`
	LineIDs        []string          `json:"source_line_ids"`
	NameJA         string            `json:"name_ja"`
	PublishedSlots int               `json:"published_slot_count"`
	Slots          []CertificateSlot `json:"slots,omitempty"`
	CoverageURL    string            `json:"section_coverage_url"`
	SourceURL      string            `json:"source_url"`
	EnglishURL     string            `json:"english_source_url"`
	Meaning        string            `json:"meaning"`
	RoutingNote    string            `json:"routing_note,omitempty"`
	Handoff        string            `json:"handoff,omitempty"`
	Alternatives   []string          `json:"choose_actual_segment_from,omitempty"`
}

var slotWindows = []string{"first train–07:00", "07:00–10:00", "10:00–16:00", "16:00–21:00", "21:00–last train (may cross midnight)"}
var minutesRE = regexp.MustCompile(`\d+`)

// ParseCertificates preserves only actual published URLs; dash cells are explicit unknowns.
func ParseCertificates(body []byte, now time.Time) ([]Certificate, SourceState, error) {
	doc, e := html.Parse(strings.NewReader(string(body)))
	if e != nil {
		return nil, SourceState{}, e
	}
	out := make([]Certificate, 0)
	for _, tr := range walk(doc, func(n *html.Node) bool { return n.Data == "tr" }) {
		name := firstClass(tr, "delaycertificate-table__routename")
		if name == nil {
			continue
		}
		ids := make([]string, 0)
		for _, a := range walk(name, func(n *html.Node) bool { return n.Data == "a" }) {
			u, e := url.Parse(attr(a, "href"))
			if e == nil {
				if id := u.Query().Get("lineid"); id != "" {
					ids = addUnique(ids, id)
				}
			}
		}
		if len(ids) == 0 {
			return nil, SourceState{}, fmt.Errorf("certificate row lacks a native line identity")
		}
		cells := walk(tr, func(n *html.Node) bool { return n.Data == "td" })
		if len(cells) != 5 {
			return nil, SourceState{}, fmt.Errorf("certificate source has %d slots; expected five", len(cells))
		}
		c := Certificate{LineIDs: ids, NameJA: text(name), Slots: make([]CertificateSlot, 0), CoverageURL: CertificateCoverageURL, SourceURL: CertificateURL, EnglishURL: CertificateEnglishURL, Meaning: "Maximum delay on the covered route/time slot, rounded by the source; does not establish individual train delay or boarding."}
		if strings.Contains(strings.Join(ids, ","), "takasakiline") {
			c.RoutingNote = "For Tokyo–Omiya on the Takasaki Line, the source directs users to the Utsunomiya Line certificate."
		}
		if strings.Contains(strings.Join(ids, ","), "omeline") {
			c.RoutingNote = "This certificate covers Tachikawa–Ome; Ome–Okutama uses DOKOTORE."
		}
		for i, td := range cells {
			slot := CertificateSlot{ID: fmt.Sprintf("%02d", i+1), Window: slotWindows[i], State: "not_published_or_below_threshold"}
			as := walk(td, func(n *html.Node) bool { return n.Data == "a" })
			if len(as) > 0 {
				u, e := url.Parse(attr(as[0], "href"))
				if e != nil {
					return nil, SourceState{}, e
				}
				base, _ := url.Parse(Origin)
				u = base.ResolveReference(u)
				q := u.Query()
				date, e := time.ParseInLocation("20060102", q.Get("D"), JST)
				if e != nil || !allowedURL(u) || u.Path != "/delay_certificate/pop.aspx" || q.Get("T") != slot.ID || !regexp.MustCompile(`^\d{2}$`).MatchString(q.Get("R")) {
					return nil, SourceState{}, fmt.Errorf("unrecognized published certificate URL")
				}
				c.SourceCode = q.Get("R")
				slot.State = "published"
				slot.Date = date.Format("2006-01-02")
				slot.URL = u.String()
				c.PublishedSlots++
				label := text(as[0])
				if m := minutesRE.FindString(normText(label)); m != "" {
					v, _ := strconv.Atoi(m)
					slot.DisplayMinutes = &v
					slot.LowerBound = strings.Contains(label, "以上") || strings.Contains(label, "more")
				}
			}
			c.Slots = append(c.Slots, slot)
		}
		out = append(out, c)
		if len(out) > 30 {
			return nil, SourceState{}, fmt.Errorf("certificate row scan exceeds 30")
		}
	}
	if len(out) == 0 {
		return nil, SourceState{}, fmt.Errorf("certificate source has no recognizable route table")
	}
	st := pageState(doc, "ja", CertificateURL, now, 20*time.Minute)
	st.ScannedRows = len(out)
	return out, st, nil
}

// CertificateRoute handles first-party through-service and regional-service handoff rules.
func CertificateRoute(line string) *Certificate {
	id := strings.TrimPrefix(line, "kanto:")
	routes := map[string][]string{
		"shonan-shinjukuline": {"utsunomiyaline", "takasakiline", "saikyoline", "kawagoeline", "yokosukaline", "sobuline_rapidservice", "tokaidoline"},
		"ueno-tokyoline":      {"utsunomiyaline", "takasakiline", "tokaidoline", "jobanline", "jobanline_rapidservice"},
		"sotetsuline":         {"saikyoline", "kawagoeline", "yokosukaline"},
	}
	if ids, ok := routes[id]; ok {
		return &Certificate{LineIDs: []string{id}, NameJA: catalogue["kanto"][id].JA, SourceURL: CertificateURL, EnglishURL: CertificateEnglishURL, CoverageURL: CertificateCoverageURL, Meaning: "Choose the certificate for the actual section travelled; through-service names have no single automatic certificate.", Alternatives: ids, Handoff: CertificateCoverageURL}
	}
	if id == "sagamiline" {
		return &Certificate{LineIDs: []string{id}, NameJA: "相模線", SourceURL: CertificateURL, EnglishURL: CertificateEnglishURL, CoverageURL: CertificateCoverageURL, Handoff: "https://doko-train.jp/en/pc/delaycertificate.html", Meaning: "JR East directs Sagami Line certificates to DOKOTORE; availability is not checked here."}
	}
	return nil
}
