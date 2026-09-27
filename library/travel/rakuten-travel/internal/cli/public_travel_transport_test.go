package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/rakuten-travel/internal/travel"
)

type travelCLIRoundTripper func(*http.Request) (*http.Response, error)

func (f travelCLIRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func travelHTTPResponse(request *http.Request, status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(body)), Request: request}
}

func injectTravelHTTP(t *testing.T, handler travelCLIRoundTripper) {
	t.Helper()
	original := publicTravelClientFactory
	publicTravelClientFactory = func(cfg travel.Config) (travel.API, error) {
		cfg.HTTPClient = &http.Client{Transport: handler}
		cfg.MinInterval = -1
		return travel.NewClient(cfg)
	}
	t.Cleanup(func() { publicTravelClientFactory = original })
}

func decodeTravelOutput(t *testing.T, out string) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v; output=%s", err, out)
	}
	return payload
}
func travelObject(t *testing.T, value any) map[string]any {
	t.Helper()
	object, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %#v", value)
	}
	return object
}
func travelRows(t *testing.T, value any) []any {
	t.Helper()
	rows, ok := value.([]any)
	if !ok {
		t.Fatalf("expected array, got %#v", value)
	}
	return rows
}

const hotelSearchCLIHTML = `<html><title>Rakuten hotels</title><body><input name="f_query" value="品川"><p>2件中</p>
<div class="hotelBox"><h2><a href="https://travel.rakuten.co.jp/HOTEL/51870/51870.html">品川の宿</a></h2><span class="rating">4.25</span><p>125件</p><p>[住所] 東京都 宿泊プラン</p></div>
<div class="hotelBox"><h2><a href="https://travel.rakuten.co.jp/HOTEL/72056/72056.html">第二の宿</a></h2><span class="rating">4.50</span><p>42件</p></div>
<a href="https://kw.travel.rakuten.co.jp/keyword/Search.do?f_next=2">next</a>
</body></html>`
const hotelBasicCLIHTML = `<html><head><title>Rakuten property</title><link rel="canonical" href="https://travel.rakuten.co.jp/HOTEL/51870/51870.html"></head><body><div id="RthNameArea">品川の宿</div><a href="https://travel.rakuten.co.jp/HOTEL/51870/51870.html">品川の宿</a><div id="hotel-info"><div class="rating">4.25</div><p>125件</p></div></body></html>`
const hotelDetailsCLIHTML = `<html><head><title>Rakuten facilities</title><link rel="canonical" href="https://travel.rakuten.co.jp/HOTEL/51870/51870_std.html"></head><body><div id="RthNameArea">品川の宿</div>
<ul><li data-locate="hotel-address"><dl><dt>住所</dt><dd>〒140-0001 東京都品川区</dd></dl></li>
<li data-locate="hotel-access"><dl><dt>交通アクセス</dt><dd><ul><li>駅から徒歩5分</li></ul></dd></dl></li>
<li data-locate="hotel-park"><dl><dt>駐車場</dt><dd>有料駐車場</dd></dl></li>
<li data-locate="hotel-facilities"><dl><dt>館内設備</dt><dd>レストラン</dd></dl></li>
<li data-locate="hotel-room-facilities"><dl><dt>部屋設備</dt><dd>インターネット接続</dd></dl></li>
<li data-locate="hotel-attention"><dl><dt>条件・注意事項</dt><dd><ul><li>宿泊税は料金に含まれておりません。</li><li>添い寝の条件をご確認ください。</li></ul></dd></dl></li>
<li data-locate="hotel-cancelPolicy"><dl><dt>キャンセルポリシー</dt><dd>前日キャンセルは宿泊料金の20%</dd></dl><span data-locate="hotel-planCancelPolicy">プランごとの条件は予約時にご確認ください。</span></li></ul>
</body></html>`

func TestPublicTravelHotelSearchJSONProjectionAndContinuation(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		requests++
		if request.URL.Query().Get("f_query") != "品川" {
			t.Fatalf("literal keyword lost: %s", request.URL)
		}
		body := hotelSearchCLIHTML
		if request.URL.Query().Get("f_next") == "2" {
			body = strings.ReplaceAll(body, "51870", "99999")
		}
		return travelHTTPResponse(request, 200, body), nil
	})
	out, diagnostics, err := runPublicTravelCLI("hotels", "search", "--query", "品川", "--limit", "1")
	if err != nil || diagnostics != "" {
		t.Fatalf("search err=%v stderr=%s", err, diagnostics)
	}
	if strings.Contains(out, "\n ") {
		t.Fatalf("default JSON is indented: %s", out)
	}
	payload := decodeTravelOutput(t, out)
	rows := travelRows(t, payload["results"])
	if len(rows) != 1 || travelObject(t, rows[0])["hotel_id"] != "51870" {
		t.Fatalf("wrong first candidate: %s", out)
	}
	meta := travelObject(t, payload["meta"])
	page := travelObject(t, meta["page"])
	if page["source_unit"] != "hotels" || page["next_page"] != float64(1) || page["next_offset"] != float64(1) || page["has_more"] != true {
		t.Fatalf("within-page continuation missing: %s", out)
	}
	rating := travelObject(t, travelObject(t, rows[0])["rating"])
	if rating["source"] != "Rakuten Travel" || rating["score"] != 4.25 || rating["review_count"] != float64(125) {
		t.Fatalf("rating evidence lost: %s", out)
	}
	out, _, err = runPublicTravelCLI("hotels", "search", "--query", "品川", "--offset", "1", "--limit", "1", "--agent")
	if err != nil {
		t.Fatal(err)
	}
	payload = decodeTravelOutput(t, out)
	rows = travelRows(t, payload["results"])
	if travelObject(t, rows[0])["hotel_id"] != "72056" || len(payload) != 2 {
		t.Fatalf("agent shape/next row wrong: %s", out)
	}
	if travelObject(t, payload["meta"])["source"] != "local" || requests != 1 {
		t.Fatalf("persistent cache provenance/request count wrong: calls=%d output=%s", requests, out)
	}
	out, _, err = runPublicTravelCLI("hotels", "search", "--query", "品川", "--page", "2", "--limit", "1", "--json", "--select", "results.hotel_id,results.name")
	if err != nil {
		t.Fatal(err)
	}
	payload = decodeTravelOutput(t, out)
	row := travelObject(t, travelRows(t, payload["results"])[0])
	if len(payload) != 1 || len(row) != 2 || row["hotel_id"] != "99999" || row["name"] != "品川の宿" {
		t.Fatalf("documented dotted selection wrong: %s", out)
	}
	if requests != 2 {
		t.Fatalf("source page did not fetch a different document: calls=%d", requests)
	}
}

func TestPublicTravelAreasAndHotelDetailsContent(t *testing.T) {
	testenv.Isolate(t)
	requests := 0
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		requests++
		body := hotelBasicCLIHTML
		switch {
		case strings.HasSuffix(request.URL.Path, "/tokyo/map.html"):
			body = `<html><body><a href="/yado/tokyo/E.html">品川</a><a href="/yado/tokyo/F.html">上野</a><a href="/yado/kyoto/A.html">京都</a></body></html>`
		case strings.HasSuffix(request.URL.Path, "_std.html"):
			body = hotelDetailsCLIHTML
		}
		return travelHTTPResponse(request, 200, body), nil
	})
	out, _, err := runPublicTravelCLI("areas", "list", "--parent", "tokyo", "--limit", "1", "--offset", "1")
	if err != nil {
		t.Fatal(err)
	}
	payload := decodeTravelOutput(t, out)
	rows := travelRows(t, payload["results"])
	if len(rows) != 1 || travelObject(t, rows[0])["id"] != "tokyo/F" || travelObject(t, rows[0])["parent"] != "tokyo" {
		t.Fatalf("area level/offset mismatch: %s", out)
	}
	out, _, err = runPublicTravelCLI("hotels", "show", "--hotel", "51870", "--agent")
	if err != nil {
		t.Fatal(err)
	}
	payload = decodeTravelOutput(t, out)
	hotel := travelObject(t, payload["results"])
	if hotel["hotel_id"] != "51870" || hotel["coordinates"] != nil {
		t.Fatalf("property identity/coordinates wrong: %s", out)
	}
	for _, field := range []string{"access", "parking", "hotel_amenities", "room_amenities", "notes", "property_cancellation_policy"} {
		if len(travelRows(t, hotel[field])) == 0 {
			t.Fatalf("property field %s missing: %s", field, out)
		}
	}
	if !strings.Contains(fmt.Sprint(hotel["notes"]), "宿泊税") || !strings.Contains(fmt.Sprint(hotel["policy_caveat"]), "予約時") {
		t.Fatalf("fee/policy evidence missing: %s", out)
	}
	if requests != 3 {
		t.Fatalf("property inspection must use basic+detail documents: requests=%d", requests)
	}
}

func TestPublicTravelEmptyHotelSearchPreservesArrayAndStatus(t *testing.T) {
	testenv.Isolate(t)
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		return travelHTTPResponse(request, 200, "<html><body><input name=\"f_query\" value=\""+html.EscapeString(request.URL.Query().Get("f_query"))+"\"><div id=\"result\">該当する宿泊施設が見つかりませんでした</div></body></html>"), nil
	})
	out, diagnostics, err := runPublicTravelCLI("hotels", "search", "--query", "NoSuchProperty", "--agent")
	if err != nil || diagnostics != "" {
		t.Fatalf("empty search err=%v stderr=%s", err, diagnostics)
	}
	payload := decodeTravelOutput(t, out)
	if len(travelRows(t, payload["results"])) != 0 || travelObject(t, payload["meta"])["status"] != "no_matches" {
		t.Fatalf("empty search shape/status wrong: %s", out)
	}
}

func TestPublicTravelTimeoutBoundsWholeInvocation(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	injectTravelHTTP(t, func(request *http.Request) (*http.Response, error) {
		calls++
		select {
		case <-time.After(25 * time.Millisecond):
			return travelHTTPResponse(request, 200, hotelBasicCLIHTML), nil
		case <-request.Context().Done():
			return nil, request.Context().Err()
		}
	})
	started := time.Now()
	out, diagnostics, err := runPublicTravelCLI("hotels", "show", "--hotel", "51870", "--timeout", "35ms")
	if err == nil || ExitCode(err) != 5 || !strings.Contains(diagnostics, "timeout") || out != "" {
		t.Fatalf("expected bounded timeout: err=%v stdout=%s stderr=%s", err, out, diagnostics)
	}
	if elapsed := time.Since(started); elapsed > 150*time.Millisecond || calls != 2 {
		t.Fatalf("timeout did not cover both requests: elapsed=%s calls=%d", elapsed, calls)
	}
	if !errorsIsContextOrTimeout(err) {
		t.Fatalf("unexpected timeout classification: %v", err)
	}
}
func errorsIsContextOrTimeout(err error) bool {
	return strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), context.DeadlineExceeded.Error())
}
