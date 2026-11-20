package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type embeddedStateDryClient struct{ *listingStateClient }

func (*embeddedStateDryClient) IsDryRun() bool { return true }

func TestEmbeddedListingFamilyState(t *testing.T) {
	for _, tc := range []struct {
		name, block                    string
		inventory, images, capped, dry bool
	}{
		{"both", `,"inventory":{"products":[{"product_id":7000000001,"sku":"JC-APRON-1"}]},"images":[{"listing_image_id":1000000501}]`, true, true, false, false},
		{"absent", "", false, false, false, false},
		{"images", `,"images":[{"listing_image_id":1000000501}]`, false, true, false, false},
		{"empty", `,"inventory":{},"images":[]`, true, true, false, false},
		{"capped", `,"inventory":{"products":[]},"images":[]`, false, false, true, false},
		{"dry", `,"inventory":{},"images":[]`, false, false, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, db := listingStatesFixture(t)
			for _, name := range []string{"listings_inventory", "listing_images"} {
				if !strings.Contains(strings.Join(shopxCommandNotes(db, name), " "), "not synced") {
					t.Fatal("fresh store lacks negative control", name)
				}
			}
			c.respond = func(state, offset string) (json.RawMessage, error) {
				if state != "active" {
					return json.RawMessage(`{"results":[],"count":0}`), nil
				}
				row := `{"listing_id":1000000001,"title":"linen apron","state":"active"` + tc.block + `}`
				rows := []json.RawMessage{json.RawMessage(row)}
				count := 1
				if tc.capped {
					rows = nil
					for i := 0; i < 100; i++ {
						var obj map[string]any
						if err := json.Unmarshal([]byte(row), &obj); err != nil {
							t.Fatal(err)
						}
						obj["listing_id"] = 1000000001 + i
						raw, _ := json.Marshal(obj)
						rows = append(rows, raw)
					}
					count = 101
				}
				return json.Marshal(map[string]any{"results": rows, "count": count})
			}
			var events bytes.Buffer
			maxPages := 0
			if tc.capped {
				maxPages = 1
			}
			var r syncResult
			if tc.dry {
				r = syncResource(context.Background(), &embeddedStateDryClient{c}, db, "listings", "", true, maxPages, false, false, nil, &events)
			} else {
				r = syncResource(context.Background(), c, db, "listings", "", true, maxPages, false, false, nil, &events)
			}
			if r.Err != nil && !tc.dry {
				t.Fatal(r.Err)
			}
			if tc.capped && r.complete {
				t.Fatal("capped walk reported complete")
			}
			for name, want := range map[string]bool{"listings_inventory": tc.inventory, "listing_images": tc.images} {
				ages := shopxSyncAges(db, []string{name})
				if (ages[name] != nil) != want {
					t.Fatalf("%s age=%v want synced=%v", name, ages[name], want)
				}
				note := strings.Contains(strings.Join(shopxCommandNotes(db, name), " "), "not synced")
				if note == want {
					t.Fatalf("%s notes=%v", name, shopxCommandNotes(db, name))
				}
				if shopxFamilySynced(db, []string{name}) != want {
					t.Fatal("family state", name)
				}
				if !want {
					var n int
					s2Must(t, db.DB().QueryRow("SELECT count(*) FROM sync_state WHERE resource_type = ?", name).Scan(&n))
					if n != 0 {
						t.Fatal("unexpected family checkpoint", name)
					}
					continue
				}
				var complete, total int
				s2Must(t, db.DB().QueryRow("SELECT last_attempt_complete,total_count FROM sync_state WHERE resource_type = ?", name).Scan(&complete, &total))
				stored, err := db.Count(name)
				s2Must(t, err)
				if complete != 1 || total != stored {
					t.Fatalf("%s complete=%d total=%d stored=%d", name, complete, total, stored)
				}
				scope := syncCheckpointScope(name, true, nil, syncClientTemplateVars(c), syncActivePathContext())
				_, before, _, err := db.ReadSyncScope(name, scope)
				s2Must(t, err)
				_, _, _, err = db.BindSyncScope(name, scope)
				s2Must(t, err)
				_, after, _, err := db.ReadSyncScope(name, scope)
				s2Must(t, err)
				if before.IsZero() || !before.Equal(after) {
					t.Fatal("dependent scope reset checkpoint", name)
				}
			}
		})
	}
}
