package walkerplus

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/cliutil"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func sourcePage(t *testing.T, name string) page {
	return page{body: fixture(t, name), source: Source{URL: "https://www.walkerplus.com/event/ar0313e603640/", FetchedAt: "2026-09-27T00:00:00Z"}}
}
func testClient(t *testing.T, handler http.HandlerFunc, opts Options) *Client {
	t.Helper()
	opts.BaseURL = "https://www.walkerplus.com"
	opts.HTTPClient = &http.Client{Transport: handlerTransport{handler}}
	if opts.CacheDir == "" {
		opts.NoCache = true
	}
	c, err := NewClient(opts)
	if err != nil {
		t.Fatal(err)
	}
	c.limiter = cliutil.NewAdaptiveLimiter(1000)
	return c
}

func TestCapturedListingIdentityAndSeasonalCertainty(t *testing.T) {
	list, err := parseListing(sourcePage(t, "event-list.html"))
	if err != nil {
		t.Fatal(err)
	}
	if len(list.events) != 10 {
		t.Fatalf("expected ten source cards, got %d", len(list.events))
	}
	first := list.events[0]
	if first.ID != "ar0313e603640" || first.TitleJA != "はてな展" || value(first.StartDate) != "2026-06-27" || value(first.EndDate) != "2026-09-30" {
		t.Fatalf("bad identity/dates: %+v", first)
	}
	if strings.Contains(first.SourceURL, "ticket-store") || len(first.OrganizerURLs) != 1 {
		t.Fatalf("organizer URL used as identity: %+v", first)
	}
	list, err = parseListing(sourcePage(t, "kyoto-seasonal.html"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range list.events {
		if strings.Contains(value(e.Schedule.Raw), "中旬") {
			found = true
			if e.DateCertainty != "approximate" {
				t.Fatalf("schema overruled seasonal prose: %+v", e)
			}
		}
	}
	if !found {
		t.Fatal("seasonal capture lacks expected wording")
	}
}

func TestCapturedSourceFacts(t *testing.T) {
	e := emptyEvent("ar0313e603640", "", "https://www.walkerplus.com/event/ar0313e603640/")
	for _, name := range []string{"detail.html", "detail-data.html", "detail-price.html"} {
		if _, err := parseDetail(sourcePage(t, name), &e); err != nil {
			t.Fatal(err)
		}
	}
	parseSchedule(&e)
	if e.Admission.Status != "paid" || e.Admission.Price != nil || e.ReservationRequired == nil || !*e.ReservationRequired || !strings.Contains(value(e.Admission.Raw), "小学生以下無料") {
		t.Fatalf("paid/reservation evidence lost: %+v", e)
	}
	if e.Timezone != "Asia/Tokyo" || len(e.Sources) != 3 || len(e.Categories) != 1 || e.Categories[0].Code != "eg0145" {
		t.Fatalf("provenance lost: %+v", e)
	}
	for _, tc := range []struct {
		name, id                 string
		indoor, holiday, weather bool
	}{
		{"indoor-data.html", "ar0727e612159", true, false, false},
		{"recurrence-data.html", "ar0204e612432", false, true, false},
		{"firework-data.html", "ar0101e66092", false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := emptyEvent(tc.id, "", "source")
			if _, err := parseDetail(sourcePage(t, tc.name), &e); err != nil {
				t.Fatal(err)
			}
			parseSchedule(&e)
			if tc.indoor && (e.Indoor == nil || !*e.Indoor) {
				t.Fatal("indoor evidence missed")
			}
			if tc.holiday && len(e.Schedule.Unresolved) == 0 {
				t.Fatal("holiday closure marked resolved")
			}
			if tc.weather && (e.Cancellation != nil || !strings.Contains(value(e.Weather), "強風中止")) {
				t.Fatal("weather condition became current cancellation")
			}
		})
	}
}

func TestDetailRetainsListingFreshnessAndCorrectsVenue(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/event/ar0313e603640/":
			w.Write(fixture(t, "detail.html"))
		case "/event/ar0313e603640/data.html":
			w.Write(fixture(t, "detail-data.html"))
		case "/event/ar0313e603640/price.html":
			w.Write(fixture(t, "detail-price.html"))
		default:
			t.Errorf("unexpected detail path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}, Options{})
	seed := emptyEvent("ar0313e603640", "はてな展", "https://www.walkerplus.com/event/ar0313e603640/")
	seed.Location.Venue = strptr("stale listing venue")
	seed.Sources = []Source{{URL: "https://www.walkerplus.com/event_list/ar0313/", FetchedAt: "2026-09-27T00:00:00Z"}}
	got, err := c.event(context.Background(), "/event/ar0313e603640/", seed, newOperation())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sources) != 4 || got.Sources[0].URL != seed.Sources[0].URL || !strings.HasPrefix(value(got.Location.Venue), "西武渋谷店") {
		t.Fatalf("provenance/stale venue incorrect: %+v", got)
	}
}

func TestAdmissionAndIndoorEvidence(t *testing.T) {
	for _, tc := range []struct{ raw, status string }{
		{"有料。平日1800円、土日祝2100円 ※小学生以下無料", "paid"},
		{"小学生以下無料", "unknown"}, {"無料", "free"}, {"入場無料。一部有料", "mixed"},
		{"0円", "free"}, {"小学生0円", "unknown"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			e := emptyEvent("ar0313e1", "event", "source")
			applyAdmission(&e, tc.raw, "source")
			if e.Admission.Status != tc.status {
				t.Fatalf("%q: got %s", tc.raw, e.Admission.Status)
			}
		})
	}
	for _, raw := range []string{"雨天の場合は屋内会場", "雨天のみ屋内開催", "屋内・屋外会場"} {
		e := emptyEvent("ar0313e1", "event", "source")
		applyIndoor(&e, raw, "source")
		applyIndoor(&e, "屋内会場", "source")
		if e.Indoor != nil {
			t.Fatalf("conditional/mixed became blanket indoor: %q", raw)
		}
	}
	if currentCancellation("雨天の場合は開催中止") || !currentCancellation("台風のため開催中止") {
		t.Fatal("current and contingent cancellation conflated")
	}
}

func scheduled(start, end, raw string) Event {
	e := emptyEvent("ar0313e1", "event", "source")
	e.StartDate = strptr(start)
	e.EndDate = strptr(end)
	e.Schedule.Raw = strptr(raw)
	parseSchedule(&e)
	return e
}
func TestScheduleDaysAndYearBoundaries(t *testing.T) {
	q := Query{From: "2026-10-03", To: "2026-10-18", Timing: "overlap"}
	e := scheduled("2026-10-03", "2026-10-18", "2026年10月3日(土)～10月18日(日) 期間中の土・日曜のみ開催。10月11日(日)は休み")
	m := matchEvent(e, q, true)
	want := []string{"2026-10-03", "2026-10-04", "2026-10-10", "2026-10-17", "2026-10-18"}
	if strings.Join(m.ConfirmedDays, ",") != strings.Join(want, ",") {
		t.Fatalf("recurrence/exclusion wrong: %+v schedule=%+v", m, e.Schedule)
	}
	e = scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 休館日：月曜(祝日の場合は翌平日)")
	q.From = "2026-10-12"
	q.To = q.From
	if m = matchEvent(e, q, true); m.State != "possible" || len(m.ConfirmedDays) != 0 {
		t.Fatalf("holiday overconfident: %+v", m)
	}
	e = scheduled("2026-10-10", "2026-10-20", "2026年10月10日～10月20日 休館日：月曜 不定休あり")
	q.From = "2026-10-13"
	q.To = q.From
	if m = matchEvent(e, q, true); m.State != "possible" {
		t.Fatalf("unsupported qualifier ignored: %+v", m)
	}
	e = scheduled("2026-12-31", "2027-01-02", "2026年12月31日・1月1日・2日")
	q.From = "2026-12-31"
	q.To = "2027-01-02"
	m = matchEvent(e, q, true)
	if strings.Join(m.ConfirmedDays, ",") != "2026-12-31,2027-01-01,2027-01-02" || e.EditionYear == nil || *e.EditionYear != 2026 {
		t.Fatalf("omitted-year list wrong: %+v %+v", e.Schedule, m)
	}
	e = scheduled("2026-12-31", "2027-01-02", "2026年12月31日～2027年1月2日 毎日開催。1月1日は休み")
	m = matchEvent(e, q, true)
	if containsDate(m.ConfirmedDays, "2027-01-01") || !containsDate(e.Schedule.ExcludedDates, "2027-01-01") {
		t.Fatalf("omitted-year exclusion wrong: %+v %+v", e.Schedule, m)
	}
	e = scheduled("", "", "2026年10月1日～31日（10月5日を除く）")
	if value(e.EndDate) != "2026-10-31" {
		t.Fatalf("exception replaced primary range end: %+v", e)
	}
	q.From = "2027-10-03"
	q.To = "2027-10-18"
	if matchEvent(e, q, true).State != "excluded" {
		t.Fatal("edition silently rolled forward")
	}
	for _, bad := range []string{"2026-02-29", "2026-04-31", "not-a-date"} {
		if _, err := NormalizeQuery(Query{From: bad, To: bad}); err == nil {
			t.Fatalf("invalid date accepted %s", bad)
		}
	}
	if _, err := NormalizeQuery(Query{From: "2026-01-01", To: "2027-01-02"}); err == nil {
		t.Fatal("derived day arrays unbounded")
	}
}

type handlerTransport struct{ handler http.HandlerFunc }

func (h handlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	recorder := httptest.NewRecorder()
	h.handler(recorder, req)
	return recorder.Result(), nil
}

func TestSearchDedupYearLocationAndBudget(t *testing.T) {
	var requests atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write(fixture(t, "event-list.html")) }, Options{})
	result, err := c.Search(context.Background(), Query{Prefecture: "tokyo", From: "2026-09-27", To: "2026-09-30", MaxPages: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.Coverage.ScannedPages != 2 || result.Coverage.RequestCount != 2 || requests.Load() != 2 {
		t.Fatalf("request count wrong: %+v", result.Coverage)
	}
	ids := map[string]bool{}
	for _, e := range result.Events {
		if ids[e.ID] || e.Match.State != "possible" || value(e.Location.PrefectureCode) != "ar0313" {
			t.Fatalf("duplicate, wrong area or false confirmation: %+v", e)
		}
		ids[e.ID] = true
	}
	result, err = c.Search(context.Background(), Query{Prefecture: "kyoto", From: "2026-09-27", To: "2026-09-30", MaxPages: 1})
	if err != nil || len(result.Events) != 0 {
		t.Fatalf("wrong region retained: %v %+v", err, result)
	}
	result, err = c.Search(context.Background(), Query{Prefecture: "tokyo", From: "2027-09-27", To: "2027-09-30", MaxPages: 1})
	if err != nil || len(result.Events) != 0 {
		t.Fatalf("wrong edition retained: %v %+v", err, result)
	}
	result, err = c.Search(context.Background(), Query{From: "2026-12-31", To: "2027-01-01", MaxPages: 2})
	if err != nil || result.Coverage.SourceTotal != nil || len(result.Coverage.Routes) != 2 {
		t.Fatalf("cross-month totals misleading: %v %+v", err, result.Coverage)
	}
}

func TestHTTPFailureCacheRetriesAndRedirect(t *testing.T) {
	var calls atomic.Int32
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(429) }, Options{Retries: 1})
	_, err := c.Event(context.Background(), "ar0313e603640")
	var rateErr *cliutil.RateLimitError
	if !errors.As(err, &rateErr) || calls.Load() != 2 {
		t.Fatalf("429/retries wrong: %v calls=%d", err, calls.Load())
	}
	c = testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }, Options{})
	if _, err = c.Event(context.Background(), "ar0313e603640"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("404 not typed: %v", err)
	}
	c = testClient(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://example.com/", 302) }, Options{})
	if _, err = c.Event(context.Background(), "ar0313e603640"); err == nil {
		t.Fatal("external redirect followed")
	}
	calls.Store(0)
	c = testClient(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.Write(fixture(t, "event-list.html")) }, Options{CacheDir: t.TempDir()})
	q := Query{MaxPages: 1}
	if _, err = c.Search(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	second, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || second.Coverage.RequestCount != 0 || second.Coverage.CacheHits != 1 {
		t.Fatalf("warm cache refetched: %+v", second.Coverage)
	}
	c.opts.Refresh = true
	if _, err = c.Search(context.Background(), q); err != nil || calls.Load() != 2 {
		t.Fatalf("refresh failed: %v calls=%d", err, calls.Load())
	}
}

func TestUnknownJSONAndStrictEventURL(t *testing.T) {
	b, err := json.Marshal(emptyEvent("ar0313e1", "event", "source"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["start_date"] != nil || decoded["indoor"] != nil || len(decoded["categories"].([]any)) != 0 || len(decoded["evidence"].([]any)) != 0 {
		t.Fatalf("null/[] contract broken: %s", b)
	}
	for _, bad := range []string{"https://evil.example/event/ar0313e1/", "https://www.walkerplus.com@evil.example/event/ar0313e1/", "https://www.walkerplus.com/event/ar0313e1/?x=1", "https://www.walkerplus.com/event/ar0313e1/data.html"} {
		c, _ := NewClient(Options{NoCache: true})
		if _, err := c.eventPath(bad); err == nil {
			t.Fatalf("invalid event URL accepted %s", bad)
		}
	}
}

func TestCacheEvictionFailedAndMissingFiles(t *testing.T) {
	files := make([]cacheEntry, 129)
	for i := range files {
		files[i] = cacheEntry{name: strconv.Itoa(i), size: 1}
	}
	attempts := 0
	count, size := evictEntries(files, 129, func(string) error { attempts++; return os.ErrPermission })
	if attempts != 129 || count != 129 || size != 129 {
		t.Fatalf("failed eviction panic/accounting: count=%d size=%d attempts=%d", count, size, attempts)
	}
	count, size = evictEntries(files, 129, func(string) error { return os.ErrNotExist })
	if count != 128 || size != 128 {
		t.Fatalf("concurrent missing file accounting wrong: count=%d size=%d", count, size)
	}
	c, err := NewClient(Options{CacheDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 129; i++ {
		u := "https://www.walkerplus.com/event/" + strconv.Itoa(i)
		c.writeCache(u, page{body: []byte("source HTML"), source: Source{FetchedAt: time.Now().UTC().Format(time.RFC3339)}})
	}
	entries, err := os.ReadDir(c.opts.CacheDir)
	if err != nil || len(entries) != 128 {
		t.Fatalf("cache entry cap failed: entries=%d err=%v", len(entries), err)
	}
}

func TestCacheEvictionPreservesUnrelatedFiles(t *testing.T) {
	for _, tc := range []struct {
		name             string
		count, bodyBytes int
	}{{"entry_cap", 129, 11}, {"size_cap", 9, 3 << 20}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			c, err := NewClient(Options{CacheDir: dir})
			if err != nil {
				t.Fatal(err)
			}
			unrelated := []string{"wp-config.json", "wp-settings.json", "wp-" + strings.Repeat("A", 64) + ".json", "wp-short.json"}
			for _, name := range unrelated {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("unrelated user data"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			linkName := "wp-" + strings.Repeat("e", 64) + ".json"
			if err := os.Symlink(filepath.Join(dir, "wp-config.json"), filepath.Join(dir, linkName)); err != nil {
				t.Fatal(err)
			}
			body := []byte(strings.Repeat("x", tc.bodyBytes))
			for i := 0; i < tc.count; i++ {
				u := "https://www.walkerplus.com/event/cache-preservation-" + strconv.Itoa(i)
				c.writeCache(u, page{body: body, source: Source{FetchedAt: time.Now().UTC().Format(time.RFC3339)}})
			}
			for _, name := range unrelated {
				b, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || string(b) != "unrelated user data" {
					t.Fatalf("unrelated file removed/changed: %s err=%v", name, err)
				}
			}
			if info, err := os.Lstat(filepath.Join(dir, linkName)); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("unrelated symlink removed: %v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			owned := 0
			var size int64
			for _, entry := range entries {
				if !ownedCacheFilenameRE.MatchString(entry.Name()) {
					continue
				}
				info, err := entry.Info()
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().IsRegular() {
					owned++
					size += info.Size()
				}
			}
			if owned >= tc.count || owned > 128 || size > 32<<20 {
				t.Fatalf("owned cache entries were not bounded: count=%d size=%d", owned, size)
			}
		})
	}
}

func TestOversizedSourceAndNontransientErrorNotRetried(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", maxBody+1))) }, Options{})
	if _, err := c.Event(context.Background(), "ar0313e603640"); err == nil || !strings.Contains(err.Error(), "4MiB") {
		t.Fatalf("oversized source accepted: %v", err)
	}
	var requests atomic.Int32
	c.http.Transport = errorTransport{calls: &requests}
	c.opts.Retries = 3
	if _, err := c.Event(context.Background(), "ar0313e603640"); err == nil || requests.Load() != 1 {
		t.Fatalf("permanent transport error retried: %v count=%d", err, requests.Load())
	}
}

type errorTransport struct{ calls *atomic.Int32 }

func (e errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	e.calls.Add(1)
	return nil, errors.New("permanent invalid request")
}
