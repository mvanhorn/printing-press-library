package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBrowserPlanDefaultsAreAgentSafe(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newBrowserCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"plan"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	items, ok := result["requested_items"].([]any)
	if !ok || len(items) != 6 {
		t.Fatalf("unexpected requested items: %#v", result["requested_items"])
	}
	if result["transport"] != "browser" || result["credentials_exported"] != false || result["remote_write_performed"] != false {
		t.Fatalf("unsafe or incomplete plan: %#v", result)
	}
}

func TestValidateBrowserQuoteSelectsLowestCompleteEstimate(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newBrowserCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"validate-quote", "--input", "testdata/ifood_browser_quote.json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result browserQuoteValidation
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Complete || result.CompleteMarketCount != 3 || result.SelectedMarketID != "market-c" {
		t.Fatalf("unexpected validation: %#v", result)
	}
	if result.Markets[0].EstimatedTotalBRL == nil || *result.Markets[0].EstimatedTotalBRL != 76.5 {
		t.Fatalf("unexpected selected total: %#v", result.Markets[0])
	}
}

func TestBrowserExampleCommandsExerciseEmbeddedQuote(t *testing.T) {
	for _, test := range []struct {
		name  string
		args  []string
		check func(*testing.T, map[string]any)
	}{
		{
			name: "validate quote",
			args: []string{"validate-quote", "--example"},
			check: func(t *testing.T, result map[string]any) {
				if result["complete"] != true || result["complete_market_count"] != float64(3) {
					t.Fatalf("unexpected example validation: %#v", result)
				}
			},
		},
		{
			name: "cart plan",
			args: []string{"cart-plan", "--example"},
			check: func(t *testing.T, result map[string]any) {
				if result["ready"] != true || result["remote_write_performed"] != false {
					t.Fatalf("unexpected example cart plan: %#v", result)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			flags := &rootFlags{asJSON: true}
			cmd := newBrowserCmd(flags)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(test.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatalf("decode output %q: %v", output.String(), err)
			}
			test.check(t, result)
		})
	}
}

func TestValidateBrowserQuoteFailsClosedOnIncompleteCoverage(t *testing.T) {
	observation := browserQuoteObservation{
		SchemaVersion:  browserQuoteSchemaVersion,
		RequestedItems: []browserRequestedItem{{Term: "papel toalha", Quantity: 1}},
		Markets: []browserMarketObservation{{
			ID: "market-a", Name: "Mercado A", Rating: 4.9,
			Items: []browserItemObservation{{Term: "papel toalha", ProductName: "Papel Toalha", UnitPrice: 10, Available: false}},
		}},
	}
	validation, err := validateBrowserQuote(observation, 4.5, 1)
	if err != nil {
		t.Fatal(err)
	}
	if validation.Complete || validation.CompleteMarketCount != 0 || len(validation.Markets[0].InvalidTerms) != 1 {
		t.Fatalf("incomplete quote accepted: %#v", validation)
	}
}

func TestBrowserCartPlanRequiresCompleteThreeMarketQuote(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newBrowserCmd(flags)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"cart-plan", "--input", "testdata/ifood_browser_quote.json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	merchant := result["merchant"].(map[string]any)
	items := result["items"].([]any)
	if result["ready"] != true || result["requires_confirmation"] != true || result["remote_write_performed"] != false || merchant["id"] != "market-c" || len(items) != 6 {
		t.Fatalf("unexpected cart plan: %#v", result)
	}
}

func TestBrowserFileCommandsDryRunWithoutReadingInput(t *testing.T) {
	for _, command := range []string{"validate-quote", "cart-plan"} {
		t.Run(command, func(t *testing.T) {
			flags := &rootFlags{asJSON: true, dryRun: true}
			cmd := newBrowserCmd(flags)
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{command, "--input", "missing-observation.json"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(output.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result["dry_run"] != true || result["remote_write_performed"] != false || result["input"] != "missing-observation.json" {
				t.Fatalf("unexpected dry-run result: %#v", result)
			}
		})
	}
}

func TestBrowserObservationRejectsCredentialFields(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newBrowserCmd(flags)
	cmd.SetIn(strings.NewReader(`{"schema_version":1,"requested_items":[{"term":"x","quantity":1}],"markets":[{"name":"M","rating":5,"items":[]}],"authorization":"secret"}`))
	cmd.SetArgs([]string{"validate-quote", "--input", "-", "--markets", "1"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected credential-like unknown field to be rejected, got %v", err)
	}
}
