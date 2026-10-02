package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cycling"
)

func TestHCEnvelopeOfflineProjectionAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.db")
	now := time.Now().UTC()
	lat, lon := 35.697315, 139.704995
	n := 1
	dock := 11
	yes := true
	s := cycling.Snapshot{ObservedAt: now, Feeds: map[string]cycling.FeedMeta{"station_status": {LastUpdated: now.Unix()}}, Information: []cycling.Info{{ID: "5112", Name: "プラーズタワー東新宿", Lat: &lat, Lon: &lon}}, Statuses: []cycling.Status{{ID: "5112", Bikes: &n, Docks: &dock, Reported: now.Unix(), Installed: &yes, Renting: &yes, Returning: &yes}}, Warnings: []string{}}
	if e := cycling.SaveSnapshot(path, s); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name string
		args []string
		ok   bool
	}{{"explicit local", []string{"stations", "show", "--id", "5112", "--data-source", "local", "--snapshot-db", path, "--agent", "--timeout", "1ns"}, true}, {"conflicting live", []string{"stations", "show", "--id", "5112", "--offline", "--data-source", "live", "--snapshot-db", path}, false}, {"pricing live only", []string{"pricing", "show", "--area", "tokyo", "--data-source", "local", "--dry-run"}, false}, {"offline agent", []string{"stations", "show", "--id", "5112", "--offline", "--snapshot-db", path, "--agent"}, true}, {"projected", []string{"stations", "find", "--query", "東新宿", "--offline", "--snapshot-db", path, "--agent", "--select", "results.id,results.name"}, true}, {"empty", []string{"stations", "find", "--query", "__no_such_station__", "--offline", "--snapshot-db", path, "--agent"}, true}, {"missing coordinates", []string{"stations", "nearby", "--dry-run", "--agent"}, false}, {"nonfinite coordinates", []string{"stations", "nearby", "--lat", "NaN", "--lon", "139", "--dry-run"}, false}, {"invalid bounds", []string{"stations", "nearby", "--lat", "35", "--lon", "139", "--limit", "51", "--dry-run"}, false}, {"invalid radius", []string{"stations", "nearby", "--lat", "35", "--lon", "139", "--radius-m", "NaN", "--dry-run"}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			c := RootCmd()
			c.SetArgs(tc.args)
			var out, errs bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&errs)
			e := c.Execute()
			if (e == nil) != tc.ok {
				t.Fatalf("err=%v out=%s", e, out.String())
			}
			if !tc.ok {
				return
			}
			var got map[string]any
			if e = json.Unmarshal(out.Bytes(), &got); e != nil {
				t.Fatal(e, out.String())
			}
			meta, ok := got["meta"].(map[string]any)
			if !ok || meta["source"] != "local" || meta["observed_at"] == nil {
				t.Fatal("provenance lost", got)
			}
			if _, ok := got["results"].([]any); !ok {
				t.Fatal("nested results envelope", got)
			}
		})
	}
}
