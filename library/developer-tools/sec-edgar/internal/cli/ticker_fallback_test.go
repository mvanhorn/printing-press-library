// Copyright 2026 Chris Drit and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"testing"
)

// tickerFakeClient serves company_tickers.json and EFTS entity lookups from
// canned bodies and records every URL requested.
type tickerFakeClient struct {
	tickerMap    string
	tickerMapErr error
	entities     map[string]string // keysTyped value -> EFTS body
	entityErr    error
	calls        []string
}

func (f *tickerFakeClient) Get(path string, _ map[string]string) (json.RawMessage, error) {
	f.calls = append(f.calls, path)
	if path == "https://www.sec.gov/files/company_tickers.json" {
		if f.tickerMapErr != nil {
			return nil, f.tickerMapErr
		}
		return json.RawMessage(f.tickerMap), nil
	}
	u, err := url.Parse(path)
	if err != nil || u.Host != "efts.sec.gov" {
		return nil, errors.New("unexpected URL " + path)
	}
	if f.entityErr != nil {
		return nil, f.entityErr
	}
	body, ok := f.entities[u.Query().Get("keysTyped")]
	if !ok {
		return json.RawMessage(`{"hits":{"hits":[]}}`), nil
	}
	return json.RawMessage(body), nil
}

var errTickerMap403 = errors.New("GET https://www.sec.gov/files/company_tickers.json returned HTTP 403: Request Rate Threshold Exceeded")

const (
	eftsAAPL = `{"hits":{"hits":[
		{"_id":"320193","_source":{"entity":"Apple Inc. (AAPL)","tickers":"AAPL"}},
		{"_id":"1215379","_source":{"entity":"APPLE JIM B"}}
	]}}`
	eftsGOOG = `{"hits":{"hits":[
		{"_id":"1652044","_source":{"entity":"Alphabet Inc. (GOOGL, GOOG)","tickers":"GOOGL, GOOG"}},
		{"_id":"1136101","_source":{"entity":"GOOGLE INC"}}
	]}}`
)

func TestFetchTickerMapPrefersFullMap(t *testing.T) {
	c := &tickerFakeClient{tickerMap: `{"0":{"cik_str":320193,"ticker":"AAPL","title":"Apple Inc."}}`}
	rows, err := fetchTickerMap(c, "AAPL")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Padded != "0000320193" {
		t.Fatalf("rows = %+v", rows)
	}
	if len(c.calls) != 1 {
		t.Fatalf("EFTS must not be queried when the full map loads; calls = %v", c.calls)
	}
}

// www.sec.gov 403s User-Agents that data.sec.gov and efts.sec.gov accept
// (issue #2446), so ticker resolution must fall back to the EFTS entity index.
func TestFetchTickerMapFallsBackToEFTSEntities(t *testing.T) {
	c := &tickerFakeClient{
		tickerMapErr: errTickerMap403,
		entities:     map[string]string{"AAPL": eftsAAPL, "GOOG": eftsGOOG},
	}
	rows, err := fetchTickerMap(c, "AAPL", " GOOG ")
	if err != nil {
		t.Fatal(err)
	}
	want := []tickerRow{
		{CIK: 320193, Ticker: "AAPL", Title: "Apple Inc.", Padded: "0000320193"},
		{CIK: 1652044, Ticker: "GOOGL", Title: "Alphabet Inc.", Padded: "0001652044"},
		{CIK: 1652044, Ticker: "GOOG", Title: "Alphabet Inc.", Padded: "0001652044"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %+v, want %+v", rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want[i])
		}
	}
}

func TestFetchTickerMapFallbackDedupesAndSkipsBlankQueries(t *testing.T) {
	c := &tickerFakeClient{
		tickerMapErr: errTickerMap403,
		entities:     map[string]string{"GOOG": eftsGOOG, "GOOGL": eftsGOOG},
	}
	rows, err := fetchTickerMap(c, "GOOG", "", "GOOGL")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected GOOGL and GOOG once each, got %+v", rows)
	}
	// One map attempt plus one EFTS call per non-blank query.
	if len(c.calls) != 3 {
		t.Fatalf("calls = %v", c.calls)
	}
}

func TestFetchTickerMapWithoutQueriesKeepsMapError(t *testing.T) {
	c := &tickerFakeClient{tickerMapErr: errTickerMap403}
	if _, err := fetchTickerMap(c); !errors.Is(err, errTickerMap403) {
		t.Fatalf("err = %v, want the company_tickers.json error", err)
	}
	if len(c.calls) != 1 {
		t.Fatalf("calls = %v", c.calls)
	}
}

func TestFetchTickerMapReportsBothFailures(t *testing.T) {
	c := &tickerFakeClient{
		tickerMapErr: errTickerMap403,
		entityErr:    errors.New("GET efts.sec.gov returned HTTP 503"),
	}
	_, err := fetchTickerMap(c, "AAPL")
	if err == nil {
		t.Fatal("expected an error when both sources fail")
	}
	if !errors.Is(err, errTickerMap403) {
		t.Errorf("error must wrap the company_tickers.json failure: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 503") {
		t.Errorf("error must name the EFTS failure: %v", err)
	}
}

func TestResolveOwnershipCIKUsesEFTSFallback(t *testing.T) {
	c := &tickerFakeClient{
		tickerMapErr: errTickerMap403,
		entities:     map[string]string{"apple": eftsAAPL},
	}
	cik, title, err := resolveOwnershipCIK(c, "apple")
	if err != nil {
		t.Fatal(err)
	}
	if cik != "0000320193" || title != "Apple Inc." {
		t.Fatalf("got %q %q", cik, title)
	}
}
