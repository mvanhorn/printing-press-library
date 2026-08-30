// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/config"
)

func TestIcpGetExpiredSessionFallsBackToOpenAPI(t *testing.T) {
	var jwtCalls, openCalls int
	jwtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwtCalls++
		if r.URL.Path != "/locations" {
			t.Errorf("JWT path = %q, want /locations", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer expired-token" {
			t.Errorf("Authorization = %q, want expired customer token", got)
		}
		http.Error(w, `{"error":"Token validation failed"}`, http.StatusUnauthorized)
	}))
	defer jwtServer.Close()

	openServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		openCalls++
		if r.URL.Path != "/testgym/locations" {
			t.Errorf("Open API path = %q, want /testgym/locations", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": 7, "name": "Main", "active": true}},
		})
	}))
	defer openServer.Close()

	installSyntheticCustomerSession(t, "testgym", "expired-token", jwtServer.URL)
	c := client.New(&config.Config{BaseURL: openServer.URL}, time.Second, 0)
	c.BaseURL = openServer.URL
	c.NoCache = true

	rows, _, gate, err := icpGet(context.Background(), c, "/testgym/locations", nil)
	if err != nil {
		t.Fatal(err)
	}
	if gate != icpGateNone || len(rows) != 1 {
		t.Fatalf("gate = %q, rows = %d; want open with one row", gate, len(rows))
	}
	if jwtCalls != 1 || openCalls != 1 {
		t.Fatalf("JWT calls = %d, open calls = %d; want 1 each", jwtCalls, openCalls)
	}
	if c.BaseURL != openServer.URL {
		t.Fatalf("client base URL = %q, want restored Open API base", c.BaseURL)
	}

	data, prov, err := resolveReadWithStrategyAndResponsePath(
		context.Background(), c, &rootFlags{}, "live", "locations", false,
		"/testgym/locations", nil, nil, "data", io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	var generatedRows []map[string]any
	if err := json.Unmarshal(data, &generatedRows); err != nil {
		t.Fatal(err)
	}
	if prov.Source != "live" || len(generatedRows) != 1 {
		t.Fatalf("generated read source = %q, rows = %d; want live with one row", prov.Source, len(generatedRows))
	}
	if jwtCalls != 2 || openCalls != 2 {
		t.Fatalf("combined JWT calls = %d, open calls = %d; want 2 each", jwtCalls, openCalls)
	}
}

func TestIcpGetValidSessionDoesNotCallOpenAPI(t *testing.T) {
	var jwtCalls, openCalls int
	jwtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwtCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"id": 9, "name": "Members", "active": true}},
		})
	}))
	defer jwtServer.Close()
	openServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		openCalls++
		http.Error(w, "unexpected open fallback", http.StatusInternalServerError)
	}))
	defer openServer.Close()

	installSyntheticCustomerSession(t, "testgym", "valid-token", jwtServer.URL)
	c := client.New(&config.Config{BaseURL: openServer.URL}, time.Second, 0)
	c.BaseURL = openServer.URL

	rows, _, _, err := icpGet(context.Background(), c, "/testgym/locations", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || jwtCalls != 1 || openCalls != 0 {
		t.Fatalf("rows = %d, JWT calls = %d, open calls = %d; want 1, 1, 0", len(rows), jwtCalls, openCalls)
	}
}

func TestIcpGetDoesNotMaskNonUnauthorizedSessionErrors(t *testing.T) {
	var openCalls int
	jwtServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer jwtServer.Close()
	openServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		openCalls++
		http.Error(w, "unexpected open fallback", http.StatusInternalServerError)
	}))
	defer openServer.Close()

	installSyntheticCustomerSession(t, "testgym", "valid-token", jwtServer.URL)
	c := client.New(&config.Config{BaseURL: openServer.URL}, time.Second, 0)
	c.BaseURL = openServer.URL

	_, _, _, err := icpGet(context.Background(), c, "/testgym/locations", nil)
	if err == nil || icpIsUnauthorized(err) {
		t.Fatalf("error = %v, want non-401 failure", err)
	}
	if openCalls != 0 {
		t.Fatalf("open calls = %d, want 0", openCalls)
	}
}

func installSyntheticCustomerSession(t *testing.T, account, token, jwtBase string) {
	t.Helper()
	previousJWTBase := icpJWTBase
	icpJWTBase = jwtBase
	icpSessionOnce = sync.Once{}
	icpSessionOnce.Do(func() {
		icpSessionCache = icpSessionFile{
			Sessions: map[string]icpSession{
				account: {Token: token, Email: "reader@example.test", Endpoint: jwtBase},
			},
			StaffSessions: map[string]icpStaffSession{},
		}
	})
	t.Cleanup(func() {
		icpJWTBase = previousJWTBase
		icpSessionOnce = sync.Once{}
		icpSessionCache = icpSessionFile{}
	})
}
