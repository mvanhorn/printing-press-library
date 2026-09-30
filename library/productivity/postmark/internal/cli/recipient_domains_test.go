// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/store"
)

func TestNovelRecipientDomainsHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"recipient-domains", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("recipient-domains --help error = %v", err)
	}
	for _, want := range []string{"recipient-domains [flags]", "Do NOT use this command for one recipient's history"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("help missing %q:\n%s", want, out.String())
		}
	}
}

func TestRecipientDomainOf(t *testing.T) {
	cases := map[string]string{
		"Jane@Gmail.COM":              "gmail.com",
		"a@b@corp.example":            "corp.example",
		"Jane Doe <jane@outlook.com>": "outlook.com",
		"no-at-sign":                  recipientDomainUnknown,
		"trailing@":                   recipientDomainUnknown,
	}
	for in, want := range cases {
		if got := recipientDomainOf(in); got != want {
			t.Errorf("recipientDomainOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRecipientDomainsCompute(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	since := now.Add(-7 * 24 * time.Hour)
	recent := now.Add(-time.Hour)
	old := now.Add(-30 * 24 * time.Hour)
	var msgs []recipientDomainMessage
	for i := 0; i < 20; i++ {
		msgs = append(msgs, recipientDomainMessage{MessageID: "g" + string(rune('a'+i)), Recipients: []string{"u@gmail.com"}, ReceivedAt: recent, TrackOpens: true})
	}
	for i := 0; i < 20; i++ {
		msgs = append(msgs, recipientDomainMessage{MessageID: "c" + string(rune('a'+i)), Recipients: []string{"x@corp.example", "X@corp.example"}, ReceivedAt: recent, TrackOpens: true})
	}
	msgs = append(msgs, recipientDomainMessage{MessageID: "old", Recipients: []string{"u@gmail.com"}, ReceivedAt: old})
	bounces := []recipientDomainBounce{
		{Email: "x@corp.example", Type: "HardBounce", BouncedAt: recent},
		{Email: "x@corp.example", Type: "HardBounce", BouncedAt: recent},
		{Email: "y@corp.example", Type: "SoftBounce", BouncedAt: recent},
		{Email: "u@gmail.com", Type: "SpamComplaint", BouncedAt: recent},
		{Email: "u@gmail.com", Type: "HardBounce", BouncedAt: old},
	}
	opens := []recipientDomainOpen{
		{Recipient: "u@gmail.com", MessageID: "ga", ReceivedAt: recent},
		{Recipient: "u@gmail.com", MessageID: "ga", ReceivedAt: recent},
		{Recipient: "u@gmail.com", MessageID: "gb", ReceivedAt: recent},
		{Recipient: "u@gmail.com", MessageID: "gc", ReceivedAt: recent},
		{Recipient: "u@gmail.com", MessageID: "gd", ReceivedAt: recent},
	}
	sups := []recipientDomainSuppression{{EmailAddress: "x@corp.example", Reason: "HardBounce"}, {EmailAddress: "X@corp.example"}, {EmailAddress: "z@nowhere.example"}}

	rows := recipientDomainsCompute(msgs, bounces, opens, sups, since, 10)
	if len(rows) != 2 {
		t.Fatalf("rows = %+v", rows)
	}
	byDomain := map[string]recipientDomainRow{}
	total := 0
	for _, r := range rows {
		byDomain[r.Domain] = r
		total += r.Sent
	}
	if total != 40 {
		t.Fatalf("sent total = %d, want 40 (duplicate recipient in one message counted once, old message excluded)", total)
	}
	corp, gmail := byDomain["corp.example"], byDomain["gmail.com"]
	if corp.Bounced != 3 || corp.HardBounces != 2 || corp.BouncesByType["SoftBounce"] != 1 || corp.Suppressed != 1 {
		t.Fatalf("corp row = %+v", corp)
	}
	if gmail.Complaints != 1 || gmail.Bounced != 0 || gmail.UniqueOpens != 4 {
		t.Fatalf("gmail row = %+v", gmail)
	}
	if !corp.Flagged || corp.Flags[0] != recipientDomainFlagHighBounce {
		t.Fatalf("corp should be flagged high-bounce-rate: %+v", corp)
	}
	// corp opened 0 of 20 while the account opened 4 of 40.
	if len(corp.Flags) != 2 || corp.Flags[1] != recipientDomainFlagLowOpen {
		t.Fatalf("corp should also be flagged low-open-rate: %+v", corp.Flags)
	}
	if gmail.Flagged {
		t.Fatalf("gmail should not be flagged: %+v", gmail)
	}
	if rows[0].Domain != "corp.example" {
		t.Fatalf("flagged domains sort first: %s", rows[0].Domain)
	}
	if corp.AccountBounceRatePct != 7.5 {
		t.Fatalf("account bounce rate = %v, want 7.5", corp.AccountBounceRatePct)
	}
}

func TestRecipientDomainsNoTrackingNoOpenFlag(t *testing.T) {
	now := time.Now()
	msgs := []recipientDomainMessage{{MessageID: "a", Recipients: []string{"a@x.example"}, ReceivedAt: now}}
	rows := recipientDomainsCompute(msgs, nil, nil, nil, now.Add(-time.Hour), 0)
	if len(rows) != 1 || rows[0].OpenRatePct != nil || rows[0].Flagged {
		t.Fatalf("untracked sends must not produce an open rate or flag: %+v", rows)
	}
}

func TestRecipientDomainsEmptyStore(t *testing.T) {
	home := testenv.Isolate(t)
	missing := filepath.Join(home, "missing.db")
	stdout, stderr, err := postmarkRun(t, "recipient-domains", "--db", missing, "--json")
	if err != nil {
		t.Fatalf("missing store should exit 0: %v", err)
	}
	var rows []recipientDomainRow
	postmarkResults(t, stdout, &rows)
	if rows == nil || len(rows) != 0 {
		t.Fatalf("want [] got %q", stdout)
	}
	if !strings.Contains(stderr, "sync") {
		t.Fatalf("missing sync hint: %s", stderr)
	}

	empty := filepath.Join(home, "empty.db")
	db, err := store.OpenWithContext(context.Background(), empty)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	stdout, _, err = postmarkRun(t, "recipient-domains", "--db", empty, "--json")
	if err != nil {
		t.Fatalf("empty store should exit 0: %v", err)
	}
	postmarkResults(t, stdout, &rows)
	if len(rows) != 0 {
		t.Fatalf("empty store rows = %+v", rows)
	}
}

func TestRecipientDomainsEmptyResultHints(t *testing.T) {
	testenv.Isolate(t)
	path := defaultDBPath("postmark-pp-cli")
	db, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	_, stderr, err := postmarkRun(t, "recipient-domains", "--window", "7d", "--json")
	if err != nil {
		t.Fatalf("unsynced store should exit 0: %v", err)
	}
	if !strings.Contains(stderr, "has not been synced yet") || strings.Contains(stderr, "no synced messages") {
		t.Fatalf("unsynced store should print exactly one sync hint:\n%s", stderr)
	}

	db, err = store.OpenWithContext(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	for _, rt := range []string{"messages", "bounces", "opens", "suppressions"} {
		if err := db.SaveSyncState(rt, "", 0); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()

	_, stderr, err = postmarkRun(t, "recipient-domains", "--window", "7d", "--json")
	if err != nil {
		t.Fatalf("synced empty store should exit 0: %v", err)
	}
	if strings.Contains(stderr, "has not been synced yet") || !strings.Contains(stderr, "no synced messages in the last 7d") {
		t.Fatalf("synced store with no mail in the window should print the window hint:\n%s", stderr)
	}
	if strings.Contains(stderr, "--db") || strings.Contains(stderr, path) {
		t.Fatalf("hint must not repeat the default archive path:\n%s", stderr)
	}

	_, stderr, err = postmarkRun(t, "recipient-domains", "--window", "7d", "--db", path, "--json")
	if err != nil {
		t.Fatalf("explicit --db should exit 0: %v", err)
	}
	if !strings.Contains(stderr, "--db "+path) {
		t.Fatalf("hint should carry an explicit --db through:\n%s", stderr)
	}
}

func TestRecipientDomainsRejectsLiveSource(t *testing.T) {
	testenv.Isolate(t)
	_, _, err := postmarkRun(t, "recipient-domains", "--data-source", "live", "--json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("--data-source live should be a usage error, got %v", err)
	}
}

func TestRecipientDomainsFlagsDominantDomain(t *testing.T) {
	// gmail.com carries most of the volume with 20% hard bounces; the rest of
	// the account is clean. An average that includes gmail.com would hide it.
	now := time.Now()
	var msgs []recipientDomainMessage
	var bounces []recipientDomainBounce
	for i := 0; i < 100; i++ {
		addr := fmt.Sprintf("user%d@gmail.com", i)
		msgs = append(msgs, recipientDomainMessage{MessageID: fmt.Sprintf("g%d", i), Recipients: []string{addr}, ReceivedAt: now})
		if i < 20 {
			bounces = append(bounces, recipientDomainBounce{Email: addr, Type: postmarkHardBounce, BouncedAt: now})
		}
	}
	for i := 0; i < 20; i++ {
		msgs = append(msgs, recipientDomainMessage{MessageID: fmt.Sprintf("o%d", i), Recipients: []string{fmt.Sprintf("user%d@outlook.com", i)}, ReceivedAt: now})
	}
	got := recipientDomainsCompute(msgs, bounces, nil, nil, now.Add(-time.Hour), 5)
	flags := map[string][]string{}
	for _, r := range got {
		flags[r.Domain] = r.Flags
	}
	if !containsString(flags["gmail.com"], recipientDomainFlagHighBounce) {
		t.Errorf("gmail.com flags = %v, want %s", flags["gmail.com"], recipientDomainFlagHighBounce)
	}
	if containsString(flags["outlook.com"], recipientDomainFlagHighBounce) {
		t.Errorf("outlook.com flagged high-bounce with zero bounces: %v", flags["outlook.com"])
	}
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
