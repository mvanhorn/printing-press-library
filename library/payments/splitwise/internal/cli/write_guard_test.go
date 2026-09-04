package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func nudgeFixture(t *testing.T, ambiguous bool) string {
	friends := map[string]string{"20": `{"id":20,"first_name":"Friend","last_name":"One","balance":[{"currency_code":"USD","amount":"10.00"}]}`}
	if ambiguous {
		friends = map[string]string{"20": `{"id":20,"first_name":"Same","last_name":"One"}`, "21": `{"id":21,"first_name":"Same","last_name":"Two"}`}
	}
	return seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user": {"10": `{"user":{"id":10}}`}, "get-friends": friends,
		"get-groups":   {"7": `{"id":7,"name":"Group Alpha","members":[{"id":10},{"id":20}],"simplified_debts":[{"from":20,"to":10,"amount":"10.00","currency_code":"USD"}]}`},
		"get-expenses": {"50": `{"id":50,"date":"2026-08-01","description":"Synthetic item","cost":"20.00","currency_code":"USD","users":[{"user_id":10,"paid_share":"20.00","owed_share":"10.00"},{"user_id":20,"paid_share":"0.00","owed_share":"10.00"}]}`},
	})
}

func TestWritePathsRefuseDogfoodHarnessWithoutHTTP(t *testing.T) {
	p := nudgeFixture(t, false)
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	t.Setenv("SPLITWISE_BASE_URL", srv.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	commands := [][]string{{"settle-up", "Group Alpha", "--record", "--json", "--db", p}, {"split", "Group Alpha", "--amount", "10", "--equal", "--record", "--json", "--db", p}, {"fairness", "nudge", "Friend One", "--send", "--json", "--db", p}}
	for _, args := range commands {
		out, _, err := runRootArgs(t, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		var env struct {
			Refused bool `json:"refused"`
		}
		if json.Unmarshal([]byte(out), &env) != nil || !env.Refused {
			t.Fatalf("%v envelope=%s", args, out)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("HTTP hits=%d", hits.Load())
	}
}

func TestFairnessNudgeSendAndAmbiguity(t *testing.T) {
	p := nudgeFixture(t, false)
	var posts atomic.Int64
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		posts.Add(1)
		if r.URL.Path != "/create_comment" {
			t.Errorf("path=%s", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{"comments": []any{map[string]any{"id": 70}}})
	}))
	defer srv.Close()
	t.Setenv("SPLITWISE_BASE_URL", srv.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	_, stderr, err := runRootArgs(t, "fairness", "nudge", "Friend One", "--send", "--message", "hi", "--json", "--db", p)
	if err != nil {
		t.Fatalf("%v stderr=%s", err, stderr)
	}
	if posts.Load() != 1 || body["expense_id"] != "50" || body["content"] != "hi" {
		t.Fatalf("posts=%d body=%v", posts.Load(), body)
	}
	amb := nudgeFixture(t, true)
	_, _, err = runRootArgs(t, "fairness", "nudge", "Same", "--send", "--message", "hi", "--json", "--db", amb)
	if ExitCode(err) != 2 {
		t.Fatalf("exit=%d err=%v", ExitCode(err), err)
	}
	if posts.Load() != 1 {
		t.Fatalf("ambiguous posted; posts=%d", posts.Load())
	}
}
