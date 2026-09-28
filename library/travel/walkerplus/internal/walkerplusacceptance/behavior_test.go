package walkerplusacceptance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
)

func TestShortlistRecurrenceExclusionsAndClosedDays(t *testing.T) {
	weekends := eventFixture("ar0313e901", "週末の体験会", "2026-10-03", "2026-10-18", "2026年10月3日(土)～10月18日(日)")
	weekends.schedule = weekends.period + " 期間中の土・日曜のみ開催。10月11日(日)は休み"
	s := fixtureSource(t, []sourceEvent{weekends})
	setListing(s, listingFixture(t, []sourceEvent{weekends}, "", ""), "/event_list/10/ar0313/eg0120/")
	c := fixtureClient(t, s)
	result, err := c.Shortlist(context.Background(), trip("2026-10-03", "2026-10-18"))
	if err != nil {
		t.Fatal(err)
	}
	e := onlyEvent(t, result)
	if e.Match == nil {
		t.Fatal("shortlist omitted derived trip match")
	}
	assertStrings(t, "confirmed_days", e.Match.ConfirmedDays, []string{"2026-10-03", "2026-10-04", "2026-10-10", "2026-10-17", "2026-10-18"})
	assertStrings(t, "possible_days", e.Match.PossibleDays, []string{})
	assertStrings(t, "excluded_dates", e.Schedule.ExcludedDates, []string{"2026-10-11"})
	if e.Schedule.Raw == nil || *e.Schedule.Raw != weekends.schedule {
		t.Errorf("schedule source text lost: %+v", e.Schedule)
	}
	if e.StartDate == nil || *e.StartDate != "2026-10-03" || e.EndDate == nil || *e.EndDate != "2026-10-18" {
		t.Errorf("source envelope was replaced with derived occurrence dates: %+v", e)
	}
	assertEvidence(t, e, "schedule", "10月11日(日)は休み")

	for _, date := range []string{"2026-10-11", "2026-10-12"} {
		t.Run("no_match_on_"+date, func(t *testing.T) {
			r, err := c.Shortlist(context.Background(), trip(date, date))
			if err != nil {
				t.Fatal(err)
			}
			if r.Events == nil || len(r.Events) != 0 {
				t.Fatalf("unsupported attendance on %s: %+v", date, r.Events)
			}
		})
	}

	closed := eventFixture("ar0313e902", "月曜休館の展示", "2026-10-10", "2026-10-20", "2026年10月10日(土)～10月20日(火)")
	closed.schedule = closed.period + " 休館日：月曜"
	s = fixtureSource(t, []sourceEvent{closed})
	setListing(s, listingFixture(t, []sourceEvent{closed}, "", ""), "/event_list/10/ar0313/eg0120/")
	c = fixtureClient(t, s)
	positive, err := c.Shortlist(context.Background(), trip("2026-10-11", "2026-10-11"))
	if err != nil {
		t.Fatal(err)
	}
	onSunday := onlyEvent(t, positive)
	if onSunday.Match == nil {
		t.Fatal("positive Sunday has no match")
	}
	// A stated closure rules out Monday; it does not independently establish
	// Sunday opening. Affirmative daily/weekly controls are covered separately.
	assertStrings(t, "positive Sunday confirmed_days", onSunday.Match.ConfirmedDays, []string{})
	assertStrings(t, "positive Sunday possible_days", onSunday.Match.PossibleDays, []string{"2026-10-11"})
	negative, err := c.Shortlist(context.Background(), trip("2026-10-12", "2026-10-12"))
	if err != nil {
		t.Fatal(err)
	}
	if negative.Events == nil || len(negative.Events) != 0 {
		t.Fatalf("explicit closed Monday must not match: %+v", negative.Events)
	}
}

func TestHolidayExceptionsAndSeasonalDatesStayPossible(t *testing.T) {
	tests := []struct{ name, schedule, from, to, certainty string }{
		{"holiday_exception", "2026年10月10日(土)～12月27日(日) 休館日：月曜(祝日の場合は翌平日)", "2026-10-12", "2026-10-12", "exact"},
		{"approximate_season", "2026年11月中旬～12月上旬 例年の見頃", "2026-11-15", "2026-11-15", "approximate"},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := eventFixture(fmt.Sprintf("ar0313e%d", 910+i), tc.name, "2026-10-10", "2026-12-27", tc.schedule)
			e.schedule = tc.schedule
			if tc.name == "approximate_season" {
				e.start, e.end = "2026-11-11", "2026-12-10"
			}
			s := fixtureSource(t, []sourceEvent{e})
			month := tc.from[5:7]
			setListing(s, listingFixture(t, []sourceEvent{e}, "", ""), "/event_list/"+month+"/ar0313/eg0120/")
			r, err := fixtureClient(t, s).Shortlist(context.Background(), trip(tc.from, tc.to))
			if err != nil {
				t.Fatal(err)
			}
			got := onlyEvent(t, r)
			if got.Match == nil {
				t.Fatal("missing match")
			}
			assertStrings(t, "confirmed_days", got.Match.ConfirmedDays, []string{})
			assertStrings(t, "possible_days", got.Match.PossibleDays, []string{tc.from})
			if got.DateCertainty != tc.certainty {
				t.Errorf("date certainty = %q, want %q", got.DateCertainty, tc.certainty)
			}
			if tc.name == "holiday_exception" && len(got.Schedule.Unresolved) == 0 {
				t.Error("holiday-dependent closure rule omitted unresolved explanation")
			}
		})
	}
}

func TestYearBoundaryAndOldEditionNeverRollForward(t *testing.T) {
	e := eventFixture("ar0313e920", "年越しの体験会2026", "2026-12-31", "2027-01-02", "2026年12月31日(木)～2027年1月2日(土)")
	e.schedule = e.period + " 期間中は毎日開催"
	s := fixtureSource(t, []sourceEvent{e})
	setListing(s, listingFixture(t, []sourceEvent{e}, "", ""), "/event_list/12/ar0313/eg0120/", "/event_list/01/ar0313/eg0120/")
	q := trip("2026-12-31", "2027-01-02")
	q.MaxPages = 2
	r, err := fixtureClient(t, s).Shortlist(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	got := onlyEvent(t, r)
	if got.Match == nil {
		t.Fatal("cross-year positive result has no match")
	}
	assertStrings(t, "cross-year confirmed_days", got.Match.ConfirmedDays, []string{"2026-12-31", "2027-01-01", "2027-01-02"})
	if got.EditionYear == nil || *got.EditionYear != 2026 || got.Timezone != "Asia/Tokyo" {
		t.Errorf("lost source edition/timezone: %+v", got)
	}
	if r.Coverage.CandidateCount != 1 {
		t.Errorf("two month routes were not deduplicated: %+v", r.Coverage)
	}

	old := eventFixture("ar0313e921", "2026年の体験会", "2026-10-10", "2026-10-11", "2026年10月10日(土)・11日(日)")
	s = fixtureSource(t, []sourceEvent{old})
	setListing(s, listingFixture(t, []sourceEvent{old}, "", ""), "/event_list/10/ar0313/eg0120/")
	c := fixtureClient(t, s)
	positive, err := c.Search(context.Background(), trip("2026-10-10", "2026-10-11"))
	if err != nil {
		t.Fatal(err)
	}
	onSourceYear := onlyEvent(t, positive)
	if onSourceYear.EditionYear == nil || *onSourceYear.EditionYear != 2026 {
		t.Fatalf("positive old-edition fixture missing its source year: %+v", onSourceYear)
	}
	wrongYear, err := c.Search(context.Background(), trip("2027-10-10", "2027-10-11"))
	if err != nil {
		t.Fatal(err)
	}
	if wrongYear.Events == nil || len(wrongYear.Events) != 0 {
		t.Fatalf("2026 edition rolled into 2027: %+v", wrongYear.Events)
	}
	if wrongYear.Coverage.ScannedPages != 1 || len(wrongYear.Coverage.NativeYearLabels) == 0 {
		t.Errorf("wrong-year empty result lost native route coverage: %+v", wrongYear.Coverage)
	}
	b, err := json.Marshal(wrongYear)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"events":[]`) {
		t.Errorf("empty results must encode as []: %s", b)
	}
}

func TestEventAdmissionReservationIndoorAndWeatherFacts(t *testing.T) {
	tests := []struct {
		name, price, reservation, weather, schedule, notice string
		hasOffers                                           bool
		offersPrice                                         any
		status                                              string
		indoor, required                                    *bool
		canceled                                            bool
	}{
		{name: "paid_children_free", price: "有料。平日1800円、土日祝2100円 ※小学生以下無料。障がい者手帳持参者と介助者1人まで500円割引", reservation: "予約必須 チケット(日時指定制)は公式サイトで販売", hasOffers: true, offersPrice: "None", status: "paid", required: boolPointer(true)},
		{name: "paid_discount_only", price: "有料。料金は公式サイト参照。500円割引", status: "paid"},
		{name: "schema_none_is_unknown", hasOffers: true, offersPrice: "None", status: "unknown"},
		{name: "schema_zero_is_not_admission_evidence", hasOffers: true, offersPrice: 0, status: "unknown"},
		{name: "free_parking_is_unknown", price: "駐車場無料", hasOffers: true, offersPrice: "None", status: "unknown"},
		{name: "genuine_free", price: "入場無料", reservation: "予約不要", status: "free", required: boolPointer(false)},
		{name: "mixed_admission", price: "入場無料。一部有料エリアあり", status: "mixed"},
		{name: "unconditional_indoor", schedule: "2026年10月11日(日) 雨天決行（屋内会場）", status: "unknown", indoor: boolPointer(true)},
		{name: "conditional_indoor", schedule: "2026年10月11日(日) 雨天の場合は屋内会場で開催", status: "unknown"},
		{name: "mixed_indoor_outdoor", schedule: "2026年10月11日(日) 屋内会場と屋外会場で開催", status: "unknown"},
		{name: "conditional_weather_cancellation", weather: "雨天決行(強風中止)", status: "unknown"},
		{name: "actual_cancellation", notice: "本イベントは開催中止となりました", status: "unknown", canceled: true},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := eventFixture(fmt.Sprintf("ar0313e%d", 930+i), tc.name, "2026-10-11", "2026-10-11", "2026年10月11日(日)")
			e.price, e.reservation, e.weather, e.schedule, e.notice = tc.price, tc.reservation, tc.weather, tc.schedule, tc.notice
			e.hasOffers, e.offersPrice = tc.hasOffers, tc.offersPrice
			s := fixtureSource(t, []sourceEvent{e})
			c := fixtureClient(t, s)
			got, err := c.Event(context.Background(), e.id)
			if err != nil {
				t.Fatal(err)
			}
			if got.ID != e.id || got.TitleJA != e.title {
				t.Fatalf("event identity was taken from organizer URL or mismatched schema: %+v", got)
			}
			if got.Admission.Status != tc.status {
				t.Errorf("admission status = %q; want %q, raw=%v", got.Admission.Status, tc.status, got.Admission.Raw)
			}
			if tc.price != "" {
				assertEvidence(t, got, "admission", tc.price)
			}
			if tc.status == "unknown" && got.Admission.Price != nil {
				t.Errorf("unknown price must remain null: %+v", got.Admission)
			}
			if (tc.name == "paid_children_free" || tc.name == "paid_discount_only") && got.Admission.Price != nil {
				t.Errorf("tiered source price should stay null rather than select a child/discount price: %+v", got.Admission)
			}
			if tc.status == "free" && (got.Admission.Price == nil || *got.Admission.Price != 0) {
				t.Errorf("explicit admission-free evidence missing zero price: %+v", got.Admission)
			}
			assertBool(t, "indoor", got.Indoor, tc.indoor)
			assertBool(t, "reservation_required", got.ReservationRequired, tc.required)
			if tc.required != nil {
				assertEvidence(t, got, "reservation_required", tc.reservation)
			}
			if tc.indoor != nil {
				assertEvidence(t, got, "indoor", "屋内会場")
			}
			if tc.weather != "" && (got.Weather == nil || *got.Weather != tc.weather) {
				t.Errorf("lost conditional weather source text: %+v", got)
			}
			if (got.Cancellation != nil) != tc.canceled {
				t.Errorf("current cancellation = %v; conditional weather must remain separate, canceled=%v", got.Cancellation, tc.canceled)
			}
			if got.Match != nil {
				t.Errorf("single Event added a trip-derived match without a query: %+v", got.Match)
			}
			if len(got.Sources) != 3 || c.Stats().RequestCount != 3 {
				t.Errorf("event did not fetch exactly linked base/data/price pages: sources=%+v stats=%+v", got.Sources, c.Stats())
			}
			if tc.name == "schema_none_is_unknown" {
				b, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				for _, fragment := range []string{`"price":null`, `"indoor":null`, `"reservation_required":null`, `"categories":[]`, `"excluded_dates":[]`, `"closed_weekdays":[]`, `"unresolved":[]`} {
					if !strings.Contains(string(b), fragment) {
						t.Errorf("unknown/empty JSON contract omitted %s: %s", fragment, b)
					}
				}
			}
		})
	}
}

func TestShortlistFreeAndIndoorConstraintsUseAND(t *testing.T) {
	eligible := eventFixture("ar0313e950", "無料の屋内体験", "2026-10-11", "2026-10-11", "2026年10月11日(日)")
	eligible.price, eligible.schedule = "入場無料", eligible.period+" 屋内会場で開催"
	paidIndoor := eventFixture("ar0313e951", "有料の屋内体験", eligible.start, eligible.end, eligible.period)
	paidIndoor.price, paidIndoor.schedule = "有料。大人1800円、小学生以下無料", eligible.schedule
	freeConditional := eventFixture("ar0313e952", "雨天だけ屋内", eligible.start, eligible.end, eligible.period)
	freeConditional.price, freeConditional.schedule = "入場無料", eligible.period+" 雨天の場合は屋内会場で開催"
	parking := eventFixture("ar0313e953", "無料駐車場の体験", eligible.start, eligible.end, eligible.period)
	parking.price, parking.schedule = "駐車場無料", eligible.schedule
	canceled := eventFixture("ar0313e954", "中止の無料屋内体験", eligible.start, eligible.end, eligible.period)
	canceled.price, canceled.schedule, canceled.notice = eligible.price, eligible.schedule, "本イベントは開催中止となりました"
	events := []sourceEvent{eligible, paidIndoor, freeConditional, parking, canceled}
	s := fixtureSource(t, events)
	setListing(s, listingFixture(t, events, "", ""), "/event_list/10/ar0313/eg0120/")
	q := trip("2026-10-11", "2026-10-11")
	q.Free, q.Indoor, q.MaxDetails = true, true, len(events)
	r, err := fixtureClient(t, s).Shortlist(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	got := onlyEvent(t, r)
	if got.ID != eligible.id {
		t.Fatalf("constraints admitted false positive or lost valid positive: %+v", r.Events)
	}
	if got.Admission.Status != "free" || got.Indoor == nil || !*got.Indoor {
		t.Errorf("returned event lacks independently sourced admission and indoor facts: %+v", got)
	}
	if r.Coverage.ExcludedCount == 0 {
		t.Error("cancelled/filtered candidates omitted exclusion coverage")
	}
}

func TestConditionalWeatherDoesNotExcludeShortlist(t *testing.T) {
	e := eventFixture("ar0313e960", "天候注意の体験会", "2026-10-11", "2026-10-11", "2026年10月11日(日)")
	e.weather = "雨天決行(強風中止)"
	e.description = "雨天決行だが、風向・風速が不安定な場合中止になることがある"
	s := fixtureSource(t, []sourceEvent{e})
	setListing(s, listingFixture(t, []sourceEvent{e}, "", ""), "/event_list/10/ar0313/eg0120/")
	r, err := fixtureClient(t, s).Shortlist(context.Background(), trip("2026-10-11", "2026-10-11"))
	if err != nil {
		t.Fatal(err)
	}
	got := onlyEvent(t, r)
	if got.Cancellation != nil || got.Weather == nil || *got.Weather != e.weather {
		t.Errorf("conditional cancellation became current cancellation: %+v", got)
	}
}

func TestSearchFestivalParentCategoryUsesOnlyCardFacts(t *testing.T) {
	castle := eventFixture("ar0726e612292", "京都南丹園部城祭り2026", "2026-10-03", "2026-10-03", "2026年10月3日(土)")
	castle.prefectureCode, castle.prefectureJA, castle.cityCode, castle.cityJA = "ar0726", "京都府", "", "南丹市"
	castle.categories = []categoryFact{{"eg0135", "祭り"}, {"eg126", "展示会"}}
	bread := castle
	bread.id, bread.title, bread.start, bread.end, bread.period = "ar0726e612484", "京都パンフェスティバルin上賀茂神社 2026", "2026-10-10", "2026-10-11", "2026年10月10日(土)・11日(日)"
	bread.categories = []categoryFact{{"eg0135", "祭り"}, {"eg0117", "グルメ・フードフェス"}}
	bread.listingFacts = `<li class="m-mainlist-item-category__item"><p class="m-mainlist-item-category__text">入場無料</p></li>`
	music := castle
	music.id, music.title, music.categories = "ar0726e999", "音楽のみのイベント", []categoryFact{{"eg0109", "ライブ・音楽イベント"}}
	navigation := `<nav><a href="/event_list/eg0055/">お祭り</a><a href="/event_list/ar0313/">東京都</a></nav>`
	s := fixtureSource(t, []sourceEvent{castle, bread, music})
	setListing(s, listingFixture(t, []sourceEvent{castle, bread, music}, "", navigation), "/event_list/10/ar0726/eg0055/", "/event_list/10/ar0313/eg0055/")
	c := fixtureClient(t, s)
	q := walkerplus.Query{Prefecture: "kyoto", Category: "festivals", From: "2026-10-01", To: "2026-10-11", MaxPages: 1, Limit: 10}
	r, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 2 {
		t.Fatalf("festival parent category rejected leaf facts or inherited navigation facts: %+v", r.Events)
	}
	ids := []string{}
	for _, got := range r.Events {
		ids = append(ids, got.ID)
		if got.Location.PrefectureCode == nil || *got.Location.PrefectureCode != "ar0726" {
			t.Errorf("card's Kyoto fact overwritten by navigation: %+v", got.Location)
		}
		if got.ID == castle.id {
			if len(got.Categories) != 2 || got.Categories[0].Code != "eg0135" || got.Categories[1].Code != "eg0126" {
				t.Errorf("nonpadded source category or scoped tags parsed incorrectly: %+v", got.Categories)
			}
		}
	}
	assertStrings(t, "festival IDs", ids, []string{castle.id, bread.id})
	requests, _ := s.snapshot()
	if len(requests) != 1 || strings.HasPrefix(requests[0], "/event/") {
		t.Errorf("Search fetched detail pages: %v", requests)
	}
	q.Prefecture = "tokyo"
	negative, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if negative.Events == nil || len(negative.Events) != 0 {
		t.Errorf("wrong-prefecture query inherited navigation area: %+v", negative.Events)
	}
}

func TestSearchDeduplicatesStableIDsAndSharesMonthPageBudget(t *testing.T) {
	a := eventFixture("ar0313e971", "同日の後ID", "2026-10-10", "2026-11-11", "2026年10月10日(土)～11月11日(水)")
	b := a
	b.id, b.title = "ar0313e970", "同日の先ID"
	s := fixtureSource(t, []sourceEvent{a, b})
	setListing(s, listingFixture(t, []sourceEvent{a, b}, "/event_list/10/ar0313/eg0120/2.html", ""), "/event_list/10/ar0313/eg0120/")
	setListing(s, listingFixture(t, []sourceEvent{b, a}, "/event_list/11/ar0313/eg0120/2.html", ""), "/event_list/11/ar0313/eg0120/")
	q := trip("2026-10-10", "2026-11-11")
	q.MaxPages = 2
	c := fixtureClient(t, s)
	r, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 2 {
		t.Fatalf("source ID dedup lost positives or returned duplicates: %+v", r.Events)
	}
	assertStrings(t, "stable tie order", []string{r.Events[0].ID, r.Events[1].ID}, []string{b.id, a.id})
	requests, _ := s.snapshot()
	if len(requests) != 2 || r.Coverage.ScannedPages != 2 || r.Coverage.RequestCount != 2 {
		t.Errorf("month scans exceeded shared max-pages: requests=%v coverage=%+v", requests, r.Coverage)
	}
	if !r.Coverage.Truncated || r.Coverage.NextPage == nil {
		t.Errorf("unscanned pages omitted bounded-coverage signal: %+v", r.Coverage)
	}
	for _, got := range r.Events {
		if got.Match == nil {
			t.Error("search omitted envelope overlap explanation")
			continue
		}
		assertStrings(t, "listing-only confirmed_days", got.Match.ConfirmedDays, []string{})
	}
}

func TestSearchAmbiguousSchemaJoinNeverBorrowsAnotherEdition(t *testing.T) {
	e := eventFixture("ar0313e980", "同じ名前の展示", "2026-10-10", "2026-10-11", "開催日未定")
	other := e
	other.id, other.start, other.end = "ar0313e981", "2025-10-10", "2025-10-11"
	s := fixtureSource(t, []sourceEvent{e})
	body := listingFixture(t, []sourceEvent{e}, "", "")
	oneSchema := jsonFixture(t, []map[string]any{eventSchema(e)})
	twoSchemas := jsonFixture(t, []map[string]any{eventSchema(e), eventSchema(other)})
	body = strings.Replace(body, oneSchema, twoSchemas, 1)
	setListing(s, body, "/event_list/10/ar0313/eg0120/")
	r, err := fixtureClient(t, s).Search(context.Background(), trip("2026-10-10", "2026-10-11"))
	if err != nil {
		t.Fatal(err)
	}
	got := onlyEvent(t, r)
	if got.StartDate != nil || got.EndDate != nil || got.EditionYear != nil {
		t.Errorf("ambiguous exact-name/venue schema invented source dates: %+v", got)
	}
	if got.Match == nil || got.Match.State != "possible" || len(got.Match.ConfirmedDays) != 0 {
		t.Errorf("ambiguous schedule should remain possible with no confirmed days: %+v", got.Match)
	}
}

func TestShortlistDetailBudgetAndConcurrency(t *testing.T) {
	events := []sourceEvent{}
	for i := 0; i < 4; i++ {
		events = append(events, eventFixture(fmt.Sprintf("ar0313e%d", 990+i), fmt.Sprintf("体験会%d", i), "2026-10-11", "2026-10-11", "2026年10月11日(日)"))
	}
	s := fixtureSource(t, events)
	setListing(s, listingFixture(t, events, "", ""), "/event_list/10/ar0313/eg0120/")
	gate := make(chan struct{})
	var mu sync.Mutex
	roots := 0
	var release sync.Once
	s.before = func(req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/event/") && strings.HasSuffix(req.URL.Path, "/") {
			mu.Lock()
			roots++
			count := roots
			mu.Unlock()
			if count >= 2 {
				release.Do(func() { close(gate) })
			}
			select {
			case <-gate:
			case <-req.Context().Done():
			}
		}
	}
	q := trip("2026-10-11", "2026-10-11")
	q.MaxDetails = 2
	r, err := fixtureClient(t, s).Shortlist(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Events) != 2 {
		t.Fatalf("detail budget should preserve two positives: %+v", r.Events)
	}
	requests, maxActive := s.snapshot()
	if maxActive != 2 {
		t.Errorf("configured concurrency 2 was not respected/exercised: max active=%d requests=%v", maxActive, requests)
	}
	if len(requests) != 7 || r.Coverage.RequestCount != 7 || r.Coverage.DetailCount != 2 {
		t.Errorf("max-details2 must bound requests to listing + 2*(base/data/price): requests=%v coverage=%+v", requests, r.Coverage)
	}
	if !r.Coverage.Truncated || r.Coverage.CandidateCount != 4 {
		t.Errorf("detail budget coverage did not expose unexamined candidates: %+v", r.Coverage)
	}
}

func TestClientBoundsErrorsCacheAndValidation(t *testing.T) {
	t.Run("query_validation_without_requests", func(t *testing.T) {
		invalid := []walkerplus.Query{
			{From: "2026-02-29", To: "2026-03-01"}, {From: "2028-02-30", To: "2028-03-01"},
			{From: "2026-10-12", To: "2026-10-11"}, {From: "2026-10-11"},
			{Limit: 101}, {MaxPages: 21}, {MaxDetails: 31}, {Page: -1},
			{Prefecture: "kyoto", City: "ar0313113"}, {Category: "invented"}, {Timing: "weekday"},
		}
		for _, q := range invalid {
			if _, err := walkerplus.NormalizeQuery(q); err == nil {
				t.Errorf("invalid query accepted: %+v", q)
			}
		}
		valid, err := walkerplus.NormalizeQuery(walkerplus.Query{City: "shibuya", Category: "activities", From: "2028-02-29", To: "2028-03-01"})
		if err != nil {
			t.Fatal(err)
		}
		if valid.Prefecture != "ar0313" || valid.City != "ar0313113" || valid.Category != "eg0120" || valid.Limit != 10 || valid.Page != 1 || valid.MaxPages != 3 {
			t.Errorf("aliases/defaults not normalized: %+v", valid)
		}
		s := fixtureSource(t, nil)
		c := fixtureClient(t, s)
		for _, value := range []string{"https://evil.example/event/ar0313e1/", "https://www.walkerplus.com/event/ar0313e1/?next=evil", "https://www.walkerplus.com/event/ar0313e1/data.html", "http://www.walkerplus.com/event/ar0313e1/", "ar0313e1/../../"} {
			if _, err := c.Event(context.Background(), value); err == nil {
				t.Errorf("invalid event input accepted: %q", value)
			}
		}
		requests, _ := s.snapshot()
		if len(requests) != 0 {
			t.Errorf("validation made network requests: %v", requests)
		}
	})

	t.Run("source_shape_error_is_not_empty_success", func(t *testing.T) {
		s := fixtureSource(t, nil)
		setListing(s, "<html><h1>広告ページ</h1></html>", "/event_list/10/ar0313/eg0120/")
		if r, err := fixtureClient(t, s).Search(context.Background(), trip("2026-10-11", "2026-10-11")); err == nil {
			t.Fatalf("source drift masqueraded as empty success: %+v", r)
		}
	})

	t.Run("explicit_zero_results_has_empty_arrays", func(t *testing.T) {
		s := fixtureSource(t, nil)
		setListing(s, listingFixture(t, nil, "", ""), "/event_list/10/ar0313/eg0120/")
		r, err := fixtureClient(t, s).Search(context.Background(), trip("2026-10-11", "2026-10-11"))
		if err != nil {
			t.Fatal(err)
		}
		if r.Events == nil || len(r.Events) != 0 || r.Coverage.RequestCount != 1 {
			t.Errorf("explicit0件 contract violated: %+v", r)
		}
		if r.Coverage.Reasons == nil || r.Coverage.Routes == nil || r.Coverage.NativeYearLabels == nil {
			t.Errorf("empty coverage arrays became null: %+v", r.Coverage)
		}
	})

	t.Run("retry_bound_and_error_not_empty_success", func(t *testing.T) {
		s := fixtureSource(t, nil)
		s.respond = func(w http.ResponseWriter, _ *http.Request) bool {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return true
		}
		c := fixtureClient(t, s, func(o *walkerplus.Options) { o.Retries = 1 })
		if r, err := c.Search(context.Background(), trip("2026-10-11", "2026-10-11")); err == nil {
			t.Fatalf("503 masqueraded as empty success: %+v", r)
		}
		requests, _ := s.snapshot()
		if len(requests) != 2 {
			t.Errorf("retries1 made %d requests, want2: %v", len(requests), requests)
		}
	})

	t.Run("body_size_bound", func(t *testing.T) {
		s := fixtureSource(t, nil)
		setListing(s, strings.Repeat("x", (4<<20)+1), "/event_list/10/ar0313/eg0120/")
		if _, err := fixtureClient(t, s).Search(context.Background(), trip("2026-10-11", "2026-10-11")); err == nil || !strings.Contains(err.Error(), "4MiB") {
			t.Errorf("response body >4MiB not rejected clearly: %v", err)
		}
	})

	t.Run("caller_deadline_bounds_waiting_request", func(t *testing.T) {
		s := fixtureSource(t, nil)
		s.before = func(req *http.Request) { <-req.Context().Done() }
		ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
		defer cancel()
		start := time.Now()
		if _, err := fixtureClient(t, s).Search(ctx, trip("2026-10-11", "2026-10-11")); err == nil {
			t.Error("context deadline returned success")
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("caller deadline not respected: %s", elapsed)
		}
	})

	t.Run("warm_cache_fetches_no_network_and_records_freshness", func(t *testing.T) {
		e := eventFixture("ar0313e9999", "キャッシュ用体験", "2026-10-11", "2026-10-11", "2026年10月11日(日)")
		s := fixtureSource(t, []sourceEvent{e})
		setListing(s, listingFixture(t, []sourceEvent{e}, "", ""), "/event_list/10/ar0313/eg0120/")
		c := fixtureClient(t, s, func(o *walkerplus.Options) { o.NoCache = false; o.CacheDir = t.TempDir() })
		q := trip("2026-10-11", "2026-10-11")
		cold, err := c.Search(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		coldEvent := onlyEvent(t, cold)
		warm, err := c.Search(context.Background(), q)
		if err != nil {
			t.Fatal(err)
		}
		warmEvent := onlyEvent(t, warm)
		requests, _ := s.snapshot()
		if len(requests) != 1 || cold.Coverage.RequestCount != 1 || warm.Coverage.RequestCount != 0 || warm.Coverage.CacheHits != 1 {
			t.Errorf("cold/warm cache request accounting: requests=%v cold=%+v warm=%+v", requests, cold.Coverage, warm.Coverage)
		}
		if len(coldEvent.Sources) != 1 || len(warmEvent.Sources) != 1 {
			t.Fatalf("missing source freshness: cold=%+v warm=%+v", coldEvent.Sources, warmEvent.Sources)
		}
		if coldEvent.Sources[0].CacheHit || !warmEvent.Sources[0].CacheHit || coldEvent.Sources[0].FetchedAt != warmEvent.Sources[0].FetchedAt {
			t.Errorf("warm cache did not preserve original fetched_at: cold=%+v warm=%+v", coldEvent.Sources, warmEvent.Sources)
		}
	})
}

func boolPointer(value bool) *bool { return &value }

func assertBool(t *testing.T, field string, got, want *bool) {
	t.Helper()
	if want == nil {
		if got != nil {
			t.Errorf("%s must remain unknown/null, got %v", field, *got)
		}
		return
	}
	if got == nil || *got != *want {
		t.Errorf("%s = %v; want %v", field, got, *want)
	}
}
