package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestUnwichConvert_FindsUnwichOption(t *testing.T) {
	fixture := `[
	  {"groupId": 1, "name": "Bread Choice", "modifiers": [
	    {"modifierId": 100, "name": "8-inch French Bread"},
	    {"modifierId": 103, "name": "Unwich (Lettuce Wrap)"}
	  ]}
	]`

	flags := &rootFlags{asJSON: true}
	cmd := newMenuUnwichConvertCmd(flags)
	cmd.SetArgs([]string{"--product-id", "33328641", "--current-bread", "8-inch French Bread"})
	cmd.SetIn(strings.NewReader(fixture))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var result unwichModifier
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}
	if result.ProductID != "33328641" {
		t.Errorf("ProductID = %q, want 33328641", result.ProductID)
	}
	if result.BreadGroup != "Bread Choice" {
		t.Errorf("BreadGroup = %q, want 'Bread Choice'", result.BreadGroup)
	}
	if len(result.Diff) != 1 || !strings.Contains(result.Diff[0], "103") {
		t.Errorf("Diff = %v, want one entry referencing modifierId=103", result.Diff)
	}
}

func TestUnwichConvert_NoSandwichErrors(t *testing.T) {
	// Modifier set for a drink — no bread/wrap group.
	fixture := `[
	  {"groupId": 5, "name": "Size", "modifiers": [
	    {"modifierId": 500, "name": "Medium"},
	    {"modifierId": 501, "name": "Large"}
	  ]}
	]`
	flags := &rootFlags{asJSON: true}
	cmd := newMenuUnwichConvertCmd(flags)
	cmd.SetArgs([]string{"--product-id", "99999"})
	cmd.SetIn(strings.NewReader(fixture))
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err == nil {
		t.Fatalf("expected error for non-sandwich product, got success: %s", out.String())
	} else if !strings.Contains(err.Error(), "no bread/wrap modifier group") {
		t.Errorf("error = %v, want 'no bread/wrap modifier group' message", err)
	}
}
