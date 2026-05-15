// Copyright 2026 andrea-m-piovesana. Licensed under Apache-2.0. See LICENSE.

package odoo

import "testing"

func TestStripHTML(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"&amp; &lt; &gt;", "& < >"},
		{"  plain text  ", "plain text"},
		{"<br/>line1<br/>line2", "line1 line2"},
	}
	for _, c := range cases {
		got := StripHTML(c.in)
		if got != c.want {
			t.Errorf("StripHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStringVal(t *testing.T) {
	if got := StringVal("hello"); got != "hello" {
		t.Errorf("StringVal(string) = %q, want \"hello\"", got)
	}
	if got := StringVal(false); got != "" {
		t.Errorf("StringVal(false) = %q, want \"\"", got)
	}
	if got := StringVal(nil); got != "" {
		t.Errorf("StringVal(nil) = %q, want \"\"", got)
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
