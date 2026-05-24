package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/lunch-money/internal/internalapi"

	"github.com/spf13/cobra"
)

// PATCH: Ensure rules update can change actions without re-specifying payee.
func TestRuleUpdateSpecPreservesExistingFields(t *testing.T) {
	payee := "EXAMPLE PAYEE"
	match := "contain"
	oldCategory := int64(111)
	oldNotes := "old note"
	existing := &internalapi.Rule{
		RuleCriteriaID:         301,
		CriteriaPayeeName:      &payee,
		CriteriaPayeeNameMatch: &match,
		CriteriaPriority:       7,
		CriteriaOnPlaid:        true,
		CriteriaOnCSV:          true,
		CriteriaOnManual:       true,
		CriteriaOnAPI:          true,
		CategoryID:             &oldCategory,
		Notes:                  &oldNotes,
	}

	var fl ruleCreateFlags
	cmd := &cobra.Command{}
	fl.bindCreate(cmd)
	mustSetFlag(t, cmd, "category-id", "74")
	mustSetFlag(t, cmd, "set-notes", "shareholder transfer repayment from ExampleVendor Corp")
	mustSetFlag(t, cmd, "desc", "ExampleVendor repayment")

	spec, err := fl.toUpdateSpec(existing, cmd)
	if err != nil {
		t.Fatalf("toUpdateSpec: %v", err)
	}
	if spec.Conditions.Payee == nil {
		t.Fatal("Payee was not preserved")
	}
	if spec.Conditions.Payee.Name != payee || spec.Conditions.Payee.Match != match {
		t.Fatalf("Payee = %+v, want existing payee/match", spec.Conditions.Payee)
	}
	if spec.Conditions.Priority != "7" {
		t.Fatalf("Priority = %q, want preserved priority 7", spec.Conditions.Priority)
	}
	if !spec.Conditions.OnPlaid || !spec.Conditions.OnCSV || !spec.Conditions.OnManual || !spec.Conditions.OnAPI {
		t.Fatalf("source flags were not preserved: %+v", spec.Conditions)
	}
	if spec.Actions.CategoryID == nil || *spec.Actions.CategoryID != 74 {
		t.Fatalf("CategoryID = %#v, want updated 74", spec.Actions.CategoryID)
	}
	if spec.Actions.Notes == nil || *spec.Actions.Notes != "shareholder transfer repayment from ExampleVendor Corp" {
		t.Fatalf("Notes = %#v, want updated notes", spec.Actions.Notes)
	}
	if spec.Actions.Description == nil || *spec.Actions.Description != "ExampleVendor repayment" {
		t.Fatalf("Description = %#v, want updated description", spec.Actions.Description)
	}
}

// PATCH: Cover the action-only rules update command path without live mutation.
func TestRulesUpdateCommandPreservesExistingRule(t *testing.T) {
	payee := "EXAMPLE PAYEE"
	match := "contain"
	var gotPUT bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rules":
			_, _ = w.Write([]byte(`{
				"total_returned": 1,
				"rules": [{
					"rule_id": 99,
					"rule_criteria_id": 301,
					"criteria_payee_name": "EXAMPLE PAYEE",
					"criteria_payee_name_match": "contain",
					"criteria_priority": 7,
					"criteria_on_plaid": true,
					"criteria_on_csv": true,
					"criteria_on_manual": true,
					"criteria_on_api": true,
					"category_id": 111,
					"notes": "old note"
				}]
			}`))
		case r.Method == http.MethodPut && r.URL.Path == "/rules/301":
			gotPUT = true
			var spec internalapi.RuleSpec
			if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
				t.Fatalf("decode PUT body: %v", err)
			}
			if spec.Conditions.Payee == nil || spec.Conditions.Payee.Name != payee || spec.Conditions.Payee.Match != match {
				t.Fatalf("PUT payee = %+v, want preserved", spec.Conditions.Payee)
			}
			if spec.Actions.CategoryID == nil || *spec.Actions.CategoryID != 74 {
				t.Fatalf("PUT category = %#v, want 74", spec.Actions.CategoryID)
			}
			if spec.Actions.Notes == nil || *spec.Actions.Notes != "shareholder transfer repayment from ExampleVendor Corp" {
				t.Fatalf("PUT notes = %#v", spec.Actions.Notes)
			}
			_, _ = w.Write([]byte(`{"rule_id":99,"rule_criteria_id":301,"criteria_priority":7}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c, err := internalapi.New("")
	if err != nil {
		t.Fatalf("internalapi.New: %v", err)
	}
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	oldClient := newInternalClientForRules
	newInternalClientForRules = func() (*internalapi.Client, error) {
		return c, nil
	}
	defer func() {
		newInternalClientForRules = oldClient
	}()

	flags := &rootFlags{asJSON: true}
	cmd := newRulesUpdateCmd(flags)
	cmd.SetArgs([]string{
		"301",
		"--category-id", "74",
		"--set-notes", "shareholder transfer repayment from ExampleVendor Corp",
		"--desc", "ExampleVendor repayment",
	})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !gotPUT {
		t.Fatal("rules update did not send PUT")
	}
	if !json.Valid(out.Bytes()) {
		t.Fatalf("output = %q, want JSON", out.String())
	}
}

// PATCH: `rules apply --dry-run=false` must not mutate unless --commit is explicit.
func TestRulesApplyDryRunFalseRequiresCommit(t *testing.T) {
	oldClient := newInternalClientForRules
	called := false
	newInternalClientForRules = func() (*internalapi.Client, error) {
		called = true
		return nil, errors.New("client should not be constructed")
	}
	defer func() {
		newInternalClientForRules = oldClient
	}()

	cmd := newRulesApplyCmd(&rootFlags{asJSON: true})
	cmd.SetArgs([]string{"11062614", "--dry-run=false"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil, want usage error")
	}
	if !strings.Contains(err.Error(), "--dry-run=false requires --commit") {
		t.Fatalf("error = %q, want --commit guidance", err.Error())
	}
	if called {
		t.Fatal("client was constructed before rejecting unsafe apply")
	}
}

// PATCH: rules delete must honor root --dry-run/--agent/--yes flags.
func TestRulesDeleteDryRunUsesRootFlags(t *testing.T) {
	oldClient := newInternalClientForRules
	called := false
	newInternalClientForRules = func() (*internalapi.Client, error) {
		called = true
		return nil, errors.New("client should not be constructed")
	}
	defer func() {
		newInternalClientForRules = oldClient
	}()

	cmd := newRulesDeleteCmd(&rootFlags{asJSON: true, dryRun: true, yes: true})
	cmd.SetArgs([]string{"11062614", "11062615"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if called {
		t.Fatal("client was constructed during delete dry-run")
	}
	var got struct {
		DryRun bool    `json:"dry_run"`
		Action string  `json:"action"`
		IDs    []int64 `json:"criteria_ids"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out.String(), err)
	}
	if !got.DryRun || got.Action != "delete" || len(got.IDs) != 2 || got.IDs[0] != 11062614 || got.IDs[1] != 11062615 {
		t.Fatalf("dry-run envelope = %+v", got)
	}
}

// PATCH: rules create dry-runs preview the internal rule spec without needing cookies.
func TestRulesCreateDryRunDoesNotConstructClient(t *testing.T) {
	oldClient := newInternalClientForRules
	called := false
	newInternalClientForRules = func() (*internalapi.Client, error) {
		called = true
		return nil, errors.New("client should not be constructed")
	}
	defer func() {
		newInternalClientForRules = oldClient
	}()

	cmd := newRulesCreateCmd(&rootFlags{asJSON: true, dryRun: true})
	cmd.SetArgs([]string{"--payee", "EXAMPLE PAYEE", "--match", "contain", "--category-id", "74", "--desc", "fashion inventory expense"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if called {
		t.Fatal("client was constructed during create dry-run")
	}
	var got struct {
		DryRun bool   `json:"dry_run"`
		Action string `json:"action"`
		Spec   struct {
			Actions struct {
				CategoryID *int64 `json:"category_id"`
			} `json:"actions"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out.String(), err)
	}
	if !got.DryRun || got.Action != "create" || got.Spec.Actions.CategoryID == nil || *got.Spec.Actions.CategoryID != 74 {
		t.Fatalf("dry-run envelope = %+v", got)
	}
}

// PATCH: internal mutating raw requests must respect global --dry-run.
func TestInternalRequestDryRunDoesNotNeedSession(t *testing.T) {
	cmd := newInternalCmd(&rootFlags{asJSON: true, dryRun: true})
	cmd.SetArgs([]string{"request", "--method", "POST", "--body", `{"x":1}`, "/rules"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var got struct {
		DryRun bool           `json:"dry_run"`
		Method string         `json:"method"`
		Path   string         `json:"path"`
		Body   map[string]any `json:"request_body"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out.String(), err)
	}
	if !got.DryRun || got.Method != "POST" || got.Path != "/rules" || got.Body["x"].(float64) != 1 {
		t.Fatalf("dry-run envelope = %+v", got)
	}
}

func mustSetFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("setting --%s: %v", name, err)
	}
}
