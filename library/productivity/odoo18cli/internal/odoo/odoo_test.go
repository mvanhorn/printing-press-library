// Copyright 2026 andreampiovesana. Licensed under Apache-2.0. See LICENSE.

package odoo

import "testing"

func TestIDFromMany2one(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int
	}{
		{[]interface{}{42.0, "Name"}, 42},
		{[]interface{}{int64(7), "Name"}, 7},
		{false, 0},
		{nil, 0},
	}
	for _, c := range cases {
		got := IDFromMany2one(c.in)
		if got != c.want {
			t.Errorf("IDFromMany2one(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNameFromMany2one(t *testing.T) {
	cases := []struct {
		in   interface{}
		want string
	}{
		{[]interface{}{42.0, "Acme Corp"}, "Acme Corp"},
		{false, ""},
		{nil, ""},
	}
	for _, c := range cases {
		got := NameFromMany2one(c.in)
		if got != c.want {
			t.Errorf("NameFromMany2one(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStringVal(t *testing.T) {
	if got := StringVal("hello"); got != "hello" {
		t.Errorf("StringVal(string) = %q, want %q", got, "hello")
	}
	if got := StringVal(false); got != "" {
		t.Errorf("StringVal(false) = %q, want %q", got, "")
	}
	if got := StringVal(nil); got != "" {
		t.Errorf("StringVal(nil) = %q, want %q", got, "")
	}
}

func TestBoolVal(t *testing.T) {
	if !BoolVal(true) {
		t.Error("BoolVal(true) = false")
	}
	if BoolVal(false) {
		t.Error("BoolVal(false) = true")
	}
	if BoolVal(nil) {
		t.Error("BoolVal(nil) = true")
	}
}

func TestIntVal(t *testing.T) {
	if got := IntVal(42.0); got != 42 {
		t.Errorf("IntVal(42.0) = %d, want 42", got)
	}
	if got := IntVal(nil); got != 0 {
		t.Errorf("IntVal(nil) = %d, want 0", got)
	}
}

func TestFloatVal(t *testing.T) {
	if got := FloatVal(3.14); got != 3.14 {
		t.Errorf("FloatVal(3.14) = %f, want 3.14", got)
	}
	if got := FloatVal(nil); got != 0 {
		t.Errorf("FloatVal(nil) = %f, want 0", got)
	}
}
