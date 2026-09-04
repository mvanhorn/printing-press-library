// Copyright 2026 Paul Gradeff and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/monitoring/utah-pmn/internal/client"
)

func testPMNClient(t *testing.T, h http.HandlerFunc) *client.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &client.Client{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
		NoCache:    true,
	}
}

func TestSweepLocationsPartialFailure(t *testing.T) {
	t.Parallel()
	c := testPMNClient(t, func(w http.ResponseWriter, r *http.Request) {
		loc := r.URL.Query().Get("zipOrCity")
		if loc == "Failville" {
			http.Error(w, "upstream timeout", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"noticeDtoList": []pmnNotice{{
				NoticeID:       1,
				PublicBodyName: "Delta City Council",
				EntityName:     loc,
			}},
		})
	})

	got, err := sweepLocations(context.Background(), c, []string{"Delta", "Failville"}, "", "", 10)
	if err == nil {
		t.Fatal("expected incomplete sweep error")
	}
	if got != nil {
		t.Fatalf("partial sweep returned notices %v", got)
	}
	msg := err.Error()
	if !strings.Contains(msg, "Failville") || !strings.Contains(msg, "1/2 locations failed") {
		t.Fatalf("error %q should name Failville and the failure count", msg)
	}
}

func TestSweepLocationsAllSucceed(t *testing.T) {
	t.Parallel()
	c := testPMNClient(t, func(w http.ResponseWriter, r *http.Request) {
		loc := r.URL.Query().Get("zipOrCity")
		id := int64(1)
		if loc == "Fillmore" {
			id = 2
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"noticeDtoList": []pmnNotice{{
				NoticeID:         id,
				PublicBodyName:   loc + " City Council",
				EntityName:       loc,
				MeetingStartTime: "2026-07-01",
			}},
		})
	})

	got, err := sweepLocations(context.Background(), c, []string{"Delta", "Fillmore"}, "", "", 10)
	if err != nil {
		t.Fatalf("sweepLocations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}
