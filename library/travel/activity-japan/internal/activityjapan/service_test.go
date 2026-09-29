package activityjapan

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/config"
)

func TestDateTokyoBoundary(t *testing.T) {
	today := time.Now().In(tokyo).Format("2006-01-02")
	if e := ValidateDate(today); e != nil {
		t.Fatalf("today in Tokyo was rejected: %v", e)
	}
	if e := ValidateDate("2026-02-30"); e == nil {
		t.Fatal("invalid calendar date accepted")
	}
	yesterday := time.Now().In(tokyo).AddDate(0, 0, -1).Format("2006-01-02")
	if e := ValidateDate(yesterday); e == nil {
		t.Fatal("past date accepted")
	}
}
func TestPriceBasisAndSubtotal(t *testing.T) {
	cases := []struct {
		suffix, basis string
		size          int
	}{{"人", "per_person", 0}, {"person", "per_person", 0}, {"ペア", "per_group", 2}, {"pair", "per_group", 2}, {"Set", "per_group", 0}, {"Adult (single rider preferred)", "unknown", 0}}
	for _, tc := range cases {
		basis, size := classifyBasis(tc.suffix)
		if basis != tc.basis {
			t.Errorf("%q basis=%s", tc.suffix, basis)
		}
		if tc.size != 0 && (size == nil || *size != tc.size) {
			t.Errorf("%q group size=%v", tc.suffix, size)
		}
	}
	p := 1800
	if n := Subtotal(&p, "per_person", nil, 2); n == nil || *n != 3600 {
		t.Fatalf("per-person subtotal %v", n)
	}
	p = 4800
	size := 2
	if n := Subtotal(&p, "per_group", &size, 2); n == nil || *n != 4800 {
		t.Fatalf("pair subtotal %v", n)
	}
	if n := Subtotal(&p, "per_group", &size, 4); n != nil {
		t.Fatalf("unproven two-pair total %v", n)
	}
	if n := Subtotal(&p, "unknown", nil, 2); n != nil {
		t.Fatalf("unknown basis total %v", n)
	}
}
func TestAvailabilityInterpretation(t *testing.T) {
	if Status("1") != "instant_confirmable" || Status("3") != "reservation_request" || Status("2") != "unknown" {
		t.Fatal("source status conflated")
	}
	if CheckAvailability("1", "3") != "reservation_request" {
		t.Fatal("instant-to-request not reflected")
	}
	if CheckAvailability("3", "2") != "instant_confirmable" {
		t.Fatal("request-to-instant not reflected")
	}
	if CheckAvailability("1", "5") != "sold_out_for_party" {
		t.Fatal("insufficient party stock not reflected")
	}
}
func TestDetailIdentityLanguageAndPartial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang_flag")
		if lang == "ja" {
			http.Error(w, "denied", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plan_data":{"plan_id":62375,"partner_id":9326,"plan_name":"English title","support_language":true,"age_start":2,"age_end":100,"people_max":0},"price_items":[{"plan_price_id":176511,"price_prefix":"Standard","price_suffix":"Adult","base_price":2000,"discount_price":200}]}`))
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 5*time.Second, 2)
	c.NoCache = true
	c.HTTPClient = srv.Client()
	s := NewService(c)
	p, e := s.Detail(context.Background(), "62375", "en")
	if e != nil {
		t.Fatal(e)
	}
	if p.PlanID != "62375" || p.OperatorID == nil || *p.OperatorID != "9326" {
		t.Fatalf("identity %+v", p)
	}
	if p.NameOriginalJA != nil || !p.Partial {
		t.Fatalf("failed Japanese request became a Japanese name: %+v", p)
	}
	if p.CanonicalURL != nil || p.SourceURL != PlanURL("62375", "en") {
		t.Fatalf("unindexed URL called canonical: %+v", p)
	}
	if p.PartyMax != nil || p.PartyMaxSource == nil || *p.PartyMaxSource != 0 {
		t.Fatal("source zero became a literal party maximum")
	}
	if p.SpokenLanguages != nil {
		t.Fatal("website support flag became spoken language")
	}
	if p.LocaleSupportFlag == nil || !*p.LocaleSupportFlag {
		t.Fatal("locale flag was lost")
	}
	if len(p.Options) != 1 || p.Options[0].Basis != "unknown" || p.Options[0].AgeClass != "unknown" || p.Options[0].NameOriginalJA != nil {
		t.Fatalf("option evidence was invented: %+v", p.Options)
	}
}

func TestTranslatedAdultLabelDoesNotChangeOriginalAgeClass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/plan/get_plan_price" {
			_, _ = w.Write([]byte(`{"planPriceList":{"1":[{"price_id":176511,"price":1800,"price_prefix":"スタンダードプラン","price_suffix":"人"}]}}`))
			return
		}
		name, suffix := "English plan", "Adult (single rider preferred)"
		if r.URL.Query().Get("lang_flag") == "ja" {
			name, suffix = "日本語プラン", "人"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"plan_data": map[string]any{"plan_id": 62375, "plan_name": name}, "price_items": []map[string]any{{"plan_price_id": 176511, "price_prefix": "スタンダードプラン", "price_suffix": suffix}}})
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
	c.NoCache = true
	c.HTTPClient = srv.Client()
	s := NewService(c)
	d := time.Now().In(tokyo).AddDate(0, 0, 7).Format("2006-01-02")
	for _, lang := range []string{"en", "ja"} {
		plan, err := s.Detail(context.Background(), "62375", lang)
		if err != nil {
			t.Fatal(err)
		}
		quotes, err := s.Price(context.Background(), "62375", d, 2, plan.Options)
		if err != nil {
			t.Fatal(err)
		}
		if len(quotes) != 1 || quotes[0].AgeClass != "unknown" || quotes[0].DerivedSubtotalJPY != nil {
			t.Fatalf("%s: translated age assertion leaked: %+v", lang, quotes)
		}
	}
}

func TestOptionUnitBoundsSuppressSubtotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"planPriceList":{"1":[{"price_id":1,"price":9000,"price_prefix":"大人","price_suffix":"人","price_item_min":3,"price_item_max":5}]}}`))
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
	c.NoCache = true
	c.HTTPClient = srv.Client()
	d := time.Now().In(tokyo).AddDate(0, 0, 7).Format("2006-01-02")
	s := NewService(c)
	for _, tc := range []struct {
		people       int
		wantSubtotal bool
	}{{2, false}, {3, true}, {6, false}} {
		quotes, err := s.Price(context.Background(), "62061", d, tc.people, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(quotes) != 1 || (quotes[0].DerivedSubtotalJPY != nil) != tc.wantSubtotal {
			t.Fatalf("people=%d quote=%+v", tc.people, quotes)
		}
	}
}

func TestMalformedSourceItemsCannotLookEmptyOrComplete(t *testing.T) {
	for _, tc := range []struct {
		body string
		kind string
	}{
		{`{"planPriceList":{"1":[{"price":9000}]}}`, "price"},
		{`{"course_name":{"bad-time":123},"course_status":{"123":1}}`, "sessions"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
		c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
		c.NoCache = true
		c.HTTPClient = srv.Client()
		s := NewService(c)
		d := time.Now().In(tokyo).AddDate(0, 0, 7).Format("2006-01-02")
		var err error
		if tc.kind == "price" {
			_, err = s.Price(context.Background(), "62061", d, 2, nil)
		} else {
			_, err = s.Sessions(context.Background(), "62061", d)
		}
		var schema *SchemaError
		if !errors.As(err, &schema) {
			t.Fatalf("%s: expected typed schema error, got %v", tc.kind, err)
		}
		srv.Close()
	}
}

func TestMixedMalformedPriceItemsArePartial(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"planPriceList":{"1":[{"price":9000},{"price_id":1,"price":9000,"price_prefix":"大人","price_suffix":"人"}]}}`))
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
	c.NoCache = true
	c.HTTPClient = srv.Client()
	d := time.Now().In(tokyo).AddDate(0, 0, 7).Format("2006-01-02")
	quotes, err := NewService(c).Price(context.Background(), "62061", d, 2, nil)
	var partial *PartialItemsError
	if len(quotes) != 1 || !errors.As(err, &partial) || partial.Count != 1 {
		t.Fatalf("quotes=%+v err=%v", quotes, err)
	}
}

func TestMalformedDetailOptionsAreErrorOrPartial(t *testing.T) {
	for _, tc := range []struct {
		body        string
		wantOptions int
		wantSchema  bool
	}{
		{`{"plan_data":{"plan_id":62061},"price_items":[{"price_prefix":"大人"}]}`, 0, true},
		{`{"plan_data":{"plan_id":62061},"price_items":[{"price_prefix":"大人"},{"plan_price_id":175524,"price_prefix":"大人","price_suffix":"人"}]}`, 1, false},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
		c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
		c.NoCache = true
		c.HTTPClient = srv.Client()
		plan, err := NewService(c).Detail(context.Background(), "62061", "ja")
		if tc.wantSchema {
			var schema *SchemaError
			if !errors.As(err, &schema) {
				t.Fatalf("expected schema error, got %v", err)
			}
		} else if err != nil || len(plan.Options) != tc.wantOptions || !plan.Partial || len(plan.Errors) == 0 {
			t.Fatalf("mixed detail: plan=%+v err=%v", plan, err)
		}
		srv.Close()
	}
}
func TestDetailRejectsWrongPlanAndHTML(t *testing.T) {
	for _, body := range []string{`{"plan_data":{"plan_id":42}}`, `<html>verification</html>`} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
		c.NoCache = true
		c.HTTPClient = srv.Client()
		_, e := NewService(c).Detail(context.Background(), "62375", "ja")
		srv.Close()
		if e == nil {
			t.Fatalf("invalid source body accepted: %s", body)
		}
	}
}
func TestSessionsPreserveCourseIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"course_name": map[string]any{"09:00": 189637, "18:30": 205944}, "course_status": map[string]any{"189637": 1, "205944": 5}})
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
	c.NoCache = true
	c.HTTPClient = srv.Client()
	d := time.Now().In(tokyo).AddDate(0, 0, 7).Format("2006-01-02")
	sessions, e := NewService(c).Sessions(context.Background(), "62375", d)
	if e != nil {
		t.Fatal(e)
	}
	if len(sessions) != 2 || sessions[0].SessionID != "189637" || sessions[1].Availability != "not_accepted" || !strings.HasSuffix(sessions[0].StartLocal, "+09:00") {
		t.Fatalf("sessions %+v", sessions)
	}
}

func TestAdultQuoteDoesNotUseChildOrInfantPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"planPriceList":{"1":[{"price_id":1,"price":9000,"base_price":9000,"price_prefix":"大人（13歳～）","price_suffix":"人"}],"2":[{"price_id":2,"price":7000,"base_price":7000,"price_prefix":"小人（4歳〜12歳）","price_suffix":"人"}],"3":[{"price_id":3,"price":0,"base_price":0,"price_prefix":"幼児（0歳〜3歳）","price_suffix":"人"}]}}`))
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
	c.NoCache = true
	c.HTTPClient = srv.Client()
	d := time.Now().In(tokyo).AddDate(0, 0, 7).Format("2006-01-02")
	quotes, e := NewService(c).Price(context.Background(), "62061", d, 2, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(quotes) != 3 || quotes[0].DerivedSubtotalJPY == nil || *quotes[0].DerivedSubtotalJPY != 18000 {
		t.Fatalf("adult quote %+v", quotes)
	}
	if quotes[1].DerivedSubtotalJPY != nil || quotes[2].DerivedSubtotalJPY != nil {
		t.Fatalf("child or infant quote applied to adults: %+v", quotes)
	}
	if quotes[1].AgeClass != "child" || quotes[2].AgeClass != "infant" {
		t.Fatalf("age labels lost: %+v", quotes)
	}
}

func TestGroupStockQuantityRequiresMoreEvidence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/get_plan_price_info" {
			t.Errorf("unexpected request to %s", r.URL.Path)
			http.Error(w, "unexpected", 500)
			return
		}
		_, _ = w.Write([]byte(`{"plan_data":{"plan_id":64974,"partner_id":9777,"people_min":1,"people_max":6,"basic_min_passenger_count":2},"price_items":[{"plan_price_id":184975,"price_prefix":"参加者","price_suffix":"ペア","base_price":4800,"discount_price":0}]}`))
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
	c.NoCache = true
	c.HTTPClient = srv.Client()
	d := time.Now().In(tokyo).AddDate(0, 0, 7).Format("2006-01-02")
	_, e := NewService(c).Check(context.Background(), "64974", d, "123", 2)
	if e == nil || !strings.Contains(e.Error(), "unverified") {
		t.Fatalf("group quantity was treated as participant count: %v", e)
	}
}

func TestHeadlineUsesPlanDataRatherThanFreeInfantOption(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"plan_data":{"plan_id":62061,"partner_id":8873,"base_price":9000,"discount_price":0},"price_items":[{"plan_price_id":175524,"price_prefix":"大人（13歳～）","price_suffix":"人","base_price":9000,"discount_price":0},{"plan_price_id":175526,"price_prefix":"幼児（0歳〜3歳）","price_suffix":"人","base_price":0,"discount_price":0}]}`))
	}))
	defer srv.Close()
	c := client.New(&config.Config{BaseURL: srv.URL}, 2*time.Second, 2)
	c.NoCache = true
	c.HTTPClient = srv.Client()
	plan, e := NewService(c).Detail(context.Background(), "62061", "ja")
	if e != nil {
		t.Fatal(e)
	}
	if plan.DerivedHeadlineFromJPY == nil || *plan.DerivedHeadlineFromJPY != 9000 {
		t.Fatalf("headline %+v", plan.DerivedHeadlineFromJPY)
	}
	if len(plan.Options) != 2 || plan.Options[1].UndatedUnitJPY == nil || *plan.Options[1].UndatedUnitJPY != 0 {
		t.Fatalf("option unit %+v", plan.Options)
	}
}

func TestParseExactDurationDoesNotConflateActivityTime(t *testing.T) {
	cases := []struct {
		source  string
		minutes *int
	}{
		{"60 minutes", intRef(60)},
		{"2 hours", intRef(120)},
		{"約90分", intRef(90)},
		{"From meeting to disbanding, it takes about 2 hours. The kayak ride takes about 1 hour.", nil},
		{"90 inches.", nil},
		{"1～2 hours", nil},
	}
	for _, tc := range cases {
		got := ParseExactDuration(&tc.source)
		if (got == nil) != (tc.minutes == nil) || (got != nil && *got != *tc.minutes) {
			t.Errorf("%q => %v, want %v", tc.source, got, tc.minutes)
		}
	}
}
func intRef(v int) *int { return &v }
