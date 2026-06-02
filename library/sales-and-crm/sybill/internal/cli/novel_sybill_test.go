// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavioral acceptance tests for the hand-authored novel features. These build
// a synthetic local store and assert the cross-entity logic produces correct
// output — the assertions check content, not just exit codes.

package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/internal/store"
	"github.com/spf13/cobra"
)

// seedStore writes a small but realistic dataset: three open deals (one with a
// recent call, one with a stale call, one with no call) plus a closed deal, the
// linked conversations, and an account.
func seedStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sybill.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	recent := now.Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	stale := now.Add(-40 * 24 * time.Hour).Format(time.RFC3339)

	deals := []map[string]any{
		{"dealId": "d-active", "name": "Acme Expansion", "accountName": "Acme Corp", "stage": "Demo", "amount": 50000.0, "closed": false, "lastActivityDate": recent, "owner": map[string]any{"name": "Jane Rep", "email": "jane@us.co"}, "crmAutofill": map[string]any{"stage": "Negotiation", "amount": 75000}},
		{"dealId": "d-stale", "name": "Beta Renewal", "accountName": "Beta LLC", "stage": "Proposal", "amount": 20000.0, "closed": false, "lastActivityDate": stale, "owner": map[string]any{"name": "Jane Rep", "email": "jane@us.co"}},
		{"dealId": "d-nocall", "name": "Gamma New", "accountName": "Gamma Inc", "stage": "Discovery", "amount": 10000.0, "closed": false, "owner": map[string]any{"name": "Sam Founder", "email": "sam@us.co"}},
		{"dealId": "d-won", "name": "Delta Closed", "accountName": "Delta Co", "stage": "Closed Won", "amount": 99000.0, "closed": true, "owner": map[string]any{"name": "Jane Rep"}},
	}
	for _, d := range deals {
		mustUpsert(t, db, "deals", d)
	}

	convs := []map[string]any{
		{"conversationId": "c1", "title": "Acme demo call about pricing", "startTime": recent, "type": "EXTERNAL", "participants": []any{map[string]any{"email": "a@acme.com"}}, "crm": map[string]any{"id": "d-active", "name": "Acme Expansion", "type": "opportunity"}, "summary": map[string]any{"nextSteps": []any{"Send revised quote"}}},
		{"conversationId": "c2", "title": "Beta proposal review", "startTime": stale, "type": "EXTERNAL", "participants": []any{}, "crm": map[string]any{"name": "Beta Renewal", "type": "opportunity"}},
		{"conversationId": "c3", "title": "Internal sync", "startTime": recent, "type": "INTERNAL", "participants": []any{}, "crm": nil},
	}
	for _, c := range convs {
		mustUpsert(t, db, "conversations", c)
	}

	mustUpsert(t, db, "accounts", map[string]any{"accountId": "a-acme", "name": "Acme Corp", "website": "acme.com", "owner": map[string]any{"name": "Jane Rep"}, "lastActivityDate": recent, "contacts": []any{map[string]any{"name": "Ann Buyer", "email": "ann@acme.com", "title": "VP Eng"}}})

	return dbPath
}

func mustUpsert(t *testing.T, db *store.Store, rt string, obj map[string]any) {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal %s: %v", rt, err)
	}
	id, _ := obj[idKeyFor(rt)].(string)
	if err := db.Upsert(rt, id, raw); err != nil {
		t.Fatalf("upsert %s: %v", rt, err)
	}
}

func idKeyFor(rt string) string {
	switch rt {
	case "deals":
		return "dealId"
	case "conversations":
		return "conversationId"
	case "accounts":
		return "accountId"
	}
	return "id"
}

// runNovel executes a novel command with --json against the seeded store and
// returns stdout. words are the command path (e.g. "deals","dark").
func runNovel(t *testing.T, dbPath string, words []string, extra ...string) string {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	root := &cobra.Command{Use: "sybill-pp-cli"}
	// Build the same tree the real root uses for the commands under test.
	root.AddCommand(newDealsCmd(flags))
	root.AddCommand(newNovelDigestCmd(flags))
	root.AddCommand(newNovelCrmAutofillCmd(flags))
	root.AddCommand(newNovelAccountCmd(flags))
	root.AddCommand(newNovelActivityCmd(flags))
	root.AddCommand(newNovelPatternsCmd(flags))

	args := append([]string{}, words...)
	args = append(args, "--db", dbPath)
	args = append(args, extra...)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("command %v failed: %v\nstderr: %s", words, err, stderr.String())
	}
	return stdout.String()
}

func TestDealsDark(t *testing.T) {
	dbPath := seedStore(t)

	// Without --include-uncovered: only the stale deal is dark.
	out := runNovel(t, dbPath, []string{"deals", "dark"}, "--days", "14")
	var dark []darkDeal
	if err := json.Unmarshal([]byte(out), &dark); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(dark) != 1 || dark[0].DealID != "d-stale" {
		t.Fatalf("expected only d-stale dark, got %+v", dark)
	}
	if dark[0].DaysSince < 30 {
		t.Fatalf("expected stale deal ~40 days dark, got %d", dark[0].DaysSince)
	}

	// With --include-uncovered: the no-call deal also appears.
	out = runNovel(t, dbPath, []string{"deals", "dark"}, "--days", "14", "--include-uncovered")
	if err := json.Unmarshal([]byte(out), &dark); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ids := map[string]bool{}
	for _, d := range dark {
		ids[d.DealID] = true
	}
	if !ids["d-stale"] || !ids["d-nocall"] {
		t.Fatalf("expected d-stale and d-nocall, got %+v", ids)
	}
	if ids["d-active"] {
		t.Fatalf("active deal (recent call) must not be dark")
	}
	if ids["d-won"] {
		t.Fatalf("closed deal must never be dark")
	}
}

func TestDigestGroupsByDeal(t *testing.T) {
	dbPath := seedStore(t)
	out := runNovel(t, dbPath, []string{"digest"}, "--since", "60d")
	var groups []digestGroup
	if err := json.Unmarshal([]byte(out), &groups); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	// Expect groups for Acme Expansion, Beta Renewal, and the unlinked internal call.
	byName := map[string]digestGroup{}
	for _, g := range groups {
		byName[g.DealName] = g
	}
	acme, ok := byName["Acme Expansion"]
	if !ok || acme.CallCount != 1 {
		t.Fatalf("expected Acme Expansion with 1 call, got %+v", byName)
	}
	if len(acme.NextSteps) != 1 || acme.NextSteps[0] != "Send revised quote" {
		t.Fatalf("expected Acme next steps extracted, got %+v", acme.NextSteps)
	}
	if _, ok := byName["(no linked deal)"]; !ok {
		t.Fatalf("expected an unlinked-call group, got %+v", byName)
	}

	// External-only filter drops the internal call group.
	out = runNovel(t, dbPath, []string{"digest"}, "--since", "60d", "--type", "external")
	groups = nil
	if err := json.Unmarshal([]byte(out), &groups); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, g := range groups {
		if g.DealName == "(no linked deal)" {
			t.Fatalf("internal call should be filtered out by --type external")
		}
	}
}

func TestCrmAutofillFromStore(t *testing.T) {
	dbPath := seedStore(t)
	out := runNovel(t, dbPath, []string{"crm-autofill"})
	var rows []autofillRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(rows) == 0 {
		t.Fatalf("expected crmAutofill rows for d-active")
	}
	var sawStage bool
	for _, r := range rows {
		if r.DealID != "d-active" {
			t.Fatalf("only d-active has autofill, got %s", r.DealID)
		}
		if r.Field == "stage" {
			sawStage = true
			if r.Suggested != "Negotiation" || r.Current != "Demo" {
				t.Fatalf("stage diff wrong: suggested=%q current=%q", r.Suggested, r.Current)
			}
		}
	}
	if !sawStage {
		t.Fatalf("expected a stage suggestion row, got %+v", rows)
	}
}

func TestAccountRollup(t *testing.T) {
	dbPath := seedStore(t)
	out := runNovel(t, dbPath, []string{"account", "rollup", "Acme Corp"})
	var roll accountRollup
	if err := json.Unmarshal([]byte(out), &roll); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if roll.Name != "Acme Corp" {
		t.Fatalf("wrong account: %q", roll.Name)
	}
	if roll.OpenDealCount != 1 {
		t.Fatalf("expected 1 open deal for Acme, got %d", roll.OpenDealCount)
	}
	if roll.CallCount != 1 {
		t.Fatalf("expected 1 linked call for Acme, got %d", roll.CallCount)
	}
	if len(roll.Contacts) == 0 || roll.Contacts[0].Email != "ann@acme.com" {
		t.Fatalf("expected Acme contact, got %+v", roll.Contacts)
	}
}

func TestActivityByOwner(t *testing.T) {
	dbPath := seedStore(t)
	out := runNovel(t, dbPath, []string{"activity"}, "--by", "owner", "--since", "60d")
	var rows []ownerActivity
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	byOwner := map[string]ownerActivity{}
	for _, r := range rows {
		byOwner[r.Owner] = r
	}
	jane := byOwner["Jane Rep"]
	if jane.OpenDeals != 2 {
		t.Fatalf("Jane should own 2 open deals, got %d", jane.OpenDeals)
	}
	if jane.ClosedDeals != 1 {
		t.Fatalf("Jane should own 1 closed deal, got %d", jane.ClosedDeals)
	}
	// Beta (stale, 40d) is dark; Acme (2d) is not.
	if jane.DealsGoneDark != 1 {
		t.Fatalf("Jane should have 1 dark deal, got %d", jane.DealsGoneDark)
	}
}

func TestPatterns(t *testing.T) {
	dbPath := seedStore(t)
	out := runNovel(t, dbPath, []string{"patterns"}, "--term", "pricing")
	var groups []patternGroup
	if err := json.Unmarshal([]byte(out), &groups); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	// Only the Acme call title mentions "pricing".
	var total int
	for _, g := range groups {
		total += g.MatchCount
		if g.DealName == "Acme Expansion" && g.Stage != "Demo" {
			t.Fatalf("expected Acme grouped at stage Demo, got %q", g.Stage)
		}
	}
	if total != 1 {
		t.Fatalf("expected exactly 1 pricing match, got %d (%s)", total, out)
	}

	// Negative test: a term nobody said returns nothing.
	out = runNovel(t, dbPath, []string{"patterns"}, "--term", "zzz-nonexistent-term")
	groups = nil
	_ = json.Unmarshal([]byte(out), &groups)
	for _, g := range groups {
		if g.MatchCount > 0 {
			t.Fatalf("expected no matches for nonsense term, got %+v", g)
		}
	}
}
