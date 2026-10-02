package smartex

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/smartex/internal/cliutil"
)

func liveFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/" + name + ".html")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestObservedFareParserAndDrift(t *testing.T) {
	data := liveFixture(t, "fare-reserved")
	f, _ := Resolve("Tokyo")
	dest, _ := Resolve("Shin-Osaka")
	q, e := ParseQuote(data, f, dest, "2026-10-02", "reserved")
	if e != nil || q.RegularJPY != 14720 || q.SmartEXJPY != 14520 || q.EXMemberJPY != 14230 || q.SeasonJapanese != "通常期" || len(q.TrainBasis) != 1 {
		t.Fatalf("quote%+v e%v", q, e)
	}
	for _, tt := range []struct{ date, class string }{{"2026-10-03", "reserved"}, {"2026-10-02", "green"}} {
		if _, e = ParseQuote(data, f, dest, tt.date, tt.class); e == nil {
			t.Fatal("wrong date/class accepted")
		}
	}
	bad := []byte(strings.ReplaceAll(string(data), "resultHyouka_t5", "missing_smart_fare"))
	if _, e = ParseQuote(bad, f, dest, "2026-10-02", "reserved"); e == nil {
		t.Fatal("source drift silently became zero fare")
	}
	if _, e = ParseQuote([]byte("<html><body>maintenance</body></html>"), f, dest, "2026-10-02", "reserved"); e == nil {
		t.Fatal("maintenance quote accepted")
	}
}

func TestReplayUsesEUCAndPreservesChildUnknowns(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		raw, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || !strings.Contains(string(raw), "%C5%EC%B5%FE") {
			t.Errorf("wrong replay encoding %s", raw)
		}
		f, _ := url.ParseQuery(string(raw))
		w.Header().Set("Content-Type", "text/html; charset=EUC-JP")
		if f.Get("INES") == "" {
			w.Write(liveFixture(t, "fare-step1"))
		} else {
			if f.Get("SEAT") != "0" {
				t.Error("reserved class wire mapping wrong")
			}
			w.Write(liveFixture(t, "fare-reserved"))
		}
	}))
	defer server.Close()
	c := NewClient()
	c.HTTP = server.Client()
	c.endpoint = server.URL
	c.limiter = cliutil.NewAdaptiveLimiter(0)
	q, e := c.Fares(context.Background(), "Tokyo", "Shin-Osaka", "2026-10-02", "reserved", 2, 1, mustNow("2026-10-02T12:00:00+09:00"))
	if e != nil || requests != 2 || q.Quotes[0].AdultSubtotalJPY != 29040 || q.Quotes[0].PartyTotalJPY != nil || q.Quotes[0].ChildFareJPY != nil {
		t.Fatalf("child/party result%+v err%v", q, e)
	}
}

func TestPublicBoundsAndThrottle(t *testing.T) {
	now := mustNow("2026-12-31T12:00:00+09:00")
	for _, date := range []string{"2026-12-30", "2027-03-01"} {
		if _, e := ValidateFareDate(date, now); e == nil {
			t.Fatal("date horizon bypass", date)
		}
	}
	if _, e := ValidateFareDate("2027-02-28", now); e != nil {
		t.Fatal(e)
	}
	for _, code := range []int{429, 503} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		c := NewClient()
		c.HTTP = server.Client()
		_, e := c.fetch(context.Background(), "GET", server.URL, "")
		if e == nil {
			t.Fatal("non200 accepted")
		}
		if code == 429 {
			var rate *cliutil.RateLimitError
			if !errors.As(e, &rate) {
				t.Fatal("429 lost typed error")
			}
		}
		server.Close()
	}
	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, strings.Repeat("x", MaxHTMLBytes+1)) }))
	defer large.Close()
	c := NewClient()
	c.HTTP = large.Client()
	if _, e := c.fetch(context.Background(), "GET", large.URL, ""); e == nil || !strings.Contains(e.Error(), "exceeds") {
		t.Fatal("oversize HTML not rejected")
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	c = NewClient()
	c.HTTP = slow.Client()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, e := c.fetch(ctx, "GET", slow.URL, ""); !errors.Is(e, context.DeadlineExceeded) {
		t.Fatalf("deadline not honored:%v", e)
	}
}
