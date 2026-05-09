// Copyright 2026 addisonk. Licensed under Apache-2.0. See LICENSE.

package cli

// mmrLadder maps tier-bracket names to lower-bound MMR.
// Approximate; varies by playlist and season, but stable within ~50 MMR
// for casual purposes. Source: community-tracked thresholds via
// rocketleague.tracker.network 2024-2025 distributions.
var mmrLadder = map[string]int{
	"Bronze 1": 0, "Bronze 2": 80, "Bronze 3": 160,
	"Silver 1": 240, "Silver 2": 320, "Silver 3": 400,
	"Gold 1": 480, "Gold 2": 560, "Gold 3": 640,
	"Platinum 1": 720, "Platinum 2": 800, "Platinum 3": 880,
	"Diamond 1": 960, "Diamond 2": 1040, "Diamond 3": 1120,
	"Champion 1": 1200, "Champion 2": 1290, "Champion 3": 1380,
	"Grand Champion 1": 1490, "Grand Champion 2": 1620, "Grand Champion 3": 1750,
	"Supersonic Legend": 1860,
}

// mmrAliases maps short names (used in --claimed-rank) to canonical tier names.
var mmrAliases = map[string]string{
	"B1": "Bronze 1", "B2": "Bronze 2", "B3": "Bronze 3",
	"S1": "Silver 1", "S2": "Silver 2", "S3": "Silver 3",
	"G1": "Gold 1", "G2": "Gold 2", "G3": "Gold 3",
	"P1": "Platinum 1", "P2": "Platinum 2", "P3": "Platinum 3",
	"D1": "Diamond 1", "D2": "Diamond 2", "D3": "Diamond 3",
	"C1": "Champion 1", "C2": "Champion 2", "C3": "Champion 3",
	"GC1": "Grand Champion 1", "GC2": "Grand Champion 2", "GC3": "Grand Champion 3",
	"GC": "Grand Champion 1", "SSL": "Supersonic Legend",
}

// resolveTier normalizes user input to a canonical tier name. Returns ("", false) if unknown.
func resolveTier(input string) (string, bool) {
	if _, ok := mmrLadder[input]; ok {
		return input, true
	}
	if canonical, ok := mmrAliases[input]; ok {
		return canonical, true
	}
	return "", false
}

// tierLowerBound returns the canonical lower-bound MMR for a tier, or 0 if unknown.
func tierLowerBound(tier string) int {
	if v, ok := mmrLadder[tier]; ok {
		return v
	}
	return 0
}

// nextTierLowerBound returns the lower-bound MMR for the tier ABOVE the given
// MMR, plus the canonical name of that tier. Returns ("", 0) when the player
// is at SSL (no higher tier exists in the ladder).
func nextTierLowerBound(currentMMR int) (string, int) {
	type bracket struct {
		name string
		mmr  int
	}
	ordered := []bracket{
		{"Bronze 1", 0}, {"Bronze 2", 80}, {"Bronze 3", 160},
		{"Silver 1", 240}, {"Silver 2", 320}, {"Silver 3", 400},
		{"Gold 1", 480}, {"Gold 2", 560}, {"Gold 3", 640},
		{"Platinum 1", 720}, {"Platinum 2", 800}, {"Platinum 3", 880},
		{"Diamond 1", 960}, {"Diamond 2", 1040}, {"Diamond 3", 1120},
		{"Champion 1", 1200}, {"Champion 2", 1290}, {"Champion 3", 1380},
		{"Grand Champion 1", 1490}, {"Grand Champion 2", 1620}, {"Grand Champion 3", 1750},
		{"Supersonic Legend", 1860},
	}
	for _, b := range ordered {
		if b.mmr > currentMMR {
			return b.name, b.mmr
		}
	}
	return "", 0
}

// tierFromMMR returns the canonical tier name whose lower bound is the
// largest value <= mmr. For mmr below Bronze 1 returns "Bronze 1".
func tierFromMMR(mmr int) string {
	type bracket struct {
		name string
		mmr  int
	}
	ordered := []bracket{
		{"Supersonic Legend", 1860},
		{"Grand Champion 3", 1750}, {"Grand Champion 2", 1620}, {"Grand Champion 1", 1490},
		{"Champion 3", 1380}, {"Champion 2", 1290}, {"Champion 1", 1200},
		{"Diamond 3", 1120}, {"Diamond 2", 1040}, {"Diamond 1", 960},
		{"Platinum 3", 880}, {"Platinum 2", 800}, {"Platinum 1", 720},
		{"Gold 3", 640}, {"Gold 2", 560}, {"Gold 1", 480},
		{"Silver 3", 400}, {"Silver 2", 320}, {"Silver 1", 240},
		{"Bronze 3", 160}, {"Bronze 2", 80}, {"Bronze 1", 0},
	}
	for _, b := range ordered {
		if mmr >= b.mmr {
			return b.name
		}
	}
	return "Bronze 1"
}
