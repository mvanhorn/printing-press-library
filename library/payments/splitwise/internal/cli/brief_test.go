// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestBriefDigest(t *testing.T) {
	p := filepath.Join(t.TempDir(), "brief.db")
	s, e := store.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	fs := []struct{ r, id, b string }{{"get-friends", "7", `{"id":7,"first_name":"Alex","balance":[{"currency_code":"USD","amount":"35.00"}]}`}, {"get-friends", "8", `{"id":8,"first_name":"Blair","balance":[{"currency_code":"USD","amount":"-12.50"}]}`}, {"get-expenses", "1", `{"id":1,"date":"2020-01-01","users":[{"user_id":7}]}`}, {"get-expenses", "2", `{"id":2,"date":"2022-01-01","users":[{"user_id":8}]}`}, {"get-expenses", "3", `{"id":3,"date":"2019-01-01","payment":true,"users":[{"user_id":7}]}`}}
	for _, f := range fs {
		if e := s.Upsert(f.r, f.id, []byte(f.b)); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
	out, stderr, e := runRootArgs(t, "brief", "--json", "--db", p, "--top", "2")
	if e != nil {
		t.Fatalf("%v stderr=%s", e, stderr)
	}
	var got briefOutput
	if e = json.Unmarshal([]byte(out), &got); e != nil {
		t.Fatalf("%v raw=%s", e, out)
	}
	if len(got.StalestDebts) != 2 {
		t.Fatalf("len=%d", len(got.StalestDebts))
	}
	if *got.StalestDebts[0].DaysOpen < *got.StalestDebts[1].DaysOpen {
		t.Fatalf("not sorted: %+v", got.StalestDebts)
	}
	if got.Net.OwedToYou != 35 || got.Net.YouOwe != 12.5 {
		t.Fatalf("net=%+v", got.Net)
	}
}

func TestBriefRecentChangesWindow(t *testing.T) {
	p := filepath.Join(t.TempDir(), "brief-window.db")
	s, err := store.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fixtures := []struct{ resource, id, body string }{
		{"get-expenses", "1", `{"id":1,"created_at":"2026-08-29T12:00:00Z","updated_at":"2026-08-29T12:00:00Z"}`},
		{"get-expenses", "2", `{"id":2,"created_at":"2026-07-01T12:00:00Z","updated_at":"2026-08-30T12:00:00Z"}`},
		{"get-expenses", "3", `{"id":3,"created_at":"2026-07-01T12:00:00Z","updated_at":"2026-08-30T12:00:00Z","deleted_at":"2026-09-01T12:00:00Z"}`},
		{"get-expenses", "4", `{"id":4,"created_at":"2026-07-01T12:00:00Z","updated_at":"2026-07-01T12:00:00Z"}`},
		{"get-notifications", "5", `{"id":5,"created_at":"2026-09-02T12:00:00Z"}`},
		{"get-notifications", "6", `{"id":6,"created_at":"2026-07-01T12:00:00Z"}`},
	}
	for _, fixture := range fixtures {
		if err := s.Upsert(fixture.resource, fixture.id, []byte(fixture.body)); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	got, err := buildBrief(s, 0, now, 7*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if got.Changes.NewExpenses != 1 || got.Changes.UpdatedExpenses != 1 || got.Changes.DeletedExpenses != 1 || got.Changes.Notifications != 1 {
		t.Fatalf("recent changes = %+v", got.Changes)
	}
}

func TestBriefEmptyStoreUsesArray(t *testing.T) {
	p := filepath.Join(t.TempDir(), "empty.db")
	s, e := store.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	out, stderr, e := runRootArgs(t, "brief", "--json", "--db", p)
	if e != nil {
		t.Fatalf("%v stderr=%s", e, stderr)
	}
	var raw map[string]json.RawMessage
	if e = json.Unmarshal([]byte(out), &raw); e != nil {
		t.Fatal(e)
	}
	if string(raw["stalest_debts"]) != "[]" {
		t.Fatalf("got %s", raw["stalest_debts"])
	}
	var got briefOutput
	json.Unmarshal([]byte(out), &got)
	if got.NextStep == "" {
		t.Fatal("empty next_step")
	}
}

func TestBriefDryRun(t *testing.T) {
	out, stderr, e := runRootArgs(t, "brief", "--dry-run", "--json")
	if e != nil {
		t.Fatalf("%v stderr=%s", e, stderr)
	}
	var got map[string]any
	if e = json.Unmarshal([]byte(out), &got); e != nil {
		t.Fatal(e)
	}
	if got["dry_run"] != true {
		t.Fatalf("got %v", got)
	}
}
