// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// flexInt64 decodes a JSON value the ElevenLabs API may serialize as either a
// number or a string (e.g. next_character_count_reset_unix), so the composite
// never panics on a shape difference.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		*f = flexInt64(i)
		return nil
	}
	fl, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("flexInt64: cannot parse %q", s)
	}
	if math.IsNaN(fl) || math.IsInf(fl, 0) || fl != math.Trunc(fl) || fl < -float64(1<<63) || fl >= float64(1<<63) {
		return fmt.Errorf("flexInt64: %q is not an integer in range", s)
	}
	*f = flexInt64(int64(fl))
	return nil
}

// subscriptionInfo is the subset of /v1/user/subscription voice-budget reads.
type subscriptionInfo struct {
	Tier                        string    `json:"tier"`
	Status                      string    `json:"status"`
	CharacterCount              flexInt64 `json:"character_count"`
	CharacterLimit              flexInt64 `json:"character_limit"`
	NextCharacterCountResetUnix flexInt64 `json:"next_character_count_reset_unix"`
	VoiceSlotsUsed              flexInt64 `json:"voice_slots_used"`
	VoiceLimit                  flexInt64 `json:"voice_limit"`
	ProfessionalVoiceLimit      flexInt64 `json:"professional_voice_limit"`
	CanExtendCharacterLimit     bool      `json:"can_extend_character_limit"`
}

// voiceBudget is the computed credit + voice-slot readout.
type voiceBudget struct {
	Tier                string   `json:"tier"`
	Status              string   `json:"status"`
	CharactersUsed      int64    `json:"characters_used"`
	CharacterLimit      int64    `json:"character_limit"`
	CharactersRemaining int64    `json:"characters_remaining"`
	PercentUsed         float64  `json:"percent_used"`
	NextResetUnix       int64    `json:"next_reset_unix,omitempty"`
	NextResetUTC        string   `json:"next_reset_utc,omitempty"`
	DaysUntilReset      *float64 `json:"days_until_reset,omitempty"`
	VoicesUsed          int64    `json:"voices_used"`
	VoiceLimit          int64    `json:"voice_limit"`
	VoiceSlotsRemaining int64    `json:"voice_slots_remaining"`
}

func newVoiceBudgetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "voice-budget",
		Short:       "Show remaining character credits, tier limits, and voice slots",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  elevenlabs-pp-cli voice-budget\n  elevenlabs-pp-cli voice-budget --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Dry-run: emit nothing (composite read-only contract).
			if flags.dryRun {
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true // A budget must reflect the current subscription.

			subData, err := c.Get("/v1/user/subscription", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			sub, err := decodeVoiceBudgetSubscription(subData)
			if err != nil {
				return fmt.Errorf("decoding subscription: %w", err)
			}

			budget := summarizeVoiceBudget(sub, time.Now().UTC())

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers, rows := voiceBudgetTable(budget)
				if err := flags.printTable(cmd, headers, rows); err != nil {
					return err
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), voiceBudgetDetails(budget))
				return err
			}

			return flags.printJSON(cmd, budget)
		},
	}
	return cmd
}

func decodeVoiceBudgetSubscription(data []byte) (subscriptionInfo, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return subscriptionInfo{}, err
	}
	for _, name := range []string{"character_count", "character_limit", "voice_slots_used", "voice_limit"} {
		raw, ok := fields[name]
		value := strings.TrimSpace(string(raw))
		if !ok || value == "null" || value == `""` {
			return subscriptionInfo{}, fmt.Errorf("missing %s counter", name)
		}
	}
	var sub subscriptionInfo
	if err := json.Unmarshal(data, &sub); err != nil {
		return subscriptionInfo{}, err
	}
	return sub, nil
}

func voiceBudgetTable(budget voiceBudget) ([]string, [][]string) {
	resetIn := "n/a"
	if budget.DaysUntilReset != nil {
		resetIn = fmt.Sprintf("%.1f days", *budget.DaysUntilReset)
	}
	return []string{"TIER", "CHARS USED", "REMAINING", "USED %", "RESET IN", "VOICES"}, [][]string{{
		budget.Tier,
		strconv.FormatInt(budget.CharactersUsed, 10),
		strconv.FormatInt(budget.CharactersRemaining, 10),
		fmt.Sprintf("%.1f%%", budget.PercentUsed),
		resetIn,
		fmt.Sprintf("%d/%d", budget.VoicesUsed, budget.VoiceLimit),
	}}
}

func voiceBudgetDetails(budget voiceBudget) string {
	resetUTC := budget.NextResetUTC
	if resetUTC == "" {
		resetUTC = "n/a"
	}
	return fmt.Sprintf("Reset UTC: %s\nVoice slots left: %d\n", resetUTC, budget.VoiceSlotsRemaining)
}

// summarizeVoiceBudget derives remaining credits, percent used, reset timing,
// and voice-slot headroom from the provider's subscription counters. The
// paginated voice list includes voices that do not occupy a paid voice slot.
func summarizeVoiceBudget(sub subscriptionInfo, now time.Time) voiceBudget {
	used := int64(sub.CharacterCount)
	limit := int64(sub.CharacterLimit)
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	percent := 0.0
	if limit > 0 {
		percent = float64(used) / float64(limit) * 100
	}

	voiceLimit := int64(sub.VoiceLimit)
	voicesUsed := int64(sub.VoiceSlotsUsed)
	slots := voiceLimit - voicesUsed
	if slots < 0 {
		slots = 0
	}

	budget := voiceBudget{
		Tier:                sub.Tier,
		Status:              sub.Status,
		CharactersUsed:      used,
		CharacterLimit:      limit,
		CharactersRemaining: remaining,
		PercentUsed:         percent,
		VoicesUsed:          voicesUsed,
		VoiceLimit:          voiceLimit,
		VoiceSlotsRemaining: slots,
	}

	if reset := int64(sub.NextCharacterCountResetUnix); reset > 0 {
		budget.NextResetUnix = reset
		resetTime := time.Unix(reset, 0).UTC()
		budget.NextResetUTC = resetTime.Format(time.RFC3339)
		days := resetTime.Sub(now).Hours() / 24
		if days < 0 {
			days = 0
		}
		budget.DaysUntilReset = &days
	}

	return budget
}
