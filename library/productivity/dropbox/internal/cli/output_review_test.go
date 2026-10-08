package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/config"
)

func TestOAuthStoreScopeKeepsDefaultDBPathAcrossAccessRefresh(t *testing.T) {
	if os.Getenv("DROPBOX_PP_REGEN") == "1" {
		t.Skip("patch guard: skipped while `generate --force` validates an unpatched tree; reapply patches, then run the full suite")
	}
	testenv.Isolate(t)
	t.Cleanup(func() { setDefaultDBScopeCredential("") })
	pathFor := func(cfg config.Config) string {
		t.Helper()
		setDefaultDBScopeCredential(cfg.StoreScopeCredential())
		return defaultDBPath("dropbox-pp-cli")
	}
	first := pathFor(config.Config{ClientID: "app-one", RefreshToken: "refresh-one", AccessToken: "access-one"})
	refreshed := pathFor(config.Config{ClientID: "app-one", RefreshToken: "refresh-one", AccessToken: "access-two"})
	if first != refreshed {
		t.Fatalf("access-token refresh changed default DB path: %q != %q", first, refreshed)
	}
	otherClient := pathFor(config.Config{ClientID: "app-two", RefreshToken: "refresh-one", AccessToken: "access-one"})
	if first == otherClient {
		t.Fatalf("different OAuth client IDs share DB path %q", first)
	}
	if first == filepath.Join(filepath.Dir(first), "data.db") {
		t.Fatalf("OAuth path is unscoped: %q", first)
	}
	bearer := config.Config{AuthHeaderVal: "Bearer static-token"}
	if got := bearer.StoreScopeCredential(); got != bearer.AuthHeaderVal {
		t.Fatalf("non-OAuth bearer scope = %q, want prior header behavior", got)
	}
	if pathFor(bearer) == pathFor(config.Config{AuthHeaderVal: "Bearer different-token"}) {
		t.Fatal("different non-OAuth bearer tokens share a DB path")
	}
}

func TestMissingIndexReadPayloadsIdentifyMissingIndex(t *testing.T) {
	testenv.Isolate(t)
	var quotaCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users/get_space_usage" {
			t.Errorf("unexpected quota path %q", r.URL.Path)
		}
		quotaCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"used":12,"allocation":{".tag":"individual","allocated":100}}`))
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	missing := filepath.Join(t.TempDir(), "missing.db")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"overview", nil},
		{"mess", nil},
		{"dupes", nil},
		{"tree", nil},
		{"search", []string{"invoice"}},
		{"conflicts", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{tc.name}, tc.args...)
			args = append(args, "--db", missing, "--json")
			data, err := runRead(t, args...)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				IndexMissing bool   `json:"index_missing"`
				Note         string `json:"note"`
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if !got.IndexMissing || got.Note != missingIndexNote {
				t.Fatalf("missing-index marker absent in %s: %s", tc.name, data)
			}
			if tc.name == "overview" {
				var overview overviewResult
				if err := json.Unmarshal(data, &overview); err != nil {
					t.Fatal(err)
				}
				if overview.Quota == nil || overview.Quota.Used != 12 || overview.Totals.Files != 0 {
					t.Fatalf("missing-index overview lost live quota or has aggregates: %s", data)
				}
			}
			agentArgs := append([]string{tc.name}, tc.args...)
			agentArgs = append(agentArgs, "--db", missing, "--agent")
			data, err = runRead(t, agentArgs...)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Meta struct {
					Source string `json:"source"`
				} `json:"meta"`
				Results struct {
					IndexMissing bool   `json:"index_missing"`
					Note         string `json:"note"`
				} `json:"results"`
			}
			if err := json.Unmarshal(data, &envelope); err != nil {
				t.Fatal(err)
			}
			if !envelope.Results.IndexMissing || envelope.Results.Note != missingIndexNote {
				t.Fatalf("agent output lost missing-index marker in %s: %s", tc.name, data)
			}
			if tc.name == "overview" && envelope.Meta.Source != "local" {
				t.Fatalf("overview labeled local aggregates %q: %s", envelope.Meta.Source, data)
			}
		})
	}
	if quotaCalls.Load() != 2 {
		t.Fatalf("overview called quota endpoint %d times, want 2", quotaCalls.Load())
	}
}

func TestMissingIndexOverviewKeepsQuotaFailureReason(t *testing.T) {
	testenv.Isolate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error_summary":"invalid_access_token/","error":{".tag":"invalid_access_token"}}`))
	}))
	defer server.Close()
	t.Setenv("DROPBOX_BASE_URL", server.URL)
	missing := filepath.Join(t.TempDir(), "missing.db")
	data, err := runRead(t, "overview", "--db", missing, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got overviewResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !got.IndexMissing || got.Quota != nil {
		t.Fatalf("want missing index and null quota: %s", data)
	}
	if !strings.HasPrefix(got.Note, missingIndexNote+"; quota unavailable: ") {
		t.Fatalf("null quota has no stated reason: %q", got.Note)
	}
}
