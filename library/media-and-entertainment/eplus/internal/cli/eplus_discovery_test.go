package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/discovery"
	"github.com/spf13/cobra"
	"testing"
	"time"
)

func TestDomainProjectionRetainsPlanningFacts(t *testing.T) {
	for _, selectFields := range []string{"", "id,sales"} {
		var b bytes.Buffer
		c := &cobra.Command{}
		c.SetOut(&b)
		c.SetErr(&bytes.Buffer{})
		f := &rootFlags{agent: true, asJSON: true, compact: true, selectFields: selectFields}
		r := discovery.Result{Data: []discovery.Row{{"id": "one", "date": "2026-11-07", "sales": []discovery.Row{{"lottery_deadline": "2026-10-12T23:59:00+09:00", "inventory": "unknown"}}}}, Meta: discovery.Row{"source": "domestic", "partial": false, "warnings": []string{}}}
		if e := writeDiscovery(c, f, r, 10); e != nil {
			t.Fatal(e)
		}
		var v map[string]any
		if json.Unmarshal(b.Bytes(), &v) != nil {
			t.Fatal(b.String())
		}
		rs := v["results"].([]any)
		row := rs[0].(map[string]any)
		if row["sales"] == nil {
			t.Fatal("sale window stripped", v)
		}
		if v["meta"].(map[string]any)["source"] != "live" {
			t.Fatal(v)
		}
		if selectFields != "" && len(row) != 2 {
			t.Fatal("projection did not apply", row)
		}
	}
}
func TestDiscoveryUsageAndDryRun(t *testing.T) {
	for _, builder := range []func(*rootFlags) *cobra.Command{newEventsSearchAdapterCmd, newInternationalSearchCmd, newPolicies, func(f *rootFlags) *cobra.Command { return newDiscoveryDetail(f, false) }, newCompare} {
		f := &rootFlags{dryRun: true, asJSON: true, timeout: time.Second}
		c := builder(f)
		c.SetOut(&bytes.Buffer{})
		if e := c.RunE(c, nil); e != nil {
			t.Fatal(e)
		}
	}
	f := &rootFlags{asJSON: true, timeout: time.Second}
	c := newCompare(f)
	if c.RunE(c, []string{"7078"}) == nil {
		t.Fatal("missing comparison input accepted")
	}
	if _, e := newDiscoveryClient(&rootFlags{dataSource: "local", timeout: time.Second}, discoveryFlags{}); e == nil {
		t.Fatal("unsupported source accepted")
	}
}
