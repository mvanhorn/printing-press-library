package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/normalizecfg"
)

func TestApplyAttributeRules(t *testing.T) {
	rules := []normalizecfg.Rule{{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}}}
	got := applyAttributeRules("vip experience", rules)
	if got["access_class"] != "vip" {
		t.Errorf("got %v, want access_class=vip", got)
	}
}

func TestApplyAttributeRulesEmpty(t *testing.T) {
	// No rules -> empty map.
	got := applyAttributeRules("vip experience", nil)
	if len(got) != 0 {
		t.Errorf("got %v, want empty map", got)
	}
}

func TestApplyAttributeRulesSkipsBadRegex(t *testing.T) {
	// A rule with an uncompilable pattern is skipped, not fatal.
	rules := []normalizecfg.Rule{
		{Match: `(unclosed`, Set: map[string]string{"access_class": "vip"}},
		{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}},
	}
	got := applyAttributeRules("vip experience", rules)
	if got["access_class"] != "vip" {
		t.Errorf("got %v, want access_class=vip (bad rule skipped, good rule applied)", got)
	}
}

func TestApplyAttributeRulesLaterOverrides(t *testing.T) {
	// Later matching rules override earlier ones for the same key.
	rules := []normalizecfg.Rule{
		{Match: `(?i)pass`, Set: map[string]string{"access_class": "ga"}},
		{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}},
	}
	got := applyAttributeRules("vip pass", rules)
	if got["access_class"] != "vip" {
		t.Errorf("got %v, want access_class=vip (later rule wins)", got)
	}
}

func TestValidateRulePromotion(t *testing.T) {
	cached := map[string]map[string]string{
		"vip lounge":     {"access_class": "vip"},
		"vip experience": {"access_class": "vip"},
		"general admit":  {"access_class": "ga"},
	}
	good := normalizecfg.Rule{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}}
	if !validateRule(good, cached) {
		t.Error("good rule should validate (matches both vip rows, no false positive on ga)")
	}
	bad := normalizecfg.Rule{Match: `(?i)admit|vip`, Set: map[string]string{"access_class": "vip"}}
	if validateRule(bad, cached) {
		t.Error("bad rule should fail (false positive on the ga row)")
	}
}

func TestValidateRuleNoMatchFails(t *testing.T) {
	cached := map[string]map[string]string{
		"general admit": {"access_class": "ga"},
	}
	// Matches zero cached names -> not validated.
	r := normalizecfg.Rule{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}}
	if validateRule(r, cached) {
		t.Error("rule matching zero cached names must not validate")
	}
}

func TestValidateRuleBadRegexFails(t *testing.T) {
	cached := map[string]map[string]string{
		"vip lounge": {"access_class": "vip"},
	}
	r := normalizecfg.Rule{Match: `(unclosed`, Set: map[string]string{"access_class": "vip"}}
	if validateRule(r, cached) {
		t.Error("rule with uncompilable regex must not validate")
	}
}

// TestCompileRulesWarnsAndSkips verifies the compile-once cache skips an
// uncompilable rule, writes a diagnostic to the warn writer, and still applies
// the good rules — the defense-in-depth half of the silent-skip fix (F1/F9).
func TestCompileRulesWarnsAndSkips(t *testing.T) {
	var warn bytes.Buffer
	rules := []normalizecfg.Rule{
		{Match: `(unclosed`, Set: map[string]string{"access_class": "vip"}},
		{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}},
	}
	crules := compileRules("ticket_type", rules, &warn)
	if len(crules) != 1 {
		t.Fatalf("want 1 compiled rule (bad one skipped), got %d", len(crules))
	}
	if !strings.Contains(warn.String(), "skipping uncompilable match") {
		t.Errorf("expected a warning about the skipped rule, got %q", warn.String())
	}
	if got := applyCompiledRules("vip experience", crules); got["access_class"] != "vip" {
		t.Errorf("good rule should still apply, got %v", got)
	}
}

// TestCompileRulesNilWarnSilent verifies a nil warn writer silences the
// diagnostic without panicking (the applyAttributeRules convenience path).
func TestCompileRulesNilWarnSilent(t *testing.T) {
	rules := []normalizecfg.Rule{{Match: `(unclosed`, Set: map[string]string{"access_class": "vip"}}}
	crules := compileRules("", rules, nil)
	if len(crules) != 0 {
		t.Errorf("uncompilable rule should be skipped, got %d compiled", len(crules))
	}
}

// TestFuzzyThresholdResolution verifies the configurable fuzzy threshold (F6):
// an unset/zero value falls back to the default, an in-range value is used
// verbatim, and an out-of-range value is treated as unset (clamped to default)
// so a stray 0 or >1 cannot collapse or disable clustering.
func TestFuzzyThresholdResolution(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want float64
	}{
		{"unset zero -> default", 0, defaultFuzzyThreshold},
		{"in range used verbatim", 0.85, 0.85},
		{"upper bound 1 allowed", 1.0, 1.0},
		{"above 1 -> default", 1.5, defaultFuzzyThreshold},
		{"negative -> default", -0.5, defaultFuzzyThreshold},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyOpts{FuzzyThreshold: c.in}.fuzzyThreshold()
			if got != c.want {
				t.Errorf("fuzzyThreshold(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestPromoteRules(t *testing.T) {
	cached := map[string]map[string]string{
		"vip lounge":     {"access_class": "vip"},
		"vip experience": {"access_class": "vip"},
		"general admit":  {"access_class": "ga"},
	}
	candidates := []normalizecfg.Rule{
		{Match: `(?i)\bvip\b`, Set: map[string]string{"access_class": "vip"}},   // good
		{Match: `(?i)admit|vip`, Set: map[string]string{"access_class": "vip"}}, // false positive on ga
		{Match: `(?i)\bga\b`, Set: map[string]string{"access_class": "ga"}},     // matches nothing -> dropped
	}
	got := promoteRules(candidates, cached)
	if len(got) != 1 {
		t.Fatalf("want 1 promoted rule, got %d: %+v", len(got), got)
	}
	if got[0].Match != `(?i)\bvip\b` {
		t.Errorf("promoted wrong rule: %+v", got[0])
	}
}
