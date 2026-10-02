// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package yamato

import (
	"context"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/yamato/internal/cliutil"
	"regexp"
	"strconv"
	"strings"
)

var productPaths = map[string]string{"takkyubin": "takkyubin", "airport": "airport", "roundtrip": "bothways", "same-day": "same-day-delivery"}

type SizeLimit struct {
	Size     int `json:"size"`
	LinearCM int `json:"linear_cm"`
	WeightKG int `json:"weight_kg"`
}
type Product struct {
	ID             string      `json:"id"`
	URL            string      `json:"url"`
	Title          string      `json:"title"`
	SizeLimits     []SizeLimit `json:"size_limits,omitempty"`
	Conditions     []string    `json:"conditions"`
	SourceEvidence []string    `json:"source_evidence"`
	Acceptance     string      `json:"acceptance"`
}

var limitsRe = regexp.MustCompile(`Size (60|80|100|120|140|160|180|200) Up to (\d+)\s*cm Up to (\d+)\s*kg`)

func (c *Client) Product(ctx context.Context, id string) (Product, error) {
	p := Product{ID: id, Conditions: []string{}, SourceEvidence: []string{}, Acceptance: "unknown: policy eligibility is not parcel or destination acceptance"}
	suffix, ok := productPaths[id]
	if !ok {
		return p, fmt.Errorf("--service must be takkyubin, airport, roundtrip or same-day")
	}
	p.URL = Main + "/ytc/en/send/services/" + suffix + "/"
	b, e := c.Fetch(ctx, p.URL, nil, 2<<20)
	if e != nil {
		return p, e
	}
	doc, e := Parse(b)
	if e != nil {
		return p, e
	}
	titles := Nodes(doc, "title")
	if len(titles) == 0 {
		return p, fmt.Errorf("source title missing")
	}
	p.Title = Text(titles[0])
	t := Text(doc)
	for _, m := range limitsRe.FindAllStringSubmatch(t, -1) {
		s, _ := strconv.Atoi(m[1])
		l, _ := strconv.Atoi(m[2])
		w, _ := strconv.Atoi(m[3])
		p.SizeLimits = append(p.SizeLimits, SizeLimit{s, l, w})
	}
	if id != "same-day" {
		if len(p.SizeLimits) != 8 {
			return p, fmt.Errorf("source size/weight table changed; open %s", p.URL)
		}
		p.Conditions = append(p.Conditions, "Use the greater size or weight category; max200cm linear dimensions/30kg, longest side170cm or100cm upright-only.", "Some convenience stores cannot accept parcels larger than size180; verify the local counter.")
	}
	p.Conditions = append(p.Conditions, "Dates are estimated; counter dispatch cutoff, exceptional holidays, packaging, contents and receiving acceptance require confirmation.")
	switch id {
	case "takkyubin":
		p.Conditions = append(p.Conditions, "Standard TA-Q-BIN is distinct from limited-area same-day delivery.", "Drop-off100JPY, digital60JPY and member discounts require source conditions; never subtract all discounts automatically.")
	case "airport":
		p.Conditions = append(p.Conditions, "Outbound airport fee660JPY is included in calculator tariff. From-airport charges differ; Kansai return fee660JPY applies.", "Ship by source boarding deadline, usually two days before flight or three in some areas; dispatch cutoff depends on counter.", "No time-zone delivery or cash-on-delivery when sending to airports.")
	case "roundtrip":
		p.Conditions = append(p.Conditions, "Two-leg source tariff includes roundtrip reduction; return within one month of outbound dropoff.", "Lodging roundtrip arrives by day before use; use live calendar deadline and confirm reception.")
	case "same-day":
		p.Conditions = append(p.Conditions, "Only listed counters/service areas and cutoffs; standard TA-Q-BIN quote is not same-day eligibility.", "Counter fees and destination arrival times vary; express fee may apply when sending to airport.")
	}
	for _, needle := range []string{"The parcel size is", "When either the parcel size", "two days prior", "The return service can", "The cut-off time", "Fees vary", "Business hours and fees"} {
		at := strings.Index(t, needle)
		if at >= 0 {
			end := min(at+350, len(t))
			text := t[at:end]
			if stop := strings.Index(text, "."); stop >= 0 {
				text = text[:stop+1]
			}
			p.SourceEvidence = append(p.SourceEvidence, text)
		}
	}
	return p, nil
}

type SameDaySchedule struct {
	ID            string `json:"id"`
	NameJP        string `json:"name_jp"`
	Name          string `json:"name"`
	CutoffJST     string `json:"cutoff_jst"`
	ServiceArea   string `json:"service_area"`
	ArrivalWindow string `json:"arrival_window_jst"`
	FeeBasis      string `json:"fee_basis"`
	Acceptance    string `json:"acceptance"`
}
type FeeExample struct {
	Route string `json:"route"`
	Size  int    `json:"size"`
	JPY   int    `json:"jpy"`
	Basis string `json:"basis"`
}
type SameDay struct {
	Schedules          []SameDaySchedule `json:"schedules"`
	ScheduleStatus     string            `json:"schedule_status"`
	ScheduleVersion    string            `json:"schedule_version"`
	ScheduleObservedAt string            `json:"schedule_observed_at"`
	Coverage           string            `json:"coverage"`
	PDFURL             string            `json:"pdf_url"`
	DirectoryURL       string            `json:"directory_url"`
	FeeExamples        []FeeExample      `json:"fee_examples"`
	Conditions         []string          `json:"conditions"`
}

func (c *Client) SameDay(ctx context.Context, q string, limit int) (SameDay, error) {
	d := SameDay{Schedules: []SameDaySchedule{}, ScheduleVersion: "2026/09/01ver", ScheduleObservedAt: ObservedAt, Coverage: "Selected Narita/Haneda airport-to-hotel rows. For all other counters, directions and destinations use full source PDF and directory.", PDFURL: SameDayPDF, DirectoryURL: Main + "/ytc/en/send/services/baggage-branch-list/", FeeExamples: []FeeExample{}, Conditions: []string{"Source schedule evidence is not confirmed acceptance or a guarantee; verify destination, cutoff and operating calendar at the counter.", "Cutoff is separate from counter closing time. Optional/express fees and calendar exceptions must be confirmed.", "Example prices are route/size examples, not a quote for the listed schedules."}}
	b, e := c.Fetch(ctx, SameDayPDF, nil, 8<<20)
	if e != nil {
		return d, e
	}
	if len(b) < 5 || string(b[:5]) != "%PDF-" {
		return d, fmt.Errorf("same-day source is not a PDF; no schedule claimed")
	}
	if c.Sources[len(c.Sources)-1].SHA256 == "7764d6a096766794144bf3dfda916c5c848f37243aac9026168b939902dc23bc" {
		d.ScheduleStatus = "current PDF bytes match verified source version"
		for _, row := range sameDaySnapshot() {
			if Matches(q, row.ID, row.Name, row.NameJP, row.ServiceArea) {
				d.Schedules = append(d.Schedules, row)
				if len(d.Schedules) == limit {
					break
				}
			}
		}
	} else {
		d.ScheduleStatus = "unknown: source PDF changed; schedules suppressed; inspect full current PDF"
	}
	purl := Main + "/ytc/en/send/services/same-day-delivery/"
	page, e := c.Fetch(ctx, purl, nil, 2<<20)
	if e != nil {
		var rl *cliutil.RateLimitError
		if errors.As(e, &rl) {
			return d, e
		}
		c.Failures = append(c.Failures, FetchFailure{purl, e.Error()})
		d.Conditions = append(d.Conditions, "Example fees unavailable: "+e.Error())
		return d, nil
	}
	doc, e := Parse(page)
	if e != nil {
		return d, e
	}
	t := Text(doc)
	price := regexp.MustCompile(`160 size ([0-9,]+) yen/piece`)
	for _, route := range []string{"Delivery within the 23 wards of Tokyo from Haneda Airport", "Delivery wihin Kyoto city from Kyoto Station Service Counter"} {
		at := strings.Index(t, route)
		if at < 0 {
			continue
		}
		chunk := t[at:min(at+250, len(t))]
		m := price.FindStringSubmatch(chunk)
		if m == nil {
			continue
		}
		v, e := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
		if e == nil && v > 0 {
			d.FeeExamples = append(d.FeeExamples, FeeExample{route, 160, v, "live published example, tax/discount details require source confirmation"})
		}
	}
	return d, nil
}
func sameDaySnapshot() []SameDaySchedule {
	areaN := "Ibaraki, Tochigi, Gunma, Saitama, Chiba, Tokyo, Kanagawa and Yamanashi prefectures"
	areaH := "Hotels in Tokyo23 wards, Urayasu, Kawasaki, Yokohama (Nishi/Naka/Tsurumi/Kanagawa/Hodogaya/Minami/Konan), Ichikawa, Funabashi, Narashino and Chiba (Hanamigawa/Inage/Mihama/Chuo)"
	var out []SameDaySchedule
	for _, r := range []struct{ id, jp, en, cutoff string }{{"narita1_north_send", "成田空港第1ターミナル北ウィング1階", "Narita Terminal1 North1F", "10:30"}, {"narita1_south_send", "成田空港第1ターミナル南ウィング1階", "Narita Terminal1 South1F", "10:30"}, {"narita2_send", "成田空港第2ターミナル1階", "Narita Terminal2 1F", "10:30"}, {"narita3", "成田空港第3ターミナル1階", "Narita Terminal3 1F", "09:50"}} {
		out = append(out, SameDaySchedule{r.id, r.jp, r.en, r.cutoff, areaN, "18:00–21:00", "TA-Q-BIN dropoff tariff, size/address dependent", "unknown; source lists service area and cutoff"})
	}
	for _, r := range []struct{ id, jp, en string }{{"haneda_terminal1", "羽田空港第1旅客ターミナル国内線手荷物カウンター（発送）", "Haneda Terminal1 Domestic Send"}, {"haneda_terminal2_send", "羽田空港第2旅客ターミナル国際線手荷物カウンター（発送）", "Haneda Terminal2 International Send"}, {"haneda_terminal2", "羽田空港第2旅客ターミナル国内線手荷物カウンター（発送）", "Haneda Terminal2 Domestic Send"}, {"haneda_terminal2_p", "羽田空港第2旅客ターミナル国際線手荷物カウンター（受取）", "Haneda Terminal2 International Pickup"}, {"haneda_send", "羽田空港第3旅客ターミナル国際線手荷物カウンター（発送）", "Haneda Terminal3 International Send"}} {
		out = append(out, SameDaySchedule{r.id, r.jp, r.en, "11:00", areaH, "18:00 onward", "standard TA-Q-BIN plus value-added surcharge; amount unknown for this row", "unknown; only listed hotels/areas"})
	}
	return out
}
