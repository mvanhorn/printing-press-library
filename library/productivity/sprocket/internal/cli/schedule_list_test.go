// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestScheduleListItemsReadsDeclaredEventsEnvelope(t *testing.T) {
	items, ok := scheduleListItems(json.RawMessage(`{"events":[{"id":42,"title":"Fixture Game","start":"2026-06-18T18:00:00"}]}`))
	if !ok {
		t.Fatal("events envelope was not recognized")
	}
	if len(items) != 1 || items[0]["title"] != "Fixture Game" {
		t.Fatalf("events = %#v, want one Fixture Game", items)
	}
}

func TestScheduleListItemsKeepsDataEnvelopeCompatibility(t *testing.T) {
	items, ok := scheduleListItems(json.RawMessage(`{"data":[{"id":7,"title":"Practice"}]}`))
	if !ok || len(items) != 1 || items[0]["title"] != "Practice" {
		t.Fatalf("data envelope = %#v, ok=%v", items, ok)
	}
}
