package cli

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/ikyu/internal/ikyu"
	"strings"
	"sync"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }
func comparableFixture() ikyu.OfferResult {
	s := ikyu.Stay{CheckIn: "2026-10-18", CheckOut: "2026-10-19", Adults: 2, Rooms: 1}
	return ikyu.OfferResult{Data: ikyu.OfferData{Property: ikyu.Property{ID: "00002889", Name: "旅館", Notes: ptr("宿泊税・入湯税は別途。")}, Room: ikyu.Room{ID: "10193741", Name: "客室"}, Plan: ikyu.Plan{ID: "11055986", Name: "素泊まり", PointVariation: ptr(0), Meal: ikyu.Meal{Code: "000", Name: "食事なし"}, MealDetailsKnown: true, Payment: ptr("CARD_COMBI"), Content: ptr("共通の公開条件"), UseCheckInOut: ptr(true), CheckInFrom: ptr("14:00"), CheckInTo: ptr("23:00"), CheckOut: ptr("11:00"), Cancellation: ikyu.Cancellation{Known: true, ID: "001", Rules: []ikyu.CancellationRule{{Type: "CancelPolicyRuleNoShow", Rate: ptr(100)}, {Type: "CancelPolicyRuleDay", Day: ptr(0), Rate: ptr(100)}}}}, Offer: ikyu.DatedOffer{Stay: s, EchoStay: &s, Available: ptr(true), DateVerified: true, Price: &ikyu.Price{Currency: "JPY", Unit: "booking_total", Amount: ptr(int64(30800)), PointRate: ptr(20.0), InstantPointRate: ptr(20.0), EligibilityKnown: true}}}}
}
func copyOffer(o ikyu.OfferResult) ikyu.OfferResult {
	b, _ := json.Marshal(o)
	var out ikyu.OfferResult
	_ = json.Unmarshal(b, &out)
	return out
}
func TestKnownOfferEquivalenceAndUnknowns(t *testing.T) {
	for _, tc := range []struct {
		name       string
		alter      func(*ikyu.OfferResult)
		equivalent bool
	}{{"same known room terms", func(o *ikyu.OfferResult) { o.Data.Plan.ID = "11055981"; o.Data.Offer.Price.Amount = ptr(int64(30000)) }, true}, {"different room", func(o *ikyu.OfferResult) { o.Data.Room.ID = "10193727" }, false}, {"different meal code", func(o *ikyu.OfferResult) { o.Data.Plan.Meal.Code = "001" }, false}, {"different cancellation", func(o *ikyu.OfferResult) { o.Data.Plan.Cancellation.Rules[1].Rate = ptr(80) }, false}, {"different date", func(o *ikyu.OfferResult) {
		o.Data.Offer.Stay.CheckIn = "2026-10-19"
		o.Data.Offer.Stay.CheckOut = "2026-10-20"
		s := o.Data.Offer.Stay
		o.Data.Offer.EchoStay = &s
	}, false}, {"unknown eligibility", func(o *ikyu.OfferResult) { o.Data.Offer.Price.EligibilityKnown = false }, false}, {"two missing content terms", func(o *ikyu.OfferResult) { o.Data.Plan.Content = nil }, false}, {"different point variant", func(o *ikyu.OfferResult) { o.Data.Plan.PointVariation = ptr(1) }, false}, {"stale observation", func(o *ikyu.OfferResult) { o.Freshness.Stale = true }, false}, {"different checkin limits", func(o *ikyu.OfferResult) { o.Data.Plan.CheckInTo = ptr("18:00") }, false}} {
		t.Run(tc.name, func(t *testing.T) {
			a := comparableFixture()
			b := copyOffer(a)
			tc.alter(&b)
			if tc.name == "two missing content terms" {
				a.Data.Plan.Content = nil
			}
			assessment := assessOffers(a, b)
			if assessment.Equivalent != tc.equivalent {
				t.Fatalf("assessment %+v", assessment)
			}
			if !tc.equivalent && assessment.SourceAmountDifference != nil {
				t.Fatal("incompatible/unknown savings fabricated")
			}
			if tc.equivalent && (assessment.SourceAmountDifference == nil || *assessment.SourceAmountDifference != -800) {
				t.Fatalf("source amount delta %+v", assessment)
			}
		})
	}
}
func TestSameMealCodeDifferentMealInclusions(t *testing.T) {
	a := comparableFixture()
	a.Data.Plan.Meal = ikyu.Meal{Code: "003", Name: "夕朝食付"}
	a.Data.Plan.MealDetails = []ikyu.MealDetail{{Type: ptr("DINNER"), Name: ptr("夕食"), Place: ptr("個室"), Menu: ptr("和食コース")}}
	b := copyOffer(a)
	b.Data.Plan.MealDetails[0].Menu = ptr("洋食コース")
	assessment := assessOffers(a, b)
	if assessment.Equivalent || !strings.Contains(strings.Join(assessment.Differences, ","), "meal inclusions") {
		t.Fatalf("same-code meal conflict ignored %+v", assessment)
	}
	b = copyOffer(a)
	b.Data.Plan.MealDetailsKnown = false
	b.Data.Plan.MealDetails = nil
	if assessment = assessOffers(a, b); assessment.Equivalent || len(assessment.Unknowns) == 0 {
		t.Fatalf("missing inclusions assumed equal %+v", assessment)
	}
}

type workflowReader struct {
	mockStayReader
	mu                       sync.Mutex
	active, maxActive, total int
	behavior                 func(ikyu.OfferRequest) (ikyu.OfferResult, error)
}

func (r *workflowReader) Offer(ctx context.Context, q ikyu.OfferRequest) (ikyu.OfferResult, error) {
	r.mu.Lock()
	r.active++
	r.total++
	if r.active > r.maxActive {
		r.maxActive = r.active
	}
	r.mu.Unlock()
	defer func() { r.mu.Lock(); r.active--; r.mu.Unlock() }()
	select {
	case <-time.After(15 * time.Millisecond):
	case <-ctx.Done():
		return ikyu.OfferResult{}, ctx.Err()
	}
	if r.behavior != nil {
		return r.behavior(q)
	}
	out := comparableFixture()
	out.Data.Property.ID = q.PropertyID
	out.Data.Room.ID = q.RoomID
	out.Data.Plan.ID = q.PlanID
	out.Data.Offer.Stay = q.Stay
	s := q.Stay
	out.Data.Offer.EchoStay = &s
	return out, nil
}
func (r *workflowReader) Stats() ikyu.Stats {
	r.mu.Lock()
	defer r.mu.Unlock()
	return ikyu.Stats{Requests: r.total}
}
func TestCompareRetainsPartialAndAllFailedRows(t *testing.T) {
	for _, all := range []bool{false, true} {
		reader := &workflowReader{}
		reader.behavior = func(q ikyu.OfferRequest) (ikyu.OfferResult, error) {
			if all || q.PlanID == "11055981" {
				return ikyu.OfferResult{}, errors.New("source read failed")
			}
			o := comparableFixture()
			o.Data.Offer.Stay = q.Stay
			o.Data.Offer.EchoStay = &q.Stay
			return o, nil
		}
		result, _, e := runStay(t, reader, "stay", "compare", "--offers", "00002889:10193741:11055986,00002889:10193741:11055981", "--check-in", "2026-10-18", "--check-out", "2026-10-19")
		if (e != nil) != all {
			t.Fatalf("all=%v err=%v", all, e)
		}
		rows := result["data"].([]any)
		if len(rows) != 2 || result["partial"] != true || len(result["fetch_failures"].([]any)) == 0 {
			t.Fatalf("partial rows lost %v", result)
		}
		if !all && rows[0].(map[string]any)["data"] == nil {
			t.Fatal("successful observation discarded")
		}
		if rows[1].(map[string]any)["data"] != nil {
			t.Fatal("phantom failed offer")
		}
		if reader.maxActive > 2 {
			t.Fatalf("concurrency %d", reader.maxActive)
		}
	}
}
func TestDatesRetainsEveryDateSoldOutAndError(t *testing.T) {
	reader := &workflowReader{}
	reader.behavior = func(q ikyu.OfferRequest) (ikyu.OfferResult, error) {
		if q.Stay.CheckIn == "2026-10-20" {
			return ikyu.OfferResult{}, errors.New("temporarily unavailable")
		}
		o := comparableFixture()
		o.Data.Offer.Stay = q.Stay
		o.Data.Offer.EchoStay = &q.Stay
		if q.Stay.CheckIn == "2026-10-19" {
			o.Data.Offer.Available = ptr(false)
			o.Data.Offer.Price = nil
			o.Data.Offer.UnavailableReason = "空室がありません"
		}
		return o, nil
	}
	result, _, e := runStay(t, reader, "stay", "dates", "00002889", "10193741", "11055986", "--check-ins", "2026-10-18,2026-10-19,2026-10-20", "--nights", "2")
	if e != nil {
		t.Fatal(e)
	}
	rows := result["data"].([]any)
	if len(rows) != 3 || result["requested"] != float64(3) || result["partial"] != true {
		t.Fatalf("date rows %v", result)
	}
	for i, row := range rows {
		expected := []string{"2026-10-18", "2026-10-19", "2026-10-20"}[i]
		if row.(map[string]any)["selector"] != expected {
			t.Fatal("date order changed")
		}
	}
	sold := rows[1].(map[string]any)["data"].(map[string]any)["offer"].(map[string]any)
	if sold["available"] != false || sold["price"] != nil {
		t.Fatal("sold-out offer fabricated")
	}
	if rows[2].(map[string]any)["error"] == nil {
		t.Fatal("failure dropped")
	}
	if reader.maxActive > 2 || reader.total != 3 {
		t.Fatalf("fanout bounds %d/%d", reader.maxActive, reader.total)
	}
}
func TestWorkflowInputCapsAndDryRuns(t *testing.T) {
	for _, args := range [][]string{{"stay", "compare", "--offers", "00002889:10193741:11055986,00002889:10193741:11055986", "--check-in", "2026-10-18", "--check-out", "2026-10-19"}, {"stay", "dates", "00002889", "10193741", "11055986", "--check-ins", "2026-10-18,2026-10-18"}} {
		reader := &workflowReader{}
		_, _, e := runStay(t, reader, args...)
		if e == nil || reader.total != 0 {
			t.Fatalf("bad workflow input %v %v", args, e)
		}
	}
	for _, leaf := range []string{"compare", "dates"} {
		reader := &workflowReader{}
		result, _, e := runStay(t, reader, "stay", leaf, "--dry-run")
		if e != nil || reader.total != 0 || result["network_requests"] != float64(0) {
			t.Fatalf("dry run %s %v %v", leaf, result, e)
		}
	}
}

func TestAllFailedRateLimitRetainsTypedExit(t *testing.T) {
	reader := &workflowReader{behavior: func(q ikyu.OfferRequest) (ikyu.OfferResult, error) {
		return ikyu.OfferResult{}, &cliutil.RateLimitError{URL: "https://www.ikyu.com/graphql", Body: strings.Repeat("x", 1000)}
	}}
	result, _, err := runStay(t, reader, "stay", "compare", "--offers", "00002889:10193741:11055986,00002889:10193741:11055981", "--check-in", "2026-10-18", "--check-out", "2026-10-19")
	if err == nil || ExitCode(err) != 7 || len(result["data"].([]any)) != 2 {
		t.Fatalf("typed rate error lost %v result=%v", err, result)
	}
	message := result["fetch_failures"].([]any)[0].(map[string]any)["error"].(string)
	if len([]rune(message)) > 513 {
		t.Fatal("unbounded diagnostic")
	}
}
