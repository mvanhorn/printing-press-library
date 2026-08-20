// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestScheduledDeparturesPageFlattensRouteSegments(t *testing.T) {
	raw := []byte(`{
		"flights": [
			{"segments": [
				{
					"ident": "UAL99",
					"operator": "United",
					"scheduled_out": "2026-06-08T20:00:00Z",
					"inbound_fa_flight_id": "UAL98-1749400000-airline-0001"
				}
			]}
		]
	}`)

	var page scheduledDeparturesPage
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatalf("unmarshal route page: %v", err)
	}
	items := page.items()
	if len(items) != 1 {
		t.Fatalf("items len = %d, want 1", len(items))
	}
	if items[0].Ident != "UAL99" || items[0].InboundFAID == "" {
		t.Fatalf("route segment did not populate expected flight fields: %+v", items[0])
	}
}

func TestScheduledDeparturesPageEmptyRouteSegmentsReturnNoItems(t *testing.T) {
	raw := []byte(`{"flights": [{"segments": []}]}`)

	var page scheduledDeparturesPage
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatalf("unmarshal empty route page: %v", err)
	}
	if got := len(page.items()); got != 0 {
		t.Fatalf("items len = %d, want 0; items=%+v", got, page.items())
	}
}

func TestParseNASStatusFiltersRouteAirports(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<AIRPORT_STATUS_INFORMATION>
  <Update_Time>Mon Jun 8 03:32:04 2026 GMT</Update_Time>
  <Delay_type>
    <Name>Ground Delay Programs</Name>
    <Ground_Delay_List>
      <Ground_Delay>
        <ARPT>SFO</ARPT>
        <Reason>other</Reason>
        <Avg>1 hour and 28 minutes</Avg>
        <Max>5 hours and 33 minutes</Max>
      </Ground_Delay>
    </Ground_Delay_List>
  </Delay_type>
  <Delay_type>
    <Name>General Arrival/Departure Delay Info</Name>
    <Arrival_Departure_Delay_List>
      <Delay>
        <ARPT>ORD</ARPT>
        <Reason>WX:Thunderstorms</Reason>
        <Arrival_Departure Type="Departure">
          <Min>30 minutes</Min>
          <Max>44 minutes</Max>
          <Trend>Decreasing</Trend>
        </Arrival_Departure>
      </Delay>
    </Arrival_Departure_Delay_List>
  </Delay_type>
  <Delay_type>
    <Name>Airport Closures</Name>
    <Airport_Closure_List>
      <Airport_Closure>
        <ARPT>DCA</ARPT>
        <Reason>construction</Reason>
        <Start>Mon Jun 8 01:00:00 2026 GMT</Start>
        <Reopen>Mon Jun 8 02:00:00 2026 GMT</Reopen>
      </Airport_Closure>
    </Airport_Closure_List>
  </Delay_type>
</AIRPORT_STATUS_INFORMATION>`)

	status, err := parseNASStatus(body, "test://nas", normalizeAirportCodes("SFO"), normalizeAirportCodes("DCA"))
	if err != nil {
		t.Fatalf("parse NAS status: %v", err)
	}
	if status.UpdatedAt == "" {
		t.Fatal("NAS status did not preserve update time")
	}
	if len(status.Events) != 2 {
		t.Fatalf("events len = %d, want 2: %+v", len(status.Events), status.Events)
	}
	if status.Events[0].Airport != "SFO" || status.Events[0].Average == "" {
		t.Fatalf("first NAS event not the SFO GDP: %+v", status.Events[0])
	}
	if status.Events[1].Airport != "DCA" || status.Events[1].Start == "" {
		t.Fatalf("second NAS event not the DCA closure: %+v", status.Events[1])
	}
}

func TestAssessDecisionMixedSystemicAndFlightSpecific(t *testing.T) {
	report := &assessReport{
		Evidence: assessEvidence{
			Origin: assessAirportCondition{
				Airport: "KSFO",
				Role:    "origin",
				AirportDelays: airportDelaySummary{
					Available: true,
					Active:    true,
					Signals:   []string{"airport volume"},
				},
				DisruptionCounts: disruptionCountsSummary{
					Available: true,
					Delays:    147,
					Total:     641,
					DelayRate: 0.22,
					Signal:    "22.9% delayed (147 of 641)",
				},
				NASEvents: []nasEvent{{Airport: "SFO", Category: "Ground Delay Programs", Reason: "other"}},
			},
			Destination: assessAirportCondition{Airport: "KDCA", Role: "destination"},
		},
		DelayedFlight: &assessedFlight{
			Ident:   "UAL123",
			Risk:    "high",
			Reasons: []string{"departure delay 90 min", "uses inbound aircraft"},
		},
		Sources: []assessSource{{Name: "aeroapi.origin_delays", Status: "ok"}},
	}

	decision := buildAssessDecision(report)
	if decision.Verdict != "mixed_systemic_and_flight_specific" {
		t.Fatalf("verdict = %q, want mixed_systemic_and_flight_specific", decision.Verdict)
	}
	if decision.Confidence != "high" {
		t.Fatalf("confidence = %q, want high", decision.Confidence)
	}
	if len(decision.SystemicSignals) < 2 || len(decision.FlightSignals) == 0 {
		t.Fatalf("decision did not retain systemic and flight-specific signals: %+v", decision)
	}
}

func TestCollectMissingEvidenceIncludesEmptyRouteAlternatives(t *testing.T) {
	report := &assessReport{
		Evidence: assessEvidence{
			Origin: assessAirportCondition{
				Airport:          "KSFO",
				Role:             "origin",
				AirportDelays:    airportDelaySummary{Available: true},
				DisruptionCounts: disruptionCountsSummary{Available: true},
			},
			Destination: assessAirportCondition{
				Airport:          "KDCA",
				Role:             "destination",
				AirportDelays:    airportDelaySummary{Available: true},
				DisruptionCounts: disruptionCountsSummary{Available: true},
			},
		},
		Sources: []assessSource{{Name: "aeroapi.route_alternatives", Status: "empty"}},
	}
	missing := collectMissingEvidence(report)
	if len(missing) != 1 || missing[0] != "aeroapi.route_alternatives: no result" {
		t.Fatalf("missing evidence = %+v, want route empty source", missing)
	}
}

func TestSelectAssessFlightCandidatePrefersRouteNearWindow(t *testing.T) {
	query := buildAssessQuery(assessOptions{origin: "SFO", destination: "JFK", lookahead: 6 * time.Hour}, "2026-06-08", mustParseTime(t, "2026-06-08T03:50:00Z"))
	target := mustParseTime(t, "2026-06-08T03:50:00Z")
	items := []scheduledDeparture{
		{
			Ident:        "DAL365",
			ScheduledOut: "2026-06-10T05:40:00Z",
			Origin:       airportRef{CodeIATA: "SFO"},
			Destination:  airportRef{CodeIATA: "JFK"},
		},
		{
			Ident:        "DAL365",
			FAFlightID:   "today",
			ScheduledOut: "2026-06-08T05:15:00Z",
			Origin:       airportRef{CodeIATA: "SFO"},
			Destination:  airportRef{CodeIATA: "JFK"},
		},
	}

	got := selectAssessFlightCandidate(items, query, target)
	if got == nil {
		t.Fatal("expected selected flight")
	}
	if got.FAFlightID != "today" {
		t.Fatalf("selected fa_flight_id = %q, want today", got.FAFlightID)
	}
}

func TestCollectMissingEvidenceIncludesDestinationDelayAvailability(t *testing.T) {
	report := &assessReport{
		Evidence: assessEvidence{
			Origin: assessAirportCondition{
				Airport:          "KSFO",
				Role:             "origin",
				AirportDelays:    airportDelaySummary{Available: true},
				DisruptionCounts: disruptionCountsSummary{Available: true},
			},
			Destination: assessAirportCondition{
				Airport:          "KDCA",
				Role:             "destination",
				DisruptionCounts: disruptionCountsSummary{Available: true},
			},
		},
	}
	missing := collectMissingEvidence(report)
	if len(missing) != 1 || missing[0] != "destination airport delay advisory unavailable or empty" {
		t.Fatalf("missing evidence = %+v, want destination airport delay advisory only", missing)
	}
}

func TestCollectMissingEvidenceDoesNotDoubleCountFailedDelaySources(t *testing.T) {
	report := &assessReport{
		Evidence: assessEvidence{
			Origin: assessAirportCondition{
				Airport:          "KSFO",
				Role:             "origin",
				Weather:          weatherSummary{Available: true},
				DisruptionCounts: disruptionCountsSummary{Available: true},
			},
			Destination: assessAirportCondition{
				Airport:          "KDCA",
				Role:             "destination",
				Weather:          weatherSummary{Available: true},
				DisruptionCounts: disruptionCountsSummary{Available: true},
			},
		},
		Sources: []assessSource{
			{Name: "aeroapi.origin_delays", Status: "error", Error: "origin unavailable"},
			{Name: "aeroapi.destination_delays", Status: "error", Error: "destination unavailable"},
		},
	}

	missing := collectMissingEvidence(report)
	if len(missing) != 2 {
		t.Fatalf("missing evidence len = %d, want 2: %+v", len(missing), missing)
	}
	for _, notWant := range []string{
		"origin airport delay advisory unavailable or empty",
		"destination airport delay advisory unavailable or empty",
	} {
		if containsString(missing, notWant) {
			t.Fatalf("missing evidence double-counted %q: %+v", notWant, missing)
		}
	}

	decision := buildAssessDecision(report)
	if decision.Verdict == "insufficient_data" {
		t.Fatalf("verdict = insufficient_data with only two failed sources; decision=%+v", decision)
	}
	if decision.Confidence == "low" {
		t.Fatalf("confidence = low with only two failed sources; decision=%+v", decision)
	}
}

func TestAssessReadinessUsesResolvedInboundArrival(t *testing.T) {
	item := scheduledDeparture{
		Ident:       "UAL456",
		InboundFAID: "UAL455-1749400000-airline-0001",
		Status:      "Scheduled",
	}
	inbound := &assessedInbound{Ident: "UAL455", ActualIn: "2026-06-08T19:30:00Z", Status: "Arrived"}

	got := assessReadiness(item, inbound)
	if got != "inbound_arrived_at_origin" {
		t.Fatalf("readiness = %q, want inbound_arrived_at_origin", got)
	}
}

func TestSummarizeWeatherOnlyMarksActualGustGroup(t *testing.T) {
	noGust := summarizeWeather(json.RawMessage(`{"observations":[{"raw_data":"KBGR 081856Z 25012KT 10SM SKC"}]}`))
	if containsString(noGust.Signals, "gusty wind marker") {
		t.Fatalf("non-gusty KBGR METAR was marked gusty: %+v", noGust.Signals)
	}

	gusty := summarizeWeather(json.RawMessage(`{"observations":[{"raw_data":"KSFO 081856Z 28015G24KT 10SM FEW014"}]}`))
	if !containsString(gusty.Signals, "gusty wind marker") {
		t.Fatalf("gusty METAR did not get gusty signal: %+v", gusty.Signals)
	}
}

func TestCollectStringsByKeysEnforcesLimitWithinMapLevel(t *testing.T) {
	payload := map[string]any{
		"reason":  "one",
		"type":    "two",
		"trend":   "three",
		"color":   "four",
		"message": "five",
		"name":    "six",
	}
	got := collectStringsByKeys(payload, 3, "reason", "type", "trend", "color", "message", "name")
	if len(got) > 3 {
		t.Fatalf("collectStringsByKeys returned %d values, want <= 3: %+v", len(got), got)
	}
}

func TestAssessDryRunShowsNASAndAeroAPIRequests(t *testing.T) {
	t.Setenv("FLIGHT_GOAT_CONFIG", t.TempDir()+"/missing.toml")
	t.Setenv("FLIGHT_GOAT_BASE_URL", "https://example.test/aeroapi")

	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetArgs([]string{
		"--dry-run",
		"assess",
		"--origin", "SFO",
		"--destination", "DCA",
		"--delayed-flight", "UAL123",
		"--no-prices",
		"--depart-after", "2026-06-08T20:00:00Z",
	})

	stderr := captureStderr(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute assess dry-run: %v", err)
		}
	})

	for _, want := range []string{
		"GET /airports/KSFO/delays",
		"GET /airports/KDCA/delays",
		"GET /flights/UAL123?start=2026-06-08T08:00:00Z&end=2026-06-10T08:00:00Z&max_pages=2",
		defaultNASStatusURL,
		"/airports/KSFO/flights/to/KDCA",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, stderr)
		}
	}
}

func TestAssessCommandReturnsProcessedPartialReport(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/airports/KSFO/delays", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"delays": []map[string]any{{"reason": "airport volume", "delay_secs": 3600}}})
	})
	mux.HandleFunc("/airports/KDCA/delays", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{})
	})
	mux.HandleFunc("/airports/KSFO/weather/observations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"observations": []map[string]any{{"raw_data": "KSFO 081856Z 28015G24KT 10SM FEW014"}}})
	})
	mux.HandleFunc("/airports/KDCA/weather/observations", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{})
	})
	mux.HandleFunc("/disruption_counts/origin/KSFO", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"delays": 147, "cancellations": 3, "total": 641})
	})
	mux.HandleFunc("/disruption_counts/destination/KDCA", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"delays": 3, "cancellations": 0, "total": 200})
	})
	mux.HandleFunc("/airports/KSFO/flights/to/KDCA", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{
			"flights": []map[string]any{{
				"segments": []map[string]any{{
					"ident":                "UAL456",
					"operator":             "United",
					"origin":               map[string]any{"code_iata": "SFO"},
					"destination":          map[string]any{"code_iata": "DCA"},
					"scheduled_out":        "2026-06-08T21:00:00Z",
					"estimated_out":        "2026-06-08T21:05:00Z",
					"registration":         "N456UA",
					"gate_origin":          "F12",
					"inbound_fa_flight_id": "UAL455-1749400000-airline-0001",
					"departure_delay":      300,
					"status":               "Scheduled",
				}},
			}},
		})
	})
	mux.HandleFunc("/flights/UAL123", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"flights": []map[string]any{{
			"ident":                "UAL123",
			"operator":             "United",
			"origin":               map[string]any{"code_iata": "SFO"},
			"destination":          map[string]any{"code_iata": "DCA"},
			"scheduled_out":        "2026-06-08T20:00:00Z",
			"estimated_out":        "2026-06-08T21:30:00Z",
			"inbound_fa_flight_id": "UAL122-1749400000-airline-0001",
			"departure_delay":      5400,
			"status":               "Delayed",
		}}})
	})
	mux.HandleFunc("/flights/UAL122-1749400000-airline-0001", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"flights": []map[string]any{{
			"ident":         "UAL122",
			"arrival_delay": 3600,
			"estimated_in":  "2026-06-08T20:45:00Z",
			"status":        "Delayed",
		}}})
	})
	mux.HandleFunc("/flights/UAL455-1749400000-airline-0001", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, map[string]any{"flights": []map[string]any{{
			"ident":     "UAL455",
			"actual_in": "2026-06-08T19:30:00Z",
			"status":    "Arrived",
		}}})
	})
	mux.HandleFunc("/nas", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<AIRPORT_STATUS_INFORMATION>
  <Update_Time>Mon Jun 8 03:32:04 2026 GMT</Update_Time>
  <Delay_type>
    <Name>Ground Delay Programs</Name>
    <Ground_Delay_List>
      <Ground_Delay>
        <ARPT>SFO</ARPT><Reason>other</Reason><Avg>1 hour</Avg><Max>2 hours</Max>
      </Ground_Delay>
    </Ground_Delay_List>
  </Delay_type>
</AIRPORT_STATUS_INFORMATION>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("FLIGHT_GOAT_CONFIG", t.TempDir()+"/missing.toml")
	t.Setenv("FLIGHT_GOAT_BASE_URL", server.URL)
	t.Setenv("FLIGHT_GOAT_API_KEY_AUTH", "test-key")

	var stdout bytes.Buffer
	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{
		"--no-cache",
		"assess",
		"--origin", "SFO",
		"--destination", "DCA",
		"--delayed-flight", "UAL123",
		"--no-prices",
		"--nas-url", server.URL + "/nas",
		"--depart-after", "2026-06-08T20:00:00Z",
		"--lookahead", "8h",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute assess: %v", err)
	}

	var report assessReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal assess report: %v\n%s", err, stdout.String())
	}
	if report.Decision.Verdict != "mixed_systemic_and_flight_specific" {
		t.Fatalf("verdict = %q, want mixed_systemic_and_flight_specific; report=%+v", report.Decision.Verdict, report.Decision)
	}
	if report.Evidence.Origin.Airport != "KSFO" || len(report.Evidence.Origin.NASEvents) != 1 {
		t.Fatalf("origin evidence missing normalized airport/NAS event: %+v", report.Evidence.Origin)
	}
	if report.DelayedFlight == nil || report.DelayedFlight.Inbound == nil {
		t.Fatalf("delayed flight did not include inbound evidence: %+v", report.DelayedFlight)
	}
	if len(report.Alternatives) != 1 || report.Alternatives[0].Ident != "UAL456" {
		t.Fatalf("alternatives not parsed from route segments: %+v", report.Alternatives)
	}
	if report.Alternatives[0].Readiness != "inbound_arrived_at_origin" {
		t.Fatalf("alternative readiness = %q, want inbound_arrived_at_origin", report.Alternatives[0].Readiness)
	}
	if report.Raw != nil {
		t.Fatalf("raw payloads should be omitted unless --include-raw is set")
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %s: %v", value, err)
	}
	return parsed
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
