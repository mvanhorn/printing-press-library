// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.
// Integration test: confirms the novel commands read the local store using the
// same kebab-case resource_type keys that sync writes. A key mismatch would
// silently return empty results, so this guards the local-mirror contract.

package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/3cx-xapi/internal/store"
)

func TestNovelStoreIntegration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	upsert := func(rt, id, data string) {
		if err := db.Upsert(rt, id, json.RawMessage(data)); err != nil {
			t.Fatalf("upsert %s/%s: %v", rt, id, err)
		}
	}
	upsert(rtUsers, "1", `{"Id":1,"Number":"214","FirstName":"Jane","LastName":"Doe"}`)
	upsert(rtUsers, "2", `{"Id":2,"Number":"215","FirstName":"Bob","LastName":"Roe"}`)
	upsert(rtRingGroups, "800", `{"Number":"800","Name":"Sales","Members":[{"Number":"214"},{"Number":"999"}]}`)
	upsert(rtQueues, "801", `{"Number":"801","Name":"Support","Agents":[{"Number":"215"}],"RingStrategy":"RingAll"}`)
	upsert(rtInboundRules, "10", `{"Id":10,"RuleName":"Main","OfficeHoursDestination":{"Number":"800"}}`)

	// dnNumberSet must union users + ring groups + queues by Number.
	valid, err := dnNumberSet(db)
	if err != nil {
		t.Fatalf("dnNumberSet: %v", err)
	}
	for _, n := range []string{"214", "215", "800", "801"} {
		if !valid[n] {
			t.Errorf("expected %s in valid DN set, set=%v", n, valid)
		}
	}

	rg, _ := listObjects(db, rtRingGroups)
	q, _ := listObjects(db, rtQueues)
	ib, _ := listObjects(db, rtInboundRules)
	findings := findDanglingRefs(valid, rg, q, ib)
	if len(findings) != 1 || findings[0].Ref != "999" {
		t.Fatalf("expected exactly one dangling ref (999), got %+v", findings)
	}

	// trace 214: in ring group Sales (800) and reachable; queue 801 has 215 not 214.
	users, _ := listObjects(db, rtUsers)
	dids, _ := listObjects(db, rtDidNumbers)
	tr := findRoutesToExtension("214", users, rg, q, ib, dids)
	if !tr.Exists || len(tr.RingGroups) != 1 {
		t.Fatalf("expected 214 to exist and be in 1 ring group, got %+v", tr)
	}

	// qrollup rows from real store data.
	rows, totalAgents := buildQueueRows(q, map[string]map[string][]json.RawMessage{})
	if len(rows) != 1 || rows[0].AgentCount != 1 || rows[0].RingStrategy != "RingAll" {
		t.Fatalf("expected one queue row with 1 agent and RingAll, got %+v", rows)
	}
	if totalAgents != 1 {
		t.Errorf("expected total agents 1, got %d", totalAgents)
	}
}
