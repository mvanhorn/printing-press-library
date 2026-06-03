// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestFlexInt64Unmarshal(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{`"1717000000"`, 1717000000},
		{`42`, 42},
		{`""`, 0},
		{`null`, 0},
		{`"12.0"`, 12},
	}
	for _, tc := range cases {
		var f flexInt64
		if err := json.Unmarshal([]byte(tc.in), &f); err != nil {
			t.Fatalf("unmarshal %s: %v", tc.in, err)
		}
		if int64(f) != tc.want {
			t.Fatalf("flexInt64(%s) = %d, want %d", tc.in, int64(f), tc.want)
		}
	}
	var f flexInt64
	if err := json.Unmarshal([]byte(`"nope"`), &f); err == nil {
		t.Fatal("expected error for non-numeric string")
	}
}

// Subscription decodes whether reset-unix arrives as a number or a string.
func TestSubscriptionDecodeShapes(t *testing.T) {
	raws := []string{
		`{"tier":"creator","character_count":12000,"character_limit":100000,"next_character_count_reset_unix":1717000000,"voice_limit":30}`,
		`{"tier":"creator","character_count":"12000","character_limit":"100000","next_character_count_reset_unix":"1717000000","voice_limit":"30"}`,
	}
	for i, raw := range raws {
		var sub subscriptionInfo
		if err := json.Unmarshal([]byte(raw), &sub); err != nil {
			t.Fatalf("case %d decode: %v", i, err)
		}
		if int64(sub.CharacterCount) != 12000 || int64(sub.CharacterLimit) != 100000 {
			t.Fatalf("case %d char counts wrong: %d/%d", i, int64(sub.CharacterCount), int64(sub.CharacterLimit))
		}
		if int64(sub.NextCharacterCountResetUnix) != 1717000000 {
			t.Fatalf("case %d reset wrong: %d", i, int64(sub.NextCharacterCountResetUnix))
		}
	}
}

func TestCountVoices(t *testing.T) {
	if n := countVoices([]byte(`{"voices":[{"voice_id":"a"},{"voice_id":"b"}]}`)); n != 2 {
		t.Fatalf("wrapped count = %d, want 2", n)
	}
	if n := countVoices([]byte(`[{"voice_id":"a"}]`)); n != 1 {
		t.Fatalf("bare array count = %d, want 1", n)
	}
	if n := countVoices([]byte(`{"voices":[]}`)); n != 0 {
		t.Fatalf("empty count = %d, want 0", n)
	}
}

func TestSummarizeVoiceBudget(t *testing.T) {
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	reset := now.AddDate(0, 0, 10).Unix() // 10 days out
	sub := subscriptionInfo{
		Tier:                        "creator",
		Status:                      "active",
		CharacterCount:              flexInt64(30000),
		CharacterLimit:              flexInt64(100000),
		NextCharacterCountResetUnix: flexInt64(reset),
		VoiceLimit:                  flexInt64(30),
	}
	b := summarizeVoiceBudget(sub, 12, now)

	if b.CharactersRemaining != 70000 {
		t.Fatalf("remaining = %d, want 70000", b.CharactersRemaining)
	}
	if math.Abs(b.PercentUsed-30.0) > 1e-9 {
		t.Fatalf("percent = %v, want 30.0", b.PercentUsed)
	}
	if b.VoiceSlotsRemaining != 18 {
		t.Fatalf("voice slots = %d, want 18", b.VoiceSlotsRemaining)
	}
	if b.DaysUntilReset == nil || math.Abs(*b.DaysUntilReset-10.0) > 0.01 {
		t.Fatalf("days until reset = %v, want ~10", b.DaysUntilReset)
	}
	if b.NextResetUTC == "" {
		t.Fatal("expected a formatted reset timestamp")
	}
}

func TestSummarizeVoiceBudgetEdges(t *testing.T) {
	// Over limit should clamp remaining at 0 and omit reset when unix is 0.
	sub := subscriptionInfo{
		CharacterCount: flexInt64(120000),
		CharacterLimit: flexInt64(100000),
		VoiceLimit:     flexInt64(5),
	}
	b := summarizeVoiceBudget(sub, 9, time.Now().UTC())
	if b.CharactersRemaining != 0 {
		t.Fatalf("over-limit remaining = %d, want 0", b.CharactersRemaining)
	}
	if b.VoiceSlotsRemaining != 0 {
		t.Fatalf("over voice limit slots = %d, want 0", b.VoiceSlotsRemaining)
	}
	if b.DaysUntilReset != nil {
		t.Fatal("reset unset should yield nil days-until-reset")
	}
}

func TestSummarizeVoiceBudgetStaleReset(t *testing.T) {
	// A reset timestamp in the past (subscription not yet refreshed) should
	// clamp days-until-reset to 0 rather than render a negative value.
	now := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)
	reset := now.AddDate(0, 0, -2).Unix() // 2 days in the past
	sub := subscriptionInfo{
		CharacterCount:              flexInt64(10000),
		CharacterLimit:              flexInt64(100000),
		VoiceLimit:                  flexInt64(10),
		NextCharacterCountResetUnix: flexInt64(reset),
	}
	b := summarizeVoiceBudget(sub, 1, now)
	if b.DaysUntilReset == nil {
		t.Fatal("stale reset should still set days-until-reset")
	}
	if *b.DaysUntilReset != 0 {
		t.Fatalf("stale reset days = %v, want 0 (clamped)", *b.DaysUntilReset)
	}
}

// Dry-run contract: returns before any network call and emits nothing.
func TestVoiceBudgetDryRunEmitsNothing(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newVoiceBudgetCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run execute: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("dry-run should emit nothing, got %q", out.String())
	}
}
