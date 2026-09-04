package cli

import (
	"encoding/json"
	"testing"
)

func TestEmptyStoreMachineListsAreArrays(t *testing.T) {
	p := seedSyntheticStore(t, map[string]map[string]string{})
	tests := []struct {
		name   string
		args   []string
		fields []string
	}{
		{"spend", []string{"spend", "--json", "--db", p}, nil}, {"debts", []string{"debts", "--json", "--db", p}, nil},
		{"net", []string{"net", "--json", "--db", p}, []string{"plan", "by_currency", "savings"}}, {"audit", []string{"audit", "--json", "--db", p}, []string{"duplicates", "outliers"}},
		{"report", []string{"report", "--format", "json", "--db", p}, []string{"people", "categories", "expenses"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, stderr, err := runRootArgs(t, tc.args...)
			if err != nil {
				t.Fatalf("%v stderr=%s", err, stderr)
			}
			var raw any
			if err := json.Unmarshal([]byte(out), &raw); err != nil {
				t.Fatalf("%v raw=%s", err, out)
			}
			if tc.fields == nil {
				if _, ok := raw.([]any); !ok {
					t.Fatalf("want top-level []: %s", out)
				}
				return
			}
			obj := raw.(map[string]any)
			for _, f := range tc.fields {
				if _, ok := obj[f].([]any); !ok {
					t.Errorf("%s.%s not []: %s", tc.name, f, out)
				}
			}
		})
	}
}
