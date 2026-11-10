package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// Duplicate proposals look (board #95, app f2d638db6): the reprint renamed the two contact commands, added the list's paging
// flags and the import rows command. Hand-written: the old names as aliases, the paging guards, send-only-given flags.
const dpImport = "7a1b2c3d-4e5f-4a6b-8c7d-9e0f1a2b3c4d"

func TestDuplicateProposalsOldNamesStayAliases(t *testing.T) {
	cmd := newContactsCmd(&rootFlags{})
	for alias, name := range map[string]string{"get-contacts-id": "list-contact", "post-contacts-id-proposal-id": "decide-contact"} {
		sub, _, err := cmd.Find([]string{"duplicate-proposals", alias})
		if err != nil || sub == nil || sub.Name() != name {
			t.Errorf("%s: find = %v %v, want %s", alias, sub, err, name)
		}
	}
}

func TestDuplicateProposalsListSendsEachPagingFlag(t *testing.T) {
	cases := map[string]struct {
		args []string
		want map[string]string
	}{
		"cursor":      {[]string{"--cursor", "eyJ0Ijo+/="}, map[string]string{"cursor": "eyJ0Ijo+/="}},
		"limit":       {[]string{"--limit", "100"}, map[string]string{"limit": "100"}},
		"held-cursor": {[]string{"--held-cursor", "abc"}, map[string]string{"held_cursor": "abc"}},
		"held-limit":  {[]string{"--held-limit", "1"}, map[string]string{"held_limit": "1"}},
		"all four":    {[]string{"--cursor", "c1", "--limit", "10", "--held-cursor", "h0", "--held-limit", "7"}, map[string]string{"cursor": "c1", "limit": "10", "held_cursor": "h0", "held_limit": "7"}},
	}
	for name, tc := range cases {
		reqs, err := runUnknownCaller(t, newContactsDuplicateProposalsListContactCmd, append([]string{dpContact}, tc.args...)...)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(reqs) != 1 || reqs[0].path != "/api/v1/contacts/"+dpContact+"/duplicate-proposals" || len(reqs[0].query) != len(tc.want) {
			t.Fatalf("%s: requests = %+v", name, reqs)
		}
		for k, v := range tc.want {
			if reqs[0].query.Get(k) != v {
				t.Errorf("%s: %s = %q, want %q", name, k, reqs[0].query.Get(k), v)
			}
		}
	}
}

func TestDuplicateProposalsListRefusesBadPagingBeforeAnyRequest(t *testing.T) {
	for name, args := range map[string][]string{
		"limit 0":           {"--limit", "0"},
		"limit 101":         {"--limit", "101"},
		"held-limit 0":      {"--held-limit", "0"},
		"held-limit 101":    {"--held-limit", "101"},
		"limit not int":     {"--limit", "2.5"},
		"blank cursor":      {"--cursor", " "},
		"empty held-cursor": {"--held-cursor", ""},
		"no --all":          {"--all"},
	} {
		reqs, err := runUnknownCaller(t, newContactsDuplicateProposalsListContactCmd, append([]string{dpContact}, args...)...)
		if err == nil || len(reqs) != 0 {
			t.Errorf("%s: err=%v requests=%d", name, err, len(reqs))
		}
	}
}

func TestDuplicateProposalsHelpDocumentsPagingAndRevisions(t *testing.T) {
	list := newContactsDuplicateProposalsListContactCmd(&rootFlags{})
	for _, want := range []string{"total", "nextCursor", "heldIdentifiersTotal", "heldIdentifiersNextCursor", "membershipRevision", "heldIdentifiersMembershipRevision", "read page one again", "invalid_cursor"} {
		if !strings.Contains(list.Long, want) {
			t.Errorf("list Long is missing %q", want)
		}
	}
	decide := newContactsDuplicateProposalsDecideContactCmd(&rootFlags{})
	for _, want := range []string{"previousMembershipRevision", "membershipRevision", "total", "AFTER", "BEFORE"} {
		if !strings.Contains(decide.Long, want) {
			t.Errorf("decide Long is missing %q", want)
		}
	}
}

func TestImportsDuplicateProposalsSendsGETAndOnlyGivenPaging(t *testing.T) {
	reqs, err := runUnknownCaller(t, newImportsDuplicateProposalsGetImportsIdCmd, dpImport)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].method != "GET" || reqs[0].path != "/api/v1/imports/"+dpImport+"/duplicate-proposals" || len(reqs[0].query) != 0 {
		t.Fatalf("bare: requests = %+v", reqs)
	}
	reqs, err = runUnknownCaller(t, newImportsDuplicateProposalsGetImportsIdCmd, dpImport, "--page", "2", "--per-page", "100")
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 || reqs[0].query.Get("page") != "2" || reqs[0].query.Get("per_page") != "100" || len(reqs[0].query) != 2 {
		t.Fatalf("paged: requests = %+v", reqs)
	}
}

func TestImportsDuplicateProposalsIsStrictBeforeAnyRequest(t *testing.T) {
	for name, args := range map[string][]string{
		"bad id":        {"nope"},
		"page 0":        {dpImport, "--page", "0"},
		"page 100001":   {dpImport, "--page", "100001"},
		"per-page 0":    {dpImport, "--per-page", "0"},
		"per-page 101":  {dpImport, "--per-page", "101"},
		"cursor":        {dpImport, "--cursor", "x"},
		"no --all flag": {dpImport, "--all"},
	} {
		reqs, err := runUnknownCaller(t, newImportsDuplicateProposalsGetImportsIdCmd, args...)
		if err == nil || len(reqs) != 0 {
			t.Errorf("%s: err=%v requests=%d", name, err, len(reqs))
		}
	}
}

func TestImportsGroupRegistersDuplicateProposals(t *testing.T) {
	cmd := newImportsCmd(&rootFlags{})
	sub, _, err := cmd.Find([]string{"duplicate-proposals", "get-imports-id"})
	if err != nil || sub == nil || sub.Name() != "get-imports-id" {
		t.Fatalf("find: %v %v", sub, err)
	}
	for _, want := range []string{"A contact you cannot view", "merged", "decide-contact", "newRecord", "matchedOn", "never a value", "contacts:view"} {
		if !strings.Contains(sub.Long, want) {
			t.Errorf("Long is missing %q", want)
		}
	}
}

// Parity r2 (Codex P2): in a terminal the provenance line counts the nested arrays of the object-shaped answers, never "0 results".
func runInTerminal(t *testing.T, build func(*rootFlags) *cobra.Command, answer string, args ...string) (stderr string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(answer))
	}))
	defer srv.Close()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	tty, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer tty.Close()
	var errBuf bytes.Buffer
	cmd := build(&rootFlags{noCache: true})
	cmd.SetOut(tty) // /dev/null is a character device, so isTerminal(stdout) is true
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return errBuf.String()
}

func TestDuplicateProposalsProvenanceCountsNestedArrays(t *testing.T) {
	got := runInTerminal(t, newContactsDuplicateProposalsListContactCmd,
		`{"proposals":[{"id":"a"},{"id":"b"},{"id":"c"}],"heldIdentifiers":[{"kind":"email"},{"kind":"phone"}],"total":3,"nextCursor":null}`, dpContact)
	if !strings.Contains(got, "5 results (live)") || strings.Contains(got, "0 results") {
		t.Errorf("contact list stderr = %q, want 5 results (live)", got)
	}
	got = runInTerminal(t, newImportsDuplicateProposalsGetImportsIdCmd,
		`{"rows":[{"proposalId":"a"},{"proposalId":"b"}],"meta":{"page":1,"per_page":50,"total":2}}`, dpImport)
	if !strings.Contains(got, "2 results (live)") || strings.Contains(got, "0 results") {
		t.Errorf("import rows stderr = %q, want 2 results (live)", got)
	}
}

func TestDuplicateProposalsHelpAndManifestAgreeOnBlankCursor(t *testing.T) {
	flag := newContactsDuplicateProposalsListContactCmd(&rootFlags{}).Flags().Lookup("cursor")
	if flag == nil || !strings.Contains(flag.Usage, "Omit it for the first page; an explicitly blank value is refused") || strings.Contains(flag.Usage, "empty") {
		t.Errorf("cursor usage = %+v", flag)
	}
	raw, err := os.ReadFile("../../tools-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "Omitted or empty = the first page") {
		t.Error("tools-manifest.json still says an empty cursor means the first page")
	}
	for _, old := range []string{"contacts_duplicate-proposals_get-contacts-id", "contacts_duplicate-proposals_post-contacts-id-proposal-id"} {
		if !strings.Contains(string(raw), `"name": "`+old+`"`) {
			t.Errorf("tools-manifest.json lost the deprecated alias %s", old)
		}
	}
}
