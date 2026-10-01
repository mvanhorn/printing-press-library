package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/cliutil/testenv"
	"testing"
	"time"
)

func TestPlanningZeroFlagsAndAutomaticDefaults(t *testing.T) {
	for _, tc := range []struct {
		name         string
		flags        rootFlags
		maxAge, pace time.Duration
	}{
		{"zero disables", rootFlags{maxAge: 0, rateLimit: 0}, 0, 0},
		{"automatic defaults", rootFlags{maxAge: 30 * time.Minute, rateLimit: -1}, 30 * time.Minute, 500 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			s, err := newPlanningSession(&tc.flags, planningOptions{lang: "en"})
			if err != nil {
				t.Fatal(err)
			}
			if s.client.MaxAge != tc.maxAge || s.client.Pace != tc.pace {
				t.Fatalf("max age=%s pace=%s; want %s and %s", s.client.MaxAge, s.client.Pace, tc.maxAge, tc.pace)
			}
		})
	}
}

func TestPlanningInputAndOfflineBoundaries(t *testing.T) {
	for _, args := range [][]string{{"restaurants", "find", "--limit", "0"}, {"restaurants", "show", "../bad"}, {"availability", "hc778124", "--date", "2026-02-30", "--party", "2"}, {"availability", "hc778124", "--party", "2"}, {"compare", "hc778124", "hc778124"}, {"courses", "hc778124", "--offline", "--refresh"}, {"restaurants", "find", "--offline", "--no-cache"}} {
		t.Run(args[0], func(t *testing.T) {
			testenv.Isolate(t)
			c := RootCmd()
			c.SetArgs(append(args, "--no-learn"))
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&out)
			if e := c.Execute(); e == nil || ExitCode(e) != 2 {
				t.Fatalf("%v => %v %s", args, e, out.String())
			}
		})
	}
}
func TestPlanningNullProjectionAndEmptyInventory(t *testing.T) {
	testenv.Isolate(t)
	c := RootCmd()
	c.SetArgs([]string{"inventory", "status", "--no-learn", "--agent"})
	var out bytes.Buffer
	c.SetOut(&out)
	if e := c.Execute(); e != nil {
		t.Fatal(e)
	}
	var v map[string]any
	if e := json.Unmarshal(out.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	if v["results"].(map[string]any)["exists"] != false {
		t.Fatal(v)
	}
	out.Reset()
	f := &rootFlags{agent: true, selectFields: "results.price,results.name"}
	if e := planningOutput(c, f, map[string]any{"meta": map[string]any{"partial": true}, "results": map[string]any{"name": "Test", "price": nil}}); e != nil {
		t.Fatal(e)
	}
	if e := json.Unmarshal(out.Bytes(), &v); e != nil {
		t.Fatal(e)
	}
	r := v["results"].(map[string]any)
	if _, ok := r["price"]; !ok || r["price"] != nil {
		t.Fatal("lost explicit unknown")
	}
}
