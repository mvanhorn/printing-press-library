// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestPlatformShapeVerdicts(t *testing.T) {
	// IG reel over the 90s cap → fail.
	s := Shot{Platform: "instagram", Format: "reel", Params: map[string]any{"duration": 120.0}}
	vs := platformShapeVerdicts(s, 0)
	if !hasVerdict(vs, "platform-duration", "fail") {
		t.Fatalf("expected duration fail, got %#v", vs)
	}

	// Within cap → pass.
	s = Shot{Platform: "instagram", Format: "reel", Params: map[string]any{"duration": 30.0}}
	vs = platformShapeVerdicts(s, 0)
	if !hasVerdict(vs, "platform-shape", "pass") {
		t.Fatalf("expected pass, got %#v", vs)
	}

	// Unknown platform → fail.
	s = Shot{Platform: "myspace"}
	vs = platformShapeVerdicts(s, 0)
	if !hasVerdict(vs, "platform-shape", "fail") {
		t.Fatalf("expected unknown-platform fail, got %#v", vs)
	}

	// Aspect mismatch → warn.
	s = Shot{Platform: "instagram", Format: "feed", AspectRatio: "16:9"}
	vs = platformShapeVerdicts(s, 0)
	if !hasVerdict(vs, "platform-aspect", "warn") {
		t.Fatalf("expected aspect warn, got %#v", vs)
	}
}

func TestPromptSafetyVerdict(t *testing.T) {
	if promptSafetyVerdict(Shot{Prompt: ""}, 0).Verdict != "fail" {
		t.Error("empty prompt should fail")
	}
	if promptSafetyVerdict(Shot{Prompt: "a clean hero"}, 0).Verdict != "pass" {
		t.Error("clean prompt should pass")
	}
	if promptSafetyVerdict(Shot{Prompt: "nsfw stuff"}, 0).Verdict != "warn" {
		t.Error("sensitive token should warn")
	}
}

func TestWorstVerdict(t *testing.T) {
	if worstVerdict([]qaVerdict{{Verdict: "pass"}, {Verdict: "warn"}}) != "warn" {
		t.Error("warn should beat pass")
	}
	if worstVerdict([]qaVerdict{{Verdict: "warn"}, {Verdict: "fail"}}) != "fail" {
		t.Error("fail should win")
	}
	if worstVerdict([]qaVerdict{{Verdict: "pass"}}) != "pass" {
		t.Error("all pass")
	}
}

func hasVerdict(vs []qaVerdict, check, verdict string) bool {
	for _, v := range vs {
		if v.Check == check && v.Verdict == verdict {
			return true
		}
	}
	return false
}
