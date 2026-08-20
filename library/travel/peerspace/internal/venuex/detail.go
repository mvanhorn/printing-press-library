// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.
// Detail-page helpers: format-fit classification for full listing payloads.

package venuex

import "strings"

// InferFormatFit classifies a listing for meetup/event planning.
// Returns one of: talk, wellness, fb, production, mixed (or empty when unknown).
// Uses space_type + amenity tokens + description/rules keywords (FR + EN).
func InferFormatFit(l Listing) string {
	text := strings.ToLower(strings.Join([]string{
		l.SpaceType, l.Title, l.Description, l.About, l.Rules, l.Included,
	}, " "))
	am := strings.ToLower(strings.Join(l.Amenities, " "))
	blob := text + " " + am

	scores := map[string]int{
		"talk":       0,
		"wellness":   0,
		"fb":         0,
		"production": 0,
	}

	// Space type strong signals
	st := strings.ToLower(l.SpaceType)
	switch {
	case strings.Contains(st, "yoga") || strings.Contains(st, "dance") || strings.Contains(st, "gym") || strings.Contains(st, "fitness"):
		scores["wellness"] += 4
	case strings.Contains(st, "photo") || strings.Contains(st, "video") || strings.Contains(st, "studio") || strings.Contains(st, "film"):
		scores["production"] += 4
	case strings.Contains(st, "restaurant") || strings.Contains(st, "bar") || strings.Contains(st, "cafe") || strings.Contains(st, "coffee"):
		scores["fb"] += 4
	case strings.Contains(st, "meeting") || strings.Contains(st, "conference") || strings.Contains(st, "classroom") || strings.Contains(st, "office") || strings.Contains(st, "training"):
		scores["talk"] += 4
	case strings.Contains(st, "event") || strings.Contains(st, "multipurpose") || strings.Contains(st, "flex") || strings.Contains(st, "loft"):
		scores["talk"] += 1
	}

	// Keyword signals (EN + FR)
	add := func(cat string, n int, words ...string) {
		for _, w := range words {
			if strings.Contains(blob, w) {
				scores[cat] += n
			}
		}
	}
	add("talk", 2, "projector", "projecteur", "écran", "screen", "hdmi", "microphone", "mic", "présentation", "presentation", "conférence", "conference", "meetup", "workshop", "réunion", "meeting", "wifi")
	add("wellness", 2, "yoga", "méditation", "meditation", "bien-être", "wellness", "pilates", "tapis", "mat", "stretch", "zen", "massage")
	add("fb", 2, "restaurant", "cocktail", "cuisine", "kitchen", "bar", "dîner", "dinner", "tapas", "vin", "wine", "catering", "buffet")
	add("production", 2, "tournage", "shooting", "photo", "vidéo", "video", "fond noir", "studio", "éclairage", "lighting", "caméra", "camera")

	// Amenities tokens
	for _, a := range l.Amenities {
		al := strings.ToLower(a)
		switch {
		case strings.Contains(al, "projector") || strings.Contains(al, "mic") || strings.Contains(al, "screen") || strings.Contains(al, "hdmi") || strings.Contains(al, "whiteboard"):
			scores["talk"] += 1
		case strings.Contains(al, "yoga"):
			scores["wellness"] += 2
		case strings.Contains(al, "kitchen") || strings.Contains(al, "restaurant") || strings.Contains(al, "bar"):
			scores["fb"] += 1
		}
	}

	bestCat := ""
	best := 0
	second := 0
	for cat, sc := range scores {
		if sc > best {
			second = best
			best = sc
			bestCat = cat
		} else if sc > second {
			second = sc
		}
	}
	if best == 0 {
		return ""
	}
	// Close race → mixed
	if second > 0 && best-second <= 1 && second >= 2 {
		return "mixed"
	}
	return bestCat
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" || max <= 0 {
		return s
	}
	// collapse whitespace for export
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max < 4 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}
