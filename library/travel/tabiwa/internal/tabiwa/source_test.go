package tabiwa

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/cliutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func str(s string) *string { return &s }
func TestQuoteUnitsAndUnknowns(t *testing.T) {
	cases := []struct {
		raw                       RawProduct
		unit, kind, amount, basis string
		old                       *string
	}{
		{RawProduct{Price: str("2,000P")}, "WESTER_POINT", "quoted", "2000", "unknown", nil},
		{RawProduct{Price: str("1,200円→ 1,000円")}, "JPY", "discounted_quote", "1000", "unknown", str("1200")},
		{RawProduct{Price: str("価格は日付により変動")}, "unknown", "variable_by_date", "", "unknown", nil},
		{RawProduct{Price: str("37,050円"), Overview: "※タクシー代金は1台あたりの金額となります"}, "JPY", "quoted", "37050", "per_vehicle", nil},
		{RawProduct{Price: str("1,000P→900円")}, "unknown", "unknown", "", "unknown", nil},
		{RawProduct{}, "unknown", "unknown", "", "unknown", nil},
	}
	for _, tc := range cases {
		p := quote(tc.raw)
		a := ""
		if p.Amount != nil {
			a = *p.Amount
		}
		if p.Unit != tc.unit || p.Kind != tc.kind || a != tc.amount || p.Basis != tc.basis {
			t.Fatalf("quote(%v)=%+v", tc.raw, p)
		}
		if tc.old != nil && (p.OriginalAmount == nil || *p.OriginalAmount != *tc.old) {
			t.Fatal("lost original quote")
		}
	}
}
func TestNormalizeRetainsContradictoryOverviewWithoutCoverageInference(t *testing.T) {
	r := RawProduct{ID: "J0000900", Name: "岡山・香川ワイドパス", Price: str("3,600円"), Areas: []Named{{"52", "小豆島・直島・豊島"}}, Overview: "※記載のない「直島」等は含まれません。\r\n※販売上限数に達した場合、終了します。"}
	p := Normalize(r, "10", "2026-10-28", Origin+"/ticketList/search", "2026-10-04T11:00:00Z")
	if p.Areas[0].Name != r.Areas[0].Name || !strings.Contains(p.Evidence[0].Text, "直島") || p.Availability != "unknown" || p.CoverageStatus != "unknown" || p.Status != "unknown" {
		t.Fatalf("unsafe normalized inference: %+v", p)
	}
	p = Normalize(RawProduct{ID: "J0000900", Name: "券", Overview: strings.Repeat("※", 2000)}, "10", "", "source", "clock")
	if !p.OverviewTruncated || !p.EvidenceTruncated {
		t.Fatal("truncation must be explicit")
	}
}
func TestPublicReadUsesOnlyDocumentedRegionPreference(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Cookie") != "regionId=20" || r.Header.Get("Authorization") != "" || r.URL.Query().Get("usage_date") != "2026-10-28" || len(r.URL.Query()["ticket_ids"]) != 2 {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		}
		json.NewEncoder(w).Encode(map[string]any{"response": []RawProduct{{ID: "J0000900", Name: "券", Price: str("3,600円")}}})
	}))
	defer s.Close()
	rows, _, _, err := New(s.URL, 2).Search(context.Background(), Query{Region: "20", Date: "2026-10-28", IDs: []string{"J0000900", "J0001900"}})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}
func TestSourceFailuresAreExplicitAndRedirectNeverFollowed(t *testing.T) {
	for _, mode := range []string{"queue", "rate", "html", "api_error", "bad_identity"} {
		t.Run(mode, func(t *testing.T) {
			followed := false
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/queue" {
					followed = true
				}
				switch mode {
				case "queue":
					http.Redirect(w, r, "/queue", 302)
				case "rate":
					w.WriteHeader(429)
				case "html":
					w.Write([]byte("<html>queue</html>"))
				case "api_error":
					w.Write([]byte(`{"response":{"errors":["error"]}}`))
				case "bad_identity":
					w.Write([]byte(`{"response":[{"name":"券"}]}`))
				}
			}))
			defer s.Close()
			rows, _, _, err := New(s.URL, 2).Search(context.Background(), Query{Region: "10"})
			if err == nil || rows != nil || followed {
				t.Fatalf("failure became facts or redirect followed: rows=%v err=%v follow=%v", rows, err, followed)
			}
			if mode == "rate" {
				var rate *cliutil.RateLimitError
				if !errors.As(err, &rate) {
					t.Fatal("429 must be typed")
				}
			}
		})
	}
}
func TestInvalidQueryDoesNotReachSource(t *testing.T) {
	c := New("http://127.0.0.1:1", 2)
	for _, q := range []Query{{Region: "11"}, {Region: "10", Date: "2026-02-30"}, {Region: "10", IDs: []string{"../secret"}}, {Region: "10", Category: "unknown"}} {
		if _, _, _, err := c.Search(context.Background(), q); err == nil {
			t.Fatalf("accepted %+v", q)
		}
	}
}
