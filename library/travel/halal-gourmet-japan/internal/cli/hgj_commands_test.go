// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/hgj"
	"github.com/mvanhorn/printing-press-library/library/travel/halal-gourmet-japan/internal/store"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHGJDetailFormatsKeepPlaceIdentityWithNestedHours(t *testing.T) {
	path := filepath.Join(t.TempDir(), "detail.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := hgj.Place{ID: "300739", Kind: hgj.Restaurant, Name: "Fixture restaurant", SourceURL: hgj.Origin + "/restaurant/300739", EvidenceScope: "detail", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), WeeklyHours: []hgj.Hours{{Day: "Monday", SourceText: "12:00–21:00"}}, Conditions: map[string]hgj.Condition{"certified": {State: hgj.NotReported}}}
	if err = hgj.SaveSnapshot(context.Background(), db.DB(), p); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for _, mode := range []string{"quiet", "plain", "csv", "json", "selected-csv"} {
		t.Run(mode, func(t *testing.T) {
			f := &rootFlags{dataSource: "local", noLearn: true, quiet: mode == "quiet", plain: mode == "plain", csv: mode == "csv" || mode == "selected-csv", asJSON: mode == "json"}
			if mode == "selected-csv" {
				f.selectFields = "id,name"
			}
			cmd := newRestaurantsGetCmd(f)
			cmd.SetContext(context.Background())
			if err := cmd.Flags().Set("db", path); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.RunE(cmd, []string{"300739"}); err != nil {
				t.Fatal(err)
			}
			text := out.String()
			if mode == "quiet" {
				if text != "300739\n" {
					t.Fatalf("quiet identities=%q", text)
				}
				return
			}
			if !strings.Contains(text, "300739") || !strings.Contains(text, "Fixture restaurant") {
				t.Fatalf("detail lost identity: %q", text)
			}
			if mode == "json" {
				var row map[string]any
				if json.Unmarshal(out.Bytes(), &row) != nil || row["id"] != "300739" {
					t.Fatal("normal JSON detail shape changed")
				}
			}
			if mode == "csv" || mode == "plain" {
				if !strings.Contains(text, "certified") {
					t.Fatal("table detail discarded condition evidence")
				}
			}
		})
	}
}
func TestHGJMachineGetWithoutIDFailsInsteadOfReturningHelp(t *testing.T) {
	for _, resource := range []string{"restaurants", "prayer"} {
		for _, surface := range []string{"json", "mcp"} {
			t.Run(resource+surface, func(t *testing.T) {
				f := &rootFlags{asJSON: surface == "json", noLearn: true}
				if surface == "mcp" {
					t.Setenv("HALAL_GOURMET_JAPAN_LEARN_SURFACE", "mcp")
				}
				cmd := newRestaurantsGetCmd(f)
				cmd.SetContext(context.Background())
				if resource == "prayer" {
					cmd = newPrayerGetCmd(f)
					cmd.SetContext(context.Background())
				}
				var out bytes.Buffer
				cmd.SetOut(&out)
				if cmd.RunE(cmd, nil) == nil {
					t.Fatal("missing ID returned success")
				}
				if strings.Contains(out.String(), "Usage:") {
					t.Fatal("machine call returned help")
				}
			})
		}
	}
}
