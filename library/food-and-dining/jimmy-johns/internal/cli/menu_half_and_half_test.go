package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func TestHalfAndHalf_HappyPath(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newMenuHalfAndHalfCmd(flags)
	cmd.SetArgs([]string{"--left", "111", "--right", "222", "--left-label", "Vito", "--right-label", "Pepe"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var plan halfAndHalfPlan
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil {
		t.Fatalf("parse output: %v\n%s", err, out.String())
	}
	if plan.Left.ProductID != "111" || plan.Right.ProductID != "222" {
		t.Errorf("sides not set correctly: %+v / %+v", plan.Left, plan.Right)
	}
	if len(plan.Cart) != 2 {
		t.Fatalf("cart len = %d, want 2", len(plan.Cart))
	}
	if plan.Native {
		t.Errorf("Native should be false — JJ doesn't support half-and-half natively")
	}
	noteJoined := strings.Join(plan.Notes, " | ")
	if !strings.Contains(strings.ToLower(noteJoined), "not natively support") {
		t.Errorf("notes should disclose lack of native support: %v", plan.Notes)
	}
}

func TestHalfAndHalf_SameProductRejected(t *testing.T) {
	flags := &rootFlags{asJSON: true}
	cmd := newMenuHalfAndHalfCmd(flags)
	cmd.SetArgs([]string{"--left", "111", "--right", "111"})
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for same product on both sides; got success")
	}
	if !strings.Contains(err.Error(), "cannot be the same") {
		t.Errorf("error = %v, want 'cannot be the same' message", err)
	}
}
