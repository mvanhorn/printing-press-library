package jreast

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/jr-east-status/internal/cliutil"
)

func at(s string) time.Time {
	t, e := time.Parse(time.RFC3339, s)
	if e != nil {
		panic(e)
	}
	return t
}

func TestReportingCoverage(t *testing.T) {
	for _, tc := range []struct{ at, state, day string }{
		{"2026-10-03T00:00:00+09:00", "open", "2026-10-02"},
		{"2026-10-03T01:59:59+09:00", "open", "2026-10-02"},
		{"2026-10-03T02:00:00+09:00", "outside_reporting_hours", "2026-10-02"},
		{"2026-10-03T03:59:59+09:00", "outside_reporting_hours", "2026-10-02"},
		{"2026-10-03T04:00:00+09:00", "open", "2026-10-03"},
		{"2026-10-02T17:00:00Z", "outside_reporting_hours", "2026-10-02"},
	} {
		t.Run(tc.at, func(t *testing.T) {
			c := ReportingCoverage(at(tc.at))
			if c.ReportingState != tc.state || c.ServiceDay != tc.day || c.ActualDelayMinutes != nil || !c.ThresholdConflict {
				t.Fatalf("coverage = %+v", c)
			}
		})
	}
}

func TestRegionsAndResolution(t *testing.T) {
	rs := Regions()
	if len(rs) != 5 {
		t.Fatal(len(rs))
	}
	for _, r := range rs {
		v, e := RegionByID(r.ID)
		if e != nil || v != r || r.NameJA == "" || !strings.HasPrefix(r.SourceEN, Origin) {
			t.Fatal(r, v, e)
		}
	}
	for _, tc := range []struct {
		id, want string
		ok       bool
	}{{"kanto", "kanto", true}, {"chyokyori", "express", true}, {"express", "express", true}, {"all", "", false}, {"../../x", "", false}} {
		v, e := RegionByID(tc.id)
		if (e == nil) != tc.ok || tc.ok && v.ID != tc.want {
			t.Fatalf("%s: %+v %v", tc.id, v, e)
		}
	}
}

func TestNormalizeAndFindLine(t *testing.T) {
	for _, tc := range []struct{ in, want string }{{" Tōkaidō Line ", "tokaido line"}, {"１０月３日", "10月3日"}, {"YAMANOTELINE", "yamanoteline"}, {"わかしお・さざなみ", "わかしお・さざなみ"}, {"ｻﾞﾝ", "ザン"}} {
		if got := Normalize(tc.in); got != tc.want {
			t.Fatalf("Normalize(%q)=%q", tc.in, got)
		}
	}
	if Normalize("わかしお・さざなみ") == Normalize("わかしお・ささなみ") {
		t.Fatal("Japanese voicing marks collapsed distinct names")
	}
	if _, e := FindLine([]Line{{ID: "express:wakashio_sazamani", NameJA: "わかしお・さざなみ"}}, "わかしお・ささなみ"); e == nil {
		t.Fatal("a misspelled Japanese exact name matched")
	}
	ls := []Line{{ID: "kanto:sobuline", SourceID: "sobuline", NameJA: "総武本線", NameEN: "Sōbu Line"}, {ID: "kanto:sobuline_rapidservice", SourceID: "sobuline_rapidservice", NameJA: "総武快速線", NameEN: "Sōbu Line(Rapid Service)"}}
	for _, tc := range []struct {
		q, id string
		ok    bool
	}{{"sobuline", "kanto:sobuline", true}, {"総武快速線", "kanto:sobuline_rapidservice", true}, {"Sobu Line", "kanto:sobuline", true}, {"Sobu", "", false}, {"unknown-line", "", false}} {
		l, e := FindLine(ls, tc.q)
		if (e == nil) != tc.ok || tc.ok && l.ID != tc.id {
			t.Fatalf("FindLine(%q)=%+v,%v", tc.q, l, e)
		}
	}
}

func jaBox(id, name, status, note string) string {
	return fmt.Sprintf(`<li class="traininfo-routes__table__item"><p class="traininfo-routes__title"><span class="traininfo-routes__line %s"></span><span class="traininfo-routes__name">%s</span></p><a href="/train_info/line.aspx?gid=1&amp;lineid=%s"><p class="traininfo-routes__status"><span>%s</span></p><p class="traininfo-routes__note">%s</p></a></li>`, id, name, id, status, note)
}
func enBox(id, name, status, note string) string {
	return fmt.Sprintf(`<div class="rosenBox"><span class="rosen_color %s"></span><span class="name">%s</span><div class="status"><img alt="%s"><p>%s</p></div><p class="status_Text">%s</p></div>`, id, name, status, status, note)
}
func jaDoc(clock, rows string) string {
	return `<html><p>` + clock + `</p><section id="direction_soubu"><h2>総武方面</h2><ul>` + rows + `</ul></section></html>`
}
func enDoc(clock, rows string) string {
	return `<html><h2 class="current_time">` + clock + `</h2><section class="area_wrapper"><h2 id="direction_soubu">Bound for Sobu</h2>` + rows + `</section></html>`
}

func TestParseRegionFreshnessAndErrors(t *testing.T) {
	r, _ := RegionByID("kanto")
	now := at("2026-10-03T00:30:00+09:00")
	for _, tc := range []struct {
		name, language, body, freshness string
		err                             bool
	}{
		{"fresh-ja", "ja", jaDoc("2026年10月3日 0時29分 現在", jaBox("sobuline", "総武本線", "平常運転", "")), "fresh", false},
		{"fresh-en", "en", enDoc("Current as of: 10/03/2026 at 00:29", enBox("sobuline", "Sōbu Line", "Normal operation", "")), "fresh", false},
		{"stale", "ja", jaDoc("2026年10月2日 22時29分 現在", jaBox("sobuline", "総武本線", "平常運転", "")), "stale", false},
		{"future", "ja", jaDoc("2026年10月3日 1時29分 現在", jaBox("sobuline", "総武本線", "平常運転", "")), "future_timestamp", false},
		{"missing", "ja", jaDoc("", jaBox("sobuline", "総武本線", "平常運転", "")), "missing_timestamp", false},
		{"challenge", "ja", "<html><h1>Access Denied</h1></html>", "", true},
		{"shell", "en", "<html><h1>Train Status Information</h1><div id=app></div></html>", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, e := ParseRegion([]byte(tc.body), r, tc.language, r.SourceJA, now, 15*time.Minute)
			if (e != nil) != tc.err {
				t.Fatalf("err=%v", e)
			}
			if !tc.err && (len(p.rows) != 1 || p.State.Freshness != tc.freshness) {
				t.Fatalf("page=%+v", p)
			}
		})
	}
	closed := at("2026-10-03T03:00:00+09:00")
	if _, e := ParseRegion([]byte("<h1>Access Denied</h1>"), r, "ja", r.SourceJA, closed, time.Minute); e == nil {
		t.Fatal("reporting hours swallowed an error page")
	}
	p, e := ParseRegion([]byte("<p>情報提供時間は4:00～翌2:00となっています。</p>"), r, "ja", r.SourceJA, closed, time.Minute)
	if e != nil || p.State.ReportingState != "outside_reporting_hours" || len(p.rows) != 0 {
		t.Fatal(p, e)
	}
}

func TestJoinRegionsReorderDuplicatesAndConflict(t *testing.T) {
	r, _ := RegionByID("kanto")
	now := at("2026-10-03T00:30:00+09:00")
	jp, e := ParseRegion([]byte(jaDoc("2026年10月3日 0時29分 現在", jaBox("sobuline", "総武本線", "運転見合わせ", "台風25号の影響で、八街～成東駅間の上下線で運転を見合わせています。")+jaBox("sobuline", "総武本線", "一部運休", "千葉～八街駅間の一部列車が運休。")+jaBox("naritaline", "成田線", "平常運転", ""))), r, "ja", r.SourceJA, now, 15*time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	ep, e := ParseRegion([]byte(enDoc("Current as of: 10/03/2026 at 00:29", enBox("naritaline", "Narita Line", "Normal operation", "")+enBox("sobuline", "Sōbu Line", "Partial cancellation of service", "Some trains on Between Chiba and Yachimata Station are cancelled.")+enBox("sobuline", "Sōbu Line", "Operation suspended", "Between Yachimata and Narutō Station, Inbound and outbound lines."))), r, "en", r.SourceEN, now, 15*time.Minute)
	if e != nil {
		t.Fatal(e)
	}
	s := JoinRegions(jp, ep, now)
	if len(s.Lines) != 2 {
		t.Fatal(s)
	}
	l, e := FindLine(s.Lines, "sobuline")
	if e != nil || l.NameEN != "Sōbu Line" || l.LanguageConflict || l.Assessment != "reported_suspended" || len(l.Notices) != 4 || l.Notices[0].Sections[0].From != "八街" || l.Notices[0].Direction != "both" {
		t.Fatalf("line=%+v err=%v", l, e)
	}
	ep.rows[1].Status = "normal_label"
	s = JoinRegions(jp, ep, now)
	l, _ = FindLine(s.Lines, "sobuline")
	if !l.LanguageConflict {
		t.Fatal("bilingual status conflict was hidden")
	}
}

func TestFactsPlannedDirectionsAndEstimates(t *testing.T) {
	for _, tc := range []struct{ text, dir string }{{"上下線で運転見合わせ", "both"}, {"上り線に遅れ", "inbound"}, {"下り線に遅れ", "outbound"}, {"内・外回りに遅れ", "inner_and_outer"}, {"方向記載なし", "unknown"}} {
		n := facts(row{Language: "ja", Text: tc.text})
		if n.Direction != tc.dir {
			t.Fatal(n)
		}
	}
	n := facts(row{Language: "ja", Status: "notice", Text: "小海線は、日中時間帯の作業・工事のため、１０月１３日（火）～１５日（木）、１１月１７日（火）～１９日（木）の９時３０分頃から１５時頃まで、小淵沢～小海駅間の上下線で運休となります。代行輸送は行いません。"})
	if !n.Planned || n.Year != nil || n.LocalStart != "09:30" || n.LocalEnd != "15:00" || !n.Approximate || n.Replacement != "not_provided" || len(n.Dates) != 2 || len(n.Sections) != 1 || n.Sections[0].From != "小淵沢" {
		t.Fatal(n)
	}
	n = facts(row{Language: "ja", Status: "cancelled", Text: "全区間で運休。運転再開まで少なくとも３か月程度かかる見込みです。"})
	if n.AffectedScope != "all_sections" || n.ResumeMinimumMonths == nil || *n.ResumeMinimumMonths != 3 || !n.ResumeEstimate {
		t.Fatal(n)
	}
}

func TestAssessmentConservativeStates(t *testing.T) {
	now := at("2026-10-03T00:30:00+09:00")
	l := Line{Statuses: []string{"normal_label"}}
	for _, tc := range []struct{ freshness, reporting, want string }{{"fresh", "open", "normal_label_only"}, {"stale", "open", "source_stale"}, {"missing_timestamp", "open", "source_missing_timestamp"}, {"future_timestamp", "open", "source_future_timestamp"}, {"fresh", "outside_reporting_hours", "outside_reporting_hours"}} {
		v := Assessment(l, []SourceState{{Freshness: tc.freshness, ReportingState: tc.reporting}}, now)
		if v != tc.want {
			t.Fatalf("%+v: %s", tc, v)
		}
	}
	l.Notices = []Notice{{Planned: true}}
	if Assessment(l, []SourceState{{Freshness: "fresh", ReportingState: "open"}}, now) != "planned_notice" {
		t.Fatal("planned closure became normal or current suspension")
	}
}

func TestParseAreasInertScript(t *testing.T) {
	var b strings.Builder
	for _, r := range Regions() {
		fmt.Fprintf(&b, `document.write('<li><a href="%s"><img alt="Normal operation"></a></li>');`, r.SourceEN)
	}
	a, s, e := ParseAreas([]byte(b.String()), at("2026-10-03T00:30:00+09:00"))
	if e != nil || len(a) != 5 || a[0].Status != "normal_label" || s.Freshness != "missing_timestamp" {
		t.Fatal(a, s, e)
	}
	for _, body := range []string{"<h1>Access Denied</h1>", "document.write('ignored');"} {
		if _, _, e := ParseAreas([]byte(body), time.Now()); e == nil {
			t.Fatal("bad summary was accepted")
		}
	}
}

func TestParsePlannedSelectedFacts(t *testing.T) {
	line := Line{ID: "kanto:koumiline", NameJA: "小海線", Notices: []Notice{{Planned: true, Language: "ja", LocalStart: "09:30"}}}
	fixture := `<section class="mb80"><h2>小海線 保守工事に伴う列車の運休について</h2><p>代行輸送は行いません。</p><table><tr><th>実施日</th><th>運休区間</th></tr><tr><td>10月13日（火）～15日（木）<br>11月17日（火）～19日（木）</td><td>小淵沢～小海間</td></tr></table></section><section class="mb80"><h2>山手線の工事</h2></section>`
	for _, tc := range []struct {
		name string
		want int
	}{{"小海線", 1}, {"存在しない線", 0}} {
		line.NameJA = tc.name
		p, clipped, e := ParsePlanned([]byte(fixture), line)
		if e != nil || clipped || len(p) != tc.want {
			t.Fatal(p, e)
		}
		if tc.want > 0 && (p[0].Year != nil || len(p[0].DateExpressions) != 2 || p[0].Replacement != "not_provided" || len(p[0].LineNoticeFacts) != 1 || strings.Contains(p[0].Tables[0][0], "\n")) {
			t.Fatal(p)
		}
	}
	if _, _, e := ParsePlanned([]byte("<h1>Access Denied</h1>"), line); e == nil {
		t.Fatal("changed planned page accepted")
	}
}

func TestPlannedMixedYearsStayUnknown(t *testing.T) {
	text := "東海道本線で11月22日（日）昼間～23日（月・祝）早朝に工事。10月13日（火）～2027年1月22日（金）の間、一部列車の時刻変更。"
	n := facts(row{Language: "ja", Status: "notice", Text: text})
	if n.Year != nil || len(n.Dates) != 3 || n.Dates[2] != "2027年1月22日（金）" {
		t.Fatalf("mixed dates acquired an inferred year: %+v", n)
	}
	fixture := `<section class="mb80"><h2>東海道本線 線路切換工事に伴う列車の時刻変更について</h2><p>` + text + `</p></section>`
	line := Line{ID: "kanto:tokaidoline", NameJA: "東海道線"}
	plans, clipped, e := ParsePlanned([]byte(fixture), line)
	if e != nil || clipped || len(plans) != 1 || plans[0].Year != nil || len(plans[0].DateExpressions) != 3 {
		t.Fatal(plans, e)
	}
	line.ID = "kanto:yokosukaline"
	line.NameJA = "横須賀線"
	plans, _, e = ParsePlanned([]byte(fixture), line)
	if e != nil || len(plans) != 0 {
		t.Fatal("planned alias matched another line", plans, e)
	}
	for _, tc := range []struct {
		text string
		want *int
	}{
		{"2026年10月13日、2026年11月17日", intPtr(2026)},
		{"2026年12月31日、2027年1月1日", nil},
		{"10月13日、2027年1月22日", nil},
		{"2027年に予定する10月13日", nil},
	} {
		got := commonCalendarYear(dateJA.FindAllString(tc.text, -1))
		if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Fatal(tc.text, got, tc.want)
		}
	}
}

func intPtr(n int) *int { return &n }

func TestParsePlannedClippingIsExplicit(t *testing.T) {
	line := Line{ID: "kanto:koumiline", NameJA: "小海線"}
	for _, tc := range []struct {
		sections, rows int
		want           bool
	}{{6, 12, false}, {7, 12, true}, {1, 13, true}} {
		var fixture strings.Builder
		for i := 0; i < tc.sections; i++ {
			fixture.WriteString(`<section class="mb80"><h2>小海線の工事</h2><table>`)
			for j := 0; j < tc.rows; j++ {
				fixture.WriteString(`<tr><td>運休区間の事実</td></tr>`)
			}
			fixture.WriteString(`</table></section>`)
		}
		plans, clipped, e := ParsePlanned([]byte(fixture.String()), line)
		if e != nil || clipped != tc.want || len(plans) > 6 || len(plans[0].Tables) > 12 {
			t.Fatal(tc, plans, clipped, e)
		}
	}
	for _, fixture := range []string{
		`<section class="mb80"><h2>小海線の工事</h2><table><tr><td>` + strings.Repeat("事", 161) + `</td></tr></table></section>`,
		`<section class="mb80"><h2>小海線` + strings.Repeat("事", 91) + `</h2></section>`,
	} {
		plans, clipped, e := ParsePlanned([]byte(fixture), line)
		if e != nil || !clipped || len(plans) != 1 {
			t.Fatal("text clipping was hidden", plans, clipped, e)
		}
	}
}

func certFixture(url, label string) string {
	return `<p>2026年10月3日 0時29分 現在</p><table><tr><th><span class="delaycertificate-table__routename"><a href="/train_info/line.aspx?gid=1&amp;lineid=yamanoteline">山手線</a></span></th><td>-</td><td><a href="` + url + `">` + label + `</a></td><td>-</td><td>-</td><td>-</td></tr></table>`
}
func TestParseCertificatesPublishedNotZeroAndRollover(t *testing.T) {
	now := at("2026-10-03T00:30:00+09:00")
	for _, tc := range []struct {
		label   string
		minutes int
		bound   bool
	}{{"20分", 20, false}, {"61分以上", 61, true}} {
		cs, st, e := ParseCertificates([]byte(certFixture("/delay_certificate/pop.aspx?D=20261002&amp;R=05&amp;T=02", tc.label)), now)
		if e != nil || len(cs) != 1 || cs[0].SourceCode != "05" || st.Freshness != "fresh" {
			t.Fatal(cs, st, e)
		}
		a, b := cs[0].Slots[0], cs[0].Slots[1]
		if a.State != "not_published_or_below_threshold" || a.DisplayMinutes != nil || a.URL != "" || b.Date != "2026-10-02" || b.DisplayMinutes == nil || *b.DisplayMinutes != tc.minutes || b.LowerBound != tc.bound || b.ActualTrainDelay != nil {
			t.Fatal(a, b)
		}
	}
	for _, u := range []string{"https://evil.example/delay_certificate/pop.aspx?D=20261002&R=05&T=02", "/delay_certificate/pop.aspx?D=20261302&R=05&T=02", "/delay_certificate/pop.aspx?D=20261002&R=05&T=03", "/account/create?D=20261002&R=05&T=02"} {
		if _, _, e := ParseCertificates([]byte(certFixture(u, "20分")), now); e == nil {
			t.Fatalf("invalid link accepted: %s", u)
		}
	}
	if _, _, e := ParseCertificates([]byte("<h1>Access Denied</h1>"), now); e == nil {
		t.Fatal("certificate error page accepted")
	}
}

func TestCertificateRoute(t *testing.T) {
	for _, tc := range []struct {
		id      string
		count   int
		handoff string
	}{{"shonan-shinjukuline", 7, CertificateCoverageURL}, {"kanto:ueno-tokyoline", 5, CertificateCoverageURL}, {"sotetsuline", 3, CertificateCoverageURL}, {"sagamiline", 0, "https://doko-train.jp/en/pc/delaycertificate.html"}} {
		v := CertificateRoute(tc.id)
		if v == nil || len(v.Alternatives) != tc.count || v.Handoff != tc.handoff {
			t.Fatal(tc, v)
		}
	}
	for _, id := range []string{"yamanoteline", "nonexistent", "tohoku:sagamiline"} {
		if CertificateRoute(id) != nil {
			t.Fatal(id)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
func TestClientBoundsTypedThrottleAndHeaders(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		err    bool
	}{{"ok", 200, "real source", false}, {"denied", 403, "Access Denied", true}, {"throttle", 429, "slow down", true}, {"oversize", 200, strings.Repeat("x", MaxBodyBytes+1), true}} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewClient(1, 2)
			c.Limiter = nil
			c.HTTP.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" || !strings.Contains(r.Header.Get("User-Agent"), "jr-east-status-pp-cli") || r.Header.Get("Accept") == "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
					t.Fatal(r)
				}
				return response(tc.status, tc.body), nil
			})
			b, e := c.Get(context.Background(), Origin+"/train_info/kanto.aspx")
			if (e != nil) != tc.err {
				t.Fatal(string(b), e)
			}
			if tc.status == 429 {
				var rate *cliutil.RateLimitError
				if !errors.As(e, &rate) || rate.URL != Origin+"/train_info/kanto.aspx" {
					t.Fatal(e)
				}
			}
			if !tc.err && string(b) != "real source" {
				t.Fatal(string(b))
			}
			if _, e = c.Get(context.Background(), Origin+"/train_info/kanto.aspx"); e == nil || c.RequestCount() != 1 {
				t.Fatal("request budget/count", c.RequestCount(), e)
			}
		})
	}
	c := NewClient(1, 2)
	if _, e := c.Get(context.Background(), "https://evil.example/"); e == nil || c.RequestCount() != 0 {
		t.Fatal("untrusted destination was accepted")
	}
	for _, budget := range []int{0, -1, MaxRequestBudget + 1} {
		c := NewClient(budget, 2)
		if _, e := c.Get(context.Background(), Origin+"/train_info/kanto.aspx"); e == nil || c.RequestCount() != 0 {
			t.Fatal("invalid source request budget reached transport", budget, e)
		}
	}
}

func TestClientOriginalClosedSourceStopsTranslationFetch(t *testing.T) {
	r, _ := RegionByID("shinkansen")
	c := NewClient(2, 2)
	c.Limiter = nil
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != r.SourceJA {
			t.Fatal("translation must not be requested after explicit original closure", req.URL)
		}
		return response(200, jaDoc("2026年10月3日 2時0分 現在", "<p>情報提供時間は4:00～翌2:00となっています。</p>")), nil
	})
	// A cached explicit closure remains a closure even after the local clock
	// says reporting is open. The original message supplies the evidence.
	s, e := c.Region(context.Background(), r, at("2026-10-03T04:01:00+09:00"), 15*time.Minute)
	if e != nil || c.RequestCount() != 1 || len(s.Sources) != 1 || s.Sources[0].ReportingState != "outside_reporting_hours" || len(s.Lines) != 0 {
		t.Fatal(s, c.RequestCount(), e)
	}
}

func TestClientRegionUsesBothRealPageShapes(t *testing.T) {
	r, _ := RegionByID("kanto")
	now := at("2026-10-03T00:30:00+09:00")
	c := NewClient(2, 2)
	c.Limiter = nil
	paths := make([]string, 0)
	c.HTTP.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		paths = append(paths, req.URL.Path)
		if strings.Contains(req.URL.Path, "/e/") {
			return response(200, enDoc("Current as of: 10/03/2026 at 00:29", enBox("sobuline", "Sōbu Line", "Operation suspended", "Between Yachimata and Narutō Station."))), nil
		}
		return response(200, jaDoc("2026年10月3日 0時29分 現在", jaBox("sobuline", "総武本線", "運転見合わせ", "八街～成東駅間の上下線で運転を見合わせ。"))), nil
	})
	s, e := c.Region(context.Background(), r, now, 15*time.Minute)
	if e != nil || len(s.Lines) != 1 || s.Lines[0].Assessment != "reported_suspended" || !reflect.DeepEqual(paths, []string{"/train_info/kanto.aspx", "/train_info/e/kanto.aspx"}) {
		t.Fatal(s, paths, e)
	}
}

func TestReferenceLinesContainIdentityOnly(t *testing.T) {
	for _, r := range Regions() {
		ls := ReferenceLines(r)
		if len(ls) == 0 {
			t.Fatal(r)
		}
		for _, l := range ls {
			if l.SourceID == "" || l.NameJA == "" || l.IdentitySource != "bundled_source_catalogue" || len(l.Statuses) != 0 || len(l.Notices) != 0 || l.ActualDelayMinutes != nil || l.Assessment != "outside_reporting_hours" {
				t.Fatal(l)
			}
		}
	}
}
