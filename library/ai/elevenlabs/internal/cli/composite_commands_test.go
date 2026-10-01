// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
	for _, raw := range []string{`"12.5"`, `"NaN"`, `"9223372036854775808"`} {
		if err := json.Unmarshal([]byte(raw), &f); err == nil {
			t.Fatalf("accepted non-integral or out-of-range counter %s", raw)
		}
	}
}

// Subscription decodes whether reset-unix arrives as a number or a string.
func TestSubscriptionDecodeShapes(t *testing.T) {
	raws := []string{
		`{"tier":"creator","character_count":12000,"character_limit":100000,"next_character_count_reset_unix":1717000000,"voice_slots_used":12,"voice_limit":30}`,
		`{"tier":"creator","character_count":"12000","character_limit":"100000","next_character_count_reset_unix":"1717000000","voice_slots_used":"12","voice_limit":"30"}`,
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
		if int64(sub.VoiceSlotsUsed) != 12 {
			t.Fatalf("case %d voice slots wrong: %d", i, int64(sub.VoiceSlotsUsed))
		}
	}
}

func TestVoiceBudgetRejectsMissingSubscriptionCounters(t *testing.T) {
	for _, raw := range []string{
		`{"character_limit":100,"voice_slots_used":0,"voice_limit":10}`,
		`{"character_count":0,"character_limit":null,"voice_slots_used":0,"voice_limit":10}`,
		`{"character_count":0,"character_limit":100,"voice_limit":10}`,
		`{"character_count":0,"character_limit":100,"voice_slots_used":"","voice_limit":10}`,
	} {
		if _, err := decodeVoiceBudgetSubscription([]byte(raw)); err == nil {
			t.Fatalf("accepted incomplete subscription %s", raw)
		}
	}
	if _, err := decodeVoiceBudgetSubscription([]byte(`{"character_count":0,"character_limit":100,"voice_slots_used":0,"voice_limit":10}`)); err != nil {
		t.Fatalf("valid zero counters rejected: %v", err)
	}
}

func TestVoiceBudgetHumanReadoutShowsResetAndRemainingSlots(t *testing.T) {
	resetIn := 2.5
	budget := voiceBudget{NextResetUTC: "2026-10-04T00:00:00Z", DaysUntilReset: &resetIn, VoicesUsed: 12, VoiceLimit: 30, VoiceSlotsRemaining: 18}
	headers, rows := voiceBudgetTable(budget)
	columns := make(map[string]string, len(headers))
	for i, header := range headers {
		columns[header] = rows[0][i]
	}
	if len(headers) != 6 || columns["RESET IN"] != "2.5 days" {
		t.Fatalf("human voice budget table = %v, %v", headers, columns)
	}
	details := voiceBudgetDetails(budget)
	if !strings.Contains(details, "Reset UTC: "+budget.NextResetUTC) || !strings.Contains(details, "Voice slots left: 18") {
		t.Fatalf("human voice budget details = %q", details)
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
		VoiceSlotsUsed:              flexInt64(12),
		VoiceLimit:                  flexInt64(30),
	}
	b := summarizeVoiceBudget(sub, now)

	if b.CharactersRemaining != 70000 {
		t.Fatalf("remaining = %d, want 70000", b.CharactersRemaining)
	}
	if math.Abs(b.PercentUsed-30.0) > 1e-9 {
		t.Fatalf("percent = %v, want 30.0", b.PercentUsed)
	}
	if b.VoiceSlotsRemaining != 18 {
		t.Fatalf("voice slots = %d, want 18", b.VoiceSlotsRemaining)
	}
	if b.VoicesUsed != 12 {
		t.Fatalf("voice slots used = %d, want 12", b.VoicesUsed)
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
		VoiceSlotsUsed: flexInt64(9),
		VoiceLimit:     flexInt64(5),
	}
	b := summarizeVoiceBudget(sub, time.Now().UTC())
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
		VoiceSlotsUsed:              flexInt64(1),
		NextCharacterCountResetUnix: flexInt64(reset),
	}
	b := summarizeVoiceBudget(sub, now)
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

func TestVoiceBudgetReadsSubscriptionCounterOnly(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/v1/user/subscription" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fmt.Sprintf(`{"tier":"creator","status":"active","character_count":30000,"character_limit":100000,"voice_slots_used":%d,"voice_limit":30}`, 11+requests)))
	}))
	defer srv.Close()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("ELEVENLABS_CONFIG", filepath.Join(home, "config.toml"))
	t.Setenv("ELEVENLABS_API_KEY", "test-credential")
	t.Setenv("ELEVENLABS_BASE_URL", srv.URL)

	run := func() voiceBudget {
		root := RootCmd()
		var out, stderr bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&stderr)
		root.SetArgs([]string{"voice-budget", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("voice-budget failed: %v (%s)", err, stderr.String())
		}
		var got voiceBudget
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("decode output: %v (%s)", err, out.String())
		}
		return got
	}
	first, second := run(), run()
	if requests != 2 {
		t.Fatalf("requests = %d, want one fresh subscription GET per budget read", requests)
	}
	if first.VoicesUsed != 12 || second.VoicesUsed != 13 || second.VoiceSlotsRemaining != 17 || second.CharactersRemaining != 70000 {
		t.Fatalf("budgets = %+v then %+v; want changed subscription counter without a voice-list request", first, second)
	}
}
