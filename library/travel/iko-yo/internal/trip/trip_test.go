package trip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/iko-yo/internal/cliutil"
)

var observed = time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name + ".html")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestReferenceBoundaries(t *testing.T) {
	for _, tc := range []struct {
		value string
		ok    bool
	}{{"spots/8220", true}, {BaseURL + "/events/8412", true}, {"events/0", false}, {"spots/9999999999", false}, {"https://example.com/spots/8220", false}, {"events/8412?token=x", false}, {"../spots/8220", false}} {
		_, _, e := ParseReference(tc.value)
		if (e == nil) != tc.ok {
			t.Errorf("%q error %v", tc.value, e)
		}
	}
}
func TestPublishedSchedulesDoNotInventCalendarDays(t *testing.T) {
	for _, tc := range []struct{ raw, start, end, precision, status string }{{"2026年11月15日", "2026-11-15", "2026-11-15", "single_date", "upcoming"}, {"2026年9月1日 〜 2026年10月31日", "2026-09-01", "2026-10-31", "published_span", "published_span_current"}, {"2026年7月20日（月祝）、8月9日（日）、9月6日", "", "", "unknown", "unknown"}, {"毎年5月上旬", "", "", "unknown", "unknown"}, {"2026年2月30日", "", "", "unknown", "unknown"}, {"2026年12月28日〜1月5日", "", "", "unknown", "unknown"}, {"2026年9月26日", "2026-09-26", "2026-09-26", "single_date", "ended"}} {
		s := parseSchedule(tc.raw, observed)
		if s.Start != tc.start || s.End != tc.end || s.Precision != tc.precision || s.Status != tc.status {
			t.Errorf("%s => %+v", tc.raw, s)
		}
	}
}
func TestObservedDetailFactsAndUnknowns(t *testing.T) {
	for _, tc := range []struct{ ref, fixture string }{{"spots/8220", "spots-8220"}, {"events/8412", "events-8412"}} {
		r, e := ParseDetail(fixture(t, tc.fixture), tc.ref, observed)
		if e != nil {
			t.Fatal(e)
		}
		if r.Name == "" || !r.Detail || r.SourceURL != BaseURL+"/"+tc.ref {
			t.Fatalf("missing source facts: %+v", r)
		}
		if tc.ref == "spots/8220" {
			if r.Fees.Child != "子供:300円" || !strings.Contains(r.Fees.Adult, "本場入場料100円") || r.Fees.Currency != "JPY" {
				t.Fatalf("lost fee qualifiers %+v", r.Fees)
			}
			if r.Age.MinMonths == nil || *r.Age.MinMonths != 6 || *r.Age.MaxExclusiveMonths != 156 {
				t.Fatalf("age %+v", r.Age)
			}
			if r.Amenities["nursing"].Status != "reported_present" || r.Amenities["changing"].Status != "reported_present" || r.Amenities["stroller"].Status != "unknown" {
				t.Fatalf("amenities %+v", r.Amenities)
			}
		} else {
			if r.Schedule.Start != "2026-11-15" || r.Booking.ApplicationStart != "2026-09-01" || r.Booking.ApplicationEnd != "2026-10-16" || r.Booking.CapacityPeople == nil || *r.Booking.CapacityPeople != 30 || r.Booking.Availability != "unknown" {
				t.Fatalf("event/booking %+v %+v", r.Schedule, r.Booking)
			}
		}
	}
}
func TestListingExtractsStableReferences(t *testing.T) {
	for _, kind := range []string{"spots", "events"} {
		p, e := ParseListing(fixture(t, kind), kind, BaseURL+"/"+kind, observed)
		if e != nil || len(p.Records) != 2 {
			t.Fatalf("%s %d %v", kind, len(p.Records), e)
		}
		for _, r := range p.Records {
			if r.Kind != kind || r.Name == "" || r.Location == "" || r.Detail {
				t.Fatalf("listing %+v", r)
			}
		}
	}
	if _, e := ParseListing([]byte("<h1>403 Forbidden</h1>"), "events", BaseURL+"/events", observed); e == nil {
		t.Fatal("challenge treated as empty source")
	}
}
func TestExplicitNegativeConditionalAndOffsiteAmenities(t *testing.T) {
	for _, tc := range []struct{ text, status string }{{"授乳室はありません", "reported_absent"}, {"授乳室やおむつ替えスペースもあります", "reported_present"}, {"近隣の別施設に授乳室があります", "mentioned"}, {"授乳室を設置する予定です", "mentioned"}, {"授乳室についてお問い合わせください", "mentioned"}} {
		r := blankRecord("spots", "1", observed)
		absorbSentence(&r, tc.text, "test")
		if r.Amenities["nursing"].Status != tc.status {
			t.Errorf("%q => %+v", tc.text, r.Amenities["nursing"])
		}
	}
	r := blankRecord("spots", "1", observed)
	absorbSentence(&r, "対象年齢は6カ月〜12歳未満です", "test")
	if r.Age.MaxExclusiveMonths == nil || *r.Age.MaxExclusiveMonths != 144 {
		t.Fatalf("exclusive upper age %+v", r.Age)
	}
}
func TestComparisonAgeDatesAndApplicationRemainQualified(t *testing.T) {
	r, e := ParseDetail(fixture(t, "spots-8220"), "spots/8220", observed)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		age   int
		state string
	}{{4, "excluded"}, {24, "supported"}, {155, "supported"}, {156, "excluded"}} {
		v := Evaluate(r, "", "", tc.age, nil)
		if v.Age.Status != tc.state {
			t.Errorf("age %d %+v", tc.age, v)
		}
	}
	unknown := Evaluate(r, "2026-11-15", "", 24, []string{"stroller"})
	if unknown.Overall != "unknown" || unknown.Schedule.Status != "unknown" {
		t.Fatalf("spot dated opening inferred %+v", unknown)
	}
	event, e := ParseDetail(fixture(t, "events-8412"), "events/8412", observed)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ asOf, status string }{{"2026-08-31", "before_published_window"}, {"2026-09-01", "within_published_window"}, {"2026-10-16", "within_published_window"}, {"2026-10-17", "after_published_window"}} {
		v := Evaluate(event, "2026-11-15", tc.asOf, -1, nil)
		if v.Application.Status != tc.status || v.Schedule.Status != "supported" {
			t.Fatalf("%s %+v", tc.asOf, v)
		}
	}
	event.Schedule = parseSchedule("2026年11月1日〜2026年11月30日", observed)
	if Evaluate(event, "2026-11-15", "", -1, nil).Schedule.Status != "unknown" {
		t.Fatal("span inferred daily operation")
	}
}
func TestLocalFilterCapsAndNegativeSearch(t *testing.T) {
	p, e := ParseListing(fixture(t, "events"), "events", BaseURL+"/events", observed)
	if e != nil {
		t.Fatal(e)
	}
	q := Query{Kind: "events", AgeMonths: -1, Limit: 1}
	if e := ValidateQuery(q); e != nil {
		t.Fatal(e)
	}
	v := Filter(p.Records, q)
	if v.MatchedRecords != 2 || len(v.Records) != 1 || v.OmittedMatches != 1 {
		t.Fatalf("caps %+v", v)
	}
	q.Keyword = "this-term-does-not-exist-6932"
	v = Filter(p.Records, q)
	if len(v.Records) != 0 || v.Note == "" || v.ScannedRecords != 2 {
		t.Fatalf("irrelevant/empty result %+v", v)
	}
	q.Keyword = ""
	q.From = "2028-01-01"
	q.To = "2028-01-02"
	v = Filter(p.Records, q)
	if len(v.Records) != 0 || v.ExcludedRecords != 2 {
		t.Fatalf("date mismatch %+v", v)
	}
	for _, bad := range []Query{{Limit: 0, AgeMonths: -1}, {Limit: 1, AgeMonths: -1, From: "2026-02-30"}, {Limit: 1, AgeMonths: -1, Amenities: []string{"guaranteed-safe"}}} {
		if ValidateQuery(bad) == nil {
			t.Fatal("bad query accepted")
		}
	}
	if !ValidDate("2026-10-03") || ValidDate("2026-1-1") {
		t.Fatal("ISO date validation")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHTTPBoundsStatusAndSourceContract(t *testing.T) {
	for _, tc := range []struct {
		status     int
		body, ct   string
		rate, fail bool
	}{{200, string(fixture(t, "spots-8220")), "text/html", false, false}, {429, "limited", "text/html", true, true}, {404, "missing", "text/html", false, true}, {200, "{}", "application/json", false, true}, {200, strings.Repeat("x", MaxBody+1), "text/html", false, true}} {
		c := New(1)
		c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: http.Header{"Content-Type": []string{tc.ct}}, Request: req}, nil
		})
		_, e := c.Inspect(context.Background(), "spots/8220")
		if (e != nil) != tc.fail {
			t.Fatalf("status %d error %v", tc.status, e)
		}
		var rate *cliutil.RateLimitError
		if errors.As(e, &rate) != tc.rate {
			t.Fatalf("rate error %v", e)
		}
	}
	c := New(1)
	req, _ := http.NewRequest("GET", "https://evil.example/", nil)
	if c.http.CheckRedirect(req, nil) == nil {
		t.Fatal("cross-host redirect permitted")
	}
}

func TestRefreshStatusKeepsObservation(t *testing.T) {
	r, e := ParseDetail(fixture(t, "events-8412"), "events/8412", observed)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct{ asOf, want string }{{"2026-10-03", "upcoming"}, {"2026-11-15", "published_span_current"}, {"2026-11-16", "ended"}} {
		got := RefreshStatus(r, tc.asOf)
		if got.Schedule.Status != tc.want || got.ObservedAt != r.ObservedAt {
			t.Fatalf("%s: %+v", tc.asOf, got.Schedule)
		}
	}
}
func TestEvidenceBudgetNeverClaimsWithoutExcerpt(t *testing.T) {
	r := blankRecord("spots", "1", observed)
	for i := 0; i < 8; i++ {
		addEvidence(&r, "paragraph", fmt.Sprintf("bounded statement %d", i))
	}
	absorbSentence(&r, "授乳室があります。対象年齢は6カ月～12歳。", "paragraph")
	if r.Amenities["nursing"].Status != "unknown" || r.Age.Status != "unknown" {
		t.Fatalf("unsupported assertion: %+v", r)
	}
}

func TestAmenityClauseAttribution(t *testing.T) {
	for _, tc := range []struct{ text, field, want string }{
		{"授乳室はありますが、ベビーカーの持ち込みは不可です", "nursing", "reported_present"},
		{"ベビーカーでの入場はできませんが、授乳室はあります", "stroller", "reported_absent"},
		{"授乳室はありますが、おむつ替えスペースはありません", "nursing", "reported_present"},
		{"授乳室はなく、おむつ替えスペースがあります", "nursing", "reported_absent"},
		{"授乳室やおむつ替えスペースもあります", "nursing", "reported_present"},
		{"授乳室やおむつ替えスペースもあります", "changing", "reported_present"},
	} {
		r := blankRecord("spots", "1", observed)
		absorbSentence(&r, tc.text, "paragraph")
		if r.Amenities[tc.field].Status != tc.want {
			t.Errorf("%s: %s", tc.text, r.Amenities[tc.field].Status)
		}
	}
}

func TestNegativeAndSpeculativeEvidence(t *testing.T) {
	for _, tc := range []struct{ text, field, want string }{
		{"授乳室を設置していません", "nursing", "reported_absent"},
		{"授乳室の設置を検討しています", "nursing", "mentioned"},
		{"ベビーカーには対応しておりません", "stroller", "reported_absent"},
	} {
		r := blankRecord("spots", "1", observed)
		absorbSentence(&r, tc.text, "paragraph")
		if r.Amenities[tc.field].Status != tc.want {
			t.Errorf("%s: %s", tc.text, r.Amenities[tc.field].Status)
		}
	}
	for _, text := range []string{"対象年齢は6カ月〜12歳ではありません", "対象年齢は6カ月〜12歳ではなく、別の対象です", "対象年齢は6カ月〜12歳とは限りません"} {
		r := blankRecord("spots", "1", observed)
		absorbSentence(&r, text, "paragraph")
		if r.Age.Status != "unknown" {
			t.Errorf("negated age claimed: %s", text)
		}
	}
}
