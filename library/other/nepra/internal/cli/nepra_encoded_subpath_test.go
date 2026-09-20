// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.

package cli

import (
	"strings"
	"testing"
)

// TestReplaceEncodedSubPath pins the fix for the multi-segment path param.
//
// The generated replacePathParam routes every path param through
// url.PathEscape, which is right for one segment and wrong for a param that
// IS a sub-path. MEASURED before the fix, the spec's own happy_args produced
//
//	/Standards/2020%2FPER%2520DISCOs%25202018-19.pdf
//
// for a real path of /Standards/2020/PER%20DISCOs%202018-19.pdf — the
// separator escaped to %2F and the spaces double-encoded. Every Performance
// Evaluation Report 404'd, which is the whole input surface of
// internal/nepraper.
func TestReplaceEncodedSubPath(t *testing.T) {
	const tmpl = "/Standards/{path}"

	t.Run("the spec's happy_args reaches the real published path", func(t *testing.T) {
		got, err := replaceEncodedSubPath(tmpl, "path", "2020/PER%20DISCOs%202018-19.pdf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "/Standards/2020/PER%20DISCOs%202018-19.pdf"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		// The two specific corruptions must be gone.
		if strings.Contains(got, "%2F") {
			t.Error("the path separator was escaped to %2F")
		}
		if strings.Contains(got, "%25") {
			t.Error("an already-encoded value was double-encoded")
		}
	})

	t.Run("the spec's example also works as written", func(t *testing.T) {
		got, err := replaceEncodedSubPath(tmpl, "path", "2023/PER-DISCO%20FY%202021-22%20final.pdf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "/Standards/2023/PER-DISCO%20FY%202021-22%20final.pdf"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("a trailing space survives as %20", func(t *testing.T) {
		// One published filename is load-bearing on a TRAILING SPACE. In the
		// encoded form the caller copies from `sources` that is a %20, and it
		// must reach the URL intact.
		got, err := replaceEncodedSubPath(tmpl, "path", "2019/PER%20of%20DISCOs%20.pdf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(got, "%20.pdf") {
			t.Errorf("the trailing space was lost: %q", got)
		}
	})

	t.Run("multiple segments are preserved", func(t *testing.T) {
		got, err := replaceEncodedSubPath(tmpl, "path", "a/b/c/d.pdf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := "/Standards/a/b/c/d.pdf"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

// TestReplaceEncodedSubPathRefusals checks the function REFUSES rather than
// repairs. A repaired value would be a guess, and a guessed URL that 404s is
// indistinguishable from a document NEPRA never published.
func TestReplaceEncodedSubPathRefusals(t *testing.T) {
	tests := []struct {
		name, value, wantIn string
	}{
		{"empty", "", "required"},
		{"whitespace only", "   ", "required"},
		{"raw space is not repaired", "2020/PER DISCOs 2018-19.pdf", "raw space"},
		{"absolute path", "/Standards/x.pdf", "must be relative"},
		{"parent traversal", "../etc/passwd", `must not contain ".."`},
		{"traversal mid-path", "2020/../../x.pdf", `must not contain ".."`},
		{"stray percent at end", "2020/PER%.pdf", "stray %"},
		{"percent with one hex digit", "2020/PER%2.pdf", "stray %"},
		{"percent with non-hex", "2020/PER%ZZ.pdf", "stray %"},
		{"bare percent sign", "2020/100%.pdf", "stray %"},
		{"non-ascii", "2020/PER x.pdf", "must be percent-encoded"},
		{"control character", "2020/PER\tx.pdf", "must be percent-encoded"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := replaceEncodedSubPath("/Standards/{path}", "path", tc.value)
			if err == nil {
				t.Fatalf("%q was accepted, producing %q; want a refusal", tc.value, got)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err, tc.wantIn)
			}
			if got != "" {
				t.Errorf("a refused value still returned a path: %q", got)
			}
		})
	}

	// A validly-encoded value that merely looks odd must still be accepted:
	// %2D is a real escape (both 2 and D are hex digits) for "-".
	if _, err := replaceEncodedSubPath("/Standards/{path}", "path", "2020/PER%2Dx.pdf"); err != nil {
		t.Errorf("%%2D is a valid escape but was refused: %v", err)
	}
}
