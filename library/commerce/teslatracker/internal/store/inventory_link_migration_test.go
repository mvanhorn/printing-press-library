package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestUpgradeRekeysLegacyInventoryLinkWithoutTouchingVehicle(t *testing.T) {
	const vin = "5YJ3E1EA7KF317000"
	link := json.RawMessage(`{"name":"Model 3","slug":"model-3","url":"https://teslatracker.com/inventory/5YJ3E1EA7KF317000"}`)
	vehicle := json.RawMessage(`{"vin":"5YJ3E1EA7KF317000","model":"Model 3","mileage":27000}`)
	for _, fullInventoryTarget := range []bool{false, true} {
		name := "no VIN target"
		if fullInventoryTarget {
			name = "preserve existing full VIN target"
		}
		t.Run(name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "inventory.db")
			s, err := Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Upsert("inventory", "Model 3", link); err != nil {
				t.Fatal(err)
			}
			if fullInventoryTarget {
				if err := s.Upsert("inventory", vin, vehicle); err != nil {
					t.Fatal(err)
				}
			}
			if err := s.Upsert("vehicle", vin, vehicle); err != nil {
				t.Fatal(err)
			}
			result, err := s.DB().Exec(`INSERT INTO search_learnings
				(query_pattern, resource_type, resource_id, action, source, confidence)
				VALUES ('find this Model 3', 'inventory', 'Model 3', 'boost', 'taught', 2)`)
			if err != nil {
				t.Fatal(err)
			}
			oldLearningID, err := result.LastInsertId()
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().Exec(`INSERT INTO learn_events
				(ts, event, matched_row_id, surface) VALUES ('2026-10-01', 'recall_hit', ?, 'cli')`, oldLearningID); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().Exec(`PRAGMA user_version = 9`); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}

			s, err = Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			stored, skipped, err := s.UpsertBatch("inventory", []json.RawMessage{link})
			if err != nil || stored != 1 || skipped != 0 {
				t.Fatalf("sync same link after upgrade: stored=%d skipped=%d err=%v", stored, skipped, err)
			}
			count, err := s.Count("inventory")
			if err != nil || count != 1 {
				t.Fatalf("inventory count after upgrade = %d, %v", count, err)
			}
			if _, err := s.Get("inventory", "Model 3"); err == nil {
				t.Fatal("legacy display-name row survived upgrade")
			}
			got, err := s.Get("inventory", vin)
			if err != nil {
				t.Fatal(err)
			}
			if fullInventoryTarget {
				var detail struct {
					VIN     string `json:"vin"`
					Model   string `json:"model"`
					Mileage int    `json:"mileage"`
					URL     string `json:"url"`
				}
				if err := json.Unmarshal(got, &detail); err != nil || detail.VIN != vin || detail.Model != "Model 3" || detail.Mileage != 27000 || detail.URL != "https://teslatracker.com/inventory/5YJ3E1EA7KF317000" {
					t.Fatalf("full VIN detail after upgrade and sync = %s, %v", got, err)
				}
			} else if string(got) != string(link) {
				t.Fatalf("VIN inventory link after upgrade and sync = %s", got)
			}
			got, err = s.Get("vehicle", vin)
			if err != nil || string(got) != string(vehicle) {
				t.Fatalf("hydrated vehicle record = %s, %v", got, err)
			}
			var indexed int
			if err := s.DB().QueryRow(`SELECT COUNT(*) FROM resources_fts WHERE resource_type = 'inventory'`).Scan(&indexed); err != nil || indexed != 1 {
				t.Fatalf("inventory search rows after upgrade = %d, %v", indexed, err)
			}
			var learningCount, eventLearningID int64
			if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE resource_type = 'inventory' AND resource_id = 'Model 3'`).Scan(&learningCount); err != nil || learningCount != 1 {
				t.Fatalf("preserved learned lookup count = %d, %v", learningCount, err)
			}
			var alias string
			if err := s.DB().QueryRow(`SELECT new_id FROM resource_id_aliases WHERE resource_type = 'inventory' AND old_id = 'Model 3'`).Scan(&alias); err != nil || alias != vin {
				t.Fatalf("inventory ID alias = %q, %v", alias, err)
			}
			if err := s.DB().QueryRow(`SELECT matched_row_id FROM learn_events WHERE event = 'recall_hit'`).Scan(&eventLearningID); err != nil || eventLearningID != oldLearningID {
				t.Fatalf("learned event row = %d, want %d, err=%v", eventLearningID, oldLearningID, err)
			}
		})
	}
}

func TestUpgradePreservesLearningsAndSkipsUnscopedReference(t *testing.T) {
	const vin = "5YJ3E1EA7KF317000"
	link := json.RawMessage(`{"name":"Model 3","url":"https://teslatracker.com/inventory/5YJ3E1EA7KF317000"}`)
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, *Store)
	}{
		{
			name: "unscoped old ID",
			setup: func(t *testing.T, s *Store) {
				t.Helper()
				if _, err := s.DB().Exec(`INSERT INTO search_learnings
					(query_pattern, resource_id, action, source)
					VALUES ('find Model 3', 'Model 3', 'boost', 'taught')`); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "cross-resource VIN conflict",
			setup: func(t *testing.T, s *Store) {
				t.Helper()
				if _, err := s.DB().Exec(`INSERT INTO search_learnings
					(query_pattern, resource_type, resource_id, action, source)
					VALUES ('find Model 3', 'inventory', 'Model 3', 'boost', 'taught'),
					       ('find Model 3', 'vehicle', ?, 'boost', 'taught')`, vin); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "same-resource duplicate learning metadata",
			setup: func(t *testing.T, s *Store) {
				t.Helper()
				if _, err := s.DB().Exec(`INSERT INTO search_learnings
					(query_pattern, resource_type, resource_id, action, source, confidence, notes, alias_target)
					VALUES ('find Model 3', 'inventory', 'Model 3', 'boost', 'taught', 5, 'old note', 'old alias'),
					       ('find Model 3', 'inventory', ?, 'boost', 'taught', 2, 'new note', 'new alias')`, vin); err != nil {
					t.Fatal(err)
				}
				if _, err := s.DB().Exec(`INSERT INTO learn_events (ts, event, matched_row_id, surface)
					SELECT '2026-10-01', 'recall_hit', id, 'cli' FROM search_learnings
					WHERE resource_id IN ('Model 3', ?)`, vin); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "inventory.db")
			s, err := Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Upsert("inventory", "Model 3", link); err != nil {
				t.Fatal(err)
			}
			tc.setup(t, s)
			if _, err := s.DB().Exec(`PRAGMA user_version = 9`); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			got, err := s.Get("inventory", "Model 3")
			if tc.name == "unscoped old ID" {
				if err != nil || string(got) != string(link) {
					t.Fatalf("unscoped legacy link changed = %s, %v", got, err)
				}
			} else {
				if err == nil {
					t.Fatalf("legacy link survived safe rekey = %s", got)
				}
				if _, err := s.Get("inventory", vin); err != nil {
					t.Fatalf("VIN inventory link missing: %v", err)
				}
				var alias string
				if err := s.DB().QueryRow(`SELECT new_id FROM resource_id_aliases WHERE resource_type = 'inventory' AND old_id = 'Model 3'`).Scan(&alias); err != nil || alias != vin {
					t.Fatalf("inventory alias = %q, %v", alias, err)
				}
			}
			var refs int
			if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE resource_id = 'Model 3'`).Scan(&refs); err != nil || refs != 1 {
				t.Fatalf("old-ID learned references after upgrade = %d, %v", refs, err)
			}
			if tc.name == "same-resource duplicate learning metadata" {
				var confidence int
				var notes, alias string
				if err := s.DB().QueryRow(`SELECT confidence, notes, alias_target FROM search_learnings WHERE resource_id = 'Model 3'`).Scan(&confidence, &notes, &alias); err != nil || confidence != 5 || notes != "old note" || alias != "old alias" {
					t.Fatalf("old learning metadata after upgrade = confidence %d, notes %q, alias %q, err=%v", confidence, notes, alias, err)
				}
				if err := s.DB().QueryRow(`SELECT confidence, notes, alias_target FROM search_learnings WHERE resource_id = ?`, vin).Scan(&confidence, &notes, &alias); err != nil || confidence != 2 || notes != "new note" || alias != "new alias" {
					t.Fatalf("VIN learning metadata after upgrade = confidence %d, notes %q, alias %q, err=%v", confidence, notes, alias, err)
				}
				var linkedEvents int
				if err := s.DB().QueryRow(`SELECT COUNT(*) FROM learn_events e
					JOIN search_learnings l ON e.matched_row_id = l.id
					WHERE l.resource_id IN ('Model 3', ?)`, vin).Scan(&linkedEvents); err != nil || linkedEvents != 2 {
					t.Fatalf("preserved learning event links = %d, %v", linkedEvents, err)
				}
			}
		})
	}
}

func TestLegacyInventoryLinkCanUseSlugEvenWithName(t *testing.T) {
	obj := map[string]any{
		"name": "Model 3", "slug": "model-3",
		"url": "https://teslatracker.com/inventory/5YJ3E1EA7KF317000",
	}
	if !isLegacyInventoryLinkID("model-3", obj) {
		t.Fatal("slug-keyed legacy link was missed")
	}
}

func TestUpgradeSkipsDisplayNameSharedByTwoVINs(t *testing.T) {
	const otherVIN = "5YJ3E1EA7KF318000"
	for _, tc := range []struct {
		name, oldID string
		legacy      json.RawMessage
		other       json.RawMessage
	}{
		{"name", "Model 3", json.RawMessage(`{"name":"Model 3","url":"https://teslatracker.com/inventory/5YJ3E1EA7KF317000"}`), json.RawMessage(`{"vin":"5YJ3E1EA7KF318000","name":"Model 3"}`)},
		{"key", "listing-3", json.RawMessage(`{"key":"listing-3","url":"https://teslatracker.com/inventory/5YJ3E1EA7KF317000"}`), json.RawMessage(`{"vin":"5YJ3E1EA7KF318000","key":"listing-3"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "inventory.db")
			s, err := Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Upsert("inventory", tc.oldID, tc.legacy); err != nil {
				t.Fatal(err)
			}
			if err := s.Upsert("inventory", otherVIN, tc.other); err != nil {
				t.Fatal(err)
			}
			if _, err := s.DB().Exec(`PRAGMA user_version = 9`); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = Open(dbPath)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if got, err := s.Get("inventory", tc.oldID); err != nil || string(got) != string(tc.legacy) {
				t.Fatalf("ambiguous legacy listing = %s, %v", got, err)
			}
			var aliasCount int
			if err := s.DB().QueryRow(`SELECT COUNT(*) FROM resource_id_aliases WHERE resource_type = 'inventory' AND old_id = ?`, tc.oldID).Scan(&aliasCount); err != nil || aliasCount != 0 {
				t.Fatalf("ambiguous alias count = %d, %v", aliasCount, err)
			}
		})
	}
}
