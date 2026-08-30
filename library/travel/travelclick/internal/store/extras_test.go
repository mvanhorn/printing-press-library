// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestQueryRateSnapshotsOrdersByCapturedAtThenID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	sameStamp := "2026-08-10T00:00:00Z"
	rows := []*RateSnapshot{
		{
			HotelID:      "102306",
			CheckIn:      "2026-09-15",
			CheckOut:     "2026-09-18",
			RoomTypeCode: "B",
			RatePlanCode: "CHEAP",
			NightlyRate:  50,
			Currency:     "USD",
			CapturedAt:   sameStamp,
		},
		{
			HotelID:      "102306",
			CheckIn:      "2026-09-15",
			CheckOut:     "2026-09-18",
			RoomTypeCode: "A",
			RatePlanCode: "BAR",
			NightlyRate:  120,
			Currency:     "USD",
			CapturedAt:   sameStamp,
		},
		{
			HotelID:      "102306",
			CheckIn:      "2026-09-15",
			CheckOut:     "2026-09-18",
			RoomTypeCode: "A",
			RatePlanCode: "BAR",
			NightlyRate:  100,
			Currency:     "USD",
			CapturedAt:   "2026-08-01T00:00:00Z",
		},
	}
	for _, row := range rows {
		if err := s.InsertRateSnapshot(ctx, row); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	got, err := s.QueryRateSnapshots(ctx, "102306")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(got))
	}
	if got[0].CapturedAt != "2026-08-01T00:00:00Z" || got[0].NightlyRate != 100 {
		t.Fatalf("expected earliest BAR 100 first, got %+v", got[0])
	}
	if got[1].RoomTypeCode != "B" || got[1].ID >= got[2].ID {
		t.Fatalf("same-timestamp rows must stay insertion/id order: %+v then %+v", got[1], got[2])
	}
	if got[2].RoomTypeCode != "A" || got[2].NightlyRate != 120 {
		t.Fatalf("expected later same-timestamp BAR 120 last, got %+v", got[2])
	}
}

func TestQueryRateSnapshotsSameTimestampOrderIsDeterministic(t *testing.T) {
	ctx := context.Background()
	sameStamp := "2026-08-10T00:00:00Z"

	insertOrders := [][]RateSnapshot{
		{
			{HotelID: "102306", CheckIn: "2026-09-15", CheckOut: "2026-09-18", RoomTypeCode: "A", RatePlanCode: "BAR", NightlyRate: 120, Currency: "USD", CapturedAt: sameStamp},
			{HotelID: "102306", CheckIn: "2026-09-15", CheckOut: "2026-09-18", RoomTypeCode: "B", RatePlanCode: "CHEAP", NightlyRate: 50, Currency: "USD", CapturedAt: sameStamp},
		},
		{
			{HotelID: "102306", CheckIn: "2026-09-15", CheckOut: "2026-09-18", RoomTypeCode: "B", RatePlanCode: "CHEAP", NightlyRate: 50, Currency: "USD", CapturedAt: sameStamp},
			{HotelID: "102306", CheckIn: "2026-09-15", CheckOut: "2026-09-18", RoomTypeCode: "A", RatePlanCode: "BAR", NightlyRate: 120, Currency: "USD", CapturedAt: sameStamp},
		},
	}

	for i, order := range insertOrders {
		dbPath := filepath.Join(t.TempDir(), "data.db")
		s, err := Open(dbPath)
		if err != nil {
			t.Fatalf("open store %d: %v", i, err)
		}
		for _, row := range order {
			sn := row
			if err := s.InsertRateSnapshot(ctx, &sn); err != nil {
				t.Fatalf("insert order %d: %v", i, err)
			}
		}
		got, err := s.QueryRateSnapshots(ctx, "102306")
		s.Close()
		if err != nil {
			t.Fatalf("query order %d: %v", i, err)
		}
		if len(got) != 2 || got[0].ID > got[1].ID {
			t.Fatalf("order %d: expected id-ascending same-timestamp rows, got %+v", i, got)
		}
		if got[0].RoomTypeCode != order[0].RoomTypeCode {
			t.Fatalf("order %d: expected insertion order preserved by id, first=%s", i, got[0].RoomTypeCode)
		}
	}
}
