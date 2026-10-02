package limousine

import (
 "context"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "strings"
 "testing"
 "time"
)

type reviewRedirectTransport struct { hits int }
func (r *reviewRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
 r.hits++
 next := ""
 switch req.URL.Query().Get("redirect") {
 case "": next = "1"
 case "1": next = "2"
 }
 response := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("{}")), Request: req}
 if next != "" {
  response.StatusCode = 302
  response.Header.Set("Location", Origin+"/en/review/?redirect="+next)
 }
 return response, nil
}

func TestReviewRedirectBudgetCountsWireRequests(t *testing.T) {
 provider := New(5*time.Second, 1000)
 transport := &reviewRedirectTransport{}
 provider.HTTP.Transport = transport
 var last error
 for i:=0; i<3; i++ {
  _, last = provider.Fetch(context.Background(), "GET", "/en/", "")
  if last != nil { break }
 }
 t.Logf("wire_requests=%d reported_requests=%d last_error=%v", transport.hits, provider.Requests, last)
 if transport.hits > 8 {
  t.Fatal(fmt.Sprintf("provider cap is 8 but three supported reads sent %d requests and reported %d", transport.hits, provider.Requests))
 }
}


func TestReviewPageSourceURLQueryPreserved(t *testing.T) {
 provider := New(5*time.Second, 0)
 provider.HTTP.Transport = providerRoundTripFunc(func(req *http.Request) (*http.Response, error) {
  return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("<html><title>Stops</title><a href=\"/en/busstop/detail/HanedaAirportTerminal3\">Terminal 3</a></html>")), Request: req}, nil
 })
 envelope, err := provider.PageLinks(context.Background(), "/en/timetable/detail/Haneda-Narita/?d=2026-10-03&dir=1", []string{"/en/busstop/detail"})
 if err != nil { t.Fatal(err) }
 page := envelope.Results.([]PageSummary)[0]
 source, err := url.Parse(page.SourceURL)
 if err != nil { t.Fatal(err) }
 t.Logf("source_url=%s", page.SourceURL)
 if source.Query().Get("dir") != "1" || source.Query().Get("d") != "2026-10-03" {
  t.Fatalf("source query was corrupted: dir=%q date=%q", source.Query().Get("dir"), source.Query().Get("d"))
 }
}
