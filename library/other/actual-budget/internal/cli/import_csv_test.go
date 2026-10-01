// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type s3ImportPlan struct {
	DryRun       bool   `json:"dry_run"`
	Preview      bool   `json:"preview"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	AccountID    string `json:"account_id"`
	Account      string `json:"account"`
	Submitted    int    `json:"submitted"`
	HTTPStatus   int    `json:"http_status"`
	Transactions []struct {
		Date          string `json:"date"`
		Amount        int64  `json:"amount"`
		PayeeName     string `json:"payee_name"`
		ImportedPayee string `json:"imported_payee"`
		ImportedID    string `json:"imported_id"`
	} `json:"transactions"`
	InvalidRows []struct {
		Line  int    `json:"line"`
		Error string `json:"error"`
	} `json:"invalid_rows"`
}

var s3ImportArgs = []string{"import-csv", "testdata/sample.csv", "--account", "Checking", "--date-col", "Date", "--amount-col", "Amount", "--payee-col", "Description", "--id-col", "Reference"}

func TestImportCSVPostsToSidecarImport(t *testing.T) {
	s3Fixture(t)
	requests := s3Sidecar(t, nil)
	var plan s3ImportPlan
	s3RunJSON(t, &plan, s3ImportArgs...)
	reqs := requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %+v, want 1", reqs)
	}
	r := reqs[0]
	if r.Method != "POST" || r.Path != "/budgets/fixture-budget/accounts/acct-checking/transactions/import" {
		t.Fatalf("request = %s %s", r.Method, r.Path)
	}
	raw, _ := json.Marshal(r.Body["transactions"])
	var sent []map[string]any
	if err := json.Unmarshal(raw, &sent); err != nil || len(sent) != 3 {
		t.Fatalf("body transactions = %s", raw)
	}
	want := []struct {
		date, payee, id string
		amount          float64
	}{
		{"2026-09-03", "KROGER #123 CINCINNATI", "ref-1001", -8450},
		{"2026-09-05", "ACME CORP PAYROLL", "ref-1002", 400000},
		{"2026-09-07", "NETFLIX.COM", "ref-1003", -1549},
	}
	for i, w := range want {
		s := sent[i]
		if s["date"] != w.date || s["amount"] != w.amount || s["payee_name"] != w.payee || s["imported_payee"] != w.payee || s["imported_id"] != w.id {
			t.Errorf("row %d = %+v, want %+v", i, s, w)
		}
	}
	if plan.Submitted != 3 || plan.HTTPStatus != 200 || plan.AccountID != "acct-checking" || len(plan.InvalidRows) != 1 || plan.InvalidRows[0].Line != 5 {
		t.Fatalf("result = %+v", plan)
	}
}

func TestImportCSVDryRunAndPreviewSendNothing(t *testing.T) {
	s3Fixture(t)
	requests := s3Sidecar(t, nil)
	var plan s3ImportPlan
	s3RunJSON(t, &plan, append(append([]string{}, s3ImportArgs...), "--dry-run")...)
	if !plan.DryRun || plan.Method != "POST" || plan.AccountID != "acct-checking" || len(plan.Transactions) != 3 || len(plan.InvalidRows) != 1 {
		t.Fatalf("dry-run plan = %+v", plan)
	}
	plan = s3ImportPlan{}
	s3RunJSON(t, &plan, append(append([]string{}, s3ImportArgs...), "--preview")...)
	if !plan.Preview || len(plan.Transactions) != 3 || plan.Transactions[2].Amount != -1549 {
		t.Fatalf("preview = %+v", plan)
	}
	if n := len(requests()); n != 0 {
		t.Fatalf("dry-run/preview sent %d requests", n)
	}
}

func TestImportCSVDryRunWithoutFileExitsZero(t *testing.T) {
	s3NoMirror(t)
	out, _, err := s3Run(t, "import-csv", "testdata/does-not-exist.csv", "--account", "Checking", "--date-col", "Date", "--amount-col", "Amount", "--dry-run")
	if err != nil || !strings.Contains(out, `"dry_run"`) {
		t.Fatalf("dry-run without file: out=%q err=%v", out, err)
	}
}

func TestImportCSVValidation(t *testing.T) {
	s3Fixture(t)
	requests := s3Sidecar(t, nil)
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"missing account", []string{"import-csv", "testdata/sample.csv", "--date-col", "Date", "--amount-col", "Amount"}, 2},
		// Unknown names are "not found" (3), as in report and ledger.
		{"unknown account name", []string{"import-csv", "testdata/sample.csv", "--account", "Nope", "--date-col", "Date", "--amount-col", "Amount"}, 3},
		{"missing amount mapping", []string{"import-csv", "testdata/sample.csv", "--account", "Checking", "--date-col", "Date"}, 2},
		{"unknown column", []string{"import-csv", "testdata/sample.csv", "--account", "Checking", "--date-col", "Posted", "--amount-col", "Amount"}, 2},
		{"missing file", []string{"import-csv", "testdata/nope.csv", "--account", "Checking", "--date-col", "Date", "--amount-col", "Amount"}, 2},
		{"missing file arg", []string{"import-csv", "--account", "Checking", "--date-col", "Date", "--amount-col", "Amount"}, 2},
	}
	for _, c := range cases {
		if _, _, err := s3Run(t, c.args...); s3ExitCode(err) != c.code {
			t.Errorf("%s: exit %d (%v), want %d", c.name, s3ExitCode(err), err, c.code)
		}
	}
	if n := len(requests()); n != 0 {
		t.Fatalf("invalid invocations sent %d requests", n)
	}
}

func TestImportCSVAccountWithoutMirror(t *testing.T) {
	s3NoMirror(t)
	requests := s3Sidecar(t, nil)
	if _, _, err := s3Run(t, s3ImportArgs...); s3ExitCode(err) != 2 {
		t.Fatalf("name without mirror: exit %d, want 2", s3ExitCode(err))
	}
	id := "0b6a3c3e-5f43-4c7b-9d55-1a2b3c4d5e6f"
	var plan s3ImportPlan
	s3RunJSON(t, &plan, "import-csv", "testdata/sample.csv", "--account", id, "--date-col", "Date", "--amount-col", "Amount", "--payee-col", "Description")
	reqs := requests()
	if len(reqs) != 1 || reqs[0].Path != "/budgets/fixture-budget/accounts/"+id+"/transactions/import" {
		t.Fatalf("requests = %+v", reqs)
	}
}

func TestImportCSVBareShowsHelp(t *testing.T) {
	s3NoMirror(t)
	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"import-csv"})
	if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), "Usage:") {
		t.Fatalf("bare import-csv: err=%v out=%s", err, out.String())
	}
}
