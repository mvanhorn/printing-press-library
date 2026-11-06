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
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/store"
)

// TestNovelInboxHelpWires smoke-tests that the inbox command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelInboxHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"inbox", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("inbox --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "inbox"} {
		if !strings.Contains(help, want) {
			t.Fatalf("inbox --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestBuildInboxFiltering(t *testing.T) {
	cutoff := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	recent := "2026-11-05T12:00:00.123456Z"
	old := "2026-11-04T12:00:00Z"
	tests := []struct {
		name           string
		record         inboxRecord
		space, section string
		want           int
	}{
		{"old member", inboxRecord{ResourceType: "community-members", CreatedAt: old}, "", "new_members", 0},
		{"fractional member", inboxRecord{ResourceType: "community-members", CreatedAt: recent}, "", "new_members", 1},
		{"boundary member", inboxRecord{ResourceType: "community-members", CreatedAt: cutoff.Format(time.RFC3339)}, "", "new_members", 1},
		{"invalid member time", inboxRecord{ResourceType: "community-members", CreatedAt: "bad"}, "", "new_members", 0},
		{"post with comments", inboxRecord{ResourceType: "posts", CreatedAt: recent, CommentsCount: 1}, "", "unreplied_posts", 0},
		{"space id", inboxRecord{ResourceType: "posts", CreatedAt: recent, SpaceID: "1000969"}, "1000969", "unreplied_posts", 1},
		{"space name", inboxRecord{ResourceType: "posts", CreatedAt: recent, SpaceName: "Introductions"}, "INTRODUCTIONS", "unreplied_posts", 1},
		{"wrong space", inboxRecord{ResourceType: "posts", CreatedAt: recent, SpaceID: "1000969"}, "1000970", "unreplied_posts", 0},
		{"old post", inboxRecord{ResourceType: "posts", CreatedAt: old}, "", "unreplied_posts", 0},
		{"refunded charge", inboxRecord{ResourceType: "community-member-charges", CreatedAt: recent, Status: "refunded"}, "", "failed_charges", 0},
		{"succeeded charge", inboxRecord{ResourceType: "community-member-charges", CreatedAt: recent, Status: "succeeded"}, "", "failed_charges", 0},
		{"failed charge", inboxRecord{ResourceType: "community-member-charges", CreatedAt: recent, Status: "FAILED"}, "", "failed_charges", 1},
		{"declined charge", inboxRecord{ResourceType: "community-member-charges", CreatedAt: recent, Status: "Declined"}, "", "failed_charges", 1},
		{"requires payment method", inboxRecord{ResourceType: "community-member-charges", CreatedAt: recent, Status: "requires_payment_method"}, "", "failed_charges", 1},
		{"old failed charge", inboxRecord{ResourceType: "community-member-charges", CreatedAt: old, Status: "failed"}, "", "failed_charges", 0},
		{"old unresolved flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: old, Status: "pending"}, "", "flagged_content", 0},
		{"recent resolved flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: recent, Status: "resolved"}, "", "flagged_content", 0},
		{"recent approved flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: recent, Status: "APPROVED"}, "", "flagged_content", 0},
		{"recent dismissed flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: recent, Status: "dismissed"}, "", "flagged_content", 0},
		{"recent rejected flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: recent, Status: " rejected "}, "", "flagged_content", 0},
		{"recent pending flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: recent, Status: "pending"}, "", "flagged_content", 1},
		{"boundary pending flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: cutoff.Format(time.RFC3339), Status: "pending"}, "", "flagged_content", 1},
		{"undated pending flag", inboxRecord{ResourceType: "flagged-contents", CreatedAt: recent, Status: "pending"}, "", "flagged_content", 0},
		{"old resolved flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: old, Status: "resolved"}, "", "flagged_content", 0},
		{"old approved flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: old, Status: "approved"}, "", "flagged_content", 0},
		{"old dismissed flag", inboxRecord{ResourceType: "flagged-contents", ReportedAt: old, Status: "dismissed"}, "", "flagged_content", 0},
		{"form submission", inboxRecord{ResourceType: "forms-submissions", CreatedAt: recent}, "", "form_submissions", 1},
		{"other submission", inboxRecord{ResourceType: "survey_submissions", CreatedAt: recent}, "", "form_submissions", 1},
		{"old submission", inboxRecord{ResourceType: "forms-submissions", CreatedAt: old}, "", "form_submissions", 0},
		{"canceled without end", inboxRecord{ResourceType: "community-member-subscriptions", Status: "canceled", CreatedAt: old}, "", "cancellations", 0},
		{"recent canceled without end", inboxRecord{ResourceType: "community-member-subscriptions", Status: "canceled", CreatedAt: recent}, "", "cancellations", 1},
		{"cancelled recent end", inboxRecord{ResourceType: "community-member-subscriptions", Status: "cancelled", EndsAt: recent}, "", "cancellations", 1},
		{"scheduled cancel", inboxRecord{ResourceType: "community-member-subscriptions", Status: "active", CreatedAt: recent, ScheduledToCancel: true}, "", "cancellations", 1},
		{"undated scheduled cancel", inboxRecord{ResourceType: "community-member-subscriptions", Status: "active", ScheduledToCancel: true}, "", "cancellations", 0},
		{"old cancellation", inboxRecord{ResourceType: "community-member-subscriptions", Status: "canceled", EndsAt: old, CreatedAt: recent}, "", "cancellations", 0},
		{"invalid cancellation time", inboxRecord{ResourceType: "community-member-subscriptions", Status: "canceled", EndsAt: "bad"}, "", "cancellations", 0},
		{"active subscription", inboxRecord{ResourceType: "community-member-subscriptions", Status: "active", EndsAt: recent}, "", "cancellations", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInbox([]inboxRecord{tt.record}, cutoff, tt.space, 50)
			if got.Counts[tt.section] != tt.want {
				t.Fatalf("%s = %d, want %d", tt.section, got.Counts[tt.section], tt.want)
			}
		})
	}
}

func TestBuildInboxEmpty(t *testing.T) {
	for _, records := range [][]inboxRecord{nil, {}} {
		report := buildInbox(records, time.Now(), "", 50)
		if report.NewMembers == nil || report.UnrepliedPosts == nil || report.FlaggedContent == nil || report.FormSubmissions == nil || report.FailedCharges == nil || report.Cancellations == nil {
			t.Fatal("empty report contains a nil section")
		}
		for key, count := range report.Counts {
			if count != 0 {
				t.Fatalf("%s = %d, want zero", key, count)
			}
		}
		var out bytes.Buffer
		if err := printInbox(&out, report); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "Nothing needs you since "+report.Since+".") {
			t.Fatal(out.String())
		}
	}
}

func TestBuildInboxOrderingLimitsAndTotals(t *testing.T) {
	cutoff := inboxTime("2026-11-05T00:00:00Z")
	records := make([]inboxRecord, 0)
	for _, resource := range []string{"community-members", "posts", "flagged-contents", "forms-submissions", "community-member-charges", "community-member-subscriptions", "comments", "spaces"} {
		for _, timestamp := range []string{"2026-11-05T01:00:00Z", "2026-11-05T04:00:00+02:00", "2026-11-05T01:30:00Z"} {
			records = append(records, inboxRecord{ID: timestamp, ResourceType: resource, CreatedAt: timestamp, ReportedAt: timestamp, EndsAt: timestamp, Status: "failed", ScheduledToCancel: true})
		}
	}
	before := append([]inboxRecord(nil), records...)
	report := buildInbox(records, cutoff, "", 1)
	if !reflect.DeepEqual(records, before) {
		t.Fatal("buildInbox mutated input")
	}
	for _, section := range []string{"new_members", "unreplied_posts", "flagged_content", "form_submissions", "failed_charges", "cancellations"} {
		if report.Counts[section] != 3 {
			t.Fatalf("%s count = %d", section, report.Counts[section])
		}
	}
	for _, shown := range []int{len(report.NewMembers), len(report.UnrepliedPosts), len(report.FlaggedContent), len(report.FormSubmissions), len(report.FailedCharges), len(report.Cancellations)} {
		if shown != 1 {
			t.Fatalf("shown items = %d, want limit 1", shown)
		}
	}
	for _, total := range []string{"members", "posts", "comments", "spaces"} {
		if report.Counts[total] != 3 {
			t.Fatalf("%s total = %d", total, report.Counts[total])
		}
	}
	want := "2026-11-05T04:00:00+02:00"
	for _, id := range []string{report.NewMembers[0].ID, report.UnrepliedPosts[0].ID, report.FlaggedContent[0].ID, report.FormSubmissions[0].ID, report.FailedCharges[0].ID, report.Cancellations[0].ID} {
		if id != want {
			t.Fatalf("first id = %s, want newest %s", id, want)
		}
	}
	var out bytes.Buffer
	if err := printInbox(&out, report); err != nil {
		t.Fatal(err)
	}
	for _, heading := range []string{"New members", "Unreplied posts", "Flagged content", "Form submissions", "Failed charges", "Cancellations"} {
		if !strings.Contains(out.String(), heading) {
			t.Fatalf("missing heading %q", heading)
		}
	}
}

func TestInboxLocalCommand(t *testing.T) {
	testenv.Isolate(t)
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Now().UTC().Format(time.RFC3339Nano)
	fixtures := []struct{ resource, id, data string }{
		{"community-members", "101", `{"name":"Test Member","email":"member@example.test","created_at":"` + created + `"}`},
		{"posts", "102", `{"name":"A question","space_id":1000969,"space_name":"Introductions","comments_count":0,"created_at":"` + created + `"}`},
		{"flagged-contents", "103", `{"content_id":102,"status":"pending","reported_at":"` + created + `"}`},
		{"forms-submissions", "104", `{"created_at":"` + created + `"}`},
		{"community-member-charges", "105", `{"amount":1000000,"currency":"usd","status":"FAILED","created_at":"` + created + `"}`},
		{"community-member-subscriptions", "106", `{"status":"active","scheduled_to_cancel":true,"ends_at":null,"created_at":"` + created + `"}`},
		{"comments", "107", `{}`},
		{"spaces", "1000969", `{}`},
	}
	for _, f := range fixtures {
		if _, err := db.DB().ExecContext(context.Background(), `INSERT INTO resources (resource_type, id, data) VALUES (?, ?, ?)`, f.resource, f.id, f.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	flags := &rootFlags{asJSON: true}
	cmd := newNovelInboxCmd(flags)
	cmd.SetArgs([]string{"--db", dbPath, "--space", "1000969", "--since", "1w"})
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report inboxReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("%v: %s", err, out.String())
	}
	for section, count := range report.Counts {
		if count != 1 {
			t.Fatalf("%s = %d, want 1", section, count)
		}
	}
	if report.NewMembers[0].Email != "member@example.test" || report.FailedCharges[0].Amount != 1000000 || report.FailedCharges[0].AmountDisplay != "10000.00 USD" || report.FlaggedContent[0].ContentID != "102" {
		t.Fatalf("SQL field decoding mismatch: %+v", report)
	}
	if !strings.Contains(stderr.String(), "not been synced") {
		t.Fatal("missing sync hint")
	}
	if report.Mirror.Path != dbPath || !report.Mirror.Exists || report.Mirror.LastSyncedAt != "" || len(report.Warnings) != 1 {
		t.Fatalf("bad unsynced mirror metadata: %+v", report)
	}
}

func TestInboxValidationAndMissingDB(t *testing.T) {
	testenv.Isolate(t)
	for _, tt := range []struct {
		name    string
		args    []string
		dryRun  bool
		wantErr string
	}{
		{"missing db", nil, false, ""},
		{"invalid duration", []string{"--since", "yesterday"}, false, "--since"},
		{"negative duration", []string{"--since=-1h"}, false, "--since"},
		{"zero limit", []string{"--limit", "0"}, false, "--limit"},
		{"positional", []string{"extra"}, false, "positional"},
		{"dry run before validation", []string{"--since", "bad", "--limit", "0"}, true, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := filepath.Join(t.TempDir(), "absent.db")
			flags := &rootFlags{asJSON: true, dryRun: tt.dryRun}
			cmd := newNovelInboxCmd(flags)
			cmd.SetArgs(append([]string{"--db", dbPath}, tt.args...))
			var out, stderr bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&stderr)
			err := cmd.Execute()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
				t.Fatalf("missing database was created: %v", err)
			}
			if tt.dryRun {
				var result struct {
					DryRun  bool `json:"dry_run"`
					Success bool `json:"success"`
				}
				if err := json.Unmarshal(out.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if !result.DryRun || !result.Success || stderr.Len() != 0 {
					t.Fatalf("out = %s; stderr = %s", out.String(), stderr.String())
				}
			} else {
				if !strings.Contains(stderr.String(), "no local mirror at "+dbPath+"\nrun: circle-pp-cli sync") {
					t.Fatal(stderr.String())
				}
				var report inboxReport
				if err := json.Unmarshal(out.Bytes(), &report); err != nil {
					t.Fatal(err)
				}
				if report.NewMembers == nil || len(report.NewMembers) != 0 {
					t.Fatalf("bad empty result: %s", out.String())
				}
				if report.Mirror.Path != dbPath || report.Mirror.Exists || report.Mirror.LastSyncedAt != "" || !reflect.DeepEqual(report.Warnings, []string{"mirror not synced; run circle-pp-cli sync"}) {
					t.Fatalf("bad missing mirror metadata: %s", out.String())
				}
			}
		})
	}
}

func TestInboxAmountDisplay(t *testing.T) {
	for _, tc := range []struct {
		cents          int64
		currency, want string
	}{
		{1000000, "usd", "10000.00 USD"}, {1995, "USD", "19.95 USD"},
		{1, "eur", "0.01 EUR"}, {0, "USD", "0.00 USD"},
		{-1, "USD", "-0.01 USD"}, {-1995, "USD", "-19.95 USD"},
		{9223372036854775807, "USD", "92233720368547758.07 USD"},
		{-9223372036854775808, "USD", "-92233720368547758.08 USD"},
	} {
		if got := inboxAmountDisplay(tc.cents, tc.currency); got != tc.want {
			t.Errorf("%d %s: got %s, want %s", tc.cents, tc.currency, got, tc.want)
		}
	}
	cutoff := time.Now().UTC()
	report := buildInbox([]inboxRecord{{ResourceType: "community-member-charges", Amount: 1000000, Currency: "USD", Status: "failed", CreatedAt: cutoff.Format(time.RFC3339Nano)}}, cutoff, "", 50)
	var out bytes.Buffer
	if err := printInbox(&out, report); err != nil || !strings.Contains(out.String(), "10000.00 USD") {
		t.Fatalf("human amount: %s, %v", &out, err)
	}
	raw, err := json.Marshal(report)
	if err != nil || !bytes.Contains(raw, []byte(`"amount":1000000,`)) || !bytes.Contains(raw, []byte(`"amount_display":"10000.00 USD"`)) {
		t.Fatalf("JSON amount: %s, %v", raw, err)
	}
}

func TestInboxSyncedMirror(t *testing.T) {
	testenv.Isolate(t)
	dbPath := filepath.Join(t.TempDir(), "mirror.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	latest := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	for resource, at := range map[string]time.Time{"posts": latest, "spaces": latest.Add(-time.Hour)} {
		if err := db.SaveSyncStateAt(resource, "", 0, at); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncCursor("comments", "page-2"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	cmd := newNovelInboxCmd(&rootFlags{asJSON: true})
	cmd.SetArgs([]string{"--db", dbPath})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report inboxReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Mirror.Path != dbPath || !report.Mirror.Exists || report.Mirror.LastSyncedAt != latest.Format(time.RFC3339) || len(report.Warnings) != 0 {
		t.Fatalf("bad synced mirror metadata: %s", &out)
	}
}

func TestInboxLiveSourceRejected(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"inbox", "--data-source", "live", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "inbox reads the local mirror only; run sync first") {
		t.Fatalf("error = %v", err)
	}
}
