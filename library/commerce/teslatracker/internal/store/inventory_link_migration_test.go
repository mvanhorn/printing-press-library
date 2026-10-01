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
			wantLearningID := oldLearningID
			if fullInventoryTarget {
				result, err = s.DB().Exec(`INSERT INTO search_learnings
					(query_pattern, resource_type, resource_id, action, source, confidence)
					VALUES ('find this Model 3', 'inventory', ?, 'boost', 'taught', 3)`, vin)
				if err != nil {
					t.Fatal(err)
				}
				wantLearningID, err = result.LastInsertId()
				if err != nil {
					t.Fatal(err)
				}
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
			if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE resource_type = 'inventory' AND resource_id = ?`, vin).Scan(&learningCount); err != nil || learningCount != 1 {
				t.Fatalf("VIN-keyed learned lookup count = %d, %v", learningCount, err)
			}
			if err := s.DB().QueryRow(`SELECT COUNT(*) FROM search_learnings WHERE resource_type = 'inventory' AND resource_id = 'Model 3'`).Scan(&learningCount); err != nil || learningCount != 0 {
				t.Fatalf("legacy learned lookup count = %d, %v", learningCount, err)
			}
			if err := s.DB().QueryRow(`SELECT matched_row_id FROM learn_events WHERE event = 'recall_hit'`).Scan(&eventLearningID); err != nil || eventLearningID != wantLearningID {
				t.Fatalf("learned event row = %d, want %d, err=%v", eventLearningID, wantLearningID, err)
			}
		})
	}
}
