package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// newTestCmd builds a bare cobra.Command we can hand to emitCandidateError so
// we can capture stdout. The command is never executed.
func newTestCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	return cmd, &out, &errOut
}

func TestEmitCandidateError_JSONEnvelopeHasAttemptsAndHint(t *testing.T) {
	app := newTestApp(t)
	app.JSON = true
	cmd, out, _ := newTestCmd()

	ce := &addCandidateError{
		LastErrorType: notFoundBasketProduct,
		Attempts: []addAttempt{
			{ItemID: "items_1576-A", Name: "Strawberries, 1 lb", ErrorType: notFoundBasketProduct},
			{ItemID: "items_1576-B", Name: "Organic Strawberries, 2 lbs", ErrorType: notFoundBasketProduct},
		},
	}
	err := emitCandidateError(cmd, app, "costco", "strawberries", ce)
	if err == nil {
		t.Fatal("expected non-nil coded error")
	}
	if _, ok := err.(CodedError); !ok {
		t.Fatalf("err type=%T, want CodedError", err)
	}

	var envelope map[string]any
	if derr := json.Unmarshal(out.Bytes(), &envelope); derr != nil {
		t.Fatalf("envelope parse: %v, raw=%q", derr, out.String())
	}
	if envelope["error"] != notFoundBasketProduct {
		t.Errorf("error=%v, want %q", envelope["error"], notFoundBasketProduct)
	}
	if envelope["retailer"] != "costco" {
		t.Errorf("retailer=%v, want costco", envelope["retailer"])
	}
	if envelope["query"] != "strawberries" {
		t.Errorf("query=%v, want strawberries", envelope["query"])
	}
	hint, _ := envelope["hint"].(string)
	if !strings.Contains(hint, "instacart search") {
		t.Errorf("hint missing search guidance: %q", hint)
	}
	if !strings.Contains(hint, "--item-id") {
		t.Errorf("hint missing --item-id guidance: %q", hint)
	}
	if !strings.Contains(hint, "--no-history") {
		t.Errorf("hint missing --no-history guidance: %q", hint)
	}
	attempts, ok := envelope["attempts"].([]any)
	if !ok || len(attempts) != 2 {
		t.Fatalf("attempts=%v, want 2 entries", envelope["attempts"])
	}
}

func TestEmitCandidateError_TextModeIncludesHintInError(t *testing.T) {
	app := newTestApp(t)
	app.JSON = false
	cmd, out, _ := newTestCmd()

	ce := &addCandidateError{
		LastErrorType: notFoundBasketProduct,
		Attempts: []addAttempt{
			{ItemID: "items_1576-A", Name: "Thing", ErrorType: notFoundBasketProduct},
		},
	}
	err := emitCandidateError(cmd, app, "costco", "strawberries", ce)
	if err == nil {
		t.Fatal("expected error")
	}
	if out.Len() != 0 {
		t.Errorf("stdout should be empty in text mode, got %q", out.String())
	}
	msg := err.Error()
	if !strings.Contains(msg, "notFoundBasketProduct") {
		t.Errorf("error missing error type: %q", msg)
	}
	if !strings.Contains(msg, "instacart search") {
		t.Errorf("error missing guidance: %q", msg)
	}
	if !strings.Contains(msg, "strawberries") {
		t.Errorf("error missing user's query verbatim: %q", msg)
	}
}

func TestEmitCandidateError_EmptyQueryUsesAlternateHint(t *testing.T) {
	app := newTestApp(t)
	app.JSON = false
	cmd, _, _ := newTestCmd()

	ce := &addCandidateError{LastErrorType: notFoundBasketProduct}
	err := emitCandidateError(cmd, app, "costco", "", ce)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, "instacart search") {
		t.Errorf("no-query path should not suggest search: %q", msg)
	}
	if !strings.Contains(msg, "cart show") {
		t.Errorf("no-query path should suggest cart show: %q", msg)
	}
}

func TestEmitCandidateError_ExitCodeIsConflict(t *testing.T) {
	app := newTestApp(t)
	cmd, _, _ := newTestCmd()
	ce := &addCandidateError{LastErrorType: notFoundBasketProduct}
	err := emitCandidateError(cmd, app, "costco", "x", ce)
	codedErr, ok := err.(CodedError)
	if !ok {
		t.Fatalf("err type=%T, want CodedError", err)
	}
	if codedErr.Code() != ExitConflict {
		t.Errorf("exit code=%d, want %d", codedErr.Code(), ExitConflict)
	}
}
