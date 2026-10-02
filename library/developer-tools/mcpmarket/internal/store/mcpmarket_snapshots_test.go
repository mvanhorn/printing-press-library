// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCaptureSnapshotSameDayRefreshReplacesOnlyRequestedTypes(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	for _, resource := range []struct {
		resourceType string
		id           string
	}{
		{resourceType: "server", id: "keep"},
		{resourceType: "server", id: "removed"},
		{resourceType: "skill", id: "preserved-skill"},
	} {
		data := json.RawMessage(`{"id":"` + resource.id + `"}`)
		if err := s.Upsert(resource.resourceType, resource.id, data); err != nil {
			t.Fatalf("upsert %s/%s: %v", resource.resourceType, resource.id, err)
		}
	}

	date, captured, err := s.CaptureSnapshot(ctx, "server", "skill")
	if err != nil {
		t.Fatalf("initial capture: %v", err)
	}
	if captured != 3 {
		t.Fatalf("initial captured = %d, want 3", captured)
	}

	if _, err := s.DB().ExecContext(ctx,
		`DELETE FROM resources WHERE (resource_type = 'server' AND id = 'removed') OR resource_type = 'skill'`,
	); err != nil {
		t.Fatalf("delete current resources: %v", err)
	}

	refreshedDate, captured, err := s.CaptureSnapshot(ctx, "server")
	if err != nil {
		t.Fatalf("same-day server refresh: %v", err)
	}
	if refreshedDate != date {
		t.Fatalf("refreshed date = %q, want %q", refreshedDate, date)
	}
	if captured != 1 {
		t.Fatalf("refreshed captured = %d, want 1", captured)
	}

	serverRows, err := s.SnapshotRows(ctx, date, "server")
	if err != nil {
		t.Fatalf("server snapshot rows: %v", err)
	}
	if len(serverRows) != 1 || serverRows[0].ResourceID != "keep" {
		t.Fatalf("server snapshot rows = %+v, want only keep", serverRows)
	}

	// The refresh was explicitly scoped to servers. Even though the skill was
	// removed from the current resources table, its existing snapshot must not
	// be altered by that server-only capture.
	skillRows, err := s.SnapshotRows(ctx, date, "skill")
	if err != nil {
		t.Fatalf("skill snapshot rows: %v", err)
	}
	if len(skillRows) != 1 || skillRows[0].ResourceID != "preserved-skill" {
		t.Fatalf("skill snapshot rows = %+v, want preserved-skill", skillRows)
	}

	if _, err := s.DB().ExecContext(ctx, `DELETE FROM resources WHERE resource_type = 'server'`); err != nil {
		t.Fatal(err)
	}
	if _, captured, err := s.CaptureSnapshot(ctx, "server"); err != nil || captured != 0 {
		t.Fatalf("empty scoped refresh captured=%d error=%v", captured, err)
	}
	serverRows, err = s.SnapshotRows(ctx, date, "server")
	if err != nil || len(serverRows) != 0 {
		t.Fatalf("empty scoped refresh left server rows=%v error=%v", serverRows, err)
	}
	skillRows, err = s.SnapshotRows(ctx, date, "skill")
	if err != nil || len(skillRows) != 1 {
		t.Fatalf("empty server refresh removed skill rows=%v error=%v", skillRows, err)
	}
}

func TestSnapshotDateQueriesAreScopedByResourceType(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	for _, snapshot := range []struct {
		resourceType string
		resourceID   string
		date         string
	}{
		{resourceType: "server", resourceID: "server-1", date: "2026-08-01"},
		{resourceType: "skill", resourceID: "skill-1", date: "2026-08-05"},
	} {
		if _, err := s.DB().ExecContext(ctx, `
			INSERT INTO resource_snapshots (resource_type, resource_id, data, snapshot_date, captured_at)
			VALUES (?, ?, '{}', ?, '2026-08-05T00:00:00Z')
		`, snapshot.resourceType, snapshot.resourceID, snapshot.date); err != nil {
			t.Fatalf("insert %s snapshot: %v", snapshot.resourceType, err)
		}
	}

	date, ok, err := s.NearestSnapshotDateOnOrBefore(ctx, "2026-08-06", "server")
	if err != nil {
		t.Fatalf("nearest server date: %v", err)
	}
	if !ok || date != "2026-08-01" {
		t.Fatalf("nearest server date = %q, %v; want 2026-08-01, true", date, ok)
	}

	date, ok, err = s.NearestSnapshotDateOnOrBefore(ctx, "2026-08-06", "skill")
	if err != nil {
		t.Fatalf("nearest skill date: %v", err)
	}
	if !ok || date != "2026-08-05" {
		t.Fatalf("nearest skill date = %q, %v; want 2026-08-05, true", date, ok)
	}

	dates, err := s.SnapshotDates(ctx, "server")
	if err != nil {
		t.Fatalf("server snapshot dates: %v", err)
	}
	if want := []string{"2026-08-01"}; !reflect.DeepEqual(dates, want) {
		t.Fatalf("server snapshot dates = %v, want %v", dates, want)
	}

	if date, ok, err = s.NearestSnapshotDateOnOrBefore(ctx, "2026-08-06", "mcpclient"); err != nil {
		t.Fatalf("nearest missing type: %v", err)
	} else if ok || date != "" {
		t.Fatalf("nearest missing type = %q, %v; want empty, false", date, ok)
	}
}
