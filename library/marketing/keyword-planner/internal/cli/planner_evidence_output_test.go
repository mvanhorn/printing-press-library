package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPlannerEvidenceAgentPreservesNestedValues(t *testing.T) {
	input := json.RawMessage(`{"checks":[{"name":"receipt_hashes","passed":false,"issue_count":1}],"groups":[{"metric_id":"fixture","sum_monthly_searches":"9223372036854775808","observed_mean":"4611686018427387904","monthly_searches":9007199254740993,"short_window":true}],"submitted":[{"index":0,"text":"beef, brisket","kind":"keyword"}],"nullable":null}`)
	flags := &rootFlags{asJSON: true, agent: true, compact: true}
	var output bytes.Buffer
	if err := printPlannerEvidenceOutput(&output, input, flags, map[string]any{"source": "local"}, map[string]bool{"checks": true, "groups": true}); err != nil {
		t.Fatal(err)
	}
	for _, wanted := range []string{`"passed": false`, `"issue_count": 1`, `"text": "beef, brisket"`, `"metric_id": "fixture"`, `"monthly_searches": 9007199254740993`, `"sum_monthly_searches": "9223372036854775808"`, `"short_window": true`, `"nullable": null`} {
		if !strings.Contains(output.String(), wanted) {
			t.Fatalf("lost evidence field %s in %s", wanted, output.String())
		}
	}
	if !flags.compact {
		t.Fatal("renderer mutated shared root flags")
	}
}
