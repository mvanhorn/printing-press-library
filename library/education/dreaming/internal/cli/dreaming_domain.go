// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

// Dreaming domain helpers shared by the hand-built commands (roadmap, next,
// diet, plan, transcript, concordance). Not generated — safe to edit.

package cli

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/education/dreaming/internal/store"
)

// RoadmapLevel is one rung of the Dreaming comprehensible-input fluency ladder.
// Hour thresholds are the community-stable convention (see research brief).
type RoadmapLevel struct {
	Level int
	Hours float64
	Note  string
}

var roadmapLevels = []RoadmapLevel{
	{1, 0, "Start — Superbeginner content"},
	{2, 50, "Recognize common words and basic phrases"},
	{3, 150, "Follow simple stories and adapted beginner content"},
	{4, 300, "Understand more; speaking becomes optional"},
	{5, 600, "Understand most native content; speaking recommended; reading introduced"},
	{6, 1000, "Understand patient native speakers across most contexts"},
	{7, 1500, "Native-like for practical purposes"},
}

// levelForHours returns the current roadmap level (1-7) for a cumulative-hours total.
func levelForHours(hours float64) int {
	lvl := 1
	for _, r := range roadmapLevels {
		if hours >= r.Hours {
			lvl = r.Level
		}
	}
	return lvl
}

// nextLevel returns the next rung above the given level, or false if at the top.
func nextLevel(level int) (RoadmapLevel, bool) {
	for _, r := range roadmapLevels {
		if r.Level == level+1 {
			return r, true
		}
	}
	return RoadmapLevel{}, false
}

// videoLevelBands maps a roadmap level to the catalog difficulty tiers that are
// comprehensible-but-challenging at that level. Used by `next`.
func videoLevelBands(level int) []string {
	switch {
	case level <= 1:
		return []string{"superbeginner"}
	case level == 2:
		return []string{"superbeginner", "beginner"}
	case level == 3:
		return []string{"beginner"}
	case level == 4:
		return []string{"beginner", "intermediate"}
	case level == 5:
		return []string{"intermediate"}
	default:
		return []string{"intermediate", "advanced"}
	}
}

// openDreamingStore opens the local SQLite store at the default path.
func openDreamingStore(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, defaultDBPath("dreaming-pp-cli"))
}

// resolveDBPath returns the override when set, else the canonical default DB
// path. Used by commands that accept a --db flag (concordance, transcript) so
// a shared corpus file can be searched/extended without clobbering the
// canonical store.
func resolveDBPath(override string) string {
	if strings.TrimSpace(override) != "" {
		return override
	}
	return defaultDBPath("dreaming-pp-cli")
}

// openDreamingStoreAt opens the store at the override path when given, else the
// default. The opened DB gets the full schema migration, so a brand-new file
// (e.g. a fresh shared-corpus DB) is initialized correctly.
func openDreamingStoreAt(ctx context.Context, override string) (*store.Store, error) {
	return store.OpenWithContext(ctx, resolveDBPath(override))
}

// --- VTT parsing -----------------------------------------------------------

// VTTCue is one caption cue with its timing and cleaned text.
type VTTCue struct {
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
	Text    string `json:"text"`
}

var vttTimestampRE = regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2})[.,](\d{3})\s*-->\s*(\d{2}):(\d{2}):(\d{2})[.,](\d{3})`)
var vttCueNumberRE = regexp.MustCompile(`^\d+$`)

// parseVTT splits a WEBVTT blob into timed cues with cleaned text.
func parseVTT(raw string) []VTTCue {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	blocks := strings.Split(raw, "\n\n")
	var cues []VTTCue
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		var start, end int64 = -1, -1
		var textLines []string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || line == "WEBVTT" || strings.HasPrefix(line, "WEBVTT") {
				continue
			}
			if vttCueNumberRE.MatchString(line) {
				continue
			}
			if m := vttTimestampRE.FindStringSubmatch(line); m != nil {
				start = hmsToMS(m[1], m[2], m[3], m[4])
				end = hmsToMS(m[5], m[6], m[7], m[8])
				continue
			}
			if strings.HasPrefix(line, "NOTE") || strings.HasPrefix(line, "STYLE") {
				continue
			}
			textLines = append(textLines, stripVTTTags(line))
		}
		text := strings.TrimSpace(strings.Join(textLines, " "))
		if text == "" {
			continue
		}
		if start < 0 {
			start = 0
		}
		cues = append(cues, VTTCue{StartMS: start, EndMS: end, Text: text})
	}
	return cues
}

var vttTagRE = regexp.MustCompile(`<[^>]+>`)

func stripVTTTags(s string) string {
	return strings.TrimSpace(vttTagRE.ReplaceAllString(s, ""))
}

func hmsToMS(h, m, s, ms string) int64 {
	var hh, mm, ss, mmm int64
	fmt.Sscanf(h, "%d", &hh)
	fmt.Sscanf(m, "%d", &mm)
	fmt.Sscanf(s, "%d", &ss)
	fmt.Sscanf(ms, "%d", &mmm)
	return ((hh*60+mm)*60+ss)*1000 + mmm
}

// formatTimestamp renders milliseconds as HH:MM:SS for human display.
func formatTimestamp(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	total := ms / 1000
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, (total%3600)/60, total%60)
}

// vttToPlainText collapses a WEBVTT blob to one flat, timestamp-free paragraph.
func vttToPlainText(raw string) string {
	cues := parseVTT(raw)
	parts := make([]string, 0, len(cues))
	for _, c := range cues {
		parts = append(parts, c.Text)
	}
	return strings.Join(parts, " ")
}

// --- Spanish verb-tense presets (heuristic regex over conjugation endings) --

// tensePresets maps a preset name to a heuristic, case-insensitive regex that
// matches Spanish words with the conjugation endings of that tense. These are
// first-order filters, not a morphological parser — they over-match nouns that
// happen to share endings. Documented as heuristic in the command help.
var tensePresets = map[string]string{
	"gerund":                `(?i)[\p{L}]{1,}(ando|iendo|yendo)(?:[^\p{L}]|$)`,
	"imperfect":             `(?i)[\p{L}]{1,}(aba|abas|ábamos|abais|aban|ía|ías|íamos|íais|ían)(?:[^\p{L}]|$)`,
	"preterite":             `(?i)[\p{L}]{2,}(aste|ó|amos|asteis|aron|iste|ió|isteis|ieron)(?:[^\p{L}]|$)`,
	"future":                `(?i)[\p{L}]{1,}(ré|rás|rá|remos|réis|rán)(?:[^\p{L}]|$)`,
	"conditional":           `(?i)[\p{L}]{1,}(ría|rías|ríamos|ríais|rían)(?:[^\p{L}]|$)`,
	"subjunctive-imperfect": `(?i)[\p{L}]{1,}(ara|aras|áramos|arais|aran|ase|ases|ásemos|aseis|asen|iera|ieras|iéramos|ierais|ieran|iese|ieses|iésemos|ieseis|iesen)(?:[^\p{L}]|$)`,
	"present-subjunctive":   `(?i)\b(sea|seas|seamos|sean|esté|estés|estemos|estén|haya|hayas|hayamos|hayan|vaya|vayas|vayamos|vayan|tenga|tengas|tengan|haga|hagas|hagan|pueda|puedas|puedan|quiera|quieras|quieran)\b`,
	"compound":              `(?i)\b(he|has|ha|hemos|habéis|han|había|habías|habíamos|habían|habré|habrás|habrá|habremos|habrán|habría|haya|hayas|hayamos|hayan|hubiera|hubieras|hubiéramos|hubieran|hubiese)\s+[\p{L}]{2,}(ado|ido)(?:[^\p{L}]|$)`,
}

func tensePresetNames() string {
	names := make([]string, 0, len(tensePresets))
	for k := range tensePresets {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// hoursFromSeconds is a tiny readability helper for converting stored seconds.
func hoursFromSeconds(sec int64) float64 { return float64(sec) / 3600.0 }

// daysToDate returns whole calendar days from today until t (negative if in
// the past). It compares local-timezone midnights: t arrives as UTC midnight
// from a date-only time.Parse, so measuring from the local wall clock would
// skew the count by the timezone offset. Rounding absorbs DST-transition
// days (23/25 hours).
func daysToDate(t time.Time) int {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	target := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
	return int(math.Round(target.Sub(today).Hours() / 24))
}
