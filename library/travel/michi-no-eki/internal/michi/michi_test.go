package michi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const fixtureSearch = `<main><div class="searchTotal">検索結果<span>2件</span></div><div class="searchList"><a href="/stations/views/10001"><h3>試験駅A</h3><div class="txt">長野県 試験市</div><img src="/assets/img/stations/facility03.svg"><img src="/assets/img/stations/facility06.svg"></a><a href="/stations/views/10002"><h3>試験駅B</h3><div class="txt">長野県 試験村</div><img src="/assets/img/stations/facility03.svg"><img src="/assets/img/stations/facility06_off.svg"></a></div><div class="js-data-box" data-link="/stations/views/10001" data-lat="35" data-lng="138"></div><div class="js-data-box" data-link="/stations/views/10002" data-lat="36" data-lng="138"></div></main>`
const fixtureIndex = `<main><div class="noticesList"><a href="/notices/views/20001"><time datetime="2026-9-30">2026年9月30日</time><span>長野県</span><p>試験駅のお知らせ</p></a><a href="/notices/views/20002"><time datetime="2026-10-1">2026年10月1日</time><span>長野県</span><p>別の駅のお知らせ</p></a></div><a href="/notices?page=1">2</a></main>`

func fixtureStation(name string) string {
	return `<main><div class="viewTitle"><span>長野県</span><h2>道の駅</h2></div><div class="viewFacility"><img src="/assets/img/stations/facility03.svg"><img src="/assets/img/stations/facility06.svg"><img src="/assets/img/stations/facility07_off.svg"><img src="/assets/img/stations/facility12.svg"></div><div class="viewContent"><dl><dt>道の駅名</dt><dd>` + name + `</dd></dl><dl><dt>所在地</dt><dd>長野県 試験住所</dd></dl><dl><dt>TEL</dt><dd><a href="tel:誤った住所">0260-00-0000</a></dd></dl><dl><dt>駐車場</dt><dd>大型：18台 普通車：83（身障者用4）台</dd></dl><dl><dt>営業時間</dt><dd>9:00～21:00</dd></dl><dl><dt>ホームページ</dt><dd><a href="https://operator.example/">operator</a></dd></dl></div><iframe src="https://www.google.com/maps/embed/v1/place?q=35,138&amp;key=EXAMPLE"></iframe></main>`
}
func fixtureNotice(station string) string {
	return `<article class="noticesView__content"><h3>試験日程のお知らせ</h3><a href="/stations/views/` + station + `">試験駅</a><p>2026年10月6日は休業予定。掲載日は2026年9月30日。</p><p class="createdDate">2026年9月30日</p></article>`
}
func fixtureSource() *Source {
	return &Source{BaseURL: Origin, Fetch: func(ctx context.Context, path string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch {
		case strings.HasPrefix(path, "/stations/search/"):
			return []byte(fixtureSearch), nil
		case path == "/stations/views/10001":
			return []byte(fixtureStation("試験駅A")), nil
		case path == "/stations/views/10002":
			return []byte(fixtureStation("試験駅B")), nil
		case path == "/notices":
			return []byte(fixtureIndex), nil
		case path == "/notices?page=1":
			return []byte(`<main><div class="noticesList"></div></main>`), nil
		case path == "/notices/views/20001":
			return []byte(fixtureNotice("10001")), nil
		case path == "/notices/views/20002":
			return []byte(fixtureNotice("10002")), nil
		default:
			return nil, fmt.Errorf("HTTP404 fixture %s", path)
		}
	}}
}
func fixtureQuery() Query {
	return Query{Prefecture: "nagano", Facility: "onsen,restaurant", Match: "all", Limit: 10, MaxCandidates: 2000}
}
func TestCatalogAndGuidance(t *testing.T) {
	for _, tc := range []struct {
		name  string
		check func() bool
	}{{"catalog", func() bool {
		c := CatalogData()
		return len(c.Prefectures) == 47 && len(c.Facilities) == 18 && len(c.Regions) == 9 && c.SourceURL == Origin+"/"
	}}, {"guidance", func() bool {
		g := GeneralGuidance()
		return g["source_url"] == PolicyURL && g["station_specific_permissions"] == "unknown"
	}}} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.check() {
				t.Fatal("catalog/guidance contract mismatch")
			}
		})
	}
}
func TestParseSearch(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		want       int
		bad        bool
	}{{"source cards and coordinates", fixtureSearch, 2, false}, {"known empty", `<div class="searchTotal">0件</div><div class="searchList"></div>`, 0, false}, {"challenge shell", `<html>Access denied</html>`, 0, true}, {"layout drift nonempty", `<div class="searchTotal">3件</div><div class="searchList"></div>`, 0, true}} {
		t.Run(tc.name, func(t *testing.T) {
			v, e := ParseSearch([]byte(tc.html), Origin+"/stations/search/22/105/all", "observed")
			if (e != nil) != tc.bad {
				t.Fatalf("err=%v", e)
			}
			if tc.bad {
				return
			}
			if len(v.Stations) != tc.want {
				t.Fatalf("got%d", len(v.Stations))
			}
			b, _ := json.Marshal(v)
			if strings.Contains(string(b), `"stations":null`) {
				t.Fatal("nil empty result")
			}
			if tc.want > 0 {
				a := v.Stations[0]
				if a.Name != "試験駅A" || a.Coordinates == nil || a.Coordinates.Latitude != 35 || a.Facilities["onsen"] != "present" || v.Stations[1].Facilities["onsen"] != "not_listed" || a.Facilities["shower"] != "unknown" {
					t.Fatalf("bad evidence %+v", a)
				}
			}
		})
	}
}
func TestParseStation(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		bad        bool
	}{{"facts", fixtureStation("試験駅A"), false}, {"missing title", "<main></main>", true}, {"missing name", `<div class="viewTitle"><span>長野県</span></div>`, true}} {
		t.Run(tc.name, func(t *testing.T) {
			s, e := ParseStation([]byte(tc.html), "10001", Origin, "seen")
			if (e != nil) != tc.bad {
				t.Fatal(e)
			}
			if tc.bad {
				return
			}
			if s.Name != "試験駅A" || s.Phone != "0260-00-0000" || s.Parking.Cars == nil || *s.Parking.Cars != 83 || *s.Parking.Large != 18 || *s.Parking.Accessible != 4 || s.Permissions.Camping != "unknown" || s.Permissions.Overnight != "unknown" || s.ChargerAvailability != "unknown" || s.Facilities["campground"] != "not_listed" || s.Facilities["ev-charger"] != "present" || len(s.OperatorURLs) != 1 || s.Coordinates == nil {
				t.Fatalf("bad station %+v", s)
			}
		})
	}
}
func TestParseNotices(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		want       int
		bad        bool
	}{{"dated index", fixtureIndex, 2, false}, {"empty index", `<div class="noticesList"></div>`, 0, false}, {"drift", `<main>unavailable</main>`, 0, true}} {
		t.Run(tc.name, func(t *testing.T) {
			a, next, e := ParseNotices([]byte(tc.html), 0, Origin, "seen")
			if (e != nil) != tc.bad {
				t.Fatal(e)
			}
			if tc.bad {
				return
			}
			if len(a) != tc.want {
				t.Fatal(len(a))
			}
			if tc.want > 0 && (a[0].PublishedDate != "2026-09-30" || a[0].Timezone != "Asia/Tokyo" || next == nil || *next != 1) {
				t.Fatalf("bad date/next %+v", a)
			}
		})
	}
}
func TestParseNotice(t *testing.T) {
	for _, tc := range []struct {
		name, html string
		bad        bool
	}{{"exact linked ID", fixtureNotice("10001"), false}, {"long excerpt", strings.Replace(fixtureNotice("10001"), "2026年10月6日は休業予定。掲載日は2026年9月30日。", strings.Repeat("説明", 100), 1), false}, {"drift", `<main>not found</main>`, true}} {
		t.Run(tc.name, func(t *testing.T) {
			v, e := ParseNotice([]byte(tc.html), "20001", Origin, "seen")
			if (e != nil) != tc.bad {
				t.Fatal(e)
			}
			if tc.bad {
				return
			}
			if v.PublishedDate != "2026-09-30" || len(v.StationIDs) != 1 || v.StationIDs[0] != "10001" || len([]rune(v.Excerpt)) > 160 {
				t.Fatalf("bad notice %+v", v)
			}
			if tc.name == "long excerpt" && !v.ExcerptTruncated {
				t.Fatal("missing cap marker")
			}
		})
	}
}
func TestQueryValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		q    Query
		bad  bool
	}{{"aliases", fixtureQuery(), false}, {"Japanese", Query{Prefecture: "長野県", Facility: "温泉施設", Match: "all", Limit: 1, MaxCandidates: 10}, false}, {"numeric IDs", Query{Prefecture: "29", Facility: "105", Match: "any", Limit: 1, MaxCandidates: 10}, false}, {"provider region", Query{Region: "chubu", Match: "all", Limit: 1, MaxCandidates: 10}, false}, {"unknown", Query{Prefecture: "nowhere", Match: "all", Limit: 1, MaxCandidates: 10}, true}, {"invalid match", Query{Match: "both", Limit: 1, MaxCandidates: 10}, true}, {"union ambiguity", Query{Prefecture: "nagano", Region: "kanto", Match: "all", Limit: 1, MaxCandidates: 10}, true}, {"reserved keyword", Query{Keyword: "all", Match: "all", Limit: 1, MaxCandidates: 10}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			if e := ValidateQuery(tc.q); (e != nil) != tc.bad {
				t.Fatal(e)
			}
		})
	}
	for _, tc := range []struct {
		csv string
		bad bool
	}{{"10002,10001", false}, {"x", true}, {"10001,10001", true}, {"", true}, {"1,2,3,4,5,6,7", true}} {
		t.Run(tc.csv, func(t *testing.T) {
			if e := ValidateIDs(tc.csv); (e != nil) != tc.bad {
				t.Fatal(e)
			}
		})
	}
}
func noticeIndexPage(start, n, next int) string {
	var b strings.Builder
	b.WriteString(`<main><div class="noticesList">`)
	for i := 0; i < n; i++ {
		id := start + i
		fmt.Fprintf(&b, `<a href="/notices/views/%d"><time datetime="2026-9-30">2026年9月30日</time><span>長野県</span><p>お知らせ%d</p></a>`, id, id)
	}
	b.WriteString(`</div>`)
	if next > 0 {
		fmt.Fprintf(&b, `<a href="/notices?page=%d">next</a>`, next)
	}
	b.WriteString(`</main>`)
	return b.String()
}

func TestStationNoticesRetainsScannedIndexBeforeDetailCap(t *testing.T) {
	var mu sync.Mutex
	fetched := map[string]int{}
	src := &Source{BaseURL: Origin, Fetch: func(ctx context.Context, path string) ([]byte, error) {
		switch {
		case path == "/stations/views/10001":
			return []byte(fixtureStation("試験駅A")), nil
		case path == "/notices":
			return []byte(noticeIndexPage(30000, 30, 1)), nil
		case path == "/notices?page=1":
			return []byte(noticeIndexPage(30030, 25, 0)), nil
		case strings.HasPrefix(path, "/notices/views/"):
			id := strings.TrimPrefix(path, "/notices/views/")
			mu.Lock()
			fetched[id]++
			mu.Unlock()
			station := "10002"
			if id == "30000" || id == "30054" {
				station = "10001"
			}
			return []byte(fixtureNotice(station)), nil
		default:
			return nil, fmt.Errorf("unexpected %s", path)
		}
	}}
	v, err := src.StationNotices(context.Background(), "10001", 2, 50, 10)
	if err != nil {
		t.Fatal(err)
	}
	if v.ScannedPages != 2 || v.ScannedRecords != 55 || v.DetailRecords != 50 {
		t.Fatalf("coverage = pages %d records %d details %d", v.ScannedPages, v.ScannedRecords, v.DetailRecords)
	}
	if len(fetched) != 50 || fetched["30054"] != 0 || fetched["30000"] != 1 {
		t.Fatalf("detail fetches = %d, includes late id %d", len(fetched), fetched["30054"])
	}
	if len(v.Notices) != 1 || v.Notices[0].ID != "30000" || v.Notices[0].MatchReason != "explicit_station_id_link" {
		t.Fatalf("matches = %+v", v.Notices)
	}
	if !strings.Contains(v.Note, "Scanned 55") || !strings.Contains(v.Note, "first 50") || !strings.Contains(v.Note, "not fetched") {
		t.Fatalf("note does not describe the unopened index rows: %s", v.Note)
	}
}

func TestExportNoticesFollowsIndexAndRejectsIncompleteCeiling(t *testing.T) {
	src := &Source{BaseURL: Origin, Fetch: func(ctx context.Context, path string) ([]byte, error) {
		switch path {
		case "/notices":
			return []byte(noticeIndexPage(50000, 2, 1)), nil
		case "/notices?page=1":
			return []byte(noticeIndexPage(50002, 2, 0)), nil
		default:
			return nil, fmt.Errorf("unexpected %s", path)
		}
	}}
	rows, err := src.ExportNotices(context.Background(), 0)
	if err != nil || len(rows) != 4 || rows[3].ID != "50003" {
		t.Fatalf("unlimited export = %d %v", len(rows), err)
	}
	limited, err := src.ExportNotices(context.Background(), 3)
	if err != nil || len(limited) != 3 || limited[2].ID != "50002" {
		t.Fatalf("limited export = %+v %v", limited, err)
	}

	prev := maxBulletinExportPages
	maxBulletinExportPages = 2
	t.Cleanup(func() { maxBulletinExportPages = prev })
	paging := &Source{BaseURL: Origin, Fetch: func(ctx context.Context, path string) ([]byte, error) {
		page := 0
		if strings.Contains(path, "page=") {
			fmt.Sscanf(path, "/notices?page=%d", &page)
		}
		return []byte(noticeIndexPage(60000+page, 1, page+1)), nil
	}}
	if _, err := paging.ExportNotices(context.Background(), 0); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("ceiling error = %v", err)
	}
}

func TestSourceWorkflows(t *testing.T) {
	src := fixtureSource()
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		run  func() error
	}{{"Station", func() error {
		s, e := src.Station(ctx, "10001")
		if e == nil && s.Name != "試験駅A" {
			return errors.New("station wrong")
		}
		return e
	}}, {"Notice", func() error {
		n, e := src.Notice(ctx, "20001")
		if e == nil && n.StationIDs[0] != "10001" {
			return errors.New("notice link wrong")
		}
		return e
	}}, {"Find all", func() error {
		v, e := src.Find(ctx, fixtureQuery())
		if e == nil && (v.ReturnedCount != 1 || v.Stations[0].ID != "10001" || v.ScannedCandidates != 2) {
			return errors.New("all-of filter incorrect")
		}
		return e
	}}, {"Find any", func() error {
		q := fixtureQuery()
		q.Match = "any"
		v, e := src.Find(ctx, q)
		if e == nil && v.ReturnedCount != 2 {
			return errors.New("any-of filter incorrect")
		}
		return e
	}}, {"Nearby", func() error {
		q := fixtureQuery()
		q.Match = "any"
		v, e := src.Nearby(ctx, q, 35, 138, 0)
		if e == nil && (v.Stations[0].ID != "10001" || *v.Stations[0].DistanceKM != 0 || *v.Stations[1].DistanceKM < 100) {
			return errors.New("distance ranking wrong")
		}
		return e
	}}, {"Compare", func() error {
		v, e := src.Compare(ctx, "10002,10001", "comparison")
		if e == nil && (v.Stations[0].ID != "10002" || v.Stations[1].ID != "10001") {
			return errors.New("input order lost")
		}
		return e
	}}, {"Readiness", func() error {
		v, e := src.Readiness(ctx, "10001")
		if e == nil && v["station"].(Station).Permissions.Overnight != "unknown" {
			return errors.New("permission invented")
		}
		return e
	}}, {"Notices", func() error {
		v, e := src.Notices(ctx, 2, 1, "nagano", "")
		if e == nil && (v.ScannedPages != 2 || v.ScannedRecords != 2 || len(v.Notices) != 1 || v.NextPage != nil) {
			return errors.New("scan/output caps incorrect")
		}
		return e
	}}, {"StationNotices", func() error {
		v, e := src.StationNotices(ctx, "10001", 1, 2, 10)
		if e == nil && (len(v.Notices) != 1 || v.Notices[0].ID != "20001" || v.Notices[0].MatchReason != "explicit_station_id_link" || v.DetailRecords != 2) {
			return errors.New("exact station join incorrect")
		}
		return e
	}}} {
		t.Run(tc.name, func(t *testing.T) {
			if e := tc.run(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestBoundsFailuresAndUnicodePath(t *testing.T) {
	src := fixtureSource()
	ctx := context.Background()
	v, e := src.Compare(ctx, "10003,10001", "comparison")
	if e != nil || v.SuccessfulCount != 1 || len(v.FetchFailures) != 1 || v.FetchFailures[0].ID != "10003" {
		t.Fatalf("partial failure lost %+v %v", v, e)
	}
	v, e = src.Compare(ctx, "10003", "comparison")
	if e == nil || len(v.FetchFailures) != 1 {
		t.Fatal("all failed silently succeeded")
	}
	q := fixtureQuery()
	q.Match = "any"
	q.MaxCandidates = 1
	found, e := src.Find(ctx, q)
	if e != nil || found.CoverageComplete || found.ScannedCandidates != 1 || found.Note == "" {
		t.Fatal("scan cap lost")
	}
	for _, p := range [][3]float64{{math.NaN(), 0, 0}, {0, math.Inf(1), 0}, {91, 0, 0}, {0, 0, -1}} {
		if _, e := src.Nearby(ctx, q, p[0], p[1], p[2]); e == nil {
			t.Fatal("invalid coordinate/radius accepted")
		}
	}
	var path string
	src.Fetch = func(_ context.Context, p string) ([]byte, error) { path = p; return []byte(fixtureSearch), nil }
	q = fixtureQuery()
	q.Keyword = "信州 /駅"
	if _, e := src.Find(ctx, q); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(path, "%2F") {
		t.Fatal("keyword slash not encoded", path)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := fixtureSource().Station(cancelled, "10001"); !errors.Is(e, context.Canceled) {
		t.Fatal("cancel lost", e)
	}
}
func TestSnapshotDiff(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(Comparison, Comparison) (Comparison, Comparison)
		want    string
	}{{"demo changed", func(a, b Comparison) (Comparison, Comparison) { return a, b }, "changed"}, {"timestamp only", func(a, b Comparison) (Comparison, Comparison) {
		b.Stations[0].PublishedHours = a.Stations[0].PublishedHours
		return a, b
	}, ""}, {"membership added", func(a, b Comparison) (Comparison, Comparison) { a.Stations = []Station{}; return a, b }, "added"}, {"failure unresolved", func(a, b Comparison) (Comparison, Comparison) {
		b.FetchFailures = []FetchFailure{{ID: a.Stations[0].ID, Error: "provider down"}}
		b.Stations = []Station{}
		return a, b
	}, "unresolved"}} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := DemoSnapshots()
			a, b = tc.prepare(a, b)
			v, e := Diff(a, b)
			if e != nil {
				t.Fatal(e)
			}
			if tc.want == "" {
				if len(v.Changes) != 0 {
					t.Fatal("timestamps produced false changes")
				}
				return
			}
			if len(v.Changes) != 1 || v.Changes[0].Kind != tc.want {
				t.Fatalf("diff %+v", v)
			}
			if tc.want == "changed" && (len(v.Changes[0].Fields) != 1 || v.Changes[0].Fields[0].Field != "published_hours") {
				t.Fatal("wrong field diff", v)
			}
		})
	}
}
func TestReadSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name string
		body func() []byte
		bad  bool
	}{{"valid", func() []byte { a, _ := DemoSnapshots(); b, _ := json.Marshal(a); return b }, false}, {"invalid schema", func() []byte { return []byte(`{"schema_version":2,"kind":"snapshot"}`) }, true}, {"malformed", func() []byte { return []byte(`{`) }, true}, {"too large", func() []byte { return []byte(strings.Repeat("x", 2*1024*1024+1)) }, true}} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "sample.json")
			if e := os.WriteFile(p, tc.body(), 0600); e != nil {
				t.Fatal(e)
			}
			a, e := ReadSnapshot(p)
			if (e != nil) != tc.bad {
				t.Fatal(e)
			}
			if !tc.bad && (a.Kind != "snapshot" || len(a.Stations) != 1) {
				t.Fatal("roundtrip failed")
			}
		})
	}
}

func TestObservedNativeEmptyContract(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		bad        bool
	}{
		{"observed paired empty data", `<main><div class="pageStation"><div class="searchTop"></div><div class="searchMap"></div><!--配列データ--><div style="display: none;"></div><!--配列データ--></div></main>`, false},
		{"missing closing data marker", `<main><div class="pageStation"><div class="searchTop"></div><div class="searchMap"></div><!--配列データ--><div style="display: none;"></div></div></main>`, true},
		{"nonempty omitted count", `<main><div class="pageStation"><div class="searchTop"></div><div class="searchMap"></div><!--配列データ--><div style="display:none;"><div class="js-data-box"></div></div><!--配列データ--></div></main>`, true},
		{"generic shell", `<main><div class="searchMap"></div></main>`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := ParseSearch([]byte(tc.body), Origin+"/stations/search/22/105/all", "seen")
			if (err != nil) != tc.bad {
				t.Fatal(err)
			}
			if !tc.bad && (out.Stations == nil || len(out.Stations) != 0 || out.SourceTotal != nil || out.EmptyResultEvidence == "" || !out.CoverageComplete) {
				t.Fatalf("unknown-count empty contract lost: %+v", out)
			}
		})
	}
}

func TestCanonicalLinkAuthority(t *testing.T) {
	for _, tc := range []struct{ href, want string }{
		{"/stations/views/19487", "19487"}, {"https://www.michi-no-eki.jp/stations/views/19487", "19487"}, {"//www.michi-no-eki.jp/stations/views/19487", "19487"},
		{"//foreign.example/stations/views/19487", ""}, {"https://foreign.example/stations/views/19487", ""}, {"ftp://www.michi-no-eki.jp/stations/views/19487", ""}, {"https://user@www.michi-no-eki.jp/stations/views/19487", ""}, {"javascript:/stations/views/19487", ""}, {"https:/stations/views/19487", ""},
	} {
		if got := pathID(tc.href, stationPath); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.href, got, tc.want)
		}
	}
	body := strings.ReplaceAll(fixtureNotice("10001"), `href="/stations/views/10001"`, `href="//foreign.example/stations/views/10001"`)
	n, e := ParseNotice([]byte(body), "20001", Origin, "seen")
	if e != nil || len(n.StationIDs) != 0 {
		t.Fatalf("foreign station link treated as exact provider evidence: %+v %v", n, e)
	}
}
func TestSnapshotCompletenessAndEOF(t *testing.T) {
	for _, tc := range []struct {
		name   string
		alter  func(map[string]any)
		suffix string
		bad    bool
	}{
		{name: "complete", bad: false},
		{name: "projected station", alter: func(m map[string]any) {
			m["stations"] = []any{map[string]any{"id": "10001", "name": "架空の道の駅（デモ）"}}
		}, bad: true},
		{name: "missing metadata", alter: func(m map[string]any) { delete(m, "requested_ids") }, bad: true},
		{name: "inconsistent counts", alter: func(m map[string]any) { m["successful_count"] = 0 }, bad: true},
		{name: "duplicate requested IDs", alter: func(m map[string]any) { m["requested_ids"] = []string{"10001", "10001"}; m["requested_count"] = 2 }, bad: true},
		{name: "unrequested station", alter: func(m map[string]any) { m["requested_ids"] = []string{"10002"} }, bad: true},
		{name: "incomplete nested parking", alter: func(m map[string]any) {
			delete(m["stations"].([]any)[0].(map[string]any)["parking"].(map[string]any), "ordinary_cars")
		}, bad: true},
		{name: "missing facility", alter: func(m map[string]any) {
			delete(m["stations"].([]any)[0].(map[string]any)["facilities"].(map[string]any), "onsen")
		}, bad: true},
		{name: "trailing junk", suffix: " trailing junk", bad: true},
		{name: "second JSON value", suffix: " {}", bad: true},
		{name: "trailing whitespace", suffix: " \n\t", bad: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := DemoSnapshots()
			data, _ := json.Marshal(a)
			var m map[string]any
			_ = json.Unmarshal(data, &m)
			if tc.alter != nil {
				tc.alter(m)
			}
			data, _ = json.Marshal(m)
			data = append(data, []byte(tc.suffix)...)
			path := filepath.Join(t.TempDir(), "snapshot.json")
			if e := os.WriteFile(path, data, 0600); e != nil {
				t.Fatal(e)
			}
			_, e := ReadSnapshot(path)
			if (e != nil) != tc.bad {
				t.Fatalf("unexpected validation result: %v", e)
			}
		})
	}
}
func TestSnapshotRequiresRegularFile(t *testing.T) {
	if _, e := ReadSnapshot(t.TempDir()); e == nil {
		t.Fatal("directory accepted as snapshot")
	}
	fifo := filepath.Join(t.TempDir(), "snapshot.fifo")
	if e := exec.Command("mkfifo", fifo).Run(); e != nil {
		t.Skip("mkfifo not available on this platform")
	}
	done := make(chan error, 1)
	go func() { _, e := ReadSnapshot(fifo); done <- e }()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("FIFO accepted")
		}
	case <-time.After(time.Second):
		t.Fatal("FIFO snapshot read blocked")
	}
}

func TestNoticeIndexRejectsSilentEntryDrops(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		bad        bool
	}{
		{"genuinely empty", `<div class="noticesList"></div>`, false},
		{"all titles drifted", strings.ReplaceAll(strings.ReplaceAll(fixtureIndex, "<p>", "<h4>"), "</p>", "</h4>"), true},
		{"one title drifted", strings.Replace(strings.Replace(fixtureIndex, "<p>", "<h4>", 1), "</p>", "</h4>", 1), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _, e := ParseNotices([]byte(tc.body), 0, Origin, "seen")
			if (e != nil) != tc.bad {
				t.Fatalf("silent entry drop: %+v %v", a, e)
			}
		})
	}
}
