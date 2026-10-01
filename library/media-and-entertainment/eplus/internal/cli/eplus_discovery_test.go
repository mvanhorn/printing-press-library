package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/eplus/internal/discovery"
	"github.com/spf13/cobra"
	"strings"
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

func TestComparisonKeepsEveryInputWithinLimit(t *testing.T) {
	first := make([]discovery.Row, 30)
	for i := range first {
		first[i] = discovery.Row{"input_id": "first"}
	}
	groups := [][]discovery.Row{first, {{"input_id": "second"}}, {{"input_id": "third"}}}
	rows, truncated := balancedComparisonRows(groups, 20)
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row["input_id"].(string)] = true
	}
	if len(rows) != 20 || !truncated || len(seen) != 3 {
		t.Fatalf("comparison coverage lost under combined cap: rows=%d, seen=%v", len(rows), seen)
	}
}

func TestComparisonRejectsLimitThatCannotRepresentInputs(t *testing.T) {
	c := newCompare(&rootFlags{})
	if err := c.Flags().Set("limit", "1"); err != nil {
		t.Fatal(err)
	}
	err := c.RunE(c, []string{"4592490001", "4592490002"})
	if err == nil || !strings.Contains(err.Error(), "at least the number of comparison inputs") {
		t.Fatalf("undersized comparison cap accepted: %v", err)
	}
}

func TestJSONPartialFailureDiagnosticsUseStderr(t *testing.T) {
	var out, stderr bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&out)
	c.SetErr(&stderr)
	r := discovery.Result{Data: []discovery.Row{{"id": "one"}}, Meta: discovery.Row{"partial": true, "warnings": []string{"1 of 2 international detail reads failed; results are partial"}}}
	if err := writeDiscovery(c, &rootFlags{agent: true, asJSON: true}, r, 10); err != nil {
		t.Fatal(err)
	}
	if !json.Valid(out.Bytes()) || !strings.Contains(stderr.String(), "reads failed") {
		t.Fatalf("JSON/diagnostic channels lost: %s / %s", out.String(), stderr.String())
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
