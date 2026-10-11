package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJobsLiveCompleteAndEnriched(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Errorf("unexpected mutation %s", r.Method)
		}
		switch r.URL.Path {
		case "/jobs":
			if r.URL.Query().Get("from") != "2026-09-01" || r.URL.Query().Get("propertiesUids") != "p" {
				t.Errorf("query %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"jobs":[{"uid":"j","propertyUid":"p","serviceUid":"s","status":"CANCELLED","startLocalDateTime":"2026-09-01T11:00:00"}],"_metadata":{"count":1}}`))
		case "/services/s":
			w.Write([]byte(`{"service":{"uid":"s","type":"CLEANING_SERVICE","title":"Cleaning","serviceProviderUid":"v"}}`))
		case "/service-providers/v":
			w.Write([]byte(`{"serviceProvider":{"uid":"v","firstName":"Test","lastName":"Cleaner"}}`))
		default:
			t.Errorf("path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	t.Setenv("HOSTFULLY_BASE_URL", server.URL)
	t.Setenv("HOSTFULLY_API_KEY", "test")
	var out bytes.Buffer
	cmd := newNovelJobsListCmd(&rootFlags{timeout: time.Second, agent: true})
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--from", "2026-09-01", "--to", "2026-09-02", "--property-uids", "p"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result jobsResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if calls != 3 || !result.Complete || len(result.Jobs) != 1 || result.Jobs[0].Status != "CANCELLED" || result.Providers["v"].First != "Test" {
		t.Fatalf("unexpected result %+v; calls %d", result, calls)
	}
}

func TestJobsRejectsIncompleteAndMalformed(t *testing.T) {
	for _, body := range []string{`{"jobs":[],"_metadata":{"totalCount":1}}`, `{}`, `{"jobs":[{"uid":"j"}]}`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
			defer server.Close()
			t.Setenv("HOSTFULLY_BASE_URL", server.URL)
			t.Setenv("HOSTFULLY_API_KEY", "test")
			cmd := newNovelJobsListCmd(&rootFlags{timeout: time.Second})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--from", "2026-09-01", "--to", "2026-09-02"})
			if err := cmd.Execute(); err == nil {
				t.Fatal("accepted incomplete data")
			}
		})
	}
}
func TestJobsMockNoCredentialsOrNetwork(t *testing.T) {
	t.Setenv("HOSTFULLY_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("HOSTFULLY_API_KEY", "")
	cmd := newNovelJobsListCmd(&rootFlags{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--mock", "--from", "2026-09-01", "--to", "2026-09-02"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result jobsResult
	json.Unmarshal(out.Bytes(), &result)
	if result.Source != "mock" {
		t.Fatal(out.String())
	}
}
