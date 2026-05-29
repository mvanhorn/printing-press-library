// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"strings"
	"testing"
)

// --- Parser happy paths ---

func TestParseFilters_EmptyInput(t *testing.T) {
	expr, err := ParseFilters(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !expr.IsEmpty() {
		t.Fatalf("expected empty expression, got %d groups", len(expr.Groups))
	}
	if !expr.Match(map[string]string{"anything": "goes"}) {
		t.Fatalf("empty expression must match every row")
	}
}

func TestParseFilters_SingleEQ(t *testing.T) {
	expr, err := ParseFilters([]string{"lifecyclestage=customer"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expr.Groups) != 1 || len(expr.Groups[0].Clauses) != 1 {
		t.Fatalf("expected 1 group / 1 clause, got %+v", expr)
	}
	c := expr.Groups[0].Clauses[0]
	if c.Field != "lifecyclestage" || c.Op != OpEq || c.Value != "customer" {
		t.Fatalf("bad clause: %+v", c)
	}
}

func TestParseFilters_GroupOfTwo(t *testing.T) {
	expr, err := ParseFilters([]string{"lifecyclestage=customer !do_not_call"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expr.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(expr.Groups))
	}
	cs := expr.Groups[0].Clauses
	if len(cs) != 2 {
		t.Fatalf("expected 2 clauses, got %d", len(cs))
	}
	if cs[0].Op != OpEq || cs[0].Field != "lifecyclestage" || cs[0].Value != "customer" {
		t.Fatalf("clause 0 bad: %+v", cs[0])
	}
	if cs[1].Op != OpNotHas || cs[1].Field != "do_not_call" {
		t.Fatalf("clause 1 bad: %+v", cs[1])
	}
}

func TestParseFilters_TwoGroupsOR(t *testing.T) {
	expr, err := ParseFilters([]string{"lifecyclestage=customer", "amount~1000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expr.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(expr.Groups))
	}
	if expr.Groups[1].Clauses[0].Op != OpContainsToken {
		t.Fatalf("expected CONTAINS_TOKEN in group 2, got %v", expr.Groups[1].Clauses[0].Op)
	}
}

func TestParseFilters_ContainsToken(t *testing.T) {
	expr, err := ParseFilters([]string{"email~@servosity.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := expr.Groups[0].Clauses[0]
	if c.Field != "email" || c.Op != OpContainsToken || c.Value != "@servosity.com" {
		t.Fatalf("bad clause: %+v", c)
	}
}

func TestParseFilters_HasOnly(t *testing.T) {
	expr, err := ParseFilters([]string{"phone"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := expr.Groups[0].Clauses[0]
	if c.Field != "phone" || c.Op != OpHas || c.Value != "" {
		t.Fatalf("bad clause: %+v", c)
	}
}

// --- Parser errors ---

func TestParseFilters_RejectsEmptyFlag(t *testing.T) {
	if _, err := ParseFilters([]string{""}); err == nil {
		t.Fatalf("expected error for empty filter")
	}
}

func TestParseFilters_RejectsEqNoField(t *testing.T) {
	if _, err := ParseFilters([]string{"=value"}); err == nil {
		t.Fatalf("expected error for '=value'")
	}
}

func TestParseFilters_RejectsContainsNoField(t *testing.T) {
	if _, err := ParseFilters([]string{"~value"}); err == nil {
		t.Fatalf("expected error for '~value'")
	}
}

func TestParseFilters_RejectsBangAlone(t *testing.T) {
	if _, err := ParseFilters([]string{"!"}); err == nil {
		t.Fatalf("expected error for lone '!'")
	}
}

func TestParseFilters_RejectsBangWithEq(t *testing.T) {
	// `!field=value` is illegal — `!` is reserved for NOT_HAS only.
	if _, err := ParseFilters([]string{"!field=value"}); err == nil {
		t.Fatalf("expected error for '!field=value'")
	}
}

func TestParseFilters_RejectsBangWithContains(t *testing.T) {
	if _, err := ParseFilters([]string{"!field~value"}); err == nil {
		t.Fatalf("expected error for '!field~value'")
	}
}

// --- Match logic ---

func TestMatch_Has(t *testing.T) {
	expr, _ := ParseFilters([]string{"phone"})
	if !expr.Match(map[string]string{"phone": "555-1212"}) {
		t.Fatalf("HAS should match non-empty value")
	}
	if expr.Match(map[string]string{"phone": ""}) {
		t.Fatalf("HAS should reject empty value")
	}
	if expr.Match(map[string]string{"email": "x@y.com"}) {
		t.Fatalf("HAS should reject missing field")
	}
}

func TestMatch_NotHas(t *testing.T) {
	expr, _ := ParseFilters([]string{"!do_not_call"})
	if !expr.Match(map[string]string{}) {
		t.Fatalf("NOT_HAS should match missing field")
	}
	if !expr.Match(map[string]string{"do_not_call": ""}) {
		t.Fatalf("NOT_HAS should match empty field")
	}
	if expr.Match(map[string]string{"do_not_call": "true"}) {
		t.Fatalf("NOT_HAS should reject non-empty field")
	}
}

func TestMatch_EQ_CaseSensitive(t *testing.T) {
	expr, _ := ParseFilters([]string{"lifecyclestage=customer"})
	if !expr.Match(map[string]string{"lifecyclestage": "customer"}) {
		t.Fatalf("EQ should match exact value")
	}
	if expr.Match(map[string]string{"lifecyclestage": "Customer"}) {
		t.Fatalf("EQ must be case-sensitive (HubSpot docs)")
	}
}

func TestMatch_ContainsToken_CaseInsensitive(t *testing.T) {
	expr, _ := ParseFilters([]string{"email~EXAMPLE"})
	if !expr.Match(map[string]string{"email": "alex@example.com"}) {
		t.Fatalf("CONTAINS_TOKEN should be case-insensitive (substring match)")
	}
	if !expr.Match(map[string]string{"email": "ALEX@EXAMPLE.COM"}) {
		t.Fatalf("CONTAINS_TOKEN should be case-insensitive on the value side too")
	}
	if expr.Match(map[string]string{"email": "x@gmail.com"}) {
		t.Fatalf("CONTAINS_TOKEN should reject non-matching")
	}
}

func TestMatch_AND_WithinGroup(t *testing.T) {
	expr, _ := ParseFilters([]string{"lifecyclestage=customer !do_not_call"})
	if !expr.Match(map[string]string{"lifecyclestage": "customer"}) {
		t.Fatalf("AND should pass when both satisfied (do_not_call absent => NOT_HAS true)")
	}
	if expr.Match(map[string]string{"lifecyclestage": "customer", "do_not_call": "true"}) {
		t.Fatalf("AND should fail when NOT_HAS clause fails")
	}
	if expr.Match(map[string]string{"lifecyclestage": "lead"}) {
		t.Fatalf("AND should fail when EQ clause fails")
	}
}

func TestMatch_OR_AcrossGroups(t *testing.T) {
	expr, _ := ParseFilters([]string{"lifecyclestage=customer", "amount~1000"})
	if !expr.Match(map[string]string{"lifecyclestage": "customer"}) {
		t.Fatalf("OR group 1 should match")
	}
	if !expr.Match(map[string]string{"amount": "$1000.00"}) {
		t.Fatalf("OR group 2 should match")
	}
	if expr.Match(map[string]string{"lifecyclestage": "lead", "amount": "$50"}) {
		t.Fatalf("OR should fail when neither group satisfied")
	}
}

// --- SQL fragment ---

func TestSQLFragment_Empty(t *testing.T) {
	expr, _ := ParseFilters(nil)
	frag, args := expr.SQLFragment("data")
	if frag != "" || args != nil {
		t.Fatalf("empty expr should yield empty SQL, got %q args=%v", frag, args)
	}
}

func TestSQLFragment_SingleEQ(t *testing.T) {
	expr, _ := ParseFilters([]string{"lifecyclestage=customer"})
	frag, args := expr.SQLFragment("data")
	want := "(json_extract(data, '$.properties.lifecyclestage') = ?)"
	if frag != want {
		t.Fatalf("frag mismatch:\n got  %q\n want %q", frag, want)
	}
	if len(args) != 1 || args[0] != "customer" {
		t.Fatalf("args mismatch: %v", args)
	}
}

func TestSQLFragment_NotHasNoArgs(t *testing.T) {
	expr, _ := ParseFilters([]string{"!do_not_call"})
	frag, args := expr.SQLFragment("data")
	want := "((json_extract(data, '$.properties.do_not_call') IS NULL OR json_extract(data, '$.properties.do_not_call') = ''))"
	if frag != want {
		t.Fatalf("frag mismatch:\n got  %q\n want %q", frag, want)
	}
	if len(args) != 0 {
		t.Fatalf("expected no args, got %v", args)
	}
}

func TestSQLFragment_TwoGroupsOR(t *testing.T) {
	expr, _ := ParseFilters([]string{"lifecyclestage=customer", "amount~1000"})
	frag, args := expr.SQLFragment("data")
	// Two groups must be wrapped in `( ... OR ... )` per the contract.
	if !strings.HasPrefix(frag, "(") || !strings.HasSuffix(frag, ")") {
		t.Fatalf("expected outer parens, got %q", frag)
	}
	if !strings.Contains(frag, " OR ") {
		t.Fatalf("expected OR between groups, got %q", frag)
	}
	if !strings.Contains(frag, "json_extract(data, '$.properties.lifecyclestage') = ?") {
		t.Fatalf("missing EQ fragment in %q", frag)
	}
	if !strings.Contains(frag, "LOWER(json_extract(data, '$.properties.amount')) LIKE ?") {
		t.Fatalf("missing CONTAINS_TOKEN fragment in %q", frag)
	}
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(args), args)
	}
	if args[0] != "customer" {
		t.Fatalf("arg[0] = %v, want customer", args[0])
	}
	if args[1] != "%1000%" {
		t.Fatalf("arg[1] = %v, want %%1000%%", args[1])
	}
}

func TestSQLFragment_ContainsTokenLowered(t *testing.T) {
	// LIKE pattern must lower-case the value so the LOWER(...) on the
	// left-hand side meets a lowered RHS — case-insensitive substring.
	expr, _ := ParseFilters([]string{"email~SERVOSITY"})
	_, args := expr.SQLFragment("data")
	if len(args) != 1 || args[0] != "%servosity%" {
		t.Fatalf("CONTAINS_TOKEN arg should be lowered + %% wrapped, got %v", args)
	}
}

func TestSQLFragment_HasClause(t *testing.T) {
	expr, _ := ParseFilters([]string{"phone"})
	frag, args := expr.SQLFragment("data")
	want := "((json_extract(data, '$.properties.phone') IS NOT NULL AND json_extract(data, '$.properties.phone') != ''))"
	if frag != want {
		t.Fatalf("frag mismatch:\n got  %q\n want %q", frag, want)
	}
	if len(args) != 0 {
		t.Fatalf("HAS should produce no args, got %v", args)
	}
}

// --- DebugString ---

func TestDebugString_Empty(t *testing.T) {
	expr, _ := ParseFilters(nil)
	got := expr.DebugString()
	if !strings.Contains(got, "empty") {
		t.Fatalf("debug string for empty expr should say so: %q", got)
	}
}

func TestDebugString_TwoGroups(t *testing.T) {
	expr, _ := ParseFilters([]string{"lifecyclestage=customer !do_not_call", "amount~1000"})
	got := expr.DebugString()
	// Format brief from the plan:
	//   filter expr (2 groups, OR):
	//     group 1 (AND): lifecyclestage = "customer", !do_not_call
	//     group 2 (AND): amount ~ "1000"
	for _, want := range []string{
		"2 groups, OR",
		"group 1 (AND)",
		`lifecyclestage = "customer"`,
		"!do_not_call",
		"group 2 (AND)",
		`amount ~ "1000"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("debug string missing %q:\n%s", want, got)
		}
	}
}

// --- FieldsReferenced ---

func TestFieldsReferenced(t *testing.T) {
	expr, _ := ParseFilters([]string{"a=x !b", "c~y a=z"})
	fields := expr.FieldsReferenced()
	want := []string{"a", "b", "c"}
	if len(fields) != len(want) {
		t.Fatalf("fields = %v, want %v", fields, want)
	}
	for i, w := range want {
		if fields[i] != w {
			t.Fatalf("fields[%d] = %q, want %q (full: %v)", i, fields[i], w, fields)
		}
	}
}

// --- Sanity: value with '=' becomes part of EQ value, not a re-parse ---

func TestParseFilters_EqValueKeepsRemainder(t *testing.T) {
	// First `=` is the operator; the rest of the token is the literal value.
	// This matches HubSpot's documented precedence (first operator token wins).
	expr, err := ParseFilters([]string{"hs_object_source=API/IMPORT=v2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	c := expr.Groups[0].Clauses[0]
	if c.Field != "hs_object_source" {
		t.Fatalf("field bad: %q", c.Field)
	}
	if c.Value != "API/IMPORT=v2" {
		t.Fatalf("value bad: %q", c.Value)
	}
}
