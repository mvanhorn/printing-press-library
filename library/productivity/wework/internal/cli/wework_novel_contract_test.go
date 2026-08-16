// Copyright 2026 Paul Byrne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/wework/internal/cliutil/testenv"
)

func isolateNovelCLI(t *testing.T, baseURL string) {
	t.Helper()
	testenv.Isolate(t, cliutil.ConfigDir, cliutil.DataDir, cliutil.StateDir)
	t.Setenv("WEWORK_BASE_URL", baseURL)
	t.Setenv("WEWORK_TOKEN", authTestJWT(`{"iss":"https://idp.wework.com/","azp":"public-client","exp":4102444800}`))
	t.Setenv("WEWORK_REFRESH_TOKEN", "")
	t.Setenv("WEWORK_UUID", "account-test")
	t.Setenv("WEWORK_MEMBER_TYPE", "3")
	t.Setenv(noLearnEnvVar, "true")
}

func executeNovelCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestDesksRejectsInvalidDateBeforeNetwork(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	isolateNovelCLI(t, server.URL)

	_, _, err := executeNovelCLI(t,
		"desks", "--city", "Austin, TX", "--date", "not-a-date",
		"--data-source", "live", "--agent", "--no-learn",
	)
	if err == nil || !strings.Contains(err.Error(), "--date must be YYYY-MM-DD") {
		t.Fatalf("desks invalid date error = %v, want YYYY-MM-DD usage error", err)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("desks invalid date exit code = %d, want 2", got)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("desks invalid date made %d HTTP request(s), want 0", got)
	}
}

func TestLocationsRejectsInvalidDateBeforeNetwork(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	isolateNovelCLI(t, server.URL)

	_, _, err := executeNovelCLI(t,
		"locations", "--city", "Austin, TX", "--date", "not-a-date",
		"--data-source", "live", "--agent", "--no-learn",
	)
	if err == nil || !strings.Contains(err.Error(), "--date must be YYYY-MM-DD") {
		t.Fatalf("locations invalid date error = %v, want YYYY-MM-DD usage error", err)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("locations invalid date exit code = %d, want 2", got)
	}
	if got := hits.Load(); got != 0 {
		t.Fatalf("locations invalid date made %d HTTP request(s), want 0", got)
	}
}

func TestLiveOnlyCommandsRejectLocalDataSourceBeforeNetwork(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "cities", args: []string{"cities"}},
		{name: "desks", args: []string{"desks", "--city", "Austin, TX", "--date", "2026-08-18"}},
		{name: "locations", args: []string{"locations", "--city", "Austin, TX", "--date", "2026-08-18"}},
		{name: "bookings", args: []string{"bookings"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			t.Cleanup(server.Close)
			isolateNovelCLI(t, server.URL)

			args := append(append([]string{}, tc.args...), "--data-source", "local", "--agent", "--no-learn")
			_, _, err := executeNovelCLI(t, args...)
			if err == nil || !strings.Contains(err.Error(), "no local data source") {
				t.Fatalf("%s --data-source local error = %v, want live-only usage error", tc.name, err)
			}
			if got := ExitCode(err); got != 2 {
				t.Fatalf("%s --data-source local exit code = %d, want 2", tc.name, got)
			}
			if got := hits.Load(); got != 0 {
				t.Fatalf("%s --data-source local made %d HTTP request(s), want 0", tc.name, got)
			}
		})
	}
}

func TestCitiesAgentOutputReportsLiveSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/wework-yardi/location/get-affiliate-cities" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"Austin","marketgeo":{"name":"Austin","latitude":30.2672,"longitude":-97.7431},"countrygeo":{"name":"US","iso":"US"}}]`))
	}))
	t.Cleanup(server.Close)
	isolateNovelCLI(t, server.URL)

	stdout, stderr, err := executeNovelCLI(t,
		"cities", "--data-source", "live", "--agent", "--no-learn",
	)
	if err != nil {
		t.Fatalf("cities live agent error = %v (stderr=%q)", err, stderr)
	}
	var envelope struct {
		Meta    map[string]any   `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("cities live agent output is not JSON: %v (stdout=%q)", err, stdout)
	}
	if got := envelope.Meta["source"]; got != "live" {
		t.Fatalf("cities live agent source = %#v, want live", got)
	}
	if len(envelope.Results) != 1 || envelope.Results[0]["name"] != "Austin" {
		t.Fatalf("cities live agent results = %#v, want Austin", envelope.Results)
	}
}

func TestOtherLiveCommandsReportLiveSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wework-yardi/location/get-affiliate-cities":
			_, _ = w.Write([]byte(`[{"name":"Austin","marketgeo":{"name":"Austin","latitude":30.2672,"longitude":-97.7431}}]`))
		case "/spaces/get-affiliate-locations":
			_, _ = w.Write([]byte(`{"locationsByGeo":[{"uuid":"100"}]}`))
		case "/spaces/get-spaces":
			_, _ = w.Write([]byte(`{"getSharedWorkspaces":{"totalCount":1,"workspaces":[{"uuid":"desk-1","credits":1,"seatsAvailable":2,"productPrice":{"price":{"amount":25}},"location":{"name":"Barton Springs"}}]}}`))
		case "/wework-yardi/ondemand/get-locations-by-geo":
			_, _ = w.Write([]byte(`{"locationsByGeo":[{"uuid":"location-1","name":"Barton Springs","address":{"line1":"801 Barton Springs Rd","city":"Austin","state":"TX","zip":"78704","country":"US"},"timeZone":"America/Chicago","accountType":1,"spaceAvailabilityCount":2}]}`))
		case "/common-booking/inventory-details":
			_, _ = w.Write([]byte(`{"kubeSpaceId":123,"inventoryUuid":"inventory-1","price":{"amount":25,"currency":"USD"},"inventory":{"availableSeats":2,"capacity":10,"spaceType":1}}`))
		case "/common-booking/upcoming-bookings":
			_, _ = w.Write([]byte(`{"AllowReservation":false,"IsInactive":false,"WeWorkBookings":[]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	isolateNovelCLI(t, server.URL)

	cases := []struct {
		name string
		args []string
	}{
		{name: "desks", args: []string{"desks", "--city", "Austin, TX", "--date", "2026-08-18"}},
		{name: "locations", args: []string{"locations", "--city", "Austin, TX", "--date", "2026-08-18"}},
		{name: "bookings", args: []string{"bookings"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, tc.args...), "--data-source", "live", "--agent", "--no-learn")
			stdout, stderr, err := executeNovelCLI(t, args...)
			if err != nil {
				t.Fatalf("%s live agent error = %v (stderr=%q)", tc.name, err, stderr)
			}
			var envelope struct {
				Meta map[string]any `json:"meta"`
			}
			if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
				t.Fatalf("%s live agent output is not JSON: %v (stdout=%q)", tc.name, err, stdout)
			}
			if got := envelope.Meta["source"]; got != "live" {
				t.Fatalf("%s live agent source = %#v, want live", tc.name, got)
			}
		})
	}
}

func TestCitiesFilterMatchesCityIdentityNotNearbyAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"name":"Waco","marketgeo":{"name":"Waco"},"countrygeo":{"name":"US","iso":"US"},"nearby_location":{"address":"510 Austin Avenue, Waco, TX","city":"Waco","state":"TX"}},
			{"name":"Austin","marketgeo":{"name":"Austin"},"countrygeo":{"name":"US","iso":"US"},"nearby_location":{"address":"801 Barton Springs Rd, Austin, TX","city":"Austin","state":"TX"}}
		]`))
	}))
	t.Cleanup(server.Close)
	isolateNovelCLI(t, server.URL)

	stdout, stderr, err := executeNovelCLI(t,
		"cities", "--filter", "Austin, TX", "--data-source", "live", "--agent", "--no-learn",
	)
	if err != nil {
		t.Fatalf("cities identity filter error = %v (stderr=%q)", err, stderr)
	}
	var envelope struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("cities identity filter output is not JSON: %v (stdout=%q)", err, stdout)
	}
	if len(envelope.Results) != 1 || envelope.Results[0].Name != "Austin" {
		t.Fatalf("cities identity filter results = %#v, want only Austin", envelope.Results)
	}
}

func newLocationsFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wework-yardi/location/get-affiliate-cities":
			_, _ = w.Write([]byte(`[{"name":"Austin","marketgeo":{"name":"Austin","latitude":30.2672,"longitude":-97.7431}}]`))
		case "/wework-yardi/ondemand/get-locations-by-geo":
			_, _ = w.Write([]byte(`{"locationsByGeo":[{"uuid":"location-1","name":"Barton Springs","address":{"line1":"801 Barton Springs Rd","city":"Austin","state":"TX","zip":"78704","country":"US"},"timeZone":"America/Chicago","accountType":1,"spaceAvailabilityCount":2}]}`))
		case "/common-booking/inventory-details":
			_, _ = w.Write([]byte(`{"kubeSpaceId":123,"inventoryUuid":"inventory-1","price":{"amount":25,"currency":"USD"},"inventory":{"availableSeats":2,"capacity":10,"spaceType":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestLocationsCSVOutputsLocationRows(t *testing.T) {
	server := newLocationsFixtureServer(t)
	isolateNovelCLI(t, server.URL)

	stdout, stderr, err := executeNovelCLI(t,
		"locations", "--city", "Austin, TX", "--date", "2026-08-18",
		"--data-source", "live", "--csv", "--no-learn",
	)
	if err != nil {
		t.Fatalf("locations CSV error = %v (stderr=%q)", err, stderr)
	}
	records, err := csv.NewReader(strings.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("locations CSV is invalid: %v (stdout=%q)", err, stdout)
	}
	if len(records) != 2 {
		t.Fatalf("locations CSV record count = %d, want header + one row (stdout=%q)", len(records), stdout)
	}
	header := strings.Join(records[0], ",")
	if !strings.Contains(header, "locationId") || !strings.Contains(header, "name") {
		t.Fatalf("locations CSV header = %q, want locationId and name", header)
	}
}

func TestLocationsQuietSuppressesOutput(t *testing.T) {
	server := newLocationsFixtureServer(t)
	isolateNovelCLI(t, server.URL)

	stdout, stderr, err := executeNovelCLI(t,
		"locations", "--city", "Austin, TX", "--date", "2026-08-18",
		"--data-source", "live", "--quiet", "--no-learn",
	)
	if err != nil {
		t.Fatalf("locations quiet error = %v (stderr=%q)", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("locations quiet stdout = %q, want empty", stdout)
	}
}

func newDesksFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/wework-yardi/location/get-affiliate-cities":
			_, _ = w.Write([]byte(`[{"name":"Austin","marketgeo":{"name":"Austin","latitude":30.2672,"longitude":-97.7431}}]`))
		case "/spaces/get-affiliate-locations":
			_, _ = w.Write([]byte(`{"locationsByGeo":[{"uuid":"100"}]}`))
		case "/spaces/get-spaces":
			_, _ = w.Write([]byte(`{"getSharedWorkspaces":{"totalCount":1,"workspaces":[{"uuid":"desk-1","credits":1,"seatsAvailable":2,"productPrice":{"price":{"amount":25}},"location":{"name":"Barton Springs"}}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestDesksCSVOutputsDeskRows(t *testing.T) {
	server := newDesksFixtureServer(t)
	isolateNovelCLI(t, server.URL)

	stdout, stderr, err := executeNovelCLI(t,
		"desks", "--city", "Austin, TX", "--date", "2026-08-18",
		"--data-source", "live", "--csv", "--no-learn",
	)
	if err != nil {
		t.Fatalf("desks CSV error = %v (stderr=%q)", err, stderr)
	}
	records, err := csv.NewReader(strings.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("desks CSV is invalid: %v (stdout=%q)", err, stdout)
	}
	if len(records) != 2 {
		t.Fatalf("desks CSV record count = %d, want header + one row (stdout=%q)", len(records), stdout)
	}
	header := strings.Join(records[0], ",")
	if !strings.Contains(header, "uuid") || !strings.Contains(header, "location") {
		t.Fatalf("desks CSV header = %q, want uuid and location", header)
	}
}
