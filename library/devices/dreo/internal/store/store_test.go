// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	p := filepath.Join(t.TempDir(), "store.db")
	s, err := Open(p)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUpsertAndListDevices(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	devs := []Device{
		{Sn: "sn-A", Name: "Fan", Model: "DR-HPF004S", Room: "Office", ProductID: 1, Online: true, UpdatedAt: time.Now()},
		{Sn: "sn-B", Name: "Heater", Model: "HSH004S", Room: "Bedroom", ProductID: 2, Online: false, UpdatedAt: time.Now()},
	}
	for _, d := range devs {
		if err := s.UpsertDevice(ctx, d); err != nil {
			t.Fatalf("UpsertDevice %s: %v", d.Sn, err)
		}
	}
	got, err := s.ListDevices(ctx)
	if err != nil {
		t.Fatalf("ListDevices: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d devices want 2", len(got))
	}
	// Upsert overwrites
	devs[0].Name = "Renamed Fan"
	if err := s.UpsertDevice(ctx, devs[0]); err != nil {
		t.Fatalf("Upsert overwrite: %v", err)
	}
	d, err := s.GetDevice(ctx, "sn-A")
	if err != nil {
		t.Fatalf("GetDevice sn: %v", err)
	}
	if d.Name != "Renamed Fan" {
		t.Errorf("upsert did not overwrite name: %q", d.Name)
	}
}

func TestGetDeviceByName(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.UpsertDevice(ctx, Device{Sn: "sn-X", Name: "Bedroom Fan", UpdatedAt: time.Now()})
	d, err := s.GetDevice(ctx, "Bedroom Fan")
	if err != nil {
		t.Fatalf("GetDevice by name: %v", err)
	}
	if d.Sn != "sn-X" {
		t.Errorf("got sn=%q want sn-X", d.Sn)
	}
}

func TestSearchDevices(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	rows := []Device{
		{Sn: "sn1", Name: "Bedroom Fan", Model: "HTF008S", Room: "Bedroom", UpdatedAt: time.Now()},
		{Sn: "sn2", Name: "Office Heater", Model: "HSH004S", Room: "Office", UpdatedAt: time.Now()},
		{Sn: "sn3", Name: "Living Room Purifier", Model: "HAP002S", Room: "Living Room", UpdatedAt: time.Now()},
	}
	for _, d := range rows {
		_ = s.UpsertDevice(ctx, d)
	}
	hits, err := s.SearchDevices(ctx, "bedroom")
	if err != nil {
		t.Fatalf("SearchDevices: %v", err)
	}
	if len(hits) == 0 {
		t.Errorf("search 'bedroom' returned 0 hits")
	}
	found := false
	for _, h := range hits {
		if h.Sn == "sn1" {
			found = true
		}
	}
	if !found {
		t.Errorf("Bedroom Fan not in results")
	}
}

func TestDeviceStateRoundTrip(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	_ = s.UpsertDevice(ctx, Device{Sn: "sn-S", Name: "x", UpdatedAt: time.Now()})
	data := json.RawMessage(`{"poweron":true,"temperature":72}`)
	now := time.Now().Truncate(time.Second)
	if err := s.UpsertDeviceState(ctx, "sn-S", data, now); err != nil {
		t.Fatalf("UpsertDeviceState: %v", err)
	}
	got, ts, err := s.GetDeviceState(ctx, "sn-S")
	if err != nil {
		t.Fatalf("GetDeviceState: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("data round-trip: got %s want %s", got, data)
	}
	if ts.Unix() != now.Unix() {
		t.Errorf("ts=%v want %v", ts, now)
	}
}

func TestSensorReadings(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	base := time.Now().Truncate(time.Second)
	readings := []struct {
		sn     string
		ts     time.Time
		metric string
		value  float64
	}{
		{"sn-T", base.Add(-1 * time.Hour), "temperature", 70.5},
		{"sn-T", base.Add(-30 * time.Minute), "temperature", 72.0},
		{"sn-T", base.Add(-15 * time.Minute), "humidity", 45.0},
		{"sn-U", base.Add(-30 * time.Minute), "pm25", 12.0},
	}
	for _, r := range readings {
		if err := s.AppendSensorReading(ctx, r.sn, r.ts, r.metric, r.value); err != nil {
			t.Fatalf("AppendSensorReading: %v", err)
		}
	}
	// Query temperature for sn-T in last 2h
	out, err := s.QuerySensorReadings(ctx, "sn-T", base.Add(-2*time.Hour), base, "temperature", 10)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(out) != 2 {
		t.Errorf("got %d temperature readings for sn-T want 2", len(out))
	}
	// Query everything for sn-T
	out, err = s.QuerySensorReadings(ctx, "sn-T", base.Add(-2*time.Hour), base, "", 10)
	if err != nil {
		t.Fatalf("Query (no metric filter): %v", err)
	}
	if len(out) != 3 {
		t.Errorf("got %d readings for sn-T want 3", len(out))
	}
}

func TestScenes(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	snapshots := map[string]map[string]any{
		"sn-1": {"poweron": false, "windlevel": 0},
		"sn-2": {"poweron": true, "windmode": "sleep"},
	}
	if err := s.SaveScene(ctx, "bedtime", snapshots); err != nil {
		t.Fatalf("SaveScene: %v", err)
	}
	loaded, err := s.LoadScene(ctx, "bedtime")
	if err != nil {
		t.Fatalf("LoadScene: %v", err)
	}
	if len(loaded) != 2 {
		t.Errorf("loaded %d devices want 2", len(loaded))
	}
	if v, _ := loaded["sn-2"]["poweron"].(bool); !v {
		t.Errorf("sn-2.poweron lost in round-trip: %v", loaded["sn-2"]["poweron"])
	}
	list, err := s.ListScenes(ctx)
	if err != nil {
		t.Fatalf("ListScenes: %v", err)
	}
	if len(list) != 1 || list[0].Name != "bedtime" {
		t.Errorf("ListScenes = %+v want one [bedtime]", list)
	}
}

func TestSanitizeFTSToken(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Bedroom", "Bedroom"},
		{"Fan*", "Fan"},
		{`"quoted"`, "quoted"},
		{"with-dash", "with-dash"},
	}
	for _, c := range cases {
		if got := sanitizeFTSToken(c.in); got != c.want {
			t.Errorf("sanitizeFTSToken(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
