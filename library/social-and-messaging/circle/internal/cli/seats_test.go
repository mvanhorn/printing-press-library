// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/store"

	"github.com/spf13/cobra"
)

// TestNovelSeatsHelpWires smoke-tests that the seats command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSeatsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"seats", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("seats --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "seats"} {
		if !strings.Contains(help, want) {
			t.Fatalf("seats --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestDistinctCirclePayers(t *testing.T) {
	sub := circlePayment{MemberID: "42", Email: "payer@example.com", Name: "Example Member", PaywallID: "1000969", PaywallName: "Founding", Status: "active", CreatedAt: "2026-11-02T00:00:00Z"}
	charge := sub
	charge.Status, charge.Amount, charge.CreatedAt = "paid", 1000, "2026-11-01T00:00:00Z"
	payer := circlePayer{"42", sub.Email, sub.Name, sub.PaywallName, sub.CreatedAt, "subscription"}
	chargePayer := payer
	chargePayer.FirstPaidAt, chargePayer.Source = charge.CreatedAt, "charge"
	type payerCase struct {
		name          string
		subs, charges []circlePayment
		paywall       string
		want          []circlePayer
	}
	tests := []payerCase{
		{"active subscription", []circlePayment{sub}, nil, "", []circlePayer{payer}},
		{"paid charge", nil, []circlePayment{charge}, "", []circlePayer{chargePayer}},
		{"deduplicated earliest payment", []circlePayment{sub}, []circlePayment{charge}, "", []circlePayer{chargePayer}},
		{"matching paywall", []circlePayment{sub}, []circlePayment{charge}, "1000969", []circlePayer{chargePayer}},
		{"excluded paywall", []circlePayment{sub}, []circlePayment{charge}, "1000970", []circlePayer{}},
		{"empty", nil, nil, "", []circlePayer{}},
	}
	for _, status := range []string{"active", "ACTIVE", "trialing", "Trial", "past_due", "canceled", "ended"} {
		record := sub
		record.Status = status
		want := []circlePayer{payer}
		if status == "canceled" || status == "ended" {
			want = []circlePayer{}
		}
		tests = append(tests, payerCase{"subscription " + status, []circlePayment{record}, nil, "", want})
	}
	for _, tc := range []struct {
		status   string
		refunded int64
		counts   bool
	}{
		{"paid", 1000, false}, {"paid", 1100, false}, {"paid", 500, true},
		{"partial_refunded", 500, true}, {"partial_refunded", 1000, false}, {"partial_refunded", 1100, false},
		{"SUCCEEDED", 0, true}, {"complete", 0, true}, {"failed", 0, false},
	} {
		record := charge
		record.Status, record.AmountRefunded = tc.status, tc.refunded
		want := []circlePayer{}
		if tc.counts {
			want = []circlePayer{chargePayer}
		}
		tests = append(tests, payerCase{"charge " + tc.status + "/" + strconv.FormatInt(tc.refunded, 10), nil, []circlePayment{record}, "", want})
	}
	emailSub, emailCharge := sub, charge
	emailSub.MemberID, emailCharge.MemberID = "", ""
	emailCharge.Email = "PAYER@example.com"
	emailPayer := chargePayer
	emailPayer.MemberID = ""
	tests = append(tests, payerCase{"email fallback", []circlePayment{emailSub}, []circlePayment{emailCharge}, "", []circlePayer{emailPayer}})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := distinctCirclePayers(tc.subs, tc.charges, tc.paywall)
			if got == nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}

func circlePayerTestDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "payers.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for resource, data := range map[string]string{
		"community-member-subscriptions": `{"community_member_id":42,"community_member_email":"payer@example.com","status":"active","paywall_id":1000969}`,
		"community-member-charges":       `{"community_member_id":42,"status":"paid","amount":1000,"amount_refunded":0,"paywall_id":1000969}`,
	} {
		if err := db.Upsert(resource, "1", json.RawMessage(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return dbPath
}

func TestSeatsLocalReport(t *testing.T) {
	testenv.Isolate(t)
	dbPath := circlePayerTestDB(t)
	for _, source := range []string{"auto", "local"} {
		t.Run(source, func(t *testing.T) {
			flags := &rootFlags{asJSON: true, dataSource: source}
			cmd := newNovelSeatsCmd(flags)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--db", dbPath, "--cap", "0"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Paid, Cap, Remaining int
				Source               string
				Payers               []circlePayer
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Paid != 1 || result.Cap != 0 || result.Remaining != 0 || result.Source != "local" || len(result.Payers) != 1 {
				t.Fatalf("unexpected report: %s", out.String())
			}
		})
	}
}

func TestSeatsMissingStoreAndDryRun(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		t.Run(strconv.FormatBool(dryRun), func(t *testing.T) {
			flags := &rootFlags{asJSON: true, dataSource: "local", dryRun: dryRun}
			cmd := newNovelSeatsCmd(flags)
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			dbPath := filepath.Join(t.TempDir(), "missing.db")
			cmd.SetArgs([]string{"--db", dbPath})
			err := cmd.Execute()
			if dryRun && err != nil {
				t.Fatal(err)
			}
			if !dryRun && (err == nil || !strings.Contains(err.Error(), "no synced payment data; run sync --resources community-member-subscriptions,community-member-charges")) {
				t.Fatalf("error = %v", err)
			}
			if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
				t.Fatalf("store created: %v", err)
			}
			if dryRun {
				if stderr.Len() != 0 || !strings.Contains(out.String(), `"dry_run":true`) && !strings.Contains(out.String(), `"dry_run": true`) {
					t.Fatalf("dry run: stdout=%s stderr=%s", &out, &stderr)
				}
			} else {
				if strings.Contains(out.String(), `"payers"`) || !strings.Contains(stderr.String(), "hint:") {
					t.Fatalf("missing store: stdout=%s stderr=%s", &out, &stderr)
				}
			}
		})
	}
}

func TestPayerCommandsResolveRuntimeStore(t *testing.T) {
	for _, tc := range []struct {
		name   string
		newCmd func(*rootFlags) *cobra.Command
		args   []string
	}{
		{"seats", newNovelSeatsCmd, nil},
		{"founding", newNovelFoundingCmd, []string{"--go-no-go", "1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			hash, legacy, label := currentDefaultDBScope()
			t.Cleanup(func() {
				defaultDBScopeState.Lock()
				defaultDBScopeState.hash, defaultDBScopeState.legacyHash, defaultDBScopeState.label = hash, legacy, label
				defaultDBScopeState.Unlock()
			})
			setDefaultDBScopeIdentity("", "", "")
			cmd := tc.newCmd(&rootFlags{asJSON: true, dataSource: "local"})
			if got := cmd.Flags().Lookup("db").DefValue; got != "" {
				t.Fatalf("--db default = %q, want empty", got)
			}
			setDefaultDBScopeIdentity("payer-test-scope", "", "https://app.circle.so")
			dbPath := circlePayerTestDB(t)
			t.Setenv("CIRCLE_DATA_DIR", filepath.Dir(dbPath))
			scopedPath := filepath.Join(filepath.Dir(dbPath), "data-"+hashDefaultDBScope("payer-test-scope")+".db")
			if err := os.Rename(dbPath, scopedPath); err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result struct {
				Paid   int
				Source string
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Paid != 1 || result.Source != "local" {
				t.Fatalf("unexpected report: %s", &out)
			}
		})
	}
}

func TestCirclePayersUnsyncedHintOnce(t *testing.T) {
	for _, synced := range [][]string{
		nil,
		{"community-member-subscriptions"},
		{"community-member-charges"},
		{"community-member-subscriptions", "community-member-charges"},
	} {
		t.Run(strings.Join(synced, "+"), func(t *testing.T) {
			testenv.Isolate(t)
			dbPath := circlePayerTestDB(t)
			db, err := store.OpenWithContext(context.Background(), dbPath)
			if err != nil {
				t.Fatal(err)
			}
			for _, resource := range synced {
				if err := db.SaveSyncState(resource, "", 1); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			cmd := newNovelSeatsCmd(&rootFlags{asJSON: true, dataSource: "local"})
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--db", dbPath})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			want := 1
			if len(synced) == 2 {
				want = 0
			}
			if got := strings.Count(stderr.String(), "has not been synced yet"); got != want {
				t.Fatalf("got %d unsynced hints, want %d: %s", got, want, &stderr)
			}
		})
	}
}
