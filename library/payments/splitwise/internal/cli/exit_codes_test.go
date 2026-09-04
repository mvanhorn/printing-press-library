package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNovelTypedExitCodes(t *testing.T) {
	p := seedSyntheticStore(t, map[string]map[string]string{"get-current-user": {"1": `{"user":{"id":10}}`}})
	tests := []struct {
		name string
		args []string
		code int
	}{
		{"split missing", []string{"split", "Missing", "--amount", "1", "--json", "--db", p}, 3},
		{"settle missing", []string{"settle-up", "Missing", "--json", "--db", p}, 3},
		{"ledger group missing", []string{"ledger", "Missing", "--json", "--db", p}, 3},
		{"ledger friend missing", []string{"ledger", "--friend", "Missing", "--json", "--db", p}, 3},
		{"report missing", []string{"report", "--group", "Missing", "--json", "--db", p}, 3},
		{"reconcile since", []string{"reconcile", "--since", "nonsense", "--db", p}, 2},
		{"ledger both", []string{"ledger", "Group", "--friend", "Friend", "--db", p}, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, stderr, err := runRootArgs(t, tc.args...)
			if got := ExitCode(err); got != tc.code {
				t.Fatalf("exit=%d want=%d err=%v", got, tc.code, err)
			}
			if tc.code == 3 && containsArg(tc.args, "--json") {
				var env struct {
					Code  int    `json:"code"`
					Error string `json:"error"`
				}
				if json.Unmarshal([]byte(out), &env) != nil || env.Code != 3 || env.Error == "" {
					t.Fatalf("invalid error envelope: %q", out)
				}
				if strings.Contains(stderr, "Error:") {
					t.Fatalf("unexpected cobra error line: %q", stderr)
				}
			}
		})
	}

	nudgeDB := nudgeFixture(t, false)
	out, stderr, err := runRootArgs(t, "fairness", "nudge", "Missing", "--json", "--db", nudgeDB)
	if ExitCode(err) != 3 {
		t.Fatalf("fairness nudge exit=%d err=%v", ExitCode(err), err)
	}
	var env struct {
		Code  int    `json:"code"`
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(out), &env) != nil || env.Code != 3 || env.Error == "" || strings.Contains(stderr, "Error:") {
		t.Fatalf("fairness nudge envelope=%q stderr=%q", out, stderr)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestAmbiguousWriteTargetsExitTwo(t *testing.T) {
	p := seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user": {"1": `{"user":{"id":10}}`},
		"get-groups":       {"7": `{"id":7,"name":"Trip One","members":[{"id":10}]}`, "8": `{"id":8,"name":"Trip Two","members":[{"id":10}]}`},
		"get-friends":      {"20": `{"id":20,"first_name":"Friend","last_name":"One"}`, "21": `{"id":21,"first_name":"Friend","last_name":"Two"}`},
	})
	for _, args := range [][]string{
		{"split", "Trip", "--amount", "1", "--json", "--db", p},
		{"settle-up", "Friend", "--json", "--db", p},
	} {
		_, _, err := runRootArgs(t, args...)
		if ExitCode(err) != 2 {
			t.Fatalf("%v exit=%d err=%v", args, ExitCode(err), err)
		}
	}
}
