// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/niftyindices"
)

// TestNovelCompareHelpWires smoke-tests that the compare command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCompareHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"compare", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("compare --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "compare"} {
		if !strings.Contains(help, want) {
			t.Fatalf("compare --help missing %q in output:\n%s", want, help)
		}
	}
}

type compareTransport func(*http.Request) (*http.Response, error)

func (f compareTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCompareCommandReportsBenchmarkAndConstituentStatus(t *testing.T) {
	for _, tc := range []struct {
		name, benchmark, requested, quoteName, wantValidation string
		constituentStatus                                     int
		wantFailure                                           bool
	}{
		{"matching TRI index with failed constituents", "Nifty 50 TRI Index", "Nifty-50 Index", "NIFTY 50", "matched", http.StatusServiceUnavailable, true},
		{"unreported benchmark with constituents", "", "Nifty-50 Index", "NIFTY 50", "not_reported", http.StatusOK, false},
		{"TRI quote uses base-index constituents", "Nifty 50 TRI Index", "nifty50tri", "NIFTY 50 TRI", "matched", http.StatusOK, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			benchmarkJSON, err := json.Marshal(tc.benchmark)
			if err != nil {
				t.Fatal(err)
			}
			originalTransport := http.DefaultTransport
			http.DefaultTransport = compareTransport(func(req *http.Request) (*http.Response, error) {
				status := http.StatusOK
				body := ""
				contentType := "application/json"
				switch req.URL.Host + req.URL.Path {
				case "www.indiapassivefunds.com/pages/api/login":
					body = `{"status":true,"response":{"token":"synthetic-token"}}`
				case "data.indiapassivefunds.com/api/v1/etf/funddetail":
					body = fmt.Sprintf(`{"status":true,"response":{"funddescription":{"columns":[{"field":"f_05","displayName":"Benchmark Index"}],"data":[{"f_05":%s}]}}}`, benchmarkJSON)
				case "www.nseindia.com/api/allIndices":
					body = fmt.Sprintf(`{"timestamp":"2026-10-01","data":[{"index":%q,"last":100}]}`, tc.quoteName)
				case "www.niftyindices.com/IndexConstituent/ind_nifty50list.csv":
					status = tc.constituentStatus
					if status == http.StatusOK {
						contentType = "text/csv"
						body = "Company,Industry,Symbol,Series,ISIN\nExample,Finance,EX,NSE,INE000000000\n"
					} else {
						body = "provider unavailable"
					}
				default:
					return nil, fmt.Errorf("unexpected synthetic request to %s", req.URL)
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
			})
			t.Cleanup(func() { http.DefaultTransport = originalTransport })

			cmd := RootCmd()
			cmd.SetArgs([]string{"--home", t.TempDir(), "--no-learn", "--json", "compare", "1150", tc.requested})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("compare: %v", err)
			}
			var result struct {
				BenchmarkValidation string              `json:"benchmark_validation"`
				IndexQuote          json.RawMessage     `json:"index_quote"`
				Constituents        []json.RawMessage   `json:"index_constituents_sample"`
				FetchFailures       []map[string]string `json:"fetch_failures"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("decode compare output: %v\n%s", err, out.String())
			}
			if result.BenchmarkValidation != tc.wantValidation || len(result.IndexQuote) == 0 {
				t.Fatalf("comparison validation=%q index quote=%s, want %q and quote", result.BenchmarkValidation, result.IndexQuote, tc.wantValidation)
			}
			var quote niftyindices.LiveQuote
			if err := json.Unmarshal(result.IndexQuote, &quote); err != nil || quote.IndexName != tc.quoteName {
				t.Fatalf("index quote = %#v, decode error = %v; want %q", quote, err, tc.quoteName)
			}
			if tc.wantFailure {
				if len(result.FetchFailures) != 1 || result.FetchFailures[0]["source"] != "index_constituents" {
					t.Fatalf("fetch failures = %#v, want index_constituents", result.FetchFailures)
				}
			} else if len(result.FetchFailures) != 0 || len(result.Constituents) != 1 {
				t.Fatalf("fetch failures = %#v, constituents = %#v, want one constituent and no failure", result.FetchFailures, result.Constituents)
			}
		})
	}
}

func TestCompareCommandRejectsMismatchedBenchmarkBeforeIndexFetch(t *testing.T) {
	originalTransport := http.DefaultTransport
	indexFetched := false
	http.DefaultTransport = compareTransport(func(req *http.Request) (*http.Response, error) {
		body := ""
		switch req.URL.Host + req.URL.Path {
		case "www.indiapassivefunds.com/pages/api/login":
			body = `{"status":true,"response":{"token":"synthetic-token"}}`
		case "data.indiapassivefunds.com/api/v1/etf/funddetail":
			body = `{"status":true,"response":{"funddescription":{"columns":[{"field":"f_05","displayName":"Benchmark Index"}],"data":[{"f_05":"NIFTY BANK"}]}}}`
		default:
			indexFetched = true
			return nil, fmt.Errorf("unexpected index request to %s", req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = originalTransport })

	cmd := RootCmd()
	cmd.SetArgs([]string{"--home", t.TempDir(), "--no-learn", "--json", "compare", "1150", "NIFTY 50"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	err := cmd.Execute()
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "does not match") || indexFetched {
		t.Fatalf("mismatched compare error=%v exit=%d index fetched=%t, want usage rejection before index fetch", err, ExitCode(err), indexFetched)
	}
}

func TestCompareNormalizesAndValidatesBenchmarkIdentity(t *testing.T) {
	for _, benchmark := range []string{"Nifty 50 TRI", "Nifty50TRI", "Nifty-50 (TRI)", "Nifty 50 Total Return Index", "Nifty 50 TRI Index", "Nifty 50 Index"} {
		if err := validateBenchmarkIdentity("1150", benchmark, "nifty 50"); err != nil {
			t.Fatalf("equivalent benchmark %q rejected: %v", benchmark, err)
		}
	}
	if err := validateBenchmarkIdentity("1150", "Nifty 50 TRI", "NIFTY BANK"); err == nil {
		t.Fatal("mismatched benchmark was accepted")
	}

	quotes := []niftyindices.LiveQuote{{IndexName: "NIFTY BANK", Last: 42}}
	got := findLiveQuote(quotes, "nifty bank")
	if got == nil || got.Last != 42 {
		t.Fatalf("case-insensitive live quote lookup = %#v, want NIFTY BANK", got)
	}
}

func TestComparePrefersExactQuoteAndUsesItForConstituents(t *testing.T) {
	quotes := []niftyindices.LiveQuote{
		{IndexName: "NIFTY 50", Last: 100},
		{IndexName: "NIFTY 50 TRI", Last: 120},
	}
	tri := findLiveQuote(quotes, "nifty 50 tri")
	if tri == nil || tri.IndexName != "NIFTY 50 TRI" {
		t.Fatalf("exact TRI quote was not preferred: %#v", tri)
	}
	tri = findLiveQuote(quotes, "nifty50tri")
	if tri == nil || tri.IndexName != "NIFTY 50 TRI" {
		t.Fatalf("abbreviated TRI request selected wrong quote: %#v", tri)
	}
	if got := constituentSlug("nifty50tri", tri); got != "nifty50" {
		t.Fatalf("constituent slug = %q, want base-index slug", got)
	}
	if got := findLiveQuote(quotes[:1], "nifty50tri"); got != nil {
		t.Fatalf("TRI request selected price quote: %#v", got)
	}
	base := findLiveQuote(quotes, "Nifty-50 Index")
	if base == nil || base.IndexName != "NIFTY 50" {
		t.Fatalf("price quote lookup = %#v", base)
	}
}

func TestCompareReportsConstituentFailure(t *testing.T) {
	out := map[string]any{}
	failures := addConstituentResult(out, nil, nil, errors.New("upstream unavailable"), 10)
	if len(failures) != 1 || failures[0]["source"] != "index_constituents" || failures[0]["error"] == "" {
		t.Fatalf("constituent failures = %#v, want labeled failure", failures)
	}
	if _, ok := out["index_constituents_sample"]; ok {
		t.Fatal("failed constituent fetch should not add a sample")
	}
}
