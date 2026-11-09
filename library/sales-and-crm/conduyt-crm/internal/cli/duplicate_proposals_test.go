package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Identity rules v2 (board #69, app main 61bf61876): GET /contacts/{id}/duplicate-proposals and POST /contacts/{id}/duplicate-proposals/{proposalId}.
// Hand-written: --decision, its strict validation and the UUID checks (a reprint must keep them).
const (
	dpContact  = "3f8a0c4e-1111-4222-8333-444455556666"
	dpProposal = "9b2d7c1a-5e4f-4a3b-9c8d-7e6f5a4b3c2d"
)

func TestDuplicateProposalsGetSendsGETOnTheContactPath(t *testing.T) {
	reqs, err := runUnknownCaller(t, newContactsDuplicateProposalsListContactCmd, dpContact)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != "GET" || reqs[0].path != "/api/v1/contacts/"+dpContact+"/duplicate-proposals" || len(reqs[0].query) != 0 {
		t.Fatalf("requests = %+v", reqs)
	}
}

func TestDuplicateProposalsGetRefusesANonUUIDBeforeAnyRequest(t *testing.T) {
	for _, id := range []string{"../x", "nope", "3f8a0c4e-1111-4222-8333"} {
		reqs, err := runUnknownCaller(t, newContactsDuplicateProposalsListContactCmd, id)
		if err == nil || len(reqs) != 0 {
			t.Errorf("%q: err=%v requests=%d", id, err, len(reqs))
		}
	}
}

func TestDuplicateProposalsDecideSendsOnlyTheDecision(t *testing.T) {
	for _, d := range []string{"not_sure", "different", "merge"} {
		reqs, err := runUnknownCaller(t, newContactsDuplicateProposalsDecideContactCmd, dpContact, dpProposal, "--decision", d)
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if len(reqs) != 1 || reqs[0].method != "POST" || reqs[0].path != "/api/v1/contacts/"+dpContact+"/duplicate-proposals/"+dpProposal {
			t.Fatalf("%s: requests = %+v", d, reqs)
		}
		if len(reqs[0].body) != 1 || reqs[0].body["decision"] != d {
			t.Errorf("%s: body = %v", d, reqs[0].body)
		}
	}
}

func TestDuplicateProposalsDecideIsStrictBeforeAnyRequest(t *testing.T) {
	cases := map[string][]string{
		"no decision":      {dpContact, dpProposal},
		"merged":           {dpContact, dpProposal, "--decision", "merged"},
		"upper case":       {dpContact, dpProposal, "--decision", "Different"},
		"open":             {dpContact, dpProposal, "--decision", "open"},
		"empty":            {dpContact, dpProposal, "--decision", ""},
		"bad contact id":   {"nope", dpProposal, "--decision", "different"},
		"bad proposal id":  {dpContact, "../x", "--decision", "different"},
		"missing proposal": {dpContact, "--decision", "different"},
	}
	for name, args := range cases {
		reqs, err := runUnknownCaller(t, newContactsDuplicateProposalsDecideContactCmd, args...)
		if err == nil || len(reqs) != 0 {
			t.Errorf("%s: err=%v requests=%d", name, err, len(reqs))
		}
	}
}

func TestDuplicateProposalsDecideHelpNamesTheMeanings(t *testing.T) {
	cmd := newContactsDuplicateProposalsDecideContactCmd(&rootFlags{})
	for _, want := range []string{"not_sure", "different", "FINAL", "never merges", "not_merged", "already_decided", "other_record_not_editable"} {
		if !strings.Contains(cmd.Long, want) {
			t.Errorf("Long is missing %q", want)
		}
	}
}

func TestContactsGroupRegistersDuplicateProposals(t *testing.T) {
	cmd := newContactsCmd(&rootFlags{})
	sub, _, err := cmd.Find([]string{"duplicate-proposals", "decide-contact"})
	if err != nil || sub == nil || sub.Name() != "decide-contact" {
		t.Fatalf("find: %v %v", sub, err)
	}
}

// Codex parity r1 P2: a successful decision must not write its { id, status, decidedAt } acknowledgement into the local resource store.
func TestDuplicateProposalsDecideLeavesTheLocalStoreUnchanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"` + dpProposal + `","status":"different","decidedAt":"2026-10-15T20:00:00.000Z"}}`))
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg-data"))
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(home, "config.toml"))
	dbPath := defaultDBPath("conduyt-crm-pp-cli")
	if !strings.HasPrefix(dbPath, home) {
		t.Fatalf("store path %q escapes the test home %q", dbPath, home)
	}
	cmd := newContactsDuplicateProposalsDecideContactCmd(&rootFlags{noCache: true})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{dpContact, dpProposal, "--decision", "different"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatalf("a local store was created at %s by a decision", dbPath)
	}
}
