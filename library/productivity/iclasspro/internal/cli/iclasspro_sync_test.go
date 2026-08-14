package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/store"
)

func TestIcpCollectMarksOnlyIncompletePageCapsTruncated(t *testing.T) {
	tests := []struct {
		name          string
		totalRecords  int
		wantTruncated bool
	}{
		{name: "more rows remain", totalRecords: 3, wantTruncated: true},
		{name: "last full page is complete", totalRecords: 2, wantTruncated: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/testgym/locations":
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": []map[string]any{{"id": 7, "name": "Main", "active": true}},
					})
				case "/testgym/classes":
					if got := r.URL.Query().Get("page"); got != "1" {
						t.Errorf("page = %q, want 1", got)
					}
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": []map[string]any{
							{"id": 101, "name": "Class A"},
							{"id": 102, "name": "Class B"},
						},
						"totalRecords": tt.totalRecords,
					})
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()

			c := client.New(&config.Config{BaseURL: server.URL}, time.Second, 0)
			c.BaseURL = server.URL
			coll, err := icpCollect(context.Background(), c, "testgym", icpCollectOptions{
				IncludeClasses: true,
				MaxPages:       1,
				PageSize:       2,
			})
			if err != nil {
				t.Fatal(err)
			}
			if coll.Truncated != tt.wantTruncated {
				t.Fatalf("Truncated = %v, want %v (warnings: %v)", coll.Truncated, tt.wantTruncated, coll.Warnings)
			}
			if tt.wantTruncated && (len(coll.Warnings) == 0 || !strings.Contains(coll.Warnings[0], "page cap")) {
				t.Fatalf("missing page-cap warning: %v", coll.Warnings)
			}
		})
	}
}

func TestIcpRecordSyncObservationsDoesNotPromotePartialWalk(t *testing.T) {
	ctx := context.Background()
	s, err := store.OpenWithContext(ctx, filepath.Join(t.TempDir(), "mirror.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	at := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	complete := []store.ICPObservation{
		{Account: "testgym", Kind: "class", EntityID: 101, Name: "Class A", Data: json.RawMessage(`{"id":101}`)},
		{Account: "testgym", Kind: "class", EntityID: 102, Name: "Class B", Data: json.RawMessage(`{"id":102}`)},
	}
	runID, err := icpRecordSyncObservations(ctx, s, "testgym", at, complete, true)
	if err != nil {
		t.Fatal(err)
	}
	if runID == 0 {
		t.Fatal("complete walk did not create a snapshot run")
	}

	partial := complete[:1]
	partialRunID, err := icpRecordSyncObservations(ctx, s, "testgym", at.Add(time.Second), partial, false)
	if err != nil {
		t.Fatal(err)
	}
	if partialRunID != 0 {
		t.Fatalf("partial walk created run %d, want 0", partialRunID)
	}

	runs, err := s.ICPRuns(ctx, "testgym", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 1 || runs[0].ID != runID {
		t.Fatalf("snapshot runs = %#v, want only complete run %d", runs, runID)
	}
	snapshot, err := s.ICPSnapshot(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot) != 2 {
		t.Fatalf("complete snapshot rows = %d, want 2", len(snapshot))
	}
	history, err := s.ICPHistory(ctx, "testgym", at.Add(-time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("openings observations = %d, want 3 (complete + partial)", len(history))
	}
}

func TestIcpSyncSnapshotCompleteRequiresWholeCatalog(t *testing.T) {
	tests := []struct {
		name                    string
		wantClasses, wantCamps  bool
		truncated, wantComplete bool
	}{
		{name: "complete whole catalog", wantClasses: true, wantCamps: true, wantComplete: true},
		{name: "classes only", wantClasses: true, wantCamps: false},
		{name: "camps only", wantClasses: false, wantCamps: true},
		{name: "whole catalog page capped", wantClasses: true, wantCamps: true, truncated: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := icpSyncSnapshotComplete(tt.wantClasses, tt.wantCamps, tt.truncated); got != tt.wantComplete {
				t.Fatalf("icpSyncSnapshotComplete(%v, %v, %v) = %v, want %v", tt.wantClasses, tt.wantCamps, tt.truncated, got, tt.wantComplete)
			}
		})
	}
}
