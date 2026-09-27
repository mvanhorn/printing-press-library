package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
)

func liveBinary(t *testing.T) string {
	t.Helper()
	if os.Getenv("WALKERPLUS_LIVE") != "1" {
		t.Skip("set WALKERPLUS_LIVE=1 to exercise the public Walkerplus source")
	}
	if value := os.Getenv("WALKERPLUS_BINARY"); value != "" {
		return value
	}
	_, file, _, _ := runtime.Caller(0)
	binary := filepath.Join(filepath.Dir(file), "..", "walkerplus-pp-cli")
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("build ./walkerplus-pp-cli or set WALKERPLUS_BINARY: %v", err)
	}
	return binary
}

func liveJSON(t *testing.T, args []string, result any) {
	t.Helper()
	command := exec.Command(liveBinary(t), append(args, "--cache-dir", t.TempDir(), "--refresh", "--timeout", "60s")...)
	var diagnostics strings.Builder
	command.Stderr = &diagnostics
	start := time.Now()
	output, err := command.Output()
	if err != nil {
		t.Fatalf("live %v: %v, stderr: %s, stdout: %.300s", args, err, diagnostics.String(), output)
	}
	if strings.Count(string(output), "\n") != 1 {
		t.Fatal("live output is not compact single-line JSON")
	}
	if err = json.Unmarshal(output, result); err != nil {
		t.Fatalf("invalid live JSON: %v", err)
	}
	t.Logf("PASS command=%q stdout_bytes=%d wall_ms=%d diagnostics=%q", args, len(output), time.Since(start).Milliseconds(), diagnostics.String())
}

func TestLiveMultiRegionCategories(t *testing.T) {
	liveBinary(t)
	cases := []struct{ name, prefecture, code, category, categoryCode, from, to string }{
		{"tokyo_art", "tokyo", "ar0313", "art-exhibitions", "eg0107", "2026-09-27", "2026-09-30"},
		{"kyoto_festival", "kyoto", "ar0726", "festival", "eg0055", "2026-10-01", "2026-10-31"},
		{"hokkaido_fireworks", "hokkaido", "ar0101", "fireworks", "eg0102", "2026-10-01", "2026-10-31"},
		{"miyagi_art", "miyagi", "ar0204", "art-exhibitions", "eg0107", "2026-10-01", "2026-10-31"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var result walkerplus.Result
			liveJSON(t, []string{"search", "--prefecture", c.prefecture, "--category", c.category, "--from", c.from, "--to", c.to, "--max-pages", "3", "--limit", "10"}, &result)
			if len(result.Events) == 0 {
				t.Fatalf("positive regional/category query returned no events; coverage=%+v", result.Coverage)
			}
			seen := map[string]bool{}
			for _, event := range result.Events {
				if seen[event.ID] {
					t.Fatalf("duplicate event ID %s", event.ID)
				}
				seen[event.ID] = true
				if event.Location.PrefectureCode == nil || *event.Location.PrefectureCode != c.code {
					t.Fatalf("wrong location: %+v", event.Location)
				}
				if event.StartDate == nil || event.EndDate == nil {
					t.Fatalf("dated query returned unknown envelope for %s", event.ID)
				}
				for _, date := range []string{*event.StartDate, *event.EndDate} {
					if _, err := time.Parse("2006-01-02", date); err != nil {
						t.Fatalf("non-ISO date %q", date)
					}
				}
				if *event.StartDate > c.to || *event.EndDate < c.from {
					t.Fatalf("wrong-year/date result %s: %s..%s", event.ID, *event.StartDate, *event.EndDate)
				}
				foundCategory := false
				for _, category := range event.Categories {
					if category.Code == c.categoryCode || (c.categoryCode == "eg0055" && (category.Code == "eg0135" || category.Code == "eg0140")) {
						foundCategory = true
					}
				}
				if !foundCategory {
					t.Fatalf("category mismatch %s: %+v", event.ID, event.Categories)
				}
				if event.Match == nil || event.Match.State == "confirmed" {
					t.Fatalf("listing-only search invented confirmed activity for %s", event.ID)
				}
			}
			if result.Coverage.ScannedPages > 3 || result.Coverage.DetailCount != 0 || result.Coverage.RequestCount > 9 {
				t.Fatalf("search bounds/detail laziness violated: %+v", result.Coverage)
			}
			if c.name == "kyoto_festival" {
				anchor := seen["ar0726e612292"]
				for _, event := range result.Events {
					if strings.Contains(event.TitleJA, "パン") && strings.Contains(event.TitleJA, "上賀茂") {
						anchor = true
					}
				}
				if !anchor {
					t.Fatal("verified Kyoto October festival anchors absent")
				}
			}
			if c.name == "hokkaido_fireworks" && !seen["ar0101e66092"] {
				t.Fatal("verified Hokkaido long-running fireworks anchor absent")
			}
			t.Logf("PASS assertions=%d unique_events=%d scanned_pages=%d detail_count=%d request_count=%d", len(result.Events)*5, len(result.Events), result.Coverage.ScannedPages, result.Coverage.DetailCount, result.Coverage.RequestCount)
		})
	}
}

func TestLiveKnownDetailEvidence(t *testing.T) {
	liveBinary(t)
	cases := []struct {
		name, id string
		check    func(*testing.T, walkerplus.Event)
	}{
		{"free_optional_purchases", "ar0726e612484", func(t *testing.T, e walkerplus.Event) {
			if e.Admission.Status != "free" || e.Admission.Raw == nil || !strings.Contains(*e.Admission.Raw, "入場無料") || !strings.Contains(*e.Admission.Raw, "有料") {
				t.Fatalf("free admission with paid booth caveat misclassified: %+v", e.Admission)
			}
		}},
		{"paid_reservations", "ar0313e603640", func(t *testing.T, e walkerplus.Event) {
			if e.Admission.Status != "paid" || e.ReservationRequired == nil || !*e.ReservationRequired {
				t.Fatalf("paid/reservation source lost: admission=%+v reservation=%v", e.Admission, e.ReservationRequired)
			}
			if e.EndDate == nil || *e.EndDate != "2026-09-30" {
				t.Fatalf("Tokyo edition changed unexpectedly: %v", e.EndDate)
			}
		}},
		{"weather", "ar0101e66092", func(t *testing.T, e walkerplus.Event) {
			if e.Weather == nil || (!strings.Contains(*e.Weather, "雨") && !strings.Contains(*e.Weather, "風")) {
				t.Fatalf("weather caveat lost: %v", e.Weather)
			}
		}},
		{"indoor", "ar0727e612159", func(t *testing.T, e walkerplus.Event) {
			if e.Indoor == nil || !*e.Indoor {
				t.Fatalf("explicit indoor fact lost: %v", e.Indoor)
			}
		}},
		{"holiday_rule", "ar0204e612432", func(t *testing.T, e walkerplus.Event) {
			if e.Schedule.Raw == nil || !strings.Contains(*e.Schedule.Raw, "祝日") || len(e.Schedule.Unresolved) == 0 {
				t.Fatalf("holiday-dependent closure silently resolved: %+v", e.Schedule)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var result struct {
				Event    walkerplus.Event
				Coverage walkerplus.Coverage
			}
			liveJSON(t, []string{"event", c.id}, &result)
			if result.Event.ID != c.id || result.Event.TitleJA == "" || len(result.Event.Sources) == 0 {
				t.Fatalf("missing actual source edition %s", c.id)
			}
			if result.Coverage.DetailCount != 1 || result.Coverage.RequestCount > 9 {
				t.Fatalf("detail bounds: %+v", result.Coverage)
			}
			for _, source := range result.Event.Sources {
				if source.FetchedAt == "" || source.CacheHit {
					t.Fatalf("fresh source metadata missing: %+v", source)
				}
			}
			c.check(t, result.Event)
			t.Logf("PASS actual source evidence id=%s title=%q requests=%d", c.id, result.Event.TitleJA, result.Coverage.RequestCount)
		})
	}
}

func TestLiveShortlistAndWrongYear(t *testing.T) {
	liveBinary(t)
	var result walkerplus.Result
	liveJSON(t, []string{"shortlist", "--prefecture", "kyoto", "--category", "festival", "--from", "2026-10-01", "--to", "2026-10-31", "--max-pages", "1", "--max-details", "3", "--limit", "3"}, &result)
	if len(result.Events) == 0 || result.Coverage.DetailCount == 0 || result.Coverage.DetailCount > 3 {
		t.Fatalf("positive shortlist/enrichment failed: %+v", result.Coverage)
	}
	for _, event := range result.Events {
		if event.Match == nil {
			t.Fatalf("missing derived match for %s", event.ID)
		}
		listingSource := false
		for _, source := range event.Sources {
			if strings.Contains(source.URL, "/event_list/") {
				listingSource = true
			}
			if source.FetchedAt == "" || source.CacheHit {
				t.Fatalf("shortlist must retain fresh source provenance: %+v", source)
			}
		}
		if !listingSource {
			t.Fatalf("shortlist lost listing provenance for %s", event.ID)
		}
		festival := false
		for _, category := range event.Categories {
			if category.Code == "eg0055" || category.Code == "eg0135" || category.Code == "eg0140" {
				festival = true
			}
		}
		if !festival || event.Location.PrefectureCode == nil || *event.Location.PrefectureCode != "ar0726" {
			t.Fatalf("enriched Kyoto festival relevance failed: %+v", event)
		}
		for _, date := range event.Match.ConfirmedDays {
			if date < "2026-10-01" || date > "2026-10-31" {
				t.Fatalf("confirmed day outside query: %s", date)
			}
			for _, excluded := range event.Schedule.ExcludedDates {
				if date == excluded {
					t.Fatalf("confirmed excluded day %s", date)
				}
			}
		}
		if len(event.Schedule.Unresolved) > 0 && event.Match.State == "confirmed" {
			t.Fatalf("unresolved source schedule confirmed: %s", event.ID)
		}
	}
	var detail struct{ Event walkerplus.Event }
	liveJSON(t, []string{"event", result.Events[0].ID, "--select", "id,location.venue"}, &detail)
	shortlistVenue := result.Events[0].Location.Venue
	if shortlistVenue == nil || detail.Event.Location.Venue == nil || *shortlistVenue != *detail.Event.Location.Venue {
		t.Fatalf("shortlist venue differs from fresh detail: shortlist=%v detail=%v", shortlistVenue, detail.Event.Location.Venue)
	}
	t.Logf("PASS shortlist events=%d details=%d requests=%d", len(result.Events), result.Coverage.DetailCount, result.Coverage.RequestCount)
	liveJSON(t, []string{"search", "--prefecture", "kyoto", "--category", "festival", "--from", "2100-10-01", "--to", "2100-10-31", "--max-pages", "1"}, &result)
	if len(result.Events) != 0 || result.Coverage.ScannedPages != 1 {
		t.Fatalf("wrong-year query fabricated matches: %+v", result)
	}
	t.Log("PASS negative wrong-year assertion: events=[] after one actual source page")
}

func TestLiveCatalogs(t *testing.T) {
	liveBinary(t)
	var categories walkerplus.CatalogResult
	liveJSON(t, []string{"categories"}, &categories)
	found := map[string]bool{}
	for _, item := range categories.Items {
		if item.NameJA == "" || len(item.Aliases) == 0 || !strings.HasPrefix(item.Path, "/event_list/") {
			t.Fatalf("category labels/routes missing: %+v", item)
		}
		found[item.Code] = true
	}
	for _, code := range []string{"eg0055", "eg0102", "eg0104", "eg0107", "eg0115"} {
		if !found[code] {
			t.Fatalf("missing source category %s", code)
		}
	}
	if categories.Coverage.RequestCount != 0 {
		t.Fatal("static category catalog made network requests")
	}
	var areas walkerplus.CatalogResult
	liveJSON(t, []string{"areas"}, &areas)
	if len(areas.Items) != 47 || areas.Coverage.RequestCount != 0 {
		t.Fatalf("prefecture catalog contract: count=%d coverage=%+v", len(areas.Items), areas.Coverage)
	}
	liveJSON(t, []string{"areas", "--prefecture", "tokyo"}, &areas)
	cityCount := 0
	for _, item := range areas.Items {
		if item.Kind == "city" {
			cityCount++
			if item.PrefectureCode == nil || *item.PrefectureCode != "ar0313" || item.NameJA == "" {
				t.Fatalf("irrelevant city catalog item: %+v", item)
			}
		}
	}
	if cityCount == 0 || areas.Coverage.RequestCount != 1 {
		t.Fatalf("current Tokyo city discovery failed: cities=%d coverage=%+v", cityCount, areas.Coverage)
	}
	t.Logf("PASS catalogs categories=%d prefectures=47 Tokyo_cities=%d", len(categories.Items), cityCount)
}

func TestLiveCityCodesAliasesAndPrefecture(t *testing.T) {
	liveBinary(t)
	var catalog walkerplus.CatalogResult
	liveJSON(t, []string{"areas", "--prefecture", "tokyo"}, &catalog)
	var shinjuku *walkerplus.CatalogItem
	for i := range catalog.Items {
		if catalog.Items[i].Code == "ar0313104" {
			shinjuku = &catalog.Items[i]
		}
	}
	if shinjuku == nil || !strings.Contains(shinjuku.Path, "/ar0313104/shinjuku/") {
		t.Fatalf("source Shinjuku catalog route missing: %+v", shinjuku)
	}
	expectedIDs := map[string]bool{}
	for _, city := range []string{"ar0313104", "shinjuku", "新宿区"} {
		t.Run(city, func(t *testing.T) {
			var result walkerplus.Result
			liveJSON(t, []string{"search", "--prefecture", "tokyo", "--city", city, "--limit", "3", "--max-pages", "1"}, &result)
			if len(result.Events) == 0 {
				t.Fatalf("positive city query returned no events: %+v", result.Coverage)
			}
			if result.Query.City != "ar0313104" || result.Query.Prefecture != "ar0313" {
				t.Fatalf("city not canonicalized: %+v", result.Query)
			}
			if result.Coverage.CatalogRequests < 1 || result.Coverage.CatalogRequests > 3 || len(result.Coverage.CatalogRoutes) != 1 || result.Coverage.ScannedPages != 1 || result.Coverage.DetailCount != 0 || result.Coverage.RequestCount < result.Coverage.CatalogRequests+1 || result.Coverage.RequestCount > 6 {
				t.Fatalf("city lookup/event-page bounds conflated: %+v", result.Coverage)
			}
			if len(result.Coverage.Routes) != 1 || !strings.Contains(result.Coverage.Routes[0], "/ar0313104/shinjuku/") {
				t.Fatalf("source city slug omitted: %+v", result.Coverage.Routes)
			}
			actualIDs := map[string]bool{}
			for _, event := range result.Events {
				if actualIDs[event.ID] {
					t.Fatalf("duplicate city event %s", event.ID)
				}
				actualIDs[event.ID] = true
				if event.Location.PrefectureCode == nil || *event.Location.PrefectureCode != "ar0313" || event.Location.CityCode == nil || *event.Location.CityCode != "ar0313104" || event.Location.CityJA == nil || !strings.Contains(*event.Location.CityJA, "新宿") {
					t.Fatalf("irrelevant city event %s: %+v", event.ID, event.Location)
				}
			}
			if len(expectedIDs) == 0 {
				for id := range actualIDs {
					expectedIDs[id] = true
				}
			} else {
				if len(actualIDs) != len(expectedIDs) {
					t.Fatalf("city forms returned different result counts: %#v %#v", actualIDs, expectedIDs)
				}
				for id := range expectedIDs {
					if !actualIDs[id] {
						t.Fatalf("city forms disagree on source edition %s", id)
					}
				}
			}
			t.Logf("PASS city=%s events=%d catalog_requests=%d event_pages=%d actual_HTTP=%d", city, len(result.Events), result.Coverage.CatalogRequests, result.Coverage.ScannedPages, result.Coverage.RequestCount)
		})
	}
	for _, city := range []string{"ar0313104", "shinjuku"} {
		t.Run("wrong_prefecture_"+city, func(t *testing.T) {
			command := exec.Command(liveBinary(t), "search", "--prefecture", "kyoto", "--city", city, "--max-pages", "1", "--limit", "3", "--no-cache", "--refresh")
			var diagnostic strings.Builder
			command.Stderr = &diagnostic
			output, err := command.Output()
			failure, ok := err.(*exec.ExitError)
			if !ok || failure.ExitCode() != 2 {
				t.Fatalf("wrong prefecture must exit2: err=%v stdout=%s stderr=%s", err, output, diagnostic.String())
			}
			if !strings.Contains(diagnostic.String(), "city") {
				t.Fatalf("wrong-prefecture error is not actionable: %s", diagnostic.String())
			}
			if len(output) > 0 {
				var result map[string]any
				if err := json.Unmarshal(output, &result); err != nil || result["error"] == nil || result["events"] != nil {
					t.Fatalf("mismatch fabricated successful events: %s", output)
				}
			}
			t.Logf("PASS negative city=%s prefecture=kyoto exit2 no successful event result", city)
		})
	}
}

func TestLiveFreeAdmissionShortlist(t *testing.T) {
	liveBinary(t)
	var result walkerplus.Result
	liveJSON(t, []string{"shortlist", "--prefecture", "kyoto", "--category", "festival", "--from", "2026-10-10", "--to", "2026-10-11", "--free", "--max-pages", "3", "--max-details", "10", "--limit", "10"}, &result)
	breadFestival := false
	for _, event := range result.Events {
		if event.Admission.Status != "free" {
			t.Fatalf("strict --free admitted non-free event %s: %+v", event.ID, event.Admission)
		}
		if event.Location.PrefectureCode == nil || *event.Location.PrefectureCode != "ar0726" || event.Match == nil || event.Match.State == "excluded" {
			t.Fatalf("free shortlist returned irrelevant event %s", event.ID)
		}
		if event.ID == "ar0726e612484" {
			breadFestival = true
			if event.Admission.Raw == nil || !strings.Contains(*event.Admission.Raw, "購入") || !strings.Contains(*event.Admission.Raw, "有料") {
				t.Fatal("optional paid booth caveat disappeared")
			}
		}
	}
	if !breadFestival {
		t.Fatalf("known free-admission bread festival missing: events=%+v coverage=%+v", result.Events, result.Coverage)
	}
	if result.Coverage.ScannedPages > 3 || result.Coverage.DetailCount > 10 || len(result.Events) > 10 {
		t.Fatalf("free shortlist bounds exceeded: %+v", result.Coverage)
	}
	t.Logf("PASS --free Kyoto Oct10-11 includes ar0726e612484 with paid-purchase caveat; events=%d details=%d actual_HTTP=%d", len(result.Events), result.Coverage.DetailCount, result.Coverage.RequestCount)
}

func TestLiveTimingBoundaryModes(t *testing.T) {
	liveBinary(t)
	for _, timing := range []string{"starts", "ends"} {
		t.Run(timing, func(t *testing.T) {
			var result walkerplus.Result
			liveJSON(t, []string{"shortlist", "--prefecture", "hokkaido", "--category", "fireworks", "--from", "2026-10-01", "--to", "2026-10-15", "--timing", timing, "--max-pages", "1", "--max-details", "2", "--limit", "2"}, &result)
			if len(result.Events) == 0 {
				t.Fatalf("positive %s boundary window returned no events: %+v", timing, result.Coverage)
			}
			tomakomai := false
			for _, event := range result.Events {
				if event.ID == "ar0101e66092" {
					t.Fatalf("long-running Apr28-Oct31 envelope incorrectly matched %s window", timing)
				}
				boundary := event.StartDate
				if timing == "ends" {
					boundary = event.EndDate
				}
				if boundary == nil || *boundary < "2026-10-01" || *boundary > "2026-10-15" {
					t.Fatalf("%s boundary outside exact trip window for %s: %v", timing, event.ID, boundary)
				}
				if event.Location.PrefectureCode == nil || *event.Location.PrefectureCode != "ar0101" || event.Match == nil || event.Match.Timing != timing || event.Match.State == "excluded" {
					t.Fatalf("irrelevant derived %s match: %+v", timing, event)
				}
				if event.ID == "ar0101e00714" {
					tomakomai = true
					if *boundary != "2026-10-10" || !strings.Contains(event.TitleJA, "苫小牧") {
						t.Fatalf("known Tomakomai edition boundary changed: %+v", event)
					}
				}
			}
			if !tomakomai {
				t.Fatalf("known Tomakomai Oct10 fireworks absent from %s results", timing)
			}
			if result.Coverage.ScannedPages > 1 || result.Coverage.DetailCount > 2 || result.Coverage.CatalogRequests != 0 || result.Coverage.RequestCount > 21 {
				t.Fatalf("%s mode exceeded page/detail/retry bounds: %+v", timing, result.Coverage)
			}
			t.Logf("PASS %s: TomakomaiOct10 included, long-run excluded, every source boundary insideOct1-15; events=%d details=%d HTTP=%d", timing, len(result.Events), result.Coverage.DetailCount, result.Coverage.RequestCount)
		})
	}
}

func TestLiveIndoorShortlist(t *testing.T) {
	liveBinary(t)
	// The captured event's source tags identify 展示会 (eg0126), and source
	// navigation identifies Osaka city ar0727100/osaka; its locality is
	// 大阪市鶴見区. These filters keep detail work within three candidates.
	var result walkerplus.Result
	liveJSON(t, []string{"shortlist", "--prefecture", "osaka", "--city", "ar0727100", "--category", "exhibitions", "--from", "2026-10-11", "--to", "2026-10-11", "--indoor", "--max-pages", "1", "--max-details", "3", "--limit", "3"}, &result)
	target := false
	for _, event := range result.Events {
		if event.Indoor == nil || !*event.Indoor {
			t.Fatalf("strict --indoor admitted unknown/false event %s", event.ID)
		}
		if event.Location.PrefectureCode == nil || *event.Location.PrefectureCode != "ar0727" || event.Match == nil || event.Match.State == "excluded" {
			t.Fatalf("indoor shortlist returned irrelevant Osaka edition %s", event.ID)
		}
		if event.ID == "ar0727e612159" {
			target = true
			if event.StartDate == nil || *event.StartDate != "2026-10-11" || event.EndDate == nil || *event.EndDate != "2026-10-11" || event.Location.CityJA == nil || !strings.Contains(*event.Location.CityJA, "大阪市") {
				t.Fatalf("known indoor source date/locality changed: %+v", event)
			}
			exhibition := false
			for _, category := range event.Categories {
				if category.Code == "eg0126" {
					exhibition = true
				}
			}
			if !exhibition {
				t.Fatalf("known indoor source category missing: %+v", event.Categories)
			}
		}
	}
	if !target {
		t.Fatalf("explicit indoor medaka target absent: events=%+v coverage=%+v", result.Events, result.Coverage)
	}
	if result.Coverage.ScannedPages > 1 || result.Coverage.DetailCount > 3 || len(result.Coverage.CatalogRoutes) > 1 || result.Coverage.RequestCount > 33 {
		t.Fatalf("indoor shortlist exceeded listing/catalog/detail/retry bounds: %+v", result.Coverage)
	}
	t.Logf("PASS --indoor: actual e612159 Oct11 included; every returned indoor=true; events=%d details=%d catalog_HTTP=%d total_HTTP=%d", len(result.Events), result.Coverage.DetailCount, result.Coverage.CatalogRequests, result.Coverage.RequestCount)
}
