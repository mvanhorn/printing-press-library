// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
)

// TestNovelSettleUpHelpWires smoke-tests that the settle-up command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSettleUpHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"settle-up", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("settle-up --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "settle-up"} {
		if !strings.Contains(help, want) {
			t.Fatalf("settle-up --help missing %q in output:\n%s", want, help)
		}
	}
}

func settleFixture(t *testing.T, ambiguous bool) string {
	groups := map[string]string{"7": `{"id":7,"name":"Group Alpha","members":[{"id":10,"first_name":"User","last_name":"Self"},{"id":20,"first_name":"User","last_name":"Debtor"},{"id":30,"first_name":"User","last_name":"Creditor"}],"simplified_debts":[{"from":20,"to":30,"amount":"12.34","currency_code":"USD"},{"from":10,"to":30,"amount":"0","currency_code":"USD"}]}`}
	if ambiguous {
		groups = map[string]string{"7": `{"id":7,"name":"Trip A"}`, "8": `{"id":8,"name":"Trip B"}`}
	}
	return seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user": {"10": `{"user":{"id":10}}`}, "get-groups": groups,
		"get-friends": {"20": `{"id":20,"first_name":"Friend","last_name":"One","balance":[{"currency_code":"USD","amount":"5.25"},{"currency_code":"EUR","amount":"-2.00"},{"currency_code":"CAD","amount":"0"}]}`},
	})
}

func TestSettleUpPlansAndTypedResolution(t *testing.T) {
	path := settleFixture(t, false)
	for _, tc := range []struct {
		name, target string
		want         []settleTransfer
	}{
		{"group", "Group Alpha", []settleTransfer{{FromID: 20, FromName: "User Debtor", ToID: 30, ToName: "User Creditor", Amount: 12.34, CurrencyCode: "USD"}}},
		{"friend", "Friend One", []settleTransfer{{FromID: 20, FromName: "Friend One", ToID: 10, ToName: "you", Amount: 5.25, CurrencyCode: "USD"}, {FromID: 10, FromName: "you", ToID: 20, ToName: "Friend One", Amount: 2, CurrencyCode: "EUR"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, stderr, err := runRootArgs(t, "settle-up", tc.target, "--json", "--db", path)
			if err != nil {
				t.Fatalf("%v %s", err, stderr)
			}
			var got struct {
				Transfers []settleTransfer `json:"transfers"`
			}
			if json.Unmarshal([]byte(out), &got) != nil {
				t.Fatal(out)
			}
			if fmt.Sprint(got.Transfers) != fmt.Sprint(tc.want) {
				t.Fatalf("got=%+v want=%+v", got.Transfers, tc.want)
			}
		})
	}
	amb := settleFixture(t, true)
	out, _, err := runRootArgs(t, "settle-up", "Trip", "--json", "--db", amb)
	if ExitCode(err) != 2 {
		t.Fatalf("ambiguous exit=%d", ExitCode(err))
	}
	var env map[string]any
	if json.Unmarshal([]byte(out), &env) != nil || env["code"] != float64(2) {
		t.Fatalf("ambiguous envelope=%s", out)
	}
	out, stderr, err := runRootArgs(t, "settle-up", "No Such", "--json", "--db", path)
	if ExitCode(err) != 3 {
		t.Fatalf("missing exit=%d", ExitCode(err))
	}
	if json.Unmarshal([]byte(out), &env) != nil || env["code"] != float64(3) {
		t.Fatalf("missing envelope=%s", out)
	}
	if strings.Contains(stderr, "Error:") {
		t.Fatalf("stderr=%q", stderr)
	}
	out, _, err = runRootArgs(t, "settle-up", "Group Alpha", "--dry-run", "--json", "--db", path)
	if err != nil || !strings.Contains(out, `"dry_run":true`) {
		t.Fatalf("dry=%s err=%v", out, err)
	}
}

func TestSettleUpEmptyPlanSkipsDedupScan(t *testing.T) {
	path := seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user": {"10": `{"user":{"id":10}}`},
		"get-groups":       {"7": `{"id":7,"name":"Settled","members":[{"id":10}],"simplified_debts":[]}`},
	})
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); http.Error(w, "unexpected", 500) }))
	defer srv.Close()
	t.Setenv("SPLITWISE_BASE_URL", srv.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	out, stderr, err := runRootArgs(t, "settle-up", "Settled", "--record", "--json", "--db", path)
	var env struct {
		Count int `json:"count"`
	}
	decodeErr := json.Unmarshal([]byte(out), &env)
	if err != nil || hits.Load() != 0 || decodeErr != nil || env.Count != 0 {
		t.Fatalf("err=%v hits=%d out=%s stderr=%s", err, hits.Load(), out, stderr)
	}
}

func TestSettleUpRecordDedupForceAndCache(t *testing.T) {
	path := settleFixture(t, false)
	var gets, posts atomic.Int64
	existing := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get_expenses":
			gets.Add(1)
			expenses := []any{}
			if existing {
				expenses = []any{map[string]any{"id": 90, "payment": true, "cost": "12.34", "currency_code": "USD", "users": []any{map[string]any{"user_id": 20, "paid_share": "12.34", "owed_share": "0.00"}, map[string]any{"user_id": 30, "paid_share": "0.00", "owed_share": "12.34"}}}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"expenses": expenses})
		case "/create_expense":
			posts.Add(1)
			var body struct {
				Cost     string `json:"cost"`
				Currency string `json:"currency_code"`
				GroupID  int    `json:"group_id"`
				Payment  bool   `json:"payment"`
				Users    []struct {
					UserID int    `json:"user_id"`
					Paid   string `json:"paid_share"`
					Owed   string `json:"owed_share"`
				} `json:"users"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			wantUsers := []struct {
				UserID int    `json:"user_id"`
				Paid   string `json:"paid_share"`
				Owed   string `json:"owed_share"`
			}{{20, "12.34", "0.00"}, {30, "0.00", "12.34"}}
			if body.Cost != "12.34" || body.Currency != "USD" || body.GroupID != 7 || !body.Payment || !reflect.DeepEqual(body.Users, wantUsers) {
				t.Errorf("posted body=%+v want debtor 20 paying creditor 30", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"expenses": []any{map[string]any{"id": 91, "payment": true, "cost": "12.34", "currency_code": "USD", "users": body.Users}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	t.Setenv("SPLITWISE_BASE_URL", srv.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	out, stderr, err := runRootArgs(t, "settle-up", "Group Alpha", "--record", "--json", "--db", path)
	if err != nil {
		t.Fatalf("%v %s", err, stderr)
	}
	if posts.Load() != 0 || !strings.Contains(out, `"skipped_existing"`) {
		t.Fatalf("posts=%d out=%s", posts.Load(), out)
	}
	existing = false
	out, stderr, err = runRootArgs(t, "settle-up", "Group Alpha", "--record", "--force", "--json", "--db", path)
	if err != nil {
		t.Fatalf("%v %s", err, stderr)
	}
	if posts.Load() != 1 {
		t.Fatalf("posts=%d", posts.Load())
	}
	if gets.Load() != 1 {
		t.Fatalf("force unexpectedly scanned: gets=%d", gets.Load())
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.List("get-expenses", 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("cached rows=%d err=%v", len(rows), err)
	}
}

func TestSettleUpDedupRequiresLiveExactDirection(t *testing.T) {
	deleted := "2026-09-04T00:00:00Z"
	for _, tc := range []struct {
		name      string
		from, to  int
		deletedAt *string
	}{
		{"opposite direction", 30, 20, nil},
		{"deleted exact match", 20, 30, &deleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := settleFixture(t, false)
			var posts atomic.Int64
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/get_expenses" {
					row := map[string]any{"id": 90, "payment": true, "cost": "12.34", "currency_code": "USD", "users": []any{map[string]any{"user_id": tc.from, "paid_share": "12.34", "owed_share": "0.00"}, map[string]any{"user_id": tc.to, "paid_share": "0.00", "owed_share": "12.34"}}}
					if tc.deletedAt != nil {
						row["deleted_at"] = *tc.deletedAt
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"expenses": []any{row}})
					return
				}
				posts.Add(1)
				_ = json.NewEncoder(w).Encode(map[string]any{"expenses": []any{map[string]any{"id": 91}}})
			}))
			defer srv.Close()
			t.Setenv("SPLITWISE_BASE_URL", srv.URL)
			t.Setenv("SPLITWISE_API_KEY", "test")
			t.Setenv("PRINTING_PRESS_DOGFOOD", "")
			t.Setenv("PRINTING_PRESS_VERIFY", "")
			out, stderr, err := runRootArgs(t, "settle-up", "Group Alpha", "--record", "--json", "--db", path)
			var env struct {
				Skipped []any `json:"skipped_existing"`
			}
			decodeErr := json.Unmarshal([]byte(out), &env)
			if err != nil || posts.Load() != 1 || decodeErr != nil || len(env.Skipped) != 0 {
				t.Fatalf("err=%v posts=%d out=%s stderr=%s", err, posts.Load(), out, stderr)
			}
		})
	}
}

func TestSettleUpRejectedMutationDoesNotCache(t *testing.T) {
	path := settleFixture(t, false)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get_expenses" {
			_, _ = w.Write([]byte(`{"expenses":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"expenses":[],"errors":{"base":["rejected"]}}`))
	}))
	defer srv.Close()
	t.Setenv("SPLITWISE_BASE_URL", srv.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	out, _, err := runRootArgs(t, "settle-up", "Group Alpha", "--record", "--json", "--db", path)
	var env struct {
		Count int `json:"count"`
	}
	decodeErr := json.Unmarshal([]byte(out), &env)
	if err == nil || !strings.Contains(err.Error(), "rejected") || decodeErr != nil || env.Count != 0 {
		t.Fatalf("err=%v out=%s", err, out)
	}
	s, openErr := store.Open(path)
	if openErr != nil {
		t.Fatal(openErr)
	}
	defer s.Close()
	rows, listErr := s.List("get-expenses", 0)
	if listErr != nil || len(rows) != 0 {
		t.Fatalf("rows=%d err=%v", len(rows), listErr)
	}
}

func TestSplitwiseMutationError(t *testing.T) {
	for _, raw := range []string{`{}`, `{"errors":{}}`, `{"errors":[]}`, `{"errors":null}`} {
		if err := splitwiseMutationError([]byte(raw)); err != nil {
			t.Errorf("%s: %v", raw, err)
		}
	}
	err := splitwiseMutationError([]byte(`{"errors":{"base":["rejected"]}}`))
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err=%v", err)
	}
}
