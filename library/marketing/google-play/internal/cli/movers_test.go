package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-play/internal/store"
)

func TestComputeMovers(t *testing.T) {
	prev := []store.ChartRow{
		{Rank: 1, AppID: "a", Title: "A"},
		{Rank: 2, AppID: "b", Title: "B"},
		{Rank: 3, AppID: "c", Title: "C"},
		{Rank: 4, AppID: "d", Title: "D"},
	}
	latest := []store.ChartRow{
		{Rank: 1, AppID: "b", Title: "B"}, // climbed 2 -> 1 (delta +1)
		{Rank: 2, AppID: "a", Title: "A"}, // dropped 1 -> 2 (delta -1)
		{Rank: 3, AppID: "e", Title: "E"}, // new
		{Rank: 4, AppID: "c", Title: "C"}, // dropped 3 -> 4
		// d fell off
	}
	var v moversView
	v.Climbers, v.Droppers, v.NewEntries, v.DroppedOut = []moverEntry{}, []moverEntry{}, []moverEntry{}, []moverEntry{}
	computeMovers(&v, prev, latest)

	if len(v.Climbers) != 1 || v.Climbers[0].AppID != "b" || v.Climbers[0].Delta != 1 {
		t.Errorf("climbers = %+v, want b with delta +1", v.Climbers)
	}
	if len(v.Droppers) != 2 {
		t.Errorf("droppers = %+v, want 2 (a and c)", v.Droppers)
	}
	if len(v.NewEntries) != 1 || v.NewEntries[0].AppID != "e" {
		t.Errorf("newEntries = %+v, want [e]", v.NewEntries)
	}
	if len(v.DroppedOut) != 1 || v.DroppedOut[0].AppID != "d" {
		t.Errorf("droppedOut = %+v, want [d]", v.DroppedOut)
	}
}

func TestComputeMoversNoChange(t *testing.T) {
	rows := []store.ChartRow{{Rank: 1, AppID: "a"}, {Rank: 2, AppID: "b"}}
	var v moversView
	v.Climbers, v.Droppers, v.NewEntries, v.DroppedOut = []moverEntry{}, []moverEntry{}, []moverEntry{}, []moverEntry{}
	computeMovers(&v, rows, rows)
	if len(v.Climbers) != 0 || len(v.Droppers) != 0 || len(v.NewEntries) != 0 || len(v.DroppedOut) != 0 {
		t.Errorf("identical snapshots should yield no movers, got %+v", v)
	}
}

func TestNewMoversCmd(t *testing.T) {
	cmd := newNovelMoversCmd(&rootFlags{})
	if cmd.Use != "movers" {
		t.Errorf("Use = %q", cmd.Use)
	}
	if cmd.Flag("collection") == nil || cmd.Flag("category") == nil {
		t.Error("movers should declare --collection, --category")
	}
}
