// Copyright 2026 QuantumGlitch and contributors. Licensed under Apache-2.0. See LICENSE.

package mobalytics

// TierRow is one champion / role / skill-level entry on the tier list.
type TierRow struct {
	Slug       string `json:"slug"`
	Role       string `json:"role"`
	SkillLevel string `json:"skillLevel"`
	Tier       string `json:"tier"`
}

// ChampionStats is the rolled-up patch metrics for a single champion.
type ChampionStats struct {
	Slug            string  `json:"slug"`
	Tier            string  `json:"tier,omitempty"`
	WinRate         float64 `json:"winRate,omitempty"`
	PickRate        float64 `json:"pickRate,omitempty"`
	BanRate         float64 `json:"banRate,omitempty"`
	TotalMatchCount int64   `json:"totalMatchCount,omitempty"`
	Patch           string  `json:"patch,omitempty"`
}

// ItemBlock represents one Mobalytics-recommended item group.
type ItemBlock struct {
	Type    string `json:"type"`
	Items   []int  `json:"items"`
	Slots   []int  `json:"slots,omitempty"`
	TimeToT int    `json:"timeToTarget,omitempty"`
}

// PerkSelection holds rune ids and styles.
type PerkSelection struct {
	IDs      []int `json:"ids"`
	Style    int   `json:"style"`
	SubStyle int   `json:"subStyle"`
}

// ChampionBuild is one recommended build (most popular, optional, etc.).
type ChampionBuild struct {
	ID            string        `json:"id"`
	Type          string        `json:"type"`
	Queue         string        `json:"queue"`
	Role          string        `json:"role"`
	VsChampion    string        `json:"vsChampion,omitempty"`
	Wins          int64         `json:"wins"`
	MatchCount    int64         `json:"matchCount"`
	WinRate       float64       `json:"winRate"`
	Perks         PerkSelection `json:"perks"`
	Items         []ItemBlock   `json:"items"`
	SkillOrder    []int         `json:"skillOrder,omitempty"`
	SkillMaxOrder []int         `json:"skillMaxOrder,omitempty"`
	Spells        []int         `json:"spells,omitempty"`
	Patch         string        `json:"patch,omitempty"`
}

// CounterRow describes one matchup vs another champion.
type CounterRow struct {
	OwnSlug      string  `json:"ownSlug"`
	OpponentSlug string  `json:"opponentSlug"`
	Role         string  `json:"role"`
	Wins         int64   `json:"wins"`
	Losses       int64   `json:"losses"`
	Sample       int64   `json:"sample"`
	WinRate      float64 `json:"winRate"`
	// MatchupDelta is Mobalytics's signed "edge" metric; positive means
	// the row owner wins more than baseline against the opponent.
	MatchupDelta float64 `json:"matchupDelta"`
}

// SynergyRow is a same-team teammate WR.
type SynergyRow struct {
	OwnSlug     string  `json:"ownSlug"`
	PartnerSlug string  `json:"partnerSlug"`
	PartnerRole string  `json:"partnerRole"`
	WinRate     float64 `json:"winRate"`
}

// ComboStep is one step in a combo sequence. Mobalytics models each step
// as a list of move tokens that fire together (typically one token like
// "Q" or "AA", sometimes two like "Q" + "Flash").
type ComboStep struct {
	Items []string `json:"items"`
}

// Combo is one named Mobalytics combo for a champion, with its move sequence,
// difficulty tag, prose description, and the video URL Mobalytics renders.
type Combo struct {
	Slug             string      `json:"slug"`
	ChampionSlug     string      `json:"championSlug"`
	Difficulty       string      `json:"difficulty,omitempty"`
	Tags             []string    `json:"tags,omitempty"`
	Sequence         []ComboStep `json:"sequence"`
	ShortDescription string      `json:"shortDescription,omitempty"`
	ExecutionText    string      `json:"executionText,omitempty"`
	Notes            string      `json:"notes,omitempty"`
	VideoURL         string      `json:"videoUrl,omitempty"`
	ThumbnailID      string      `json:"thumbnailId,omitempty"`
}

// ChampionBuildPage is the joined view returned to callers of `champion build`.
type ChampionBuildPage struct {
	Slug      string          `json:"slug"`
	Stats     ChampionStats   `json:"stats"`
	Builds    []ChampionBuild `json:"builds"`
	Counters  []CounterRow    `json:"counters,omitempty"`
	Synergies []SynergyRow    `json:"synergies,omitempty"`
}
