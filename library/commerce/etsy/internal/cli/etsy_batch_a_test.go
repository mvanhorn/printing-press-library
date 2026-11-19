package cli

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/store"
)

func TestBatchAWhichRealIndex(t *testing.T) {
	for q, want := range map[string]string{
		"compare two snapshots": "market trend", "what changed between snapshots": "market trend",
		"which listings entered or left": "market trend", "trend over time": "market trend",
		"capture a snapshot of search results": "market snapshot", "start a market research question": "market snapshot",
	} {
		got := rankWhich(whichIndex, q, 1)
		if len(got) == 0 || got[0].Entry.Command != want {
			t.Errorf("%q: %+v want %s", q, got, want)
		}
	}
}

func TestBatchAAnalyticsReadOnly(t *testing.T) {
	syncCLIIsolate(t)
	path := filepath.Join(t.TempDir(), "read.db")
	db, err := store.Open(path)
	s2Must(t, err)
	s2Must(t, db.Upsert("listings", "1000000001", json.RawMessage(`{"listing_id":1000000001,"title":"ceramic mug"}`)))
	s2Must(t, db.Close())
	s2Must(t, os.Chmod(path, 0400))
	before, err := os.ReadFile(path)
	s2Must(t, err)
	baseline, err := store.OpenReadOnly(path)
	s2Must(t, err)
	var originalJournal string
	s2Must(t, baseline.DB().QueryRow("PRAGMA journal_mode").Scan(&originalJournal))
	s2Must(t, baseline.Close())
	for _, mode := range [][]string{nil, {"--json"}, {"--plain"}, {"--csv"}, {"--agent"}, {"--group-by", "title", "--json"}, {"--group-by", "title", "--plain"}, {"--group-by", "title", "--csv"}} {
		args := append([]string{"analytics", "--db", path, "--type", "listings"}, mode...)
		_, _, err = s2Run(t, args...)
		s2Must(t, err)
		after, e := os.ReadFile(path)
		s2Must(t, e)
		info, e := os.Stat(path)
		s2Must(t, e)
		if sha256.Sum256(before) != sha256.Sum256(after) || info.Mode().Perm() != 0400 {
			t.Fatal("analytics mutated read-only store")
		}
	}
	ro, err := store.OpenReadOnly(path)
	s2Must(t, err)
	defer ro.Close()
	var journal string
	s2Must(t, ro.DB().QueryRow("PRAGMA journal_mode").Scan(&journal))
	if journal != originalJournal {
		t.Fatalf("journal=%s", journal)
	}
	_, _, err = s2Run(t, "analytics", "--db", path, "--type", "unknown_type")
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), "unknown resource type") {
		t.Fatal(err)
	}
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	out, _, err := s2Run(t, "analytics", "--db", path, "--type", "listings")
	s2Must(t, err)
	if !json.Valid([]byte(out)) {
		t.Fatal(out)
	}
	for _, mode := range []string{"--plain", "--csv"} {
		out, _, err = s2Run(t, "analytics", "--db", path, "--type", "listings", mode)
		s2Must(t, err)
		if json.Valid([]byte(out)) {
			t.Fatalf("explicit %s lost to MCP JSON: %s", mode, out)
		}
	}
	out, _, err = s2Run(t, "analytics", "--db", filepath.Join(t.TempDir(), "missing.db"), "--type", "unknown_type")
	etsyAssertBuyerRefusal(t, out, err)
	if strings.Contains(err.Error(), "known types") {
		t.Fatal("type discovery before privacy refusal")
	}
}

func TestBatchAAnalyticsMissingAndOldSchema(t *testing.T) {
	syncCLIIsolate(t)
	path := filepath.Join(t.TempDir(), "absent", "missing.db")
	for _, extra := range [][]string{nil, {"--type", "listings"}, {"--type", "listings", "--group-by", "title"}} {
		out, stderr, err := s2Run(t, append([]string{"analytics", "--db", path, "--json"}, extra...)...)
		s2Must(t, err)
		want := "{}"
		if len(extra) == 2 {
			want = `{"count":0,"resource_type":"listings"}`
		}
		if len(extra) > 2 {
			want = "[]"
		}
		if strings.Join(strings.Fields(out), "") != want || !strings.Contains(stderr, "local store has not been synced yet") {
			t.Fatal(out, stderr)
		}
		if _, err = os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
			t.Fatalf("created missing directory: %v", err)
		}
	}
	old := filepath.Join(t.TempDir(), "old.db")
	db, err := store.Open(old)
	s2Must(t, err)
	s2Must(t, db.Upsert("listings", "1", json.RawMessage(`{"title":"old"}`)))
	_, err = db.DB().Exec("DROP TABLE IF EXISTS etsy_payment_checks; CREATE TABLE etsy_payment_checks(receipt_id TEXT PRIMARY KEY, receipt_updated INTEGER, checked_at INTEGER); PRAGMA user_version=1")
	s2Must(t, err)
	s2Must(t, db.Close())
	before, err := os.ReadFile(old)
	s2Must(t, err)
	out, _, err := s2Run(t, "analytics", "--db", old, "--type", "listings", "--json")
	s2Must(t, err)
	if strings.Join(strings.Fields(out), "") != `{"count":1,"resource_type":"listings"}` {
		t.Fatal(out)
	}
	after, err := os.ReadFile(old)
	s2Must(t, err)
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("older store migrated")
	}
	ro, err := store.OpenReadOnly(old)
	s2Must(t, err)
	defer ro.Close()
	version, err := ro.SchemaVersion()
	s2Must(t, err)
	if version != 1 {
		t.Fatal(version)
	}
}

func TestBatchABulkResumeRestart(t *testing.T) {
	syncCLIIsolate(t)
	_, _, err := s2Run(t, "listings", "bulk-edit", "--restart", "--resume", "synthetic-job")
	if ExitCode(err) != 2 || err.Error() != "--restart and --resume can't be combined (--restart is deprecated)" {
		t.Fatal(err)
	}
}

func TestBatchAPaymentSuccessMarkers(t *testing.T) {
	syncCLIIsolate(t)
	db, err := store.Open(filepath.Join(t.TempDir(), "ledger.db"))
	s2Must(t, err)
	defer db.Close()
	s2Must(t, db.RecordPaymentDenied("404", sql.NullInt64{Int64: 100, Valid: true}))
	s2Must(t, db.RecordPaymentCheck("ok", sql.NullInt64{}))
	_, err = db.DB().Exec(`INSERT INTO etsy_payment_checks(receipt_id,receipt_updated,checked_at,status) VALUES ('legacy',100,1,NULL),('rearmed',NULL,1,NULL)`)
	s2Must(t, err)
	rows, err := db.ReadPaymentChecks()
	s2Must(t, err)
	for id, want := range map[string]bool{"404": false, "ok": true, "legacy": true, "rearmed": false} {
		if rows[id].Successful != want {
			t.Errorf("%s: %+v", id, rows[id])
		}
	}
}

func TestBatchAPaymentDecisionTable(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		capped                bool
		checked, forbidden    int64
		lost, success, strict bool
		code                  int
		needle                string
	}{
		{"cap scope", true, 0, 1, false, false, false, 1, "transactions_r"},
		{"cap foreign", true, 0, 1, false, true, false, 7, "no progress"},
		{"cap without forbidden", true, 0, 0, false, false, false, 7, "no progress"},
		{"cap progress", true, 1, 1, true, true, false, 7, "receipts checked"},
		{"no forbidden", false, 0, 0, false, false, false, 0, ""},
		{"lost", false, 1, 1, true, true, false, 1, "transactions_r"},
		{"never succeeded", false, 0, 1, false, false, false, 1, "transactions_r"},
		{"strict", false, 0, 1, false, true, true, 1, "1 receipt(s) forbidden under --strict"},
		{"foreign", false, 0, 1, false, true, false, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := etsyPaymentsDecision(tc.capped, false, tc.checked, 2, tc.forbidden, nil, tc.lost, tc.success, tc.strict)
			code := 0
			if err != nil {
				code = ExitCode(err)
			}
			if code != tc.code || (tc.needle != "" && (err == nil || !strings.Contains(err.Error(), tc.needle))) {
				t.Fatalf("%v (%d), want %d %q", err, code, tc.code, tc.needle)
			}
			if tc.strict && err != nil && strings.Contains(err.Error(), "re-authorize") {
				t.Fatal(err)
			}
		})
	}
}

func TestBatchAAnalyticsNewerAndUsage(t *testing.T) {
	syncCLIIsolate(t)
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := store.Open(path)
	s2Must(t, err)
	s2Must(t, db.Upsert("listings", "1000000001", json.RawMessage(`{"listing_id":1000000001}`)))
	s2Must(t, db.Close())
	out, _, err := s2Run(t, "analytics", "--db", path, "--type", "listings", "--group-by", "listing_id")
	s2Must(t, err)
	if !strings.Contains(out, `"1000000001"`) {
		t.Fatal(out)
	}
	_, _, err = s2Run(t, "analytics", "--db", path, "--type", "listings", "--group-by", "absent")
	if ExitCode(err) != 2 {
		t.Fatal(err)
	}
	_, _, err = s2Run(t, "analytics", "--db", path, "--type", "listings-featured")
	s2Must(t, err)
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	t.Setenv("ETSY_MCP_ALLOW_BUYER_PII", "1")
	_, _, err = s2Run(t, "analytics", "--db", path, "--type", "unknown_type")
	if ExitCode(err) != 2 {
		t.Fatal(err)
	}
	db, err = store.Open(path)
	s2Must(t, err)
	_, err = db.DB().Exec(fmt.Sprintf("PRAGMA user_version=%d", store.StoreSchemaVersion+1))
	s2Must(t, err)
	s2Must(t, db.Close())
	_, _, err = s2Run(t, "analytics", "--db", path)
	if err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatal(err)
	}
}

func TestBatchAAnalyticsEmptyShapes(t *testing.T) {
	syncCLIIsolate(t)
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprint(exists), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "empty.db")
			if exists {
				db, err := store.Open(path)
				s2Must(t, err)
				s2Must(t, db.Close())
			}
			for _, tc := range []struct {
				args []string
				want string
			}{
				{nil, `{}`},
				{[]string{"--type", "listings-featured"}, `{"count":0,"resource_type":"listings-featured"}`},
				{[]string{"--type", "listings", "--group-by", "title"}, `[]`},
			} {
				out, _, err := s2Run(t, append([]string{"analytics", "--db", path}, tc.args...)...)
				s2Must(t, err)
				if strings.Join(strings.Fields(out), "") != tc.want {
					t.Fatalf("%s want %s", out, tc.want)
				}
			}
			if !exists {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestBatchAAnalyticsHumanSummarySorted(t *testing.T) {
	syncCLIIsolate(t)
	path := filepath.Join(t.TempDir(), "summary.db")
	db, err := store.Open(path)
	s2Must(t, err)
	for _, rt := range []string{"zeta-thing", "alpha-thing", "mid-thing", "beta-thing", "omega-thing"} {
		s2Must(t, db.Upsert(rt, "1", json.RawMessage(`{"title":"synthetic"}`)))
	}
	s2Must(t, db.Close())
	old := humanFriendly
	t.Cleanup(func() { humanFriendly = old })
	// Exercise fresh map iterations so an accidentally sorted iteration cannot mask a regression.
	for attempt := 0; attempt < 20; attempt++ {
		var flags rootFlags
		root := newRootCmd(&flags)
		humanFriendly = true // Set the seam after flag registration installs defaults.
		var out, stderr bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&stderr)
		root.SetArgs([]string{"analytics", "--db", path})
		s2Must(t, root.Execute())
		want := "Resource Type\tCount\n-------------\t-----\nalpha-thing\t1\nbeta-thing\t1\nmid-thing\t1\nomega-thing\t1\nzeta-thing\t1\n"
		if out.String() != want {
			t.Fatalf("attempt %d: human summary = %q, want %q", attempt, out.String(), want)
		}
	}
}

func TestBatchAStoredPartitionsAndHumanOutput(t *testing.T) {
	syncCLIIsolate(t)
	path := filepath.Join(t.TempDir(), "partitions.db")
	db, err := store.Open(path)
	s2Must(t, err)
	for i, title := range []string{"zeta", "alpha"} {
		s2Must(t, db.Upsert("listings-active", fmt.Sprint(i), json.RawMessage(fmt.Sprintf(`{"title":%q}`, title))))
	}
	s2Must(t, db.Upsert("synthetic-unregistered", "1", json.RawMessage(`{"title":"custom"}`)))
	s2Must(t, db.Close())
	for _, rt := range []string{"listings-active", "listings_active", "LISTINGS_ACTIVE", "synthetic-unregistered"} {
		out, _, err := s2Run(t, "analytics", "--db", path, "--type", rt, "--json")
		s2Must(t, err)
		var result struct {
			Count int    `json:"count"`
			Type  string `json:"resource_type"`
		}
		s2Must(t, json.Unmarshal([]byte(out), &result))
		want := 1
		if rt != "synthetic-unregistered" {
			want = 2
			rt = "listings-active"
		}
		if result.Count != want || result.Type != rt {
			t.Fatal(out)
		}
	}
	old := humanFriendly
	t.Cleanup(func() { humanFriendly = old })
	runHuman := func(args ...string) (string, string, error) {
		var flags rootFlags
		root := newRootCmd(&flags)
		humanFriendly = true // Set the existing seam after flag registration installs defaults.
		var out, stderr bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&stderr)
		root.SetArgs(args)
		err := root.Execute()
		return out.String(), stderr.String(), err
	}
	out, _, err := runHuman("analytics", "--db", path, "--type", "listings-active", "--group-by", "title")
	s2Must(t, err)
	if strings.TrimSpace(out) != "title\tCount\n---\t-----\nalpha\t1\nzeta\t1" {
		t.Fatal(out)
	}
	out, _, err = runHuman("analytics", "--db", path)
	s2Must(t, err)
	if strings.Index(out, "listings-active") > strings.Index(out, "synthetic-unregistered") || json.Valid([]byte(out)) {
		t.Fatal(out)
	}
	t.Setenv(cliutil.MCPChildEnvVar, "1")
	out, _, err = runHuman("analytics", "--db", path, "--type", "listings-active", "--group-by", "title")
	s2Must(t, err)
	if strings.Join(strings.Fields(out), "") != `[{"value":"alpha","count":1},{"value":"zeta","count":1}]` {
		t.Fatal(out)
	}
	_, _, err = s2Run(t, "analytics", "--db", path, "--type", "typo")
	if ExitCode(err) != 2 || !strings.Contains(err.Error(), `"typo" is not a known non-buyer type; buyer-bearing types need ETSY_MCP_ALLOW_BUYER_PII=1`) {
		t.Fatal(err)
	}
}
