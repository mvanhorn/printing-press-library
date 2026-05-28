// Copyright 2026 Dave Fano and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestURLPathEscapeEncodesSpecialCharacters(t *testing.T) {
	t.Parallel()
	got := urlPathEscape("abc?foo#bar&baz%2Fqux")
	for _, forbidden := range []string{"?", "#", "&"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("urlPathEscape() = %q, still contains %q", got, forbidden)
		}
	}
	for _, want := range []string{"%3F", "%23", "%26", "%25"} {
		if !strings.Contains(got, want) {
			t.Fatalf("urlPathEscape() = %q, want encoded %s", got, want)
		}
	}
}

func TestRenderedImageNeedleUsesSafeJSLiteral(t *testing.T) {
	t.Parallel()
	got := jsStringLiteral("/job\u2028id/0_0")
	if strings.Contains(got, "\u2028") {
		t.Fatalf("jsStringLiteral() left raw JavaScript line separator in %q", got)
	}
	if !strings.Contains(got, "\\u2028") {
		t.Fatalf("jsStringLiteral() = %q, want escaped line separator", got)
	}
}
