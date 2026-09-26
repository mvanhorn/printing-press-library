package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func openTBStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.EnsureThunderbirdTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.EnsureThunderbirdTables(context.Background()); err != nil {
		t.Fatalf("second init: %v", err)
	}
	return s
}

func TestMboxStateRoundTrip(t *testing.T) {
	s := openTBStore(t)
	if _, ok, err := s.GetMboxState("/x/INBOX"); ok || err != nil {
		t.Fatalf("absent state ok=%v err=%v", ok, err)
	}
	want := MboxState{Path: "/x/INBOX", FolderKey: "account1:INBOX", Size: 100, MTime: 5, LastOffset: 100}
	if err := s.SaveMboxState(want); err != nil {
		t.Fatal(err)
	}
	want.Size, want.LastOffset = 250, 240
	if err := s.SaveMboxState(want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetMboxState("/x/INBOX")
	if err != nil || !ok || got != want {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
	if err := s.SaveMboxState(MboxState{Path: "/x/Sent", FolderKey: "account1:Sent"}); err != nil {
		t.Fatal(err)
	}
	all, err := s.ListMboxStates()
	if err != nil || len(all) != 2 || all[0].Path != "/x/INBOX" {
		t.Fatalf("list = %+v %v", all, err)
	}
	if err := s.DeleteMboxState("/x/INBOX"); err != nil {
		t.Fatal(err)
	}
	all, _ = s.ListMboxStates()
	if len(all) != 1 || all[0].Path != "/x/Sent" {
		t.Fatalf("after delete = %+v", all)
	}
}

func TestPruneResources(t *testing.T) {
	s := openTBStore(t)
	items := []json.RawMessage{
		json.RawMessage(`{"id":"a1","folder_key":"acc:INBOX","subject":"zebra crossing"}`),
		json.RawMessage(`{"id":"a2","folder_key":"acc:INBOX","subject":"zebra stripes"}`),
		json.RawMessage(`{"id":"b1","folder_key":"acc:Sent","subject":"zebra sent"}`),
	}
	if _, _, err := s.UpsertBatch("messages", items); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		run       func() (int, error)
		wantGone  int
		remaining int
	}{
		{"delete listed ids", func() (int, error) { return s.DeleteResourceIDs("messages", []string{"a2"}) }, 1, 2},
		{"empty list", func() (int, error) { return s.DeleteResourceIDs("messages", nil) }, 0, 2},
		{"not-in keep set", func() (int, error) { return s.PruneResourcesNotIn("messages", map[string]bool{"b1": true}) }, 1, 1},
	}
	for _, tt := range tests {
		n, err := tt.run()
		if err != nil || n != tt.wantGone {
			t.Fatalf("%s: removed %d err %v", tt.name, n, err)
		}
		if c, _ := s.Count("messages"); c != tt.remaining {
			t.Fatalf("%s: remaining %d", tt.name, c)
		}
	}
	hits, err := s.Search("zebra", 10, "messages")
	if err != nil || len(hits) != 1 {
		t.Fatalf("FTS rows not pruned: %d hits, %v", len(hits), err)
	}
}
