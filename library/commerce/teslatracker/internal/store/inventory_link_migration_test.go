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
		})
	}
}
