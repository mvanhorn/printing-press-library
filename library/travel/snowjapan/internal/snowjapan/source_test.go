package snowjapan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/snowjapan/internal/cliutil"
)

const testID = "nagano-prefecture/hakuba-village/able-hakuba-goryu"

func testSSR(v any) string {
	b, _ := json.Marshal(v)
	return `<html><script id="wix-warmup-data" type="application/json">` + string(b) + `</script></html>`
}
func testChartRef(id string) string {
	return testSSR(map[string]any{"html": `<div class="flourish-embed" data-src="visualisation/` + id + `"></div>`})
}
func testCatalog() string {
	columns := `{"data":{"popup_metadata":["City","Prefecture","Top (m)","Base (m)","Vertical (m)","Lifts (#)","Courses (#)","Longest (m)","Steepest (°)","Link"]}}`
	rows := `{"data":[{"nest_columns":["Goryu"],"popup_metadata":["Hakuba Village","Nagano Prefecture",1676,750,926,12,16,5000,35,"https://www.snowjapan.com/ski-areas-in-japan/` + testID + `"]}]}`
	return "var _Flourish_data_column_names = " + columns + "; var _Flourish_data = " + rows + ";"
}
func testRecord() string {
	r := map[string]any{"title": "Goryu", "nameJapanese": "五竜", "mapLocation": "Hakuba Village", "prefecture": "Nagano Prefecture", "maxElevation": 1676, "minElevation": 750, "vertical": 926, "lifts": 12, "beginner": 0, "intermediate": 75, "advanced": 25, "status": "Not yet updated", "plannedSeason": "Late November until early May", "fullAddress": "<p>Public resort address</p>", "link-copy-of-ski-areas-title": "/ski-areas-in-japan/" + testID}
	return testSSR(map[string]any{"appsWarmupData": map[string]any{"dataBinding": map[string]any{"dataStore": map[string]any{"recordsByCollectionId": map[string]any{"Ski-Areas": map[string]any{"one": r}}}}}})
}
func testSeason(first, last string) string {
	a, _ := time.Parse("2006-01-02", first)
	z, _ := time.Parse("2006-01-02", last)
	row := fmt.Sprintf(`{"rows":[{"columns":["<a href=\"https://www.snowjapan.com/ski-areas-in-japan/%s\">Goryu</a>","Hakuba","Nagano",new Date(%d),new Date(%d),%d]}]}`, testID, a.UnixMilli(), z.UnixMilli(), int(z.Sub(a).Hours()/24)+1)
	return `var _Flourish_data_column_names = {"rows":{"columns":["Ski area","Town","Prefecture","Open","Close","Days"]}}; var _Flourish_data = ` + row + ";"
}

func TestPublicMethodsUseActualProjection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/insights/japan-ski-areas-statistics":
			fmt.Fprint(w, testChartRef("1"))
		case "/insights/2025-2026-ski-season-dates-sort":
			fmt.Fprint(w, testChartRef("2"))
		case "/visualisation/1/embed":
			fmt.Fprint(w, testCatalog())
		case "/visualisation/2/embed":
			fmt.Fprint(w, testSeason("2026-01-03", "2026-03-01"))
		case "/ski-areas-in-japan/" + testID, "/ski-areas-in-japan/region":
			fmt.Fprint(w, testRecord())
		case "/daily-snow-and-weather-reports/hakuba-now-1st-october-2026":
			fmt.Fprint(w, `<h1>Hakuba Now: 1st October 2026</h1><h6>New snowfall at base 0cm</h6><h6>Snowfall at base this season 0cm</h6><p>Editorial material must stay private to the original page.</p>`)
		case "/":
			fmt.Fprint(w, `<a href="https://www.snowjapan.com/daily-snow-and-weather-reports/hakuba-now-1st-october-2026">Hakuba Now</a>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New()
	c.SiteBase = srv.URL
	c.ChartBase = srv.URL
	c.HTTP = srv.Client()
	c.limiter = cliutil.NewAdaptiveLimiter(10000)
	cases := []struct {
		name  string
		run   func() ([]Fact, error)
		field string
		want  any
	}{
		{"catalog", func() ([]Fact, error) { return c.Catalog(context.Background()) }, "installed_lifts", float64(12)},
		{"inspect", func() ([]Fact, error) { f, e := c.Inspect(context.Background(), testID); return []Fact{f}, e }, "beginner_percent", float64(0)},
		{"seasons", func() ([]Fact, error) { return c.Seasons(context.Background(), "2025-2026") }, "first_recorded_day", "2026-01-03"},
		{"report list", func() ([]Fact, error) { return c.Reports(context.Background()) }, "report_date", "2026-10-01"},
		{"dated report", func() ([]Fact, error) {
			f, e := c.Report(context.Background(), "hakuba-now-1st-october-2026")
			return []Fact{f}, e
		}, "new_snow_cm", float64(0)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, e := tc.run()
			if e != nil {
				t.Fatal(e)
			}
			if len(v) != 1 || v[0][tc.field] != tc.want {
				t.Fatalf("%s got %#v", tc.field, v)
			}
			raw, _ := json.Marshal(v)
			if strings.Contains(string(raw), "Editorial material") {
				t.Fatal("editorial prose escaped projection")
			}
		})
	}
}

func TestDeclarationNeverEvaluatesJavascript(t *testing.T) {
	cases := []struct {
		name, body string
		fail       bool
	}{
		{"literal date", `var _Flourish_data = {"date":new Date(123456),"text":"new Date(7)"};`, false},
		{"executable expression", `var _Flourish_data = {"date":evil()};`, true},
		{"nonliteral date", `var _Flourish_data = {"date":new Date(secret)};`, true},
		{"unrelated trailing date", `var _Flourish_data = {"date":new Date(123456),"text":"new Date(7)"}; var unrelated = new Date(secret);`, false},
		{"unrelated trailing incomplete string", `var _Flourish_data = {"nested":[{"date":new Date(123456)}],"text":"new Date(7)"}; var unrelated = "new Date(secret)`, false},
		{"unclosed declared data", `var _Flourish_data = {"date":new Date(123456); var unrelated = {};`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out map[string]any
			e := declaration([]byte(tc.body), "_Flourish_data", &out)
			if (e != nil) != tc.fail {
				t.Fatalf("error %v", e)
			}
			if !tc.fail && out["text"] != "new Date(7)" {
				t.Fatal("quoted text was modified")
			}
		})
	}
}

func TestInspectRequiresMatchingSourceIdentity(t *testing.T) {
	for _, link := range []string{"niigata-prefecture/yuzawa-town/kagura", "", "https://evil.example/ski-areas-in-japan/" + testID} {
		t.Run(link, func(t *testing.T) {
			body := strings.ReplaceAll(testRecord(), "/ski-areas-in-japan/"+testID, "/ski-areas-in-japan/"+link)
			if link == "" {
				body = strings.ReplaceAll(testRecord(), "/ski-areas-in-japan/"+testID, "")
			} else if strings.HasPrefix(link, "https:") {
				body = strings.ReplaceAll(testRecord(), "/ski-areas-in-japan/"+testID, link)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, body)
			}))
			defer srv.Close()
			c := New()
			c.SiteBase = srv.URL
			c.HTTP = srv.Client()
			if _, e := c.Inspect(context.Background(), testID); e == nil {
				t.Fatal("mismatched or missing identity was relabeled as requested resort")
			}
		})
	}
}

func TestOnlyCompletedWintersAreAcceptedAcrossYearBoundary(t *testing.T) {
	for _, tc := range []struct {
		now, season string
		valid       bool
	}{
		{"2027-01-03", "2026-2027", false},
		{"2027-01-03", "2025-2026", true},
		{"2027-09-03", "2026-2027", true},
	} {
		now, _ := time.Parse("2006-01-02", tc.now)
		if e := validateSeasonAt(tc.season, now); (e == nil) != tc.valid {
			t.Fatalf("now=%s season=%s error=%v", tc.now, tc.season, e)
		}
	}
}

func TestWixDetailIdentityLinksMustAgree(t *testing.T) {
	for _, tc := range []struct {
		title, detail string
		fail          bool
	}{
		{"/ski-areas-in-japan/" + testID + "/location", "/ski-areas-in-japan/" + testID, false},
		{"/ski-areas-in-japan/" + testID + "/location", "/ski-areas-in-japan/niigata-prefecture/yuzawa-town/kagura", true},
		{"", "/ski-areas-in-japan/" + testID, false},
	} {
		id, e := detailRecordID(map[string]any{"link-copy-of-ski-areas-title": tc.title, "link-copy-of-ski-areas-title-2": tc.detail})
		if (e != nil) != tc.fail || (!tc.fail && id != testID) {
			t.Fatalf("identity=%s error=%v", id, e)
		}
	}
}

func TestHistoricalSharedLinkKeepsConflictingMunicipalities(t *testing.T) {
	body := testSeason("2026-01-03", "2026-03-01")
	prefix := strings.Index(body, `{"rows":[`)
	end := strings.LastIndex(body, "]}")
	row := body[prefix+len(`{"rows":[`) : end]
	chart := body[:prefix] + `{"rows":[` + row + "," + strings.ReplaceAll(row, "Hakuba", "Akaigawa") + "]};"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.Contains(r.URL.Path, "/embed") {
			fmt.Fprint(w, chart)
		} else {
			fmt.Fprint(w, testChartRef("3"))
		}
	}))
	defer srv.Close()
	c := New()
	c.SiteBase, c.ChartBase, c.HTTP = srv.URL, srv.URL, srv.Client()
	rows, e := c.Seasons(context.Background(), "2025-2026")
	if e != nil || len(rows) != 2 || rows[0]["id"] == rows[1]["id"] || rows[0]["resort_id"] != rows[1]["resort_id"] {
		t.Fatalf("shared-link records=%v error=%v", rows, e)
	}
}

func TestHistoricalOutOfWinterDatesRemainInconsistentEvidence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if strings.Contains(r.URL.Path, "/embed") {
			fmt.Fprint(w, testSeason("2025-12-20", "2026-03-01"))
		} else {
			fmt.Fprint(w, testChartRef("4"))
		}
	}))
	defer srv.Close()
	c := New()
	c.SiteBase, c.ChartBase, c.HTTP = srv.URL, srv.URL, srv.Client()
	rows, e := c.Seasons(context.Background(), "2024-2025")
	if e != nil || len(rows) != 1 || rows[0]["endpoint_evidence_state"] != "dates_outside_requested_winter" || rows[0]["first_recorded_day"] != "2025-12-20" {
		t.Fatalf("inconsistent record=%v error=%v", rows, e)
	}
}

func TestIdentityAndSeasonValidation(t *testing.T) {
	cases := []struct {
		source string
		valid  bool
	}{
		{"https://www.snowjapan.com/ski-areas-in-japan/" + testID, true},
		{"https://evil.example/ski-areas-in-japan/" + testID, false},
		{"https://www.snowjapan.com/ski-areas-in-japan/../private", false},
		{"https://user:pass@www.snowjapan.com/ski-areas-in-japan/" + testID, false},
	}
	for _, tc := range cases {
		_, e := pathID(tc.source)
		if (e == nil) != tc.valid {
			t.Fatalf("identity %s: %v", tc.source, e)
		}
	}
	for _, tc := range []struct {
		season string
		valid  bool
	}{{"2025-2026", true}, {"2025-2027", false}, {"2026-2027", false}, {"2025", false}} {
		if (ValidateSeason(tc.season) == nil) != tc.valid {
			t.Fatal(tc.season)
		}
	}
}

func TestSourceThrottleAndBodyBoundsFailClearly(t *testing.T) {
	for _, status := range []int{429, 404} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		c := New()
		c.HTTP = srv.Client()
		_, e := c.fetch(context.Background(), srv.URL)
		srv.Close()
		if e == nil {
			t.Fatalf("status %d succeeded", status)
		}
		if status == 429 {
			var rate *cliutil.RateLimitError
			if !errors.As(e, &rate) {
				t.Fatalf("not a typed rate error: %v", e)
			}
		}
	}
	catalog := strings.Replace(testCatalog(), "Top (m)", "Wrong (m)", 1)
	if _, e := parseCatalog([]byte(catalog)); e == nil {
		t.Fatal("schema drift became successful output")
	}
}
