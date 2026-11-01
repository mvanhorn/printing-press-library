package cli

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/yahoo-fantasy/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestYahooRoundTripperTranslatesMutationsAndNormalizesXML(t *testing.T) {
	var capturedBody string
	var capturedContentType string

	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		b, _ := io.ReadAll(req.Body)
		capturedBody = string(b)
		capturedContentType = req.Header.Get("Content-Type")
		xmlResp := `<?xml version="1.0" encoding="UTF-8"?><fantasy_content><roster><coverage_type>week</coverage_type><week>9</week></roster></fantasy_content>`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/xml"}},
			Body:       io.NopCloser(strings.NewReader(xmlResp)),
		}, nil
	})

	cfg := &config.Config{BaseURL: "https://fantasysports.yahooapis.com/fantasy/v2"}
	c := client.New(cfg, 0, 0)
	rt := &yahooRoundTripper{base: base, client: c}

	req, err := http.NewRequest(http.MethodPut, "https://fantasysports.yahooapis.com/fantasy/v2/team/449.l.365083.t.1/roster", strings.NewReader(`{"coverage_type":"week","week":9,"player_key":"449.p.31862","position":"QB"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if capturedContentType != "application/xml" {
		t.Fatalf("expected Content-Type application/xml on wire, got %q", capturedContentType)
	}
	if !strings.Contains(capturedBody, "<fantasy_content>") || !strings.Contains(capturedBody, "<position>QB</position>") {
		t.Fatalf("expected <fantasy_content> XML on wire, got: %s", capturedBody)
	}
	respBytes, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(respBytes), `"fantasy_content"`) {
		t.Fatalf("expected normalized JSON response, got: %s", string(respBytes))
	}
}
