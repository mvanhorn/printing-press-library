// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"testing"
)

func TestAnalyticsGroupsStoredResourcesByAlias(t *testing.T) {
	dbPath, db := newTestCardataStore(t)
	for id, raw := range map[string]string{
		"1": `{"id":"1","vehicle_id":"alpha"}`,
		"2": `{"id":"2","vehicle_id":"alpha"}`,
		"3": `{"id":"3","vehicle_id":"beta"}`,
	} {
		if err := db.Upsert("events", id, json.RawMessage(raw)); err != nil {
			t.Fatalf("upsert event %s: %v", id, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	groups, ok := executeTestJSON(t, "analytics", "--type", "events", "--group-by", "vehicle", "--db", dbPath).([]any)
	if !ok || len(groups) != 2 {
		t.Fatalf("analytics groups = %#v", groups)
	}
	first := resultObject(t, groups[0])
	if first["value"] != "alpha" || first["count"] != float64(2) {
		t.Fatalf("analytics ordering/count = %#v", groups)
	}
}
