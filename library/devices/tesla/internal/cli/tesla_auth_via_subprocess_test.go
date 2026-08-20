package cli

import (
	"strings"
	"testing"
)

func TestParseTeslaAuthOutput_HappyPath(t *testing.T) {
	// Canonical tesla_auth Display output (Rust `Display for Tokens`).
	raw := `
--------------------------------- ACCESS TOKEN ---------------------------------

eyJacc.payload.sig

--------------------------------- REFRESH TOKEN --------------------------------

eyJref.payload.sig

----------------------------------- VALID FOR ----------------------------------

8h 0m 0s
`
	access, refresh, err := parseTeslaAuthOutput(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if access != "eyJacc.payload.sig" {
		t.Errorf("access: got %q", access)
	}
	if refresh != "eyJref.payload.sig" {
		t.Errorf("refresh: got %q", refresh)
	}
}

func TestParseTeslaAuthOutput_VariableSeparatorLength(t *testing.T) {
	// Future tesla_auth versions may change separator widths; the regex is
	// generous about dash count. Verify shorter and longer separators still parse.
	raw := `
--- ACCESS TOKEN ---
eyJacc1.b.c

--------------------- REFRESH TOKEN ---------------------
eyJref1.b.c
`
	access, refresh, err := parseTeslaAuthOutput(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if access != "eyJacc1.b.c" || refresh != "eyJref1.b.c" {
		t.Errorf("got access=%q refresh=%q", access, refresh)
	}
}

func TestParseTeslaAuthOutput_MissingRefresh(t *testing.T) {
	raw := `--- ACCESS TOKEN ---

eyJacc.b.c
`
	_, _, err := parseTeslaAuthOutput(raw)
	if err == nil || !strings.Contains(err.Error(), "REFRESH TOKEN") {
		t.Errorf("expected missing-refresh error, got %v", err)
	}
}

func TestParseTeslaAuthOutput_MissingAccess(t *testing.T) {
	raw := `--- REFRESH TOKEN ---

eyJref.b.c
`
	_, _, err := parseTeslaAuthOutput(raw)
	if err == nil || !strings.Contains(err.Error(), "ACCESS TOKEN") {
		t.Errorf("expected missing-access error, got %v", err)
	}
}

func TestParseTeslaAuthOutput_Empty(t *testing.T) {
	_, _, err := parseTeslaAuthOutput("")
	if err == nil || !strings.Contains(err.Error(), "no output") {
		t.Errorf("expected empty-output error, got %v", err)
	}
}

func TestParseTeslaAuthOutput_WithDebugLogLines(t *testing.T) {
	// --debug mode interleaves log lines; the regex should still extract.
	raw := `
DEBUG tesla_auth navigation: https://auth.tesla.com/oauth2/v3/authorize?...
INFO  reqwest::connect: connecting to auth.tesla.com:443

--------------------------------- ACCESS TOKEN ---------------------------------

eyJacc.with.debug

--------------------------------- REFRESH TOKEN --------------------------------

eyJref.with.debug

DEBUG tesla_auth: done
`
	access, refresh, err := parseTeslaAuthOutput(raw)
	if err != nil {
		t.Fatalf("parse with debug logs: %v", err)
	}
	if access != "eyJacc.with.debug" || refresh != "eyJref.with.debug" {
		t.Errorf("got access=%q refresh=%q", access, refresh)
	}
}

func TestDetectTeslaAuthBinary_NotPresent(t *testing.T) {
	// Point $PATH at an empty dir; tesla_auth shouldn't resolve.
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)
	if got := detectTeslaAuthBinary(); got != "" {
		t.Errorf("detect: got %q, expected empty (PATH was %s)", got, tmp)
	}
}

func TestTruncateOutput(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello", "hello"},
		{strings.Repeat("a", 200), strings.Repeat("a", 200)},
		{strings.Repeat("a", 250), strings.Repeat("a", 200) + "..."},
	}
	for _, c := range cases {
		if got := truncateOutput(c.in, 200); got != c.want {
			t.Errorf("truncateOutput: in_len=%d got_len=%d want_len=%d", len(c.in), len(got), len(c.want))
		}
	}
}
