package cli

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
)

func TestHandCommandsCSVNeverFlattenObjects(t *testing.T) {
	db := seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user":  {"10": `{"user":{"id":10,"first_name":"Current","last_name":"User"}}`},
		"get-groups":        {"7": `{"id":7,"name":"Example Group","members":[{"id":10,"first_name":"Current","last_name":"User","balance":[{"currency_code":"USD","amount":"12.00"}]},{"id":20,"first_name":"Example","last_name":"Friend","balance":[{"currency_code":"USD","amount":"-12.00"}]}],"simplified_debts":[{"from":20,"to":10,"amount":"12.00","currency_code":"USD"}]}`},
		"get-friends":       {"20": `{"id":20,"first_name":"Example","last_name":"Friend","balance":[{"currency_code":"USD","amount":"12.00"}]}`},
		"get-expenses":      {"100": `{"id":100,"group_id":7,"description":"Synthetic Dinner","cost":"24.00","currency_code":"USD","date":"2026-09-01T12:00:00Z","updated_at":"2026-09-01T12:00:00Z","category":{"name":"Dining"},"users":[{"user_id":10,"paid_share":"24.00","owed_share":"12.00"},{"user_id":20,"paid_share":"0.00","owed_share":"12.00"}]}`},
		"get-notifications": {"200": `{"id":200,"created_at":"2026-09-01T12:00:00Z","content":"Synthetic activity"}`},
	})
	commands := []struct {
		name string
		args []string
	}{
		{"activity", []string{"activity", "--since", "30d"}},
		{"audit", []string{"audit", "--since", "90d"}},
		{"balances", []string{"balances"}},
		{"brief", []string{"brief"}},
		{"debts", []string{"debts"}},
		{"fairness", []string{"fairness"}},
		{"forecast", []string{"forecast"}},
		{"ledger", []string{"ledger", "Example Group"}},
		{"net", []string{"net"}},
		{"normalize", []string{"normalize", "--rate", "EUR=1.1"}},
		{"recurring", []string{"recurring"}},
		{"report", []string{"report", "--group", "Example Group"}},
		{"resolve", []string{"resolve", "Example Group"}},
		{"spend", []string{"spend"}},
		{"settle-up", []string{"settle-up", "Example Group"}},
	}
	for _, tc := range commands {
		for _, mode := range []string{"--csv", "--plain"} {
			t.Run(tc.name+mode, func(t *testing.T) {
				args := append(append([]string{}, tc.args...), mode, "--db", db)
				out, stderr, err := runRootArgs(t, args...)
				if err != nil {
					t.Fatalf("err=%v stderr=%s stdout=%s", err, stderr, out)
				}
				trimmed := strings.TrimSpace(out)
				if trimmed == "" {
					if tc.name == "resolve" && stderr != "" {
						return
					}
					t.Fatalf("empty stdout; stderr=%q", stderr)
				}
				if strings.Contains(trimmed, "map[") || strings.Contains(trimmed, "[map[") {
					t.Fatalf("Go-formatted map leaked: %q", trimmed)
				}
				var doc any
				if json.Unmarshal([]byte(trimmed), &doc) == nil {
					return
				}
				r := csv.NewReader(strings.NewReader(trimmed))
				if mode == "--plain" {
					r.Comma = '\t'
				}
				records, parseErr := r.ReadAll()
				if parseErr != nil || len(records) < 1 || len(records[0]) < 1 {
					t.Fatalf("not JSON or tabular output: parse=%v output=%q", parseErr, trimmed)
				}
			})
		}
	}
}
