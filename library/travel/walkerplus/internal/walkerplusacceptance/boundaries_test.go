package walkerplusacceptance_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/walkerplus/internal/walkerplus"
)

func TestOmittedYearOccurrenceListCrossesNewYear(t *testing.T) {
	e := eventFixture("ar0313e1010", "年越しの2回開催", "2026-12-31", "2027-01-01", "2026年12月31日(木)・1月1日(金)")
	s := fixtureSource(t, []sourceEvent{e})
	setListing(s, listingFixture(t, []sourceEvent{e}, "", ""), "/event_list/12/ar0313/eg0120/")
	r, err := fixtureClient(t, s).Shortlist(context.Background(), trip("2026-12-31", "2027-01-01"))
	if err != nil {
		t.Fatal(err)
	}
	got := onlyEvent(t, r)
	assertStrings(t, "source occurrence_dates", got.Schedule.OccurrenceDates, []string{"2026-12-31", "2027-01-01"})
	if got.Match == nil {
		t.Fatal("omitted-year positive has no match")
	}
	assertStrings(t, "confirmed_days", got.Match.ConfirmedDays, []string{"2026-12-31", "2027-01-01"})
	if got.EditionYear == nil || *got.EditionYear != 2026 {
		t.Errorf("source edition year was rolled forward: %+v", got)
	}
}

func TestUnsupportedClosureQualifiersPreventOverconfirmation(t *testing.T) {
	for i, qualifier := range []string{"不定休", "臨時休館あり", "変更あり"} {
		t.Run(qualifier, func(t *testing.T) {
			e := eventFixture(fmt.Sprintf("ar0313e%d", 1020+i), "不確実な開館日の展示", "2026-10-10", "2026-10-20", "2026年10月10日(土)～10月20日(火)")
			e.schedule = e.period + " 休館日：月曜。" + qualifier
			s := fixtureSource(t, []sourceEvent{e})
			setListing(s, listingFixture(t, []sourceEvent{e}, "", ""), "/event_list/10/ar0313/eg0120/")
			r, err := fixtureClient(t, s).Shortlist(context.Background(), trip("2026-10-11", "2026-10-11"))
			if err != nil {
				t.Fatal(err)
			}
			got := onlyEvent(t, r)
			if got.Match == nil {
				t.Fatal("uncertain positive has no match")
			}
			assertStrings(t, "confirmed_days", got.Match.ConfirmedDays, []string{})
			assertStrings(t, "possible_days", got.Match.PossibleDays, []string{"2026-10-11"})
			if len(got.Schedule.Unresolved) == 0 {
				t.Errorf("%q qualifier lost unresolved explanation: %+v", qualifier, got.Schedule)
			}
		})
	}
}

func TestCancellationEvidenceIsScopedAndConditional(t *testing.T) {
	tests := []struct {
		name, titlePrefix, periodSuffix, sidebar string
		canceled                                 bool
	}{
		{name: "related_event_cancellation", sidebar: `<aside class="related-events"><div class="m-detailheader__period">関連イベント：別の催しは開催中止となりました</div></aside>`},
		{name: "conditional_weather_period", periodSuffix: " 雨天の場合は開催中止"},
		{name: "current_title_cancellation", titlePrefix: "【開催中止】", canceled: true},
	}
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := eventFixture(fmt.Sprintf("ar0313e%d", 1030+i), tc.titlePrefix+"体験会", "2026-10-11", "2026-10-11", "2026年10月11日(日)"+tc.periodSuffix)
			s := fixtureSource(t, []sourceEvent{e})
			if tc.sidebar != "" {
				s.pages["/event/"+e.id+"/"] = strings.Replace(s.pages["/event/"+e.id+"/"], "</body>", tc.sidebar+"</body>", 1)
			}
			got, err := fixtureClient(t, s).Event(context.Background(), e.id)
			if err != nil {
				t.Fatal(err)
			}
			if (got.Cancellation != nil) != tc.canceled {
				t.Errorf("cancellation scope/condition violated: got=%v want canceled=%v", got.Cancellation, tc.canceled)
			}
			if tc.canceled {
				assertEvidence(t, got, "cancellation", "開催中止")
			}
		})
	}
}

func TestZeroPriceRequiresUnconditionalAdmissionEvidence(t *testing.T) {
	for i, tc := range []struct {
		name, price string
		free        bool
	}{
		{"unconditional_zero", "0円", true},
		{"child_only_zero", "小学生以下0円", false},
		{"parking_only_zero", "駐車場0円", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := eventFixture(fmt.Sprintf("ar0313e%d", 1040+i), tc.name, "2026-10-11", "2026-10-11", "2026年10月11日(日)")
			e.price = tc.price
			s := fixtureSource(t, []sourceEvent{e})
			got, err := fixtureClient(t, s).Event(context.Background(), e.id)
			if err != nil {
				t.Fatal(err)
			}
			if (got.Admission.Status == "free") != tc.free {
				t.Errorf("price %q classified %q; want free=%v", tc.price, got.Admission.Status, tc.free)
			}
			if tc.free && (got.Admission.Price == nil || *got.Admission.Price != 0) {
				t.Errorf("unconditional zero admission missing numeric zero: %+v", got.Admission)
			}
			assertEvidence(t, got, "admission", tc.price)
		})
	}
}

func TestSearchWrongCityAndCategoryDoNotLeak(t *testing.T) {
	e := eventFixture("ar0313e1050", "渋谷区の体験会", "2026-10-11", "2026-10-11", "2026年10月11日(日)")
	s := fixtureSource(t, []sourceEvent{e})
	body := listingFixture(t, []sourceEvent{e}, "", `<nav><a href="/event_list/ar0313103/minato/">港区</a><a href="/event_list/eg0109/">ライブ</a></nav>`)
	setListing(s, body, "/event_list/10/ar0313113/shibuya/eg0120/", "/event_list/10/ar0313103/minato/eg0120/", "/event_list/10/ar0313/eg0109/")
	c := fixtureClient(t, s)
	q := walkerplus.Query{City: "shibuya", Category: "activities", From: "2026-10-11", To: "2026-10-11", MaxPages: 1}
	positive, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if got := onlyEvent(t, positive); got.ID != e.id {
		t.Fatalf("positive city fixture mismatch: %+v", got)
	}
	q.City = "minato"
	wrongCity, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if wrongCity.Events == nil || len(wrongCity.Events) != 0 {
		t.Errorf("wrong-city query borrowed nav fact: %+v", wrongCity.Events)
	}
	q.City, q.Prefecture, q.Category = "", "tokyo", "music"
	wrongCategory, err := c.Search(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if wrongCategory.Events == nil || len(wrongCategory.Events) != 0 {
		t.Errorf("wrong-category query borrowed nav fact: %+v", wrongCategory.Events)
	}
}
