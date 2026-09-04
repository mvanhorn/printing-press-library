// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
	"net/http"
	"net/http/httptest"
)

func seedSyntheticStore(t *testing.T, rows map[string]map[string]string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "synthetic.db")
	s, err := store.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	for resource, entries := range rows {
		for id, body := range entries {
			if err := s.Upsert(resource, id, []byte(body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSplitShareMath(t *testing.T) {
	members := []GroupMember{{ID: 30}, {ID: 10}, {ID: 20}}
	sorted := []GroupMember{{ID: 10}, {ID: 20}, {ID: 30}}
	set := map[int]struct{}{10: {}, 20: {}, 30: {}}
	tests := []struct {
		name  string
		total int64
		run   func(map[int]int64) error
		want  map[int]int64
	}{
		{"84 equal", 8400, func(o map[int]int64) error { return allocateEqual(8400, members, o) }, nil},
		{"29 cents equal", 29, func(o map[int]int64) error { return allocateEqual(29, sorted[:2], o) }, map[int]int64{10: 15, 20: 14}},
		{"2.675 equal", 268, func(o map[int]int64) error { return allocateEqual(268, sorted, o) }, map[int]int64{10: 90, 20: 89, 30: 89}},
		{"percent tie by id", 100, func(o map[int]int64) error {
			return allocateWeighted(100, "30:33.333333,10:33.333333,20:33.333334", sorted, set, o, 100)
		}, map[int]int64{10: 33, 20: 34, 30: 33}},
		{"exact", 1000, func(o map[int]int64) error { return allocateExact(1000, "10:1.25,20:2.75,30:6", sorted, set, o) }, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := map[int]int64{}
			if err := tc.run(o); err != nil {
				t.Fatal(err)
			}
			var sum int64
			for _, v := range o {
				sum += v
			}
			if sum != tc.total {
				t.Fatalf("sum=%d want %d (%v)", sum, tc.total, o)
			}
			for id, w := range tc.want {
				if o[id] != w {
					t.Fatalf("id %d=%d want %d", id, o[id], w)
				}
			}
		})
	}
	bad := []func() error{
		func() error { return allocateExact(100, "99:1", sorted, set, map[int]int64{}) },
		func() error { return allocateExact(100, "10:-1,20:2", sorted, set, map[int]int64{}) },
		func() error { return allocateWeighted(100, "10:40,20:40", sorted, set, map[int]int64{}, 100) },
		func() error { return allocateExact(100, "10:.20,20:.20", sorted, set, map[int]int64{}) },
	}
	for i, fn := range bad {
		if err := fn(); err == nil {
			t.Errorf("bad case %d did not error", i)
		}
	}
	if got := dollarsToCents(.29); got != 29 {
		t.Fatalf("dollarsToCents(.29)=%d", got)
	}
	if centsToMoneyString(268) != "2.68" {
		t.Fatal(centsToMoneyString(268))
	}
	if _, err := parsePairs("10"); err == nil {
		t.Fatal("parsePairs accepted malformed pair")
	}
}

func TestSplitPreviewBalancesAndDoesNotUseHTTP(t *testing.T) {
	path := seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user": {"1": `{"user":{"id":10}}`},
		"get-groups":       {"7": `{"id":7,"name":"Group Alpha","members":[{"id":30,"first_name":"Member","last_name":"C"},{"id":10,"first_name":"Member","last_name":"A"},{"id":20,"first_name":"Member","last_name":"B"}]}`},
	})
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); http.Error(w, "unexpected", 500) }))
	defer srv.Close()
	t.Setenv("SPLITWISE_BASE_URL", srv.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	out, stderr, err := runRootArgs(t, "split", "Group Alpha", "--amount", "2.675", "--equal", "--json", "--db", path)
	if err != nil {
		t.Fatalf("%v stderr=%s", err, stderr)
	}
	if hits.Load() != 0 {
		t.Fatalf("hits=%d", hits.Load())
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatalf("%v raw=%s", err, out)
	}
	body := raw["expense_body"].(map[string]any)
	toCents := func(v any) int64 {
		f, e := strconv.ParseFloat(v.(string), 64)
		if e != nil {
			t.Fatal(e)
		}
		return dollarsToCents(f)
	}
	cost := toCents(body["cost"])
	var owed, paid int64
	for _, u := range body["users"].([]any) {
		m := u.(map[string]any)
		owed += toCents(m["owed_share"])
		paid += toCents(m["paid_share"])
	}
	if owed != cost || paid != cost {
		t.Fatalf("cost=%d owed=%d paid=%d body=%v", cost, owed, paid, body)
	}
}

// TestNovelSplitHelpWires smoke-tests that the split command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSplitHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"split", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("split --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "split"} {
		if !strings.Contains(help, want) {
			t.Fatalf("split --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestSplitRecordMatchesPreviewAndCaches(t *testing.T) {
	path := seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user": {"1": `{"user":{"id":10}}`},
		"get-groups":       {"7": `{"id":7,"name":"Group Alpha","members":[{"id":10},{"id":20}]}`},
	})
	args := []string{"split", "Group Alpha", "--amount", "12.34", "--equal", "--json", "--db", path}
	preview, _, err := runRootArgs(t, args...)
	if err != nil {
		t.Fatal(err)
	}
	var previewEnv struct {
		ExpenseBody map[string]any `json:"expense_body"`
	}
	if err := json.Unmarshal([]byte(preview), &previewEnv); err != nil {
		t.Fatal(err)
	}
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/create_expense" {
			t.Errorf("path=%s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Error(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"expenses": []any{map[string]any{"id": 88, "cost": posted["cost"], "users": posted["users"]}}})
	}))
	defer srv.Close()
	t.Setenv("SPLITWISE_BASE_URL", srv.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	recorded, stderr, err := runRootArgs(t, append(args, "--record")...)
	var result struct {
		Created bool `json:"created"`
	}
	decodeErr := json.Unmarshal([]byte(recorded), &result)
	postedJSON, _ := json.Marshal(posted)
	previewJSON, _ := json.Marshal(previewEnv.ExpenseBody)
	if err != nil || decodeErr != nil || !result.Created || !bytes.Equal(postedJSON, previewJSON) {
		t.Fatalf("err=%v stderr=%s recorded=%s\nposted=%v\npreview=%v", err, stderr, recorded, posted, previewEnv.ExpenseBody)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rows, err := s.List("get-expenses", 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%d err=%v", len(rows), err)
	}
}

func TestSplitRejectedMutationDoesNotCache(t *testing.T) {
	path := seedSyntheticStore(t, map[string]map[string]string{
		"get-current-user": {"1": `{"user":{"id":10}}`},
		"get-groups":       {"7": `{"id":7,"name":"Group Alpha","members":[{"id":10},{"id":20}]}`},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"expenses":[],"errors":{"base":["rejected"]}}`))
	}))
	defer srv.Close()
	t.Setenv("SPLITWISE_BASE_URL", srv.URL)
	t.Setenv("SPLITWISE_API_KEY", "test")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	out, _, err := runRootArgs(t, "split", "Group Alpha", "--amount", "12.34", "--equal", "--record", "--json", "--db", path)
	var result struct {
		Created bool `json:"created"`
	}
	decodeErr := json.Unmarshal([]byte(out), &result)
	if err == nil || !strings.Contains(err.Error(), "rejected") || decodeErr != nil || result.Created {
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
