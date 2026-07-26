// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestReplaceDashboardObservationsRollsBackFailedReplacement(t *testing.T) {
	database, err := Open(filepath.Join(t.TempDir(), "dashboard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	old := json.RawMessage(`{
		"id":"old",
		"_observation_type":"listings",
		"_observed_at":"2026-07-01T00:00:00Z"
	}`)
	if err := database.Upsert("ads", "old", old); err != nil {
		t.Fatal(err)
	}

	err = database.ReplaceDashboardObservations(
		context.Background(),
		"ads",
		[]string{"listings"},
		[]DashboardObservation{
			{
				ID: "new", Label: "listings",
				Data: json.RawMessage(`{"id":"new","_observation_type":"listings"}`),
			},
			{ID: "broken", Label: "listings", Data: json.RawMessage(`{`)},
		},
	)
	if err == nil {
		t.Fatal("expected malformed replacement to fail")
	}

	items, err := database.List("ads", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || string(items[0]) != string(old) {
		t.Fatalf("old observation was not preserved after rollback: %s", items)
	}
}
