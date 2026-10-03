package parks

import (
	"context"
	"database/sql"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var observed = time.Date(2026, 10, 3, 1, 0, 0, 0, JST)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile(filepath.Join("testdata", name))
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestNormalizeID(t *testing.T) {
	for _, x := range []struct {
		in string
		ok bool
	}{{"rvpark/1086", true}, {Origin + "/park/rvpark/1086.html", true}, {"../1086", false}, {"rvpark/abc", false}, {"https://evil.invalid/park/rvpark/1086.html", false}, {Origin + "/park/rvpark/1086.html?x=1", false}} {
		_, e := NormalizeID(x.in)
		if (e == nil) != x.ok {
			t.Errorf("%q err=%v", x.in, e)
		}
	}
}
func TestParseDimensions(t *testing.T) {
	for _, x := range []struct {
		raw       string
		l, w, h   float64
		unlimited bool
	}{{"長さ7m×幅4m×高さ 無制限", 7, 4, 0, true}, {"長さ 7m 幅 7m 高さ 6m", 7, 7, 6, false}, {"長さ 700cm 幅 ２５０ｃｍ 高さ300cm", 7, 2.5, 3, false}, {"長さ7 幅4 高さ3", 0, 0, 0, false}} {
		d := ParseDimensions(x.raw)
		vals := []*float64{d.LengthM, d.WidthM, d.HeightM}
		want := []float64{x.l, x.w, x.h}
		for i, v := range vals {
			if want[i] == 0 && v != nil || want[i] > 0 && (v == nil || *v != want[i]) {
				t.Errorf("%q dimension %d got %v", x.raw, i, v)
			}
		}
		if d.HeightUnrestricted != x.unlimited {
			t.Errorf("%q unrestricted", x.raw)
		}
	}
}
func TestParseSearch(t *testing.T) {
	for _, x := range []struct {
		file         string
		total, count int
	}{{"search.html", 1010, 20}, {"nagano-page1.html", 28, 20}, {"nagano-page2.html", 28, 8}} {
		r, e := ParseSearch(fixture(t, x.file), observed)
		if e != nil {
			t.Fatal(e)
		}
		if *r.Meta.ProviderTotal != x.total || len(r.Results) != x.count {
			t.Errorf("%s wrong counts", x.file)
		}
		for _, p := range r.Results {
			if p.ID == "rvpark/712" && p.Facilities["dump_station"].Status != "no" {
				t.Error("legacy off icon treated as available")
			}
		}
	}
	r, e := ParseSearch([]byte(`<div>件数0件</div>`), observed)
	if e != nil || r.Results == nil || len(r.Results) != 0 {
		t.Error("zero results must be []")
	}
	if _, e = ParseSearch([]byte(`<html>Access denied</html>`), observed); e == nil {
		t.Error("shell must fail")
	}
}
func TestParseDetail(t *testing.T) {
	for _, x := range []struct {
		id, file, member, toilet, water string
		tariffs                         int
	}{{"rvpark/1086", "rvpark-1086.html", "not_required_stated", "yes", "yes", 6}, {"yypark/213", "yypark-213.html", "required", "no", "no", 1}, {"rvpark/712", "rvpark-712.html", "not_required_stated", "yes", "yes", 1}} {
		p, e := ParseDetail(x.id, fixture(t, x.file), observed)
		if e != nil {
			t.Fatal(e)
		}
		if p.Membership.Status != x.member || p.Facilities["toilet_24h"].Status != x.toilet || p.Facilities["water"].Status != x.water || len(p.Tariffs) != x.tariffs {
			t.Errorf("%s member=%s toilet=%s water=%s tariffs=%d", x.id, p.Membership.Status, p.Facilities["toilet_24h"].Status, p.Facilities["water"].Status, len(p.Tariffs))
		}
		if p.Booking.Vacancy != "unknown" {
			t.Error("no vacancy evidence")
		}
		if !strings.HasSuffix(p.ObservedAt, "+09:00") {
			t.Error("not JST")
		}
		if p.ID == "rvpark/1086" {
			if p.Facilities["shower"].Status != "yes" {
				t.Error("actual bath_shawer icon lost shower availability")
			}
			if p.Dimensions.HeightM == nil || *p.Dimensions.HeightM != 6 || p.Tariffs[0].AmountJPY == nil || *p.Tariffs[0].AmountJPY != 3080 {
				t.Error("dimensions or general tariff lost")
			}
		}
		if p.ID == "rvpark/712" && (p.Facilities["dump_station"].Status != "no" || len(p.Warnings) == 0) {
			t.Error("off icon conflict missing")
		}
	}
}
func TestFilters(t *testing.T) {
	if len(Filters().Groups["types"]) != 8 {
		t.Error("wrong type coverage")
	}
	for _, x := range []struct {
		q  Query
		ok bool
	}{{Query{Prefecture: "nagano", Type: "rvpark", Vehicle: "van", Facilities: []string{"electricity", "pets"}, MaxPages: 2, Limit: 10}, true}, {Query{Prefecture: "Atlantis", MaxPages: 1, Limit: 10}, false}, {Query{MaxPages: 6, Limit: 10}, false}, {Query{MaxPages: 1, Limit: 0}, false}} {
		v, e := QueryValues(x.q)
		if (e == nil) != x.ok {
			t.Errorf("err=%v", e)
		}
		if x.ok && (v.Get("area_pref") != "nagano" || v.Get("category[]") != "2" || v.Get("vehicle_size_search[]") != "5") {
			t.Error("wire mapping drift")
		}
	}
	v, e := VehicleLabel("van")
	if e != nil || v != "バンコン" {
		t.Error("vehicle alias")
	}
}
func TestSearchSessionAndThrottle(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			if r.Method != "POST" {
				t.Error("initial method")
			}
			_ = r.ParseForm()
			if r.Form.Get("area_pref") != "nagano" {
				t.Error("filter lost")
			}
			http.SetCookie(w, &http.Cookie{Name: "public-search-test", Value: "fixture", Path: "/"})
			w.Write(fixture(t, "nagano-page1.html"))
			return
		}
		if r.Method != "GET" || r.URL.Query().Get("start_num") != "20" {
			t.Error("pagination contract")
		}
		if _, e := r.Cookie("public-search-test"); e != nil {
			t.Error("public session missing")
		}
		w.Write(fixture(t, "nagano-page2.html"))
	}))
	defer srv.Close()
	r, e := NewClient(srv.URL, 2).Search(context.Background(), Query{Prefecture: "nagano", MaxPages: 2, Limit: 100})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Results) != 28 || r.Meta.ScannedPages != 2 || !r.Meta.ProviderPagesComplete {
		t.Error("coverage wrong")
	}
	throttle := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Retry-After", "1"); w.WriteHeader(429) }))
	defer throttle.Close()
	_, e = NewClient(throttle.URL, 2).Detail(context.Background(), "rvpark/1086")
	var rate *cliutil.RateLimitError
	if !errors.As(e, &rate) {
		t.Errorf("typed throttle missing: %v", e)
	}
}
func TestCache(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "parks.db")
	ps, e := Load(ctx, path)
	if e != nil || ps == nil || len(ps) != 0 {
		t.Error("missing cache not empty")
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Error("read created missing cache")
	}
	p, e := ParseDetail("rvpark/1086", fixture(t, "rvpark-1086.html"), observed)
	if e != nil {
		t.Fatal(e)
	}
	if e = Save(ctx, path, []Park{p}); e != nil {
		t.Fatal(e)
	}
	card := p
	card.SourceLevel = "search_card"
	card.ObservedAt = observed.Add(time.Hour).Format(time.RFC3339)
	card.Tariffs = []Tariff{}
	if e = Save(ctx, path, []Park{card}); e != nil {
		t.Fatal(e)
	}
	ps, e = Load(ctx, path)
	if e != nil {
		t.Fatal(e)
	}
	if len(ps) != 1 || ps[0].SourceLevel != "detail" || len(ps[0].Tariffs) != 6 {
		t.Error("search destroyed detailed observation")
	}
}

func TestAdditionalFilterMappings(t *testing.T) {
	for _, x := range []struct{ in, want string }{{"water", "water"}, {"水道あり", "water"}, {"電源あり", "electricity"}, {"7", "premium_benefits"}} {
		v, e := FacilityKey(x.in)
		if e != nil || v != x.want {
			t.Errorf("%s %s %v", x.in, v, e)
		}
	}
	for _, x := range []struct{ in, want string }{{"1", "通年"}, {"通年", "通年"}} {
		v, e := PeriodLabel(x.in)
		if e != nil || v != x.want {
			t.Error(v, e)
		}
	}
	if _, e := VehicleLabel("rvpark"); e == nil {
		t.Error("cross-domain alias accepted as vehicle")
	}
}
func TestDetailHTTPAndDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/park/rvpark/1086.html" {
			t.Error("detail path mismatch")
		}
		w.Write(fixture(t, "rvpark-1086.html"))
	}))
	defer srv.Close()
	p, e := NewClient(srv.URL, 2).Detail(context.Background(), "rvpark/1086")
	if e != nil || p.Name != "RVパーク・ラボランドくろひめ" {
		t.Errorf("detail %s %v", p.Name, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = NewClient(srv.URL, 2).Detail(ctx, "rvpark/1086"); e == nil {
		t.Error("command deadline did not reach HTTP")
	}
}

func TestConditionalFacilityText(t *testing.T) {
	for _, x := range []struct{ label, raw, key, want string }{{"ダンプステーション", "あり（グレーのみ、ブラック不可）", "black_water", "no"}, {"ダンプステーション", "あり（グレーのみ、ブラック不可）", "grey_water", "yes"}, {"ダンプステーション", "あり", "black_water", "unknown"}, {"電源の有無", "あり（有料）。利用料金に含まれていません", "electricity", "paid"}, {"水道", "あり（有料）。少量のみ無料", "water", "unknown"}, {"電源の有無", "あり。施設利用料に含まれます", "electricity", "included"}} {
		body := []byte(`<h3 class="commonIcoTitle"><span>Fixture Park</span><span class="label">RVパーク</span></h3><dl><dt>` + x.label + `</dt><dd>` + x.raw + `</dd></dl>`)
		p, e := ParseDetail("rvpark/1", body, observed)
		if e != nil {
			t.Fatal(e)
		}
		got := p.Facilities[x.key].Fee
		if x.key == "black_water" || x.key == "grey_water" {
			got = p.Facilities[x.key].Status
		}
		if got != x.want {
			t.Errorf("%s %s: got %s want %s", x.label, x.raw, got, x.want)
		}
	}
}

func TestCanonicalCampTypes(t *testing.T) {
	for _, typ := range []string{"camp3000", "campjrva"} {
		v, e := QueryValues(Query{Type: typ, MaxPages: 1, Limit: 1})
		if e != nil || v.Get("category[]") == "" {
			t.Errorf("%s rejected: %v", typ, e)
		}
	}
}

func TestDetailRestrictionsOverridePositiveIcons(t *testing.T) {
	for _, tc := range []struct{ name, icons, section, key, want string }{
		{"dump", `<img src="/images/ico/facility_dump.png" alt="ブラック・グレーOK">`, `<dl><dt>ダンプステーション</dt><dd>なし</dd></dl>`, "black_water", "unknown"},
		{"toilet", `<img src="/images/ico/toilet_24.png" alt="24時間利用可">`, `<dl><dt>トイレ</dt><dd>施設内 5:00〜24:00 洋式 / 水洗式</dd></dl>`, "toilet_24h", "no"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := `<h3 class="commonIcoTitle"><span>検証パーク</span></h3>` + tc.icons + tc.section
			p, err := ParseDetail("rvpark/1", []byte(body), observed)
			if err != nil {
				t.Fatal(err)
			}
			if p.Facilities[tc.key].Status != tc.want {
				t.Fatalf("%s=%+v", tc.key, p.Facilities[tc.key])
			}
			if tc.name == "dump" && p.Facilities["grey_water"].Status != "unknown" {
				t.Fatal("grey icon survived negative station evidence")
			}
			reqs, _ := ParseRequirements(tc.key)
			m, _ := Match(p, reqs, "")
			if m.Decision == "proven_match" {
				t.Fatal("conflicting evidence proved service")
			}
			conflict := false
			for _, issue := range Audit(p, observed) {
				if issue.Kind == "facility_evidence_conflict" {
					conflict = true
				}
			}
			if !conflict {
				t.Fatal("audit lost conflict")
			}
			if len(p.Facilities[tc.key].Evidence) < 2 {
				t.Fatal("icon or detail evidence lost")
			}
		})
	}
}
func TestToiletPublishedHourRanges(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"施設内 5:00〜24:00", "no"}, {"施設内 00:00～24:00", "yes"},
		{"24時間利用可能。施設内 5:00〜24:00", "unknown"}, {"洋式 / 水洗式", "unknown"},
		{"施設内 23:00-06:00", "no"}, {"施設内 99:00~24:00", "unknown"},
	} {
		if got := toiletHoursStatus(tc.raw); got != tc.want {
			t.Errorf("%q: %s, want %s", tc.raw, got, tc.want)
		}
	}
}

func TestCacheWriteWaitsForConcurrentWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.db")
	p := newPark("rvpark/1", observed)
	if err := Save(context.Background(), path, []Park{p}); err != nil {
		t.Fatal(err)
	}
	lock, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err = lock.Exec(`BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	defer lock.Exec(`ROLLBACK`)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { q := newPark("rvpark/2", observed); done <- Save(ctx, path, []Park{q}) }()
	select {
	case err := <-done:
		t.Fatalf("write returned before the competing transaction released its lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if _, err = lock.Exec(`COMMIT`); err != nil {
		t.Fatal(err)
	}
	if err = <-done; err != nil {
		t.Fatalf("bounded contention should wait and then succeed: %v", err)
	}
	rows, err := Load(context.Background(), path)
	if err != nil || len(rows) != 2 {
		t.Fatalf("cache observations lost: rows=%d err=%v", len(rows), err)
	}
}

func TestCacheContentionRespectsCallerDeadline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "observations.db")
	if err := Save(context.Background(), path, []Park{newPark("rvpark/1", observed)}); err != nil {
		t.Fatal(err)
	}
	lock, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err = lock.Exec(`BEGIN IMMEDIATE`); err != nil {
		t.Fatal(err)
	}
	defer lock.Exec(`ROLLBACK`)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = Save(ctx, path, []Park{newPark("rvpark/2", observed)})
	if err == nil {
		t.Fatal("contended write succeeded while lock held")
	}
	if time.Since(started) > 700*time.Millisecond {
		t.Fatal("cache wait exceeded caller timeout")
	}
}

func TestAbsentOrUnavailableToiletOverridesPositiveIcon(t *testing.T) {
	for _, wording := range []string{"なし", "夜間は利用不可", "夜間は利用できません"} {
		body := `<h3 class="commonIcoTitle"><span>検証パーク</span></h3><img src="/images/ico/facility_toilet_24.png" alt="24時間利用可"><dl><dt>トイレ</dt><dd>` + wording + `</dd></dl>`
		p, err := ParseDetail("rvpark/1", []byte(body), observed)
		if err != nil {
			t.Fatal(err)
		}
		if p.Facilities["toilet_24h"].Status != "no" {
			t.Errorf("%s retained positive icon: %+v", wording, p.Facilities["toilet_24h"])
		}
		reqs, _ := ParseRequirements("toilet_24h")
		match, err := Match(p, reqs, "")
		if err != nil || match.Decision != "ruled_out" {
			t.Errorf("%s match=%s err=%v", wording, match.Decision, err)
		}
	}
}

func TestUnparsedJapaneseToiletHoursDoNotPreservePositiveIcon(t *testing.T) {
	body := `<h3 class="commonIcoTitle"><span>検証パーク</span></h3><img src="/images/ico/facility_toilet_24.png" alt="24時間利用可"><dl><dt>トイレ</dt><dd>施設内 午前５時〜午後１１時</dd></dl>`
	p, err := ParseDetail("rvpark/1", []byte(body), observed)
	if err != nil {
		t.Fatal(err)
	}
	if p.Facilities["toilet_24h"].Status != "unknown" {
		t.Fatal("unparsed restricted hours retained positive icon")
	}
}
