package limousine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPageLinksOmitEmbeddedInventoryAndForeignLinks(t *testing.T) {
	body := `<html><title>Transfer &amp; stops</title><a href="/en/busstop/detail/HanedaAirportTerminal3">Terminal 3</a><a href="https://evil.example/en/busstop/detail/X">foreign</a><a href="/en/guide/terms">other</a><script>window.data={reservation_status:1,remaining_seats:9}</script></html>`
	page, n, err := ParsePageLinks(body, Origin+"/en/timetable/detail/Haneda-Narita", []string{"/en/busstop/detail"})
	if err != nil || n != 1 || len(page.Links) != 1 || page.Title != "Transfer & stops" {
		t.Fatalf("page=%+v n=%d err=%v", page, n, err)
	}
	data, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "remaining_seats") || strings.Contains(string(data), "window.data") {
		t.Fatal("embedded inventory leaked")
	}
}

func TestProviderPageSourceURLPreservesQuery(t *testing.T) {
	p := New(time.Second, 0)
	p.HTTP.Transport = providerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`<title>Timetable</title><a href="/en/busstop/detail/NaritaAirportTerminal1">Terminal 1</a>`))}, nil
	})
	env, err := p.PageLinks(context.Background(), "/en/timetable/detail/Haneda-Narita/?d=2026-10-03&dir=1", []string{"/en/busstop/detail"})
	if err != nil {
		t.Fatal(err)
	}
	page := env.Results.([]PageSummary)[0]
	u, err := url.Parse(page.SourceURL)
	if err != nil || u.Query().Get("dir") != "1" || u.Query().Get("d") != "2026-10-03" {
		t.Fatalf("corrupt source URL %s %v", page.SourceURL, err)
	}
}
