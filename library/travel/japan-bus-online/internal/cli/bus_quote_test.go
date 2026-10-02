// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-bus-online/internal/cliutil/testenv"
)

// TestNovelBusQuoteHelpWires smoke-tests that the bus quote command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBusQuoteHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"bus", "quote", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bus quote --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "quote"} {
		if !strings.Contains(help, want) {
			t.Fatalf("bus quote --help missing %q in output:\n%s", want, help)
		}
	}
}

type quoteFixtureTransport func(*http.Request) (*http.Response, error)

func (f quoteFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestBusQuoteAgentEmitsSelectedPairPartyAndOvernightEvidence(t *testing.T) {
	testenv.Isolate(t)
	for _, tc := range []struct {
		seats  int
		status string
	}{{2, "unknown"}, {6, "capacity_sufficient"}} {
		t.Run(fmt.Sprint(tc.seats), func(t *testing.T) {
			previous := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = previous })
			requests := 0
			http.DefaultTransport = quoteFixtureTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != "japanbusonline.com" {
					t.Fatalf("unexpected provider request: %s %s", r.Method, r.URL)
				}
				var body string
				switch {
				case strings.Contains(r.URL.Path, "CourseSearch"):
					body = `<div id="122001600010"><a href="/en/Detail/12200160001/0/1/2/">Select</a></div>`
				case strings.Contains(r.URL.Path, "SelectRoute"):
					body = `<input name="radioBtn" value="1/2/3/">`
				case strings.Contains(r.URL.Path, "SelectFABN"):
					body = `<input name="DepBusStop" value="8,Tomei Kakegawa,25:00"><input name="ArvBusStop" value="9,Shinjuku,06:00">`
				case strings.Contains(r.URL.Path, "GetFareTable"):
					if !strings.HasSuffix(r.URL.Path, "/8/9/0") {
						t.Fatalf("wrong selected stop pair: %s", r.URL.Path)
					}
					body = fmt.Sprintf(`<div>Seat Availability : %d</div><input id="Fare_PassengerName0" value="Adult"><input id="Fare_Passenger0" value="6100"><input id="Fare_PassengerName1" value="Child(6-12)"><input id="Fare_Passenger1" value="3050"><div>Max number of tickets per transaction : 4</div>`, tc.seats)
				default:
					body = `<div class="text_strong">Bookable Days of Operation<br>10/2/2026 ～ 12/2/2026</div><input id="SelectDate" value="10/10/2026"><div data-route="0001" data-coursecd="12200160001" data-updownflg="0" data-depdate="20261010" data-deptime="2325" data-arrdate="20261011" data-arrtime="0705">2 seats left</div>`
				}
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
			})
			cmd := RootCmd()
			cmd.SetArgs([]string{"bus", "quote", "--route", "12200160001", "--date", "2026-10-10", "--service", "0001", "--dep-stop", "8", "--arr-stop", "9", "--adults", "2", "--children", "1", "--rate-limit", "0", "--agent"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("quote command failed: %v\n%s", err, out.String())
			}
			var envelope struct {
				Results map[string]any `json:"results"`
			}
			if err := json.Unmarshal(out.Bytes(), &envelope); err != nil {
				t.Fatalf("invalid agent envelope: %v\n%s", err, out.String())
			}
			result := envelope.Results
			if result["estimated_total_jpy"] != float64(15250) || result["currency"] != "JPY" || result["quote_status"] != "fare_evidence_reported" {
				t.Fatalf("selected-pair fare evidence missing: %v", result)
			}
			party := result["party"].(map[string]any)
			capacity := party["availability"].(map[string]any)
			if party["requested_seats"] != float64(3) || party["status"] != tc.status || party["max_tickets_per_transaction"] != float64(4) || capacity["exact_seats"] != nil || capacity["seats_lower_bound"] != float64(tc.seats) {
				t.Fatalf("party capacity evidence lost or overstated: %v", party)
			}
			boarding := result["boarding"].(map[string]any)
			alighting := result["alighting"].(map[string]any)
			if boarding["stop_id"] != "8" || boarding["source_time"] != "25:00" || boarding["timestamp_jst"] != "2026-10-11T01:00:00+09:00" || alighting["stop_id"] != "9" || alighting["timestamp_jst"] != "2026-10-11T06:00:00+09:00" {
				t.Fatalf("selected-stop overnight identity lost: %v / %v", boarding, alighting)
			}
			if requests != 5 {
				t.Fatalf("fixture did not cover the complete quote flow: %d requests", requests)
			}
		})
	}
}
